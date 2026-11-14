package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"sobi/internal/holiday"
)

// ---- 회비 ----
//
// 친목회·동호회처럼 매달 정해진 돈을 내는 회비는 정기결제(Subscription) 중 Kind == KindDues 인 것이다.
//
// 정기결제는 달마다 "이번 달 결제가 나갔나"만 대조하지만, 회비는 밀리기도 하고 몇 달치를 한 번에
// 내기도 해서 장부로 본다: 사용자가 밝힌 "이 달분까지 냈음"(PaidThrough)에서 출발해, 그 뒤에 낸 돈으로
// 다음 달부터 차례로 채운다. 납부일이 지났는데 못 채운 달이 미납이다.
//
// 어느 거래가 회비 납부인지는 앱 계층이 가맹점 이름으로 고른다(대조 로직이 classifier 에 있다).
// 여기서는 그 결과로 장부를 계산하고, 사용자가 직접 정한 연결(dues_links)을 저장한다.

// KindDues 는 회비 정기결제의 Kind 값.
const KindDues = "dues"

// 사용자가 직접 정한 거래 연결 상태.
const (
	DuesLinkInclude = "include" // 회비 납부로 센다(납부 기록으로 만든 거래, 금액이 안 맞아도 넣은 거래)
	DuesLinkExclude = "exclude" // 회비 아님(같은 사람에게 보낸 회식비 등) — 자동으로 세지 않는다
)

// 장부에 보이는 거래의 상태.
const (
	DuesCounted  = "counted"  // 장부에 더함
	DuesReview   = "review"   // 받는 사람은 맞는데 금액이 회비와 안 맞아 확인이 필요함(장부에는 아직 안 더함)
	DuesExcluded = "excluded" // 회비 아님으로 뺌
)

// maxDuesMonths 는 장부를 훑는 최대 개월 수(잘못된 값으로 끝없이 돌지 않게).
const maxDuesMonths = 240

// duesSchema 는 회비 컬럼·연결 테이블(migrate 에서 본 스키마 뒤에 이어 실행).
//   - paid_through: 사용자가 밝힌 "이 달분까지 냈음"("YYYY-MM").
//   - paid_through_at / paid_through_tx: 그렇게 밝힌 날짜와 그때의 마지막 거래 id. 그 뒤에 생긴
//     납부 거래만 장부에 더한다 — 밝힐 때 이미 있던 거래는 그 "어디까지"에 들어 있으니 두 번 세지 않는다.
//   - dues_links: 사용자가 직접 정한 연결. 거래를 지우면 같이 지워진다.
const duesSchema = `
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT '';
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS paid_through TEXT NOT NULL DEFAULT '';
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS paid_through_at TEXT NOT NULL DEFAULT '';
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS paid_through_tx BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS dues_links (
	transaction_id  BIGINT PRIMARY KEY REFERENCES transactions(id) ON DELETE CASCADE,
	subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
	state           TEXT NOT NULL CHECK (state IN ('include','exclude')),
	created_at      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dues_links_sub ON dues_links(subscription_id);
`

// DuesLink 는 사용자가 거래 하나에 정한 회비 연결.
type DuesLink struct {
	SubscriptionID int64  `json:"subscriptionId"`
	TransactionID  int64  `json:"transactionId"`
	State          string `json:"state"` // DuesLinkInclude | DuesLinkExclude
}

// DuesPayment 는 회비 장부에 보이는 거래 한 건.
type DuesPayment struct {
	TransactionID     int64  `json:"transactionId"`
	Date              string `json:"date"`
	Amount            int64  `json:"amount"`
	Merchant          string `json:"merchant"`
	Memo              string `json:"memo"`
	PaymentMethodName string `json:"paymentMethodName"`
	State             string `json:"state"`  // DuesCounted | DuesReview | DuesExcluded
	Manual            bool   `json:"manual"` // 사용자가 직접 넣거나 뺀 거래(자동 판정이 아님)
}

// DuesStatus 는 회비 한 건의 장부.
type DuesStatus struct {
	Sub Subscription `json:"sub"`
	// Fee 는 이번 달 회비(요금 변경 예약 반영).
	Fee int64 `json:"fee"`
	// PaidTotal 은 기준(Sub.PaidThroughAt) 이후 낸 돈의 합.
	PaidTotal int64 `json:"paidTotal"`
	// PaidThrough 는 낸 돈으로 채워진 마지막 달("YYYY-MM").
	PaidThrough string `json:"paidThrough"`
	// Credit 은 그다음 달 몫 중 미리 낸 부분(한 달치에 못 미치는 돈).
	Credit int64 `json:"credit"`
	// Overdue* 는 납부일이 지났는데 못 채운 달들.
	OverdueMonths int    `json:"overdueMonths"`
	Overdue       int64  `json:"overdue"`
	OverdueFrom   string `json:"overdueFrom"`
	OverdueTo     string `json:"overdueTo"`
	// Next* 는 납부일이 아직 오지 않은 첫 미납 달과 그 납부일·낼 돈. 더 낼 달이 없으면 빈 값.
	NextYM   string `json:"nextYm"`
	NextDate string `json:"nextDate"`
	NextDue  int64  `json:"nextDue"`
	// AheadMonths 는 이번 달 뒤로 미리 낸 달 수.
	AheadMonths int `json:"aheadMonths"`
	// Ended 는 종료 월(Sub.EndYM)까지 다 냈다는 뜻.
	Ended bool `json:"ended"`
	// Payments 는 기준 이후의 납부·확인 필요·회비 아님 거래(최근순).
	Payments []DuesPayment `json:"payments"`
}

// addYM 은 "YYYY-MM" 에 n 개월을 더한다.
func addYM(ym string, n int) string {
	t, err := time.Parse("2006-01", ym)
	if err != nil {
		return ""
	}
	return t.AddDate(0, n, 0).Format("2006-01")
}

func validYM(ym string) bool {
	if len(ym) != 7 {
		return false
	}
	_, err := time.Parse("2006-01", ym)
	return err == nil
}

// duesDueDate 는 ym 달 회비의 납부일(휴일 처리 반영). 계산할 수 없으면 그 달 1일.
func duesDueDate(sub Subscription, ym string, cal *holiday.Calendar) time.Time {
	if _, actual, err := sub.ChargeDate(ym, cal); err == nil {
		return actual
	}
	t, _ := time.Parse("2006-01", ym)
	return t
}

// BuildDuesStatus 는 회비 한 건의 장부를 계산한다. paid 는 기준 이후 낸 돈의 합.
//
//  1. Sub.PaidThrough 다음 달부터 낸 돈으로 한 달씩 채운다(달마다 그 달 회비 AmountAt).
//  2. 채우지 못한 달 중 납부일이 오늘보다 앞인 달이 미납이다(납부일 당일은 아직 미납이 아니다).
//  3. 그다음 달이 다음에 낼 달이다. 한 달치에 못 미쳐 남은 돈(Credit)은 첫 미납 달 몫에서 뺀다.
//
// 꺼 둔 회비는 미납·다음 납부를 계산하지 않는다(장부는 보여 준다).
func BuildDuesStatus(sub Subscription, paid int64, today time.Time, cal *holiday.Calendar) DuesStatus {
	today = dayOf(today)
	cur := today.Format("2006-01")
	st := DuesStatus{
		Sub:         sub,
		Fee:         sub.AmountAt(cur),
		PaidTotal:   paid,
		PaidThrough: sub.PaidThrough,
		Payments:    []DuesPayment{},
	}
	if !validYM(sub.PaidThrough) {
		return st
	}
	owes := func(ym string) bool { return sub.EndYM == "" || ym <= sub.EndYM }

	// 1. 낸 돈으로 달을 채운다
	pool := paid
	ym := addYM(sub.PaidThrough, 1)
	for i := 0; i < maxDuesMonths && owes(ym); i++ {
		fee := sub.AmountAt(ym)
		if fee <= 0 || pool < fee {
			break
		}
		pool -= fee
		st.PaidThrough = ym
		ym = addYM(ym, 1)
	}
	st.Credit = pool
	if st.PaidThrough > cur {
		st.AheadMonths = monthsBetween(cur, st.PaidThrough)
	}
	if !owes(ym) {
		st.Ended = true
		return st
	}
	if !sub.Active {
		return st
	}

	// 2. 납부일이 지난 미납 달
	credit := st.Credit
	for i := 0; i < maxDuesMonths && owes(ym); i++ {
		if !duesDueDate(sub, ym, cal).Before(today) {
			break
		}
		st.Overdue += sub.AmountAt(ym) - credit
		credit = 0
		st.OverdueMonths++
		if st.OverdueFrom == "" {
			st.OverdueFrom = ym
		}
		st.OverdueTo = ym
		ym = addYM(ym, 1)
	}

	// 3. 다음에 낼 달
	if owes(ym) {
		st.NextYM = ym
		st.NextDate = fmtDay(duesDueDate(sub, ym, cal))
		st.NextDue = sub.AmountAt(ym) - credit
	}
	return st
}

// duesEvents 는 자금 계획에 넣을 회비 출금: 다음 납부일부터 계획 기간 끝까지 달마다 한 건.
// 밀린 회비는 다음 납부일 출금에 얹는다(그날 같이 낸다고 본다). 카드로 내는 회비는 카드 청구서에
// 들어가므로 뺀다.
func duesEvents(dues []DuesStatus, pms []PaymentMethod, cal *holiday.Calendar, today, horizon time.Time) []CashEvent {
	pmType := map[int64]string{}
	for _, p := range pms {
		pmType[p.ID] = p.Type
	}
	todayStr, horizonStr := fmtDay(today), fmtDay(horizon)
	var out []CashEvent
	for _, d := range dues {
		s := d.Sub
		if !s.Active || d.Ended {
			continue
		}
		if s.PaymentMethodID != nil && pmType[*s.PaymentMethodID] == "card" {
			continue
		}
		carry := d.Overdue
		ym := d.NextYM
		for i := 0; ym != "" && i < 24; i++ {
			if s.EndYM != "" && ym > s.EndYM {
				break
			}
			date := fmtDay(duesDueDate(s, ym, cal))
			if date > horizonStr {
				break
			}
			amt := s.AmountAt(ym)
			if ym == d.NextYM {
				amt = d.NextDue
			}
			label := s.Name + " 회비"
			if carry > 0 {
				amt += carry
				label = fmt.Sprintf("%s 회비 (밀린 %d개월 포함)", s.Name, d.OverdueMonths)
				carry = 0
			}
			if date >= todayStr && amt > 0 {
				out = append(out, CashEvent{Date: date, Kind: "dues", Label: label, Amount: -amt, RefID: s.ID})
			}
			ym = addYM(ym, 1)
		}
		// 다음 납부일이 계획 기간 밖이거나 더 낼 달이 없으면, 밀린 돈은 오늘 낼 돈으로 본다
		if carry > 0 {
			out = append(out, CashEvent{
				Date: todayStr, Kind: "dues", Label: s.Name + " 밀린 회비", Amount: -carry, RefID: s.ID,
			})
		}
	}
	return out
}

// ---- 저장 ----

// normalizeDues 는 저장 전에 회비 전용 값을 정리한다. 회비가 아니면 회비 값을 비운다.
// 회비는 매달 내고, 거래를 자동으로 만들지 않으며, 시작 월 대신 "어디까지 냈나"로 출발한다.
func (s *Store) normalizeDues(sub *Subscription) error {
	if sub.Kind != KindDues {
		sub.PaidThrough, sub.PaidThroughAt, sub.PaidThroughTx = "", "", 0
		if sub.Kind == KindTransit {
			return normalizeTransit(sub)
		}
		sub.Kind = ""
		return nil
	}
	if sub.Amount <= 0 {
		return fmt.Errorf("월 회비를 입력하세요")
	}
	if !validYM(sub.PaidThrough) {
		return fmt.Errorf("어느 달분까지 냈는지 입력하세요")
	}
	sub.Cycle = "monthly"
	sub.AutoPost = false
	sub.StartYM = ""

	// 기준(paid_through_at/tx)은 서버가 정한다. 새로 만들거나 "어디까지 냈나"를 바꿀 때만 지금으로
	// 다시 잡고, 다른 칸만 고칠 때는 그대로 둔다(고칠 때마다 그 사이 낸 돈이 사라지면 안 된다).
	if sub.ID > 0 {
		var kind, through, at string
		var txID int64
		err := s.queryRow(`SELECT kind, paid_through, paid_through_at, paid_through_tx FROM subscriptions WHERE id=?`,
			sub.ID).Scan(&kind, &through, &at, &txID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && kind == KindDues && through == sub.PaidThrough {
			sub.PaidThroughAt, sub.PaidThroughTx = at, txID
			return nil
		}
	}
	sub.PaidThroughAt = nowFunc().Format(dateLayout)
	return s.queryRow(`SELECT COALESCE(MAX(id), 0) FROM transactions`).Scan(&sub.PaidThroughTx)
}

// GetSubscription 은 정기결제 한 건.
func (s *Store) GetSubscription(id int64) (Subscription, error) {
	rows, err := s.query(subSelect+" WHERE s.id=?", id)
	if err != nil {
		return Subscription{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Subscription{}, err
		}
		return Subscription{}, ErrNotFound
	}
	return scanSub(rows)
}

func (s *Store) getDues(id int64) (Subscription, error) {
	sub, err := s.GetSubscription(id)
	if err == ErrNotFound || (err == nil && sub.Kind != KindDues) {
		return sub, fmt.Errorf("회비를 찾을 수 없습니다")
	}
	return sub, err
}

// DuesLinks 는 사용자가 정한 회비 연결 전체(거래 id → 연결).
func (s *Store) DuesLinks() (map[int64]DuesLink, error) {
	rows, err := s.query(`SELECT transaction_id, subscription_id, state FROM dues_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]DuesLink{}
	for rows.Next() {
		var l DuesLink
		if err := rows.Scan(&l.TransactionID, &l.SubscriptionID, &l.State); err != nil {
			return nil, err
		}
		out[l.TransactionID] = l
	}
	return out, rows.Err()
}

// SetDuesLink 는 거래를 회비 subID 의 납부로 넣거나(include) 회비 아님으로 뺀다(exclude).
// 이미 다른 회비에 걸려 있으면 이 회비로 옮긴다.
func (s *Store) SetDuesLink(subID, txID int64, state string) error {
	if state != DuesLinkInclude && state != DuesLinkExclude {
		return fmt.Errorf("알 수 없는 연결 상태: %s", state)
	}
	if _, err := s.getDues(subID); err != nil {
		return err
	}
	var direction string
	if err := s.queryRow(`SELECT direction FROM transactions WHERE id=?`, txID).Scan(&direction); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("거래를 찾을 수 없습니다")
		}
		return err
	}
	if direction != "expense" {
		return fmt.Errorf("지출 거래만 회비 납부로 볼 수 있습니다")
	}
	_, err := s.exec(`
INSERT INTO dues_links(transaction_id, subscription_id, state, created_at) VALUES(?,?,?,?)
ON CONFLICT (transaction_id) DO UPDATE SET subscription_id = excluded.subscription_id,
	state = excluded.state, created_at = excluded.created_at`,
		txID, subID, state, nowFunc().Format("2006-01-02 15:04:05"))
	return err
}

// ClearDuesLink 는 사용자가 정한 연결을 지워 자동 판정으로 되돌린다.
func (s *Store) ClearDuesLink(txID int64) error {
	_, err := s.exec(`DELETE FROM dues_links WHERE transaction_id=?`, txID)
	return err
}

// AddDuesPayment 는 회비 납부를 기록한다: 한 DB 트랜잭션 안에서 지출 거래를 만들고 그 회비의 납부로 넣는다.
// 받는 사람·카테고리·귀속자·결제수단은 회비 설정을 따른다.
func (s *Store) AddDuesPayment(subID int64, date string, amount int64, memo string) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("낸 금액은 0보다 커야 합니다")
	}
	if _, err := time.Parse(dateLayout, date); err != nil {
		return 0, fmt.Errorf("날짜는 YYYY-MM-DD 형식이어야 합니다")
	}
	sub, err := s.getDues(subID)
	if err != nil {
		return 0, err
	}
	t := Transaction{
		Date: date, Amount: amount, Direction: "expense", Merchant: strings.TrimSpace(sub.MatchTarget()),
		Memo: memo, CategoryID: sub.CategoryID, MemberID: sub.MemberID, PaymentMethodID: sub.PaymentMethodID,
		Source: "manual",
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var txID int64
	if err := tx.QueryRow(pq(insertTxSQL), insertTxArgs(t)...).Scan(&txID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(pq(`INSERT INTO dues_links(transaction_id, subscription_id, state, created_at) VALUES(?,?,?,?)`),
		txID, subID, DuesLinkInclude, nowFunc().Format("2006-01-02 15:04:05")); err != nil {
		return 0, err
	}
	return txID, tx.Commit()
}

// ---- 삭제 취소용: 회비 연결 백업/복원 ----

// DuesLinksFor 는 거래 ID 들에 걸린 회비 연결을 거래 ID 별로 돌려준다.
func (s *Store) DuesLinksFor(ids []int64) (map[int64]DuesLink, error) {
	out := map[int64]DuesLink{}
	for start := 0; start < len(ids); start += idChunk {
		end := start + idChunk
		if end > len(ids) {
			end = len(ids)
		}
		ph, args := idPlaceholders(ids[start:end])
		rows, err := s.query(`SELECT transaction_id, subscription_id, state FROM dues_links WHERE transaction_id IN (`+ph+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var l DuesLink
			if err := rows.Scan(&l.TransactionID, &l.SubscriptionID, &l.State); err != nil {
				rows.Close()
				return nil, err
			}
			out[l.TransactionID] = l
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// RestoreDuesLink 는 새로 다시 만든 거래(txID)에 회비 연결을 되살린다. 그 사이 회비가 지워졌다면
// 아무 일도 하지 않는다.
func (s *Store) RestoreDuesLink(txID int64, l DuesLink) error {
	_, err := s.exec(`
INSERT INTO dues_links(transaction_id, subscription_id, state, created_at)
SELECT ?, id, ?, ? FROM subscriptions WHERE id = ? AND kind = ?
ON CONFLICT (transaction_id) DO NOTHING`,
		txID, l.State, nowFunc().Format("2006-01-02 15:04:05"), l.SubscriptionID, KindDues)
	return err
}
