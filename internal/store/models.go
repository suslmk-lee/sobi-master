package store

// Member 는 거래 귀속자(아빠, 엄마, 아이, 공동 등)를 나타낸다.
type Member struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Category 의 Kind 는 income(수입) | expense(지출) | transfer(이체) 중 하나.
type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// PaymentMethod 의 Type 은 card | cash | bank 중 하나.
// 카드(card)인 경우 카드사/결제일/실적기간/실적한도 정보를 함께 가진다.
type PaymentMethod struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Issuer string `json:"issuer"` // 카드사 (예: 신한, 삼성)
	// BillingDay: 매월 결제일(1~31, 0이면 미지정)
	BillingDay int `json:"billingDay"`
	// CycleStartDay: 실적 산정 시작일. 1이면 매월 1일~말일이 실적기간.
	CycleStartDay int `json:"cycleStartDay"`
	// PerfTarget: 실적한도(전월/당월 실적 충족 기준 금액). 0이면 미지정.
	PerfTarget int64 `json:"perfTarget"`
	// Color: 목록 칩 색상(hex, 예: "#3b5fd9"). 비어 있으면 이름 기반 자동 색.
	Color string `json:"color"`
}

// CardStatus 는 카드 한 장의 현재 실적기간 현황.
type CardStatus struct {
	Card        PaymentMethod `json:"card"`
	PeriodStart string        `json:"periodStart"`
	PeriodEnd   string        `json:"periodEnd"`
	Spent       int64         `json:"spent"`     // 실적기간 내 지출 합계
	Remaining   int64         `json:"remaining"` // 한도까지 남은 금액 (달성 시 0)
	Achieved    bool          `json:"achieved"`  // 실적한도 충족 여부
}

// CardBreakdown 은 특정 카드의 기간 내 지출 분석.
type CardBreakdown struct {
	ByMerchant []NamedAmount `json:"byMerchant"`
	ByCategory []NamedAmount `json:"byCategory"`
}

// Transaction 은 단일 거래. Amount 는 원 단위 양수, Direction 이 수입/지출/이체를 구분한다.
type Transaction struct {
	ID              int64  `json:"id"`
	Date            string `json:"date"` // YYYY-MM-DD
	Amount          int64  `json:"amount"`
	Direction       string `json:"direction"` // income | expense | transfer
	Merchant        string `json:"merchant"`
	Memo            string `json:"memo"`
	MemberID        *int64 `json:"memberId"`
	CategoryID      *int64 `json:"categoryId"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	Source          string `json:"source"` // manual | import
	// 조회 편의용 조인 결과
	MemberName        string `json:"memberName"`
	CategoryName      string `json:"categoryName"`
	PaymentMethodName string `json:"paymentMethodName"`
	AutoClassified    bool   `json:"autoClassified"`
}

// Rule 은 반복 거래 핑거프린트: 가맹점명 + 금액 구간이 일치하면
// 귀속자/카테고리를 자동 부여한다. Label 은 사용자에게 보여줄 설명(예: "아빠 휴대폰").
type Rule struct {
	ID         int64  `json:"id"`
	Merchant   string `json:"merchant"`
	AmountMin  int64  `json:"amountMin"`
	AmountMax  int64  `json:"amountMax"`
	MemberID   *int64 `json:"memberId"`
	CategoryID *int64 `json:"categoryId"`
	Label      string `json:"label"`
}

// NamedAmount 는 집계 결과 한 줄.
type NamedAmount struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Amount int64  `json:"amount"`
}

// TxFilter 는 거래 조회 필터. 빈 값/0 은 해당 조건 미적용을 뜻한다.
type TxFilter struct {
	Month            string `json:"month"` // "YYYY-MM" (From/To 가 있으면 무시)
	From             string `json:"from"`  // "YYYY-MM-DD"
	To               string `json:"to"`
	UnclassifiedOnly bool   `json:"unclassifiedOnly"`
	Query            string `json:"query"` // 가맹점/메모 부분 일치
	AmountMin        int64  `json:"amountMin"`
	AmountMax        int64  `json:"amountMax"`
	Direction        string `json:"direction"` // income|expense|transfer
	MemberID         int64  `json:"memberId"`
	CategoryID       int64  `json:"categoryId"`
	PaymentMethodID  int64  `json:"paymentMethodId"`
	Sort             string `json:"sort"` // date_desc(기본)|date_asc|amount_desc|amount_asc
}

// MonthlySummary 는 대시보드 한 달치 집계.
type MonthlySummary struct {
	Year              int           `json:"year"`
	Month             int           `json:"month"`
	TotalIncome       int64         `json:"totalIncome"`
	TotalExpense      int64         `json:"totalExpense"`
	TotalTransfer     int64         `json:"totalTransfer"`
	ByMember          []NamedAmount `json:"byMember"`
	ByCategory        []NamedAmount `json:"byCategory"`
	ByPaymentMethod   []NamedAmount `json:"byPaymentMethod"`
	UnclassifiedCount int           `json:"unclassifiedCount"`
}
