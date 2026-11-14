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
	// FixedCost 는 고정비 표시(구성원 분석의 생활 소비에서 뺀다). ParentFixed 는 상위 주에 표시가 켜져 있는지 —
	// 부는 자기 표시가 없어도 주가 고정비면 고정비로 본다.
	FixedCost   bool `json:"fixedCost"`
	ParentFixed bool `json:"parentFixed"`
	// DefaultMemberID 는 카테고리 기본 귀속자(없으면 nil). ParentDefaultMemberID 는 상위 주의 기본 귀속자 —
	// 부는 자기 값이 없으면 이 값을 쓴다. 화면은 카테고리를 고를 때 귀속자 칸을 이 값으로 채운다.
	DefaultMemberID       *int64 `json:"defaultMemberId"`
	ParentDefaultMemberID *int64 `json:"parentDefaultMemberId"`
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
	// StatementStartDay: 청구(이용)기간 시작일. 결제일에 빠져나갈 금액이 어느 기간 사용분인지 정한다.
	// 예: 1일 결제 카드가 19 이면 "전전월 19일 ~ 전월 18일" 이용분이 1일에 청구된다.
	// 0이면 미지정 — 실적 시작일(>1)을, 그것도 1이면 결제일 기준 기본값을 쓴다.
	StatementStartDay int `json:"statementStartDay"`
	// ClosedAt: 해지일(YYYY-MM-DD). 비어 있으면 사용 중.
	// 해지한 결제수단은 지우지 않고 보관한다 — 과거 거래가 계속 이 결제수단에 연결돼 통계·내역이 유지된다.
	// 새 거래·정기결제·선불권을 고를 때는 목록에서 빠지고, 실적·캐시백 대상에서도 빠진다.
	// 해지 전에 쓴 카드값은 해지 뒤에도 청구되므로 자금 계획에는 남은 청구가 있으면 계속 나온다.
	ClosedAt string `json:"closedAt"`
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
	// SubscriptionName: 이 거래가 정기결제의 한 달치로 연결돼 있으면 그 정기결제 이름(아니면 "").
	SubscriptionName string `json:"subscriptionName"`
	// PrepaidName: 이 거래가 선불권 충전으로 연결돼 있으면 그 선불권 이름(아니면 "").
	PrepaidName string `json:"prepaidName"`
	// DuesName: 회비 납부로 장부에 들어간 거래면 그 회비 이름(아니면 ""). 앱 계층이 채운다.
	DuesName string `json:"duesName"`
	// TransitName: 교통카드(K-패스) 월 기록 거래면 그 교통카드 이름(아니면 ""). 앱 계층이 채운다.
	TransitName string `json:"transitName"`
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

// ---- 자금 계획 (카드대금 대비 현금 흐름) ----

// CashSettings 는 자금 계획 설정. app_settings 의 "cash.*" 키로 저장한다.
type CashSettings struct {
	SalaryDay    int   `json:"salaryDay"`    // 월급일(1~31). 기본 10
	SalaryAmount int64 `json:"salaryAmount"` // 예상 월급(원). 0이면 최근 3개월 "급여" 평균으로 자동
	Reserve      int64 `json:"reserve"`      // 항상 남겨둘 최소 잔액(원). 안전 지출 가능액 계산에 쓴다
}

// CashBalance 는 사용자가 입력한 통장 잔액 기록(기준일 시점).
type CashBalance struct {
	ID     int64  `json:"id"`
	AsOf   string `json:"asOf"` // YYYY-MM-DD
	Amount int64  `json:"amount"`
	Memo   string `json:"memo"`
}

// CardBill 은 카드 한 장의 결제일 한 번 청구분.
type CardBill struct {
	CardID      int64  `json:"cardId"`
	CardName    string `json:"cardName"`
	Color       string `json:"color"`
	NominalDue  string `json:"nominalDue"`  // 약정 결제일 (YYYY-MM-DD)
	DueDate     string `json:"dueDate"`     // 실제 출금일 — 주말·공휴일이면 다음 영업일
	PeriodStart string `json:"periodStart"` // 이용기간 시작
	PeriodEnd   string `json:"periodEnd"`   // 이용기간 끝
	Closed      bool   `json:"closed"`      // 이용기간이 끝나 금액이 확정됐는지 (오늘 > PeriodEnd)
	Total       int64  `json:"total"`       // 이용기간 내 지출 합계 (실적 제외분 포함)
	Prepaid     int64  `json:"prepaid"`     // 그중 이미 갚은(PaidAt 있는) 금액
	Due         int64  `json:"due"`         // 결제일에 빠질 금액 = Total - Prepaid (지금까지 기준)
	Projected   int64  `json:"projected"`   // 기간이 열려 있으면 기간 끝까지의 예상 청구액, 닫혔으면 Due 와 같음
	TxCount     int    `json:"txCount"`
	// 예상분(Projected − Due)의 근거: 최근 AvgDays 일 일평균 × 남은 RemainDays 일. 예상분이 없으면 둘 다 0.
	AvgDays    int `json:"avgDays"`
	RemainDays int `json:"remainDays"`
}

// CardTransfer 는 출금일이 [From, Until) 인 카드 청구서를 모은 '카드값 통장에 둘 금액'.
type CardTransfer struct {
	From      string     `json:"from"`
	Until     string     `json:"until"`
	Bills     []CardBill `json:"bills"`
	Confirmed int64      `json:"confirmed"` // 지금까지 이용한 금액(선결제 뺌)
	Estimated int64      `json:"estimated"` // 이용기간이 남은 카드의 남은 날 예상분
	Suggested int64      `json:"suggested"` // 권장 이체액 (예상분에 여유를 더해 만 원 단위로 올림)
	OpenUntil string     `json:"openUntil"` // 아직 안 끝난 이용기간 중 가장 늦은 마감일 ("" 면 모두 확정)
}

// CashEvent 는 자금 흐름 타임라인의 한 항목. Amount 는 들어오면 +, 나가면 −.
type CashEvent struct {
	Date      string `json:"date"`      // YYYY-MM-DD
	Kind      string `json:"kind"`      // card | salary | subscription
	Label     string `json:"label"`     // 표시 이름 (카드명, "월급", 정기결제명)
	Amount    int64  `json:"amount"`    // 부호 있음
	Estimated bool   `json:"estimated"` // 예상치(월급 자동 추정, 열린 이용기간 예상 카드값 등)
	Balance   int64  `json:"balance"`   // 이 항목 반영 뒤 예상 잔액
	RefID     int64  `json:"refId"`     // 카드 ID 또는 정기결제 ID (월급은 0)
}

// CashPlan 은 자금 계획 화면 전체 데이터.
type CashPlan struct {
	Today    string       `json:"today"`
	Settings CashSettings `json:"settings"`

	// 월급
	SalaryAuto  int64  `json:"salaryAuto"`  // 최근 3개월 급여 평균 (자동 추정치)
	SalaryUsed  int64  `json:"salaryUsed"`  // 계산에 쓴 월급 (설정값이 있으면 설정값, 없으면 자동)
	SalaryBasis string `json:"salaryBasis"` // 사람이 읽는 근거 (예: "최근 3개월 급여 평균")
	NextSalary  string `json:"nextSalary"`  // 다음 월급일 (주말·공휴일이면 직전 영업일)

	// 잔액
	Balance       *CashBalance `json:"balance"`       // 가장 최근 입력한 잔액 (없으면 null)
	BalanceAdjust int64        `json:"balanceAdjust"` // 기준일 이후 오늘까지 반영한 증감 (수입 − 계좌·현금 지출 − 빠져나간 카드대금)
	BalanceNow    int64        `json:"balanceNow"`    // 추정 현재 잔액 = Balance.Amount + BalanceAdjust
	FxAdjust      int64        `json:"fxAdjust"`      // BalanceAdjust 중 외화를 사고판 원화(산 만큼 −, 판 만큼 +)

	// 다음 카드 결제일
	NextDue      string     `json:"nextDue"`      // 가장 가까운 카드대금 출금일 (없으면 "")
	NextDueTotal int64      `json:"nextDueTotal"` // 그날 빠질 카드대금 합계
	NextDueBills []CardBill `json:"nextDueBills"` // 그날 빠질 카드별 청구
	AfterNextDue int64      `json:"afterNextDue"` // 그날 출금 뒤 예상 잔액 (음수면 부족)

	// 그다음 결제 (지금 쓰는 카드값이 쌓이는 청구)
	FollowingDue       string     `json:"followingDue"`
	FollowingTotal     int64      `json:"followingTotal"`     // 지금까지 쌓인 금액
	FollowingProjected int64      `json:"followingProjected"` // 이용기간 끝까지의 예상
	FollowingBills     []CardBill `json:"followingBills"`

	// 타임라인 (오늘 ~ Horizon)
	Horizon       string      `json:"horizon"`
	Events        []CashEvent `json:"events"`
	LowestBalance int64       `json:"lowestBalance"` // 기간 중 가장 낮은 예상 잔액
	LowestDate    string      `json:"lowestDate"`

	// 카드값 통장: 월급이 10일, 카드값이 다음 달 1일처럼 떨어져 있으면 그 사이 카드값을 따로 떼어 둔다.
	// SalaryTransfer 는 다음 월급날 옮길 금액(그다음 월급 전까지 빠질 카드값),
	// Holding 은 지금 카드값 통장에 있어야 할 금액(오늘~다음 월급 전에 빠질 카드값, 없으면 Bills 가 빈 목록).
	SalaryTransfer CardTransfer `json:"salaryTransfer"`
	Holding        CardTransfer `json:"holding"`

	// 다음 월급 전까지
	SafeToSpend    int64 `json:"safeToSpend"`    // 다음 월급 전까지 추가로 써도 되는 돈 = 그 사이 최저 예상 잔액 − Reserve (음수면 0 아님, 그대로)
	DaysToSalary   int   `json:"daysToSalary"`   // 오늘부터 다음 월급일까지 남은 일수
	DailyAllowance int64 `json:"dailyAllowance"` // SafeToSpend / DaysToSalary (0 이하이면 0)
}
