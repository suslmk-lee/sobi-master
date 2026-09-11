package store

// Member 는 거래 귀속자(아빠, 엄마, 아이, 공동 등)를 나타낸다.
type Member struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Category 의 Kind 는 income(수입) | expense(지출) | transfer(이체) 중 하나.
// 주/부 2단 계층: ParentID 가 nil 이면 주(대분류), 값이 있으면 그 주에 속한 부(소분류).
// 부의 Kind 는 주를 따른다.
type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	ParentID *int64 `json:"parentId"`
	// SortOrder 는 같은 그룹(같은 종류의 주끼리, 같은 주 아래 부끼리) 안에서의 표시 순서.
	SortOrder int `json:"sortOrder"`
	// 조회 편의용
	Parent   string `json:"parent"`   // 주 카테고리 이름 (주 자신이면 "")
	FullName string `json:"fullName"` // "식비 > 배달" 또는 "통신비"
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
	// 혜택 자료. 실적 조건은 PerfTarget 을 그대로 쓴다.
	AnnualFee       int64  `json:"annualFee"`       // 연회비(국내전용)
	AnnualFeeGlobal int64  `json:"annualFeeGlobal"` // 연회비(해외겸용)
	BenefitURL      string `json:"benefitUrl"`      // 혜택 자료 출처 링크
	BenefitNote     string `json:"benefitNote"`     // 유의사항 메모
	// 캐시백 설정. 요율은 만분율(bp): 1% = 100bp. 0이면 캐시백 없는 카드.
	CashbackBp int `json:"cashbackBp"`
	// CashbackBonusBp: 기한 내 카드대금을 갚으면 추가로 주는 요율(bp).
	CashbackBonusBp int `json:"cashbackBonusBp"`
	// CashbackBonusDays: 결제일로부터 며칠 안에 갚아야 추가 캐시백을 받는지.
	CashbackBonusDays int `json:"cashbackBonusDays"`
}

// CardBenefit 은 카드 혜택 한 줄(카드사 안내를 옮겨 적은 자료).
// 카드사 표기가 "최대 4.5%"처럼 실적 구간에 따라 달라지는 경우가 많아,
// 최대치 여부(IsMax)와 상세 미확인 여부(NeedsCheck)를 함께 남긴다.
type CardBenefit struct {
	ID              int64  `json:"id"`
	PaymentMethodID int64  `json:"paymentMethodId"`
	Area            string `json:"area"` // "차량유지관리 업종", "대중교통/쏘카/타다" 등
	// Kind: point(적립) | discount(할인) | service(서비스 — 요율 없음)
	Kind string `json:"kind"`
	// RateBp: 적립·할인률(만분율). 450 = 4.5%. service 면 0.
	RateBp int `json:"rateBp"`
	// IsMax: 카드사가 "최대 N%"로 안내한 값인지(실적 구간별 차등).
	IsMax bool `json:"isMax"`
	// MonthlyCap: 월 적립·할인 한도(원). 0이면 없음 또는 미확인.
	MonthlyCap int64  `json:"monthlyCap"`
	Note       string `json:"note"`
	// NeedsCheck: 상세 조건을 아직 확인하지 못한 항목 표시.
	NeedsCheck bool `json:"needsCheck"`
	SortOrder  int  `json:"sortOrder"`
}

// CardStatus 는 카드 한 장의 현재 실적기간 현황.
type CardStatus struct {
	Card        PaymentMethod `json:"card"`
	PeriodStart string        `json:"periodStart"`
	PeriodEnd   string        `json:"periodEnd"`
	Spent       int64         `json:"spent"`     // 실적기간 내 지출 합계 (실적 제외분 뺀 값)
	Excluded    int64         `json:"excluded"`  // 같은 기간 중 실적 제외로 표시된 결제 합계
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
	// ExcludePerf: 카드 실적 계산에서 제외(세금·공과금 등 카드사가 실적에 안 넣어주는 결제).
	// 일반 지출 집계에는 그대로 포함된다.
	ExcludePerf bool `json:"excludePerf"`
	// PaidAt: 이 결제에 대한 카드대금을 갚은 날("YYYY-MM-DD", 빈 값이면 미납부).
	// 기한 내 납부 시 추가 캐시백을 주는 카드의 판정에 쓴다.
	PaidAt string `json:"paidAt"`
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

// MerchantSuggestion 은 수동 등록 자동완성용: 과거 거래의 가맹점과, 그 가맹점에
// 가장 최근 쓰였던 메모/귀속자/카테고리/결제수단/구분.
type MerchantSuggestion struct {
	Merchant        string `json:"merchant"`
	Memo            string `json:"memo"`
	Direction       string `json:"direction"`
	MemberID        *int64 `json:"memberId"`
	CategoryID      *int64 `json:"categoryId"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
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
