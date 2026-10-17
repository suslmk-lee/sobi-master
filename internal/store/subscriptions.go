package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Subscription 은 직접 등록해 두는 정기결제(구독·고정비) 한 건이다.
//
// 통계 탭의 "고정비·정기결제"는 규칙에서 자동으로 추론한 목록이고, 이건 사용자가
// 명시적으로 등록해 관리하는 쪽이다. 언제까지 쓸지(EndYM), 언제부터 요금이 얼마로
// 바뀌는지(NextAmount/NextAmountYM)를 미리 적어 두고 추적한다.
//
// 기본은 추적만 한다 — 등록해 둔 정기결제와 실제 거래를 대조해 "빠짐/예정"을 보여 준다.
// AutoPost 를 켜면 결제일에 거래를 자동으로 만든다(같은 달 거래가 이미 있으면 연결만 한다).
type Subscription struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Merchant 는 실제 거래와 대조할 가맹점명. 비어 있으면 Name 으로 대조한다.
	Merchant string `json:"merchant"`
	// Amount 는 현재 요금. NextAmountYM 이 지나면 NextAmount 가 현재 요금이 된다.
	Amount int64 `json:"amount"`
	// Cycle 은 monthly | yearly.
	Cycle string `json:"cycle"`
	// BillingDay 는 결제일(1~31). BillingMonth 는 연 결제일 때의 결제 월(1~12).
	BillingDay   int `json:"billingDay"`
	BillingMonth int `json:"billingMonth"`
	// StartYM/EndYM 은 "YYYY-MM". 비어 있으면 제한 없음.
	// EndYM 은 마지막으로 결제되는 달이다(그 달까지는 빠진다).
	StartYM string `json:"startYm"`
	EndYM   string `json:"endYm"`
	// NextAmount 는 NextAmountYM 부터 적용될 요금. NextAmountYM 이 비면 예약 없음.
	NextAmount   int64  `json:"nextAmount"`
	NextAmountYM string `json:"nextAmountYm"`

	CategoryID      *int64 `json:"categoryId"`
	MemberID        *int64 `json:"memberId"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	Memo            string `json:"memo"`
	// Active 가 false 면 잠시 꺼 둔 것(삭제하지 않고 추적만 중단).
	Active bool `json:"active"`
	// AutoPost 면 결제일(주말·공휴일이면 다음 업무일)에 거래를 자동으로 만든다.
	// AutoSince 는 자동 등록을 시작하는 달("YYYY-MM"). 저장 시 서버가 정한다 —
	// 켠 시점보다 이전 달로 소급해 거래를 만들지 않기 위해서다.
	AutoPost  bool   `json:"autoPost"`
	AutoSince string `json:"autoSince"`

	// 조회 편의용 조인 결과
	CategoryName      string `json:"categoryName"`
	MemberName        string `json:"memberName"`
	PaymentMethodName string `json:"paymentMethodName"`
}

// AmountAt 은 ym("YYYY-MM") 시점에 적용되는 요금.
// 예약된 변경(NextAmountYM)이 그 달 이후면 바뀐 요금을 쓴다.
func (s Subscription) AmountAt(ym string) int64 {
	if s.NextAmountYM != "" && ym >= s.NextAmountYM {
		return s.NextAmount
	}
	return s.Amount
}

// DueIn 은 ym 에 이 정기결제가 빠질 예정인지 본다.
// 꺼져 있거나, 시작 전이거나, 종료 월을 지났으면 아니다.
// 연 결제는 결제 월에만 빠진다.
func (s Subscription) DueIn(ym string) bool {
	if !s.Active {
		return false
	}
	if len(ym) < 7 {
		return false
	}
	if s.StartYM != "" && ym < s.StartYM {
		return false
	}
	if s.EndYM != "" && ym > s.EndYM {
		return false
	}
	if s.Cycle == "yearly" {
		return s.BillingMonth >= 1 && s.BillingMonth <= 12 &&
			ym[5:7] == fmt.Sprintf("%02d", s.BillingMonth)
	}
	return true
}

// MonthlyEquivalent 는 월 환산 금액(연 결제는 12로 나눠 비교 가능하게 한다).
func (s Subscription) MonthlyEquivalent(ym string) int64 {
	amt := s.AmountAt(ym)
	if s.Cycle == "yearly" {
		return amt / 12
	}
	return amt
}

// EndsIn 은 ym 이 마지막 결제 달인지(이번 달로 끝나는지).
func (s Subscription) EndsIn(ym string) bool { return s.EndYM != "" && s.EndYM == ym }

// MatchTarget 은 실제 거래와 대조할 때 쓸 이름(가맹점이 있으면 그것, 없으면 이름).
func (s Subscription) MatchTarget() string {
	if strings.TrimSpace(s.Merchant) != "" {
		return s.Merchant
	}
	return s.Name
}

const subSelect = `
SELECT s.id, s.name, s.merchant, s.amount, s.cycle, s.billing_day, s.billing_month,
       s.start_ym, s.end_ym, s.next_amount, s.next_amount_ym,
       s.category_id, s.member_id, s.payment_method_id, s.memo, s.active, s.auto_post, s.auto_since,
       CASE WHEN c.id IS NULL THEN ''
            WHEN cp.name IS NULL THEN c.name
            ELSE cp.name || ' > ' || c.name END,
       COALESCE(m.name, ''), COALESCE(p.name, '')
FROM subscriptions s
LEFT JOIN categories c ON c.id = s.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
LEFT JOIN members m ON m.id = s.member_id
LEFT JOIN payment_methods p ON p.id = s.payment_method_id
`

func scanSub(rows *sql.Rows) (Subscription, error) {
	var s Subscription
	var cid, mid, pid sql.NullInt64
	var active, autoPost int
	err := rows.Scan(&s.ID, &s.Name, &s.Merchant, &s.Amount, &s.Cycle, &s.BillingDay, &s.BillingMonth,
		&s.StartYM, &s.EndYM, &s.NextAmount, &s.NextAmountYM,
		&cid, &mid, &pid, &s.Memo, &active, &autoPost, &s.AutoSince,
		&s.CategoryName, &s.MemberName, &s.PaymentMethodName)
	if err != nil {
		return s, err
	}
	s.CategoryID = scanNullableID(cid)
	s.MemberID = scanNullableID(mid)
	s.PaymentMethodID = scanNullableID(pid)
	s.Active = active == 1
	s.AutoPost = autoPost == 1
	return s, nil
}

// ListSubscriptions 는 등록된 정기결제를 이름순으로 돌려준다(꺼진 것도 포함).
func (s *Store) ListSubscriptions() ([]Subscription, error) {
	rows, err := s.query(subSelect + " ORDER BY s.active DESC, s.name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Subscription{}
	for rows.Next() {
		sub, err := scanSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// SaveSubscription 은 정기결제를 추가하거나(ID=0) 수정한다.
func (s *Store) SaveSubscription(sub Subscription) (Subscription, error) {
	sub.Name = strings.TrimSpace(sub.Name)
	sub.Merchant = strings.TrimSpace(sub.Merchant)
	if sub.Name == "" {
		return sub, fmt.Errorf("이름은 필수입니다")
	}
	if sub.Amount < 0 || sub.NextAmount < 0 {
		return sub, fmt.Errorf("금액은 0 이상이어야 합니다")
	}
	if sub.Cycle != "yearly" {
		sub.Cycle = "monthly"
	}
	if sub.BillingDay < 1 || sub.BillingDay > 31 {
		sub.BillingDay = 1
	}
	if sub.Cycle == "yearly" && (sub.BillingMonth < 1 || sub.BillingMonth > 12) {
		sub.BillingMonth = 1
	}
	if sub.StartYM != "" && sub.EndYM != "" && sub.EndYM < sub.StartYM {
		return sub, fmt.Errorf("종료 월이 시작 월보다 빠릅니다")
	}
	// 변경 요금만 적고 시점을 안 적었거나 그 반대면 예약을 지운다(반쪽 상태 방지).
	if sub.NextAmountYM == "" || sub.NextAmount <= 0 {
		sub.NextAmountYM, sub.NextAmount = "", 0
	}

	active := 0
	if sub.Active {
		active = 1
	}
	if err := s.resolveAutoSince(&sub); err != nil {
		return sub, err
	}
	args := []interface{}{
		sub.Name, sub.Merchant, sub.Amount, sub.Cycle, sub.BillingDay, sub.BillingMonth,
		sub.StartYM, sub.EndYM, sub.NextAmount, sub.NextAmountYM,
		nullableID(sub.CategoryID), nullableID(sub.MemberID), nullableID(sub.PaymentMethodID),
		sub.Memo, active, boolToInt(sub.AutoPost), sub.AutoSince,
	}
	if sub.ID == 0 {
		var id int64
		err := s.queryRow(`
INSERT INTO subscriptions(name, merchant, amount, cycle, billing_day, billing_month,
	start_ym, end_ym, next_amount, next_amount_ym,
	category_id, member_id, payment_method_id, memo, active, auto_post, auto_since)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`, args...).Scan(&id)
		if err != nil {
			return sub, err
		}
		sub.ID = id
		return sub, nil
	}
	_, err := s.exec(`
UPDATE subscriptions SET name=?, merchant=?, amount=?, cycle=?, billing_day=?, billing_month=?,
	start_ym=?, end_ym=?, next_amount=?, next_amount_ym=?,
	category_id=?, member_id=?, payment_method_id=?, memo=?, active=?, auto_post=?, auto_since=?
WHERE id=?`, append(args, sub.ID)...)
	return sub, err
}

// nowFunc 는 테스트에서 "오늘"을 바꿔 끼우기 위한 시계.
var nowFunc = time.Now

// resolveAutoSince 는 자동 등록 시작 달을 정한다. 클라이언트가 보낸 값은 믿지 않는다.
//   - 자동 등록을 끄면 비운다(나중에 다시 켜면 그때부터 새로 센다 — 꺼 둔 기간은 소급하지 않는다).
//   - 이미 켜져 있던 것을 수정하면 원래 시작 달을 지킨다(수정할 때마다 밀리면 그 사이 달이 빠진다).
//   - 새로 켜면 max(시작 월, 이번 달). 과거 달을 한꺼번에 만들어 내지 않기 위해서다.
func (s *Store) resolveAutoSince(sub *Subscription) error {
	if !sub.AutoPost {
		sub.AutoSince = ""
		return nil
	}
	if sub.ID > 0 {
		var was int
		var since string
		err := s.queryRow(`SELECT auto_post, auto_since FROM subscriptions WHERE id=?`, sub.ID).Scan(&was, &since)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if was == 1 && since != "" {
			sub.AutoSince = since
			return nil
		}
	}
	cur := nowFunc().Format("2006-01")
	sub.AutoSince = cur
	if sub.StartYM > cur {
		sub.AutoSince = sub.StartYM
	}
	return nil
}

func (s *Store) DeleteSubscription(id int64) error {
	_, err := s.exec(`DELETE FROM subscriptions WHERE id=?`, id)
	return err
}
