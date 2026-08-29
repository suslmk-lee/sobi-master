package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// Subscription 은 직접 등록해 두는 정기결제(구독·고정비) 한 건이다.
//
// 통계 탭의 "고정비·정기결제"는 규칙에서 자동으로 추론한 목록이고, 이건 사용자가
// 명시적으로 등록해 관리하는 쪽이다. 언제까지 쓸지(EndYM), 언제부터 요금이 얼마로
// 바뀌는지(NextAmount/NextAmountYM)를 미리 적어 두고 추적한다.
//
// 거래를 자동으로 만들지는 않는다. 카드 내역을 가져오면 진짜 거래가 들어오므로,
// 등록해 둔 정기결제와 실제 거래를 대조해 "빠짐/예정"만 보여 준다.
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
       s.category_id, s.member_id, s.payment_method_id, s.memo, s.active,
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
	var active int
	err := rows.Scan(&s.ID, &s.Name, &s.Merchant, &s.Amount, &s.Cycle, &s.BillingDay, &s.BillingMonth,
		&s.StartYM, &s.EndYM, &s.NextAmount, &s.NextAmountYM,
		&cid, &mid, &pid, &s.Memo, &active,
		&s.CategoryName, &s.MemberName, &s.PaymentMethodName)
	if err != nil {
		return s, err
	}
	s.CategoryID = scanNullableID(cid)
	s.MemberID = scanNullableID(mid)
	s.PaymentMethodID = scanNullableID(pid)
	s.Active = active == 1
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
	args := []interface{}{
		sub.Name, sub.Merchant, sub.Amount, sub.Cycle, sub.BillingDay, sub.BillingMonth,
		sub.StartYM, sub.EndYM, sub.NextAmount, sub.NextAmountYM,
		nullableID(sub.CategoryID), nullableID(sub.MemberID), nullableID(sub.PaymentMethodID),
		sub.Memo, active,
	}
	if sub.ID == 0 {
		var id int64
		err := s.queryRow(`
INSERT INTO subscriptions(name, merchant, amount, cycle, billing_day, billing_month,
	start_ym, end_ym, next_amount, next_amount_ym,
	category_id, member_id, payment_method_id, memo, active)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`, args...).Scan(&id)
		if err != nil {
			return sub, err
		}
		sub.ID = id
		return sub, nil
	}
	_, err := s.exec(`
UPDATE subscriptions SET name=?, merchant=?, amount=?, cycle=?, billing_day=?, billing_month=?,
	start_ym=?, end_ym=?, next_amount=?, next_amount_ym=?,
	category_id=?, member_id=?, payment_method_id=?, memo=?, active=?
WHERE id=?`, append(args, sub.ID)...)
	return sub, err
}

func (s *Store) DeleteSubscription(id int64) error {
	_, err := s.exec(`DELETE FROM subscriptions WHERE id=?`, id)
	return err
}
