package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"time"

	"sobi/internal/holiday"
)

// ---- 교통카드(K-패스) ----
//
// 지하철·버스를 매일 타는 교통카드는 탈 때마다 기록하지 않고 한 달에 한 번 본다: 매달 정해진 날 한 달 요금이
// 빠지고, 같은 날 K-패스 환급이 들어온다. 정기결제(Subscription) 중 Kind == KindTransit 인 것이다.
//
// 달마다 transit_months 에 요금·환급을 한 줄 두고, 가계부에는 실부담(요금 − 환급) 지출 한 건만 남긴다 —
// 환급을 수입으로 따로 잡으면 통계에서 수입이 부풀기 때문이다. 출금일이 지나면 앱이 확정한 최근 달들로
// 어림한 '추정' 기록을 만들고, 사용자가 실제 금액으로 고치면 추정 표시가 사라진다.
// 요금이 빠지는 통장(K뱅크 등)이 카드값 통장과 달라, 이 거래는 자금 계획의 잔액·출금 일정·카드값 계산에 넣지 않는다.

// KindTransit 은 교통카드 정기결제의 Kind 값.
const KindTransit = "transit"

// SourceTransit 은 교통카드 월 기록으로 만들어진 거래의 source 값.
const SourceTransit = "transit"

// transitEstimateMonths 는 다음 달 요금을 어림할 때 평균 내는 확정 달 수.
const transitEstimateMonths = 3

// transitSchema 는 교통카드 월 기록 테이블(migrate 에서 본 스키마 뒤에 이어 실행).
//   - 한 (교통카드, 달)에 한 줄. 먼저 자리를 차지하고 거래를 만들어, 같은 달이 두 번 기록되지 않는다.
//   - transaction_id: 실부담 지출 거래. 거래를 지우면 비워지고 줄은 남는다(그 달을 다시 자동 기록하지 않게).
const transitSchema = `
CREATE TABLE IF NOT EXISTS transit_months (
	subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
	ym              TEXT NOT NULL,
	date            TEXT NOT NULL DEFAULT '',
	fare            BIGINT NOT NULL DEFAULT 0,
	refund          BIGINT NOT NULL DEFAULT 0,
	estimated       INTEGER NOT NULL DEFAULT 0,
	transaction_id  BIGINT REFERENCES transactions(id) ON DELETE SET NULL,
	updated_at      TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (subscription_id, ym)
);
CREATE INDEX IF NOT EXISTS idx_transit_months_tx ON transit_months(transaction_id);
`

// transitTxFilter 는 교통카드 월 기록 거래를 빼는 조건(자금 계획 계산용). t 는 transactions 별칭.
const transitTxFilter = `NOT EXISTS (SELECT 1 FROM transit_months tm WHERE tm.transaction_id = t.id)`

// TransitMonth 는 교통카드 한 달 기록.
type TransitMonth struct {
	SubscriptionID int64  `json:"subscriptionId"`
	YM             string `json:"ym"`
	Date           string `json:"date"` // 출금일(주말·공휴일 규칙 반영)
	Fare           int64  `json:"fare"`
	Refund         int64  `json:"refund"`
	Net            int64  `json:"net"`       // 실부담 = Fare − Refund
	Estimated      bool   `json:"estimated"` // 앱이 어림해 넣은 값(아직 실제 금액으로 안 고침)
	TransactionID  *int64 `json:"transactionId"`
}

// TransitStatus 는 교통카드 한 장의 화면 자료.
type TransitStatus struct {
	Sub    Subscription   `json:"sub"`
	Months []TransitMonth `json:"months"` // 최근 달 먼저
	// 다음 출금(아직 기록이 없는 첫 달)과 그 어림값. 꺼져 있으면 빈 값.
	NextYM    string `json:"nextYm"`
	NextDate  string `json:"nextDate"`
	EstFare   int64  `json:"estFare"`
	EstRefund int64  `json:"estRefund"`
	// Basis 는 어림에 쓴 확정 달 수(0이면 설정한 예상 요금을 썼고 환급은 모름).
	Basis int `json:"basis"`
	// Pending 은 아직 추정인 달 수(실제 금액을 넣어야 할 달).
	Pending int `json:"pending"`
}

func transitMemo(fare, refund int64, estimated bool) string {
	m := fmt.Sprintf("요금 %s원 · 환급 %s원", commaInt(fare), commaInt(refund))
	if estimated {
		m += " · 추정(실제 금액으로 고쳐 주세요)"
	}
	return m
}

// commaInt 는 1234567 → "1,234,567".
func commaInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := false
	if n < 0 {
		neg, s = true, s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + s
	}
	return s
}

// TransitEstimate 는 다음 달 요금·환급 어림값: 확정한(추정이 아닌) 최근 달 중 요금이 있는 달 몇 개의 요금 평균,
// 환급은 그 달들의 환급 비율(환급 합 ÷ 요금 합)로. 확정한 달이 없으면 설정한 예상 요금(sub.Amount)과 환급 0.
// 안 탄 달(요금 0)은 평균에서 뺀다 — 휴가로 한 달 쉬었다고 다음 달 예상이 반 토막 나지 않게.
func TransitEstimate(sub Subscription, months []TransitMonth) (fare, refund int64, basis int) {
	sorted := append([]TransitMonth(nil), months...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].YM > sorted[j].YM })
	var sumFare, sumRefund int64
	for _, m := range sorted {
		if m.Estimated || m.Fare <= 0 {
			continue
		}
		sumFare += m.Fare
		sumRefund += m.Refund
		basis++
		if basis == transitEstimateMonths {
			break
		}
	}
	if basis == 0 {
		return sub.Amount, 0, 0
	}
	fare = (sumFare + int64(basis)/2) / int64(basis)
	refund = (fare*sumRefund + sumFare/2) / sumFare
	return fare, refund, basis
}

// normalizeTransit 은 저장 전에 교통카드 전용 값을 정리한다.
// 교통카드는 매달 한 번 빠지고, 출금일이 지나면 앱이 그달 기록을 만든다(AutoPost 를 켜 두어 시작 월 AutoSince 를 서버가 정하게 한다).
// 일반 정기결제 자동 등록·대조는 DueIn 이 거르므로 돌지 않는다.
func normalizeTransit(sub *Subscription) error {
	if sub.Amount <= 0 {
		return fmt.Errorf("한 달 예상 요금을 입력하세요")
	}
	sub.Cycle = "monthly"
	sub.BillingMonth = 0
	sub.StartYM, sub.EndYM = "", ""
	sub.NextAmount, sub.NextAmountYM = 0, ""
	sub.AutoPost = true
	return nil
}

// resetTransitSinceOnResume 은 꺼 두었던 교통카드를 다시 켤 때 자동 기록 시작 월을 이번 달로 옮긴다.
// 쉬는 동안의 달을 켜는 순간 한꺼번에 '추정'으로 채우지 않게.
func (s *Store) resetTransitSinceOnResume(sub *Subscription) error {
	if sub.ID == 0 || !sub.Active {
		return nil
	}
	var wasActive int
	err := s.queryRow(`SELECT active FROM subscriptions WHERE id=?`, sub.ID).Scan(&wasActive)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if wasActive == 0 {
		if cur := nowFunc().Format("2006-01"); sub.AutoSince < cur {
			sub.AutoSince = cur
		}
	}
	return nil
}

func scanTransitMonths(rows *sql.Rows) ([]TransitMonth, error) {
	defer rows.Close()
	out := []TransitMonth{}
	for rows.Next() {
		var m TransitMonth
		var est int
		var tid sql.NullInt64
		if err := rows.Scan(&m.SubscriptionID, &m.YM, &m.Date, &m.Fare, &m.Refund, &est, &tid); err != nil {
			return nil, err
		}
		m.Estimated = est == 1
		m.TransactionID = scanNullableID(tid)
		m.Net = m.Fare - m.Refund
		out = append(out, m)
	}
	return out, rows.Err()
}

const transitMonthCols = `subscription_id, ym, date, fare, refund, estimated, transaction_id`

// TransitMonths 는 교통카드 한 장의 월 기록(최근 달 먼저).
func (s *Store) TransitMonths(subID int64) ([]TransitMonth, error) {
	rows, err := s.query(`SELECT `+transitMonthCols+` FROM transit_months WHERE subscription_id=? ORDER BY ym DESC`, subID)
	if err != nil {
		return nil, err
	}
	return scanTransitMonths(rows)
}

// TransitTxNames 는 교통카드 월 기록 거래 id → 교통카드 이름 (거래내역 표시용).
func (s *Store) TransitTxNames() (map[int64]string, error) {
	rows, err := s.query(`
SELECT tm.transaction_id, s.name FROM transit_months tm
JOIN subscriptions s ON s.id = tm.subscription_id
WHERE tm.transaction_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func transitTx(sub Subscription, date string, fare, refund int64, estimated bool) Transaction {
	merchant := sub.Merchant
	if merchant == "" {
		merchant = sub.Name
	}
	return Transaction{
		Date: date, Amount: fare - refund, Direction: "expense", Merchant: merchant,
		Memo:     transitMemo(fare, refund, estimated),
		MemberID: sub.MemberID, CategoryID: sub.CategoryID, PaymentMethodID: sub.PaymentMethodID,
		Source: SourceTransit,
	}
}

// postTransitEstimate 는 아직 기록이 없는 달에만 어림값 기록과 실부담 거래를 만든다.
// 자리를 먼저 차지하고 거래를 만드는 한 트랜잭션이라, 두 PC 가 동시에 돌아도 한쪽만 성공한다.
func (s *Store) postTransitEstimate(sub Subscription, ym, date string, fare, refund int64) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(pq(`
INSERT INTO transit_months(subscription_id, ym, date, fare, refund, estimated, updated_at) VALUES(?,?,?,?,?,1,?)
ON CONFLICT (subscription_id, ym) DO NOTHING`),
		sub.ID, ym, date, fare, refund, nowFunc().Format("2006-01-02 15:04:05"))
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return false, err
	} else if n == 0 {
		return false, nil
	}
	if fare-refund > 0 {
		var txID int64
		if err := tx.QueryRow(pq(insertTxSQL), insertTxArgs(transitTx(sub, date, fare, refund, true))...).Scan(&txID); err != nil {
			return false, err
		}
		if _, err := tx.Exec(pq(`UPDATE transit_months SET transaction_id=? WHERE subscription_id=? AND ym=?`), txID, sub.ID, ym); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// transitDate 는 ym 달 출금일(주말·공휴일 규칙 반영) "YYYY-MM-DD".
func transitDate(sub Subscription, ym string, cal *holiday.Calendar) (string, error) {
	_, actual, err := sub.ChargeDate(ym, cal)
	if err != nil {
		return "", err
	}
	return actual.Format(dateLayout), nil
}

// RunTransit 은 켜진 교통카드마다, 자동 기록 시작 월부터 이번 달까지 출금일이 지났는데 기록이 없는 달을
// 어림값으로 기록한다. 기록한 달 수를 돌려준다.
func (s *Store) RunTransit(today time.Time) (int, error) {
	subs, err := s.ListSubscriptions()
	if err != nil {
		return 0, err
	}
	cal, err := s.Calendar()
	if err != nil {
		return 0, err
	}
	todayStr, curYM := today.Format(dateLayout), today.Format("2006-01")
	posted := 0
	for _, sub := range subs {
		if sub.Kind != KindTransit || !sub.Active || sub.AutoSince == "" {
			continue
		}
		months, err := s.TransitMonths(sub.ID)
		if err != nil {
			return posted, err
		}
		have := map[string]bool{}
		for _, m := range months {
			have[m.YM] = true
		}
		fare, refund, _ := TransitEstimate(sub, months)
		for ym, n := sub.AutoSince, 0; ym <= curYM && n < maxDuesMonths; ym, n = addYM(ym, 1), n+1 {
			if have[ym] {
				continue
			}
			date, err := transitDate(sub, ym, cal)
			if err != nil || date > todayStr {
				continue
			}
			ok, err := s.postTransitEstimate(sub, ym, date, fare, refund)
			if err != nil {
				return posted, err
			}
			if ok {
				posted++
			}
		}
	}
	return posted, nil
}

// SaveTransitMonth 는 교통카드 subID 의 ym 달 실제 요금·환급을 기록하거나 고친다(추정 표시를 지운다).
// 실부담이 0 이하(안 탄 달, 환급이 요금과 같음)면 지출 거래를 지우고 기록만 남긴다.
// 거래가 지워져 있었으면 다시 만든다. 이번 달보다 뒤는 기록할 수 없다.
func (s *Store) SaveTransitMonth(subID int64, ym string, fare, refund int64) (TransitMonth, error) {
	if !validYM(ym) {
		return TransitMonth{}, fmt.Errorf("어느 달인지 고르세요")
	}
	if ym > nowFunc().Format("2006-01") {
		return TransitMonth{}, fmt.Errorf("아직 오지 않은 달은 기록할 수 없습니다")
	}
	if fare < 0 || refund < 0 {
		return TransitMonth{}, fmt.Errorf("금액은 0원 이상이어야 합니다")
	}
	if refund > fare {
		return TransitMonth{}, fmt.Errorf("환급액이 요금보다 클 수 없습니다")
	}
	sub, err := s.GetSubscription(subID)
	if err != nil {
		return TransitMonth{}, err
	}
	if sub.Kind != KindTransit {
		return TransitMonth{}, fmt.Errorf("교통카드가 아닙니다")
	}
	cal, err := s.Calendar()
	if err != nil {
		return TransitMonth{}, err
	}
	date, err := transitDate(sub, ym, cal)
	if err != nil {
		return TransitMonth{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return TransitMonth{}, err
	}
	defer tx.Rollback()
	now := nowFunc().Format("2006-01-02 15:04:05")
	if _, err := tx.Exec(pq(`
INSERT INTO transit_months(subscription_id, ym, date, fare, refund, estimated, updated_at) VALUES(?,?,?,?,?,0,?)
ON CONFLICT (subscription_id, ym) DO UPDATE SET fare=EXCLUDED.fare, refund=EXCLUDED.refund, estimated=0, updated_at=EXCLUDED.updated_at`),
		subID, ym, date, fare, refund, now); err != nil {
		return TransitMonth{}, err
	}
	var curDate string
	var tid sql.NullInt64
	if err := tx.QueryRow(pq(`SELECT date, transaction_id FROM transit_months WHERE subscription_id=? AND ym=? FOR UPDATE`),
		subID, ym).Scan(&curDate, &tid); err != nil {
		return TransitMonth{}, err
	}
	// 거래가 남아 있는지 (사용자가 거래내역에서 지웠으면 ON DELETE SET NULL 로 비어 있다)
	var txID int64
	if tid.Valid {
		txID = tid.Int64
	}
	t := transitTx(sub, curDate, fare, refund, false)
	switch {
	case t.Amount > 0 && txID > 0:
		if _, err := tx.Exec(pq(`UPDATE transactions SET amount=?, memo=? WHERE id=?`), t.Amount, t.Memo, txID); err != nil {
			return TransitMonth{}, err
		}
	case t.Amount > 0:
		if err := tx.QueryRow(pq(insertTxSQL), insertTxArgs(t)...).Scan(&txID); err != nil {
			return TransitMonth{}, err
		}
		if _, err := tx.Exec(pq(`UPDATE transit_months SET transaction_id=? WHERE subscription_id=? AND ym=?`), txID, subID, ym); err != nil {
			return TransitMonth{}, err
		}
	case txID > 0:
		if _, err := tx.Exec(pq(`DELETE FROM transactions WHERE id=?`), txID); err != nil {
			return TransitMonth{}, err
		}
		txID = 0
	}
	if err := tx.Commit(); err != nil {
		return TransitMonth{}, err
	}
	out := TransitMonth{SubscriptionID: subID, YM: ym, Date: curDate, Fare: fare, Refund: refund, Net: fare - refund}
	if txID > 0 {
		out.TransactionID = &txID
	}
	return out, nil
}

// BuildTransitStatus 는 교통카드 화면 자료를 계산한다(순수 함수 — 시험하기 쉽게).
// months 는 이 교통카드의 기록 전체. 다음 출금은 기록이 없는 달 중 출금일이 오늘 이후인 첫 달.
func BuildTransitStatus(sub Subscription, months []TransitMonth, today time.Time, cal *holiday.Calendar) TransitStatus {
	st := TransitStatus{Sub: sub, Months: []TransitMonth{}}
	sorted := append([]TransitMonth(nil), months...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].YM > sorted[j].YM })
	have := map[string]bool{}
	for _, m := range sorted {
		have[m.YM] = true
		if m.Estimated {
			st.Pending++
		}
	}
	if len(sorted) > 12 {
		sorted = sorted[:12]
	}
	st.Months = append(st.Months, sorted...)
	st.EstFare, st.EstRefund, st.Basis = TransitEstimate(sub, months)
	if !sub.Active {
		return st
	}
	todayStr := today.Format(dateLayout)
	for ym, n := today.Format("2006-01"), 0; n < 3; ym, n = addYM(ym, 1), n+1 {
		if have[ym] {
			continue
		}
		date, err := transitDate(sub, ym, cal)
		if err != nil || date < todayStr {
			continue
		}
		st.NextYM, st.NextDate = ym, date
		break
	}
	return st
}

// TransitBook 은 교통카드 전체의 화면 자료(ListSubscriptions 순서).
func (s *Store) TransitBook(today time.Time) ([]TransitStatus, error) {
	out := []TransitStatus{}
	subs, err := s.ListSubscriptions()
	if err != nil {
		return out, err
	}
	var cal *holiday.Calendar
	for _, sub := range subs {
		if sub.Kind != KindTransit {
			continue
		}
		if cal == nil {
			if cal, err = s.Calendar(); err != nil {
				return out, err
			}
		}
		months, err := s.TransitMonths(sub.ID)
		if err != nil {
			return out, err
		}
		out = append(out, BuildTransitStatus(sub, months, today, cal))
	}
	return out, nil
}
