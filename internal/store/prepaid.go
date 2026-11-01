package store

// ---- 선불권 (미용실 회원권처럼 미리 충전해 두고 쓸 때마다 차감하는 결제) ----
//
// 회계 기준: 지출은 "충전할 때" 잡는다. 충전은 평소처럼 지출 거래 한 건이고(카드값·실적·자금 계획에
// 그대로 반영), 그 거래를 선불권에 연결해 적립액을 기록한다. 쓸 때(머리 할 때)는 지출 거래를 만들지 않고
// 선불권의 차감 기록(PrepaidUse)만 남긴다 — 통계에 두 번 잡히지 않게 하기 위해서다.
//
// 할인 방식 (선불권마다 고른다):
//   "bonus"    — 충전 보너스: 결제액에 BonusBp 만큼 더 적립 (30만 원 충전 → 33만 원 적립), 차감은 정가 그대로
//   "discount" — 차감 할인: 적립은 결제액 그대로, 쓸 때 정가에서 DiscountBp 만큼 깎아 차감
//   "both"     — 둘 다: 충전 때 BonusBp 만큼 더 적립하고, 쓸 때도 DiscountBp 만큼 깎아 차감
//   "none"     — 할인 없음
// 요율은 만분율(bp): 10% = 1000.

// PrepaidPass 는 선불권 한 장(가게 하나의 회원권·충전금).
type PrepaidPass struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`     // 표시 이름 (예: "OO헤어 회원권")
	Merchant string `json:"merchant"` // 가게 이름 — 충전 거래를 고를 때 가맹점 대조에 쓴다
	// DiscountMode: "bonus" | "discount" | "both" | "none"
	DiscountMode string `json:"discountMode"`
	BonusBp      int    `json:"bonusBp"`    // 충전 보너스율 (bonus·both 일 때)
	DiscountBp   int    `json:"discountBp"` // 차감 할인율 (discount·both 일 때)
	// 앱을 쓰기 전부터 남아 있던 잔액. OpeningDate 이후의 충전·차감만 잔액 계산에 넣는다(빈 값이면 전부).
	OpeningBalance int64  `json:"openingBalance"`
	OpeningDate    string `json:"openingDate"` // YYYY-MM-DD
	ExpiresAt      string `json:"expiresAt"`   // 유효기간 끝 (YYYY-MM-DD, 빈 값이면 없음)
	// 기본값: 충전 거래를 새로 만들 때 쓰는 카테고리·결제수단, 차감 기록의 귀속자
	CategoryID      *int64 `json:"categoryId"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	MemberID        *int64 `json:"memberId"`
	Memo            string `json:"memo"`
	Active          bool   `json:"active"` // false 면 다 쓰고 정리한 선불권(목록 아래로)
}

// PrepaidCharge 는 충전 한 번 = 지출 거래 한 건 + 적립액.
type PrepaidCharge struct {
	PassID        int64  `json:"passId"`
	TransactionID int64  `json:"transactionId"`
	Date          string `json:"date"`   // 거래 날짜
	Paid          int64  `json:"paid"`   // 결제액 = 거래 금액
	Credit        int64  `json:"credit"` // 적립액 (bonus 면 Paid + 보너스, 아니면 Paid)
	// 조회 편의: 거래 쪽 정보
	Merchant          string `json:"merchant"`
	PaymentMethodName string `json:"paymentMethodName"`
}

// PrepaidUse 는 사용(차감) 한 번. 지출 거래가 아니다.
type PrepaidUse struct {
	ID         int64  `json:"id"`
	PassID     int64  `json:"passId"`
	Date       string `json:"date"`       // YYYY-MM-DD
	ListPrice  int64  `json:"listPrice"`  // 정가 (모르면 Deducted 와 같게)
	Deducted   int64  `json:"deducted"`   // 실제 차감액 (discount 면 보통 정가 × (1 − 할인율))
	MemberID   *int64 `json:"memberId"`   // 누가 썼는지
	MemberName string `json:"memberName"` // 조회 편의
	Memo       string `json:"memo"`       // 예: "커트", "펌"
}

// PrepaidStatus 는 선불권 목록 한 줄(카드 탭 타일).
type PrepaidStatus struct {
	Pass         PrepaidPass `json:"pass"`
	Balance      int64       `json:"balance"`     // 잔액 = Opening + Σ적립 − Σ차감
	TotalPaid    int64       `json:"totalPaid"`   // 충전 결제액 합 (Opening 이후)
	TotalCredit  int64       `json:"totalCredit"` // 적립액 합
	TotalUsed    int64       `json:"totalUsed"`   // 차감액 합
	Saved        int64       `json:"saved"`       // 절약액 = Σ(적립 − 결제) + Σ(정가 − 차감)
	ChargeCount  int         `json:"chargeCount"`
	UseCount     int         `json:"useCount"`
	LastUse      string      `json:"lastUse"`      // 마지막 사용일 ("" 없음)
	AvgUseAmount int64       `json:"avgUseAmount"` // 최근 사용 평균 차감액
	AvgUseDays   int         `json:"avgUseDays"`   // 최근 사용 평균 간격(일), 2번 미만이면 0
	RunOutDate   string      `json:"runOutDate"`   // 이 속도면 잔액이 바닥나는 예상일 ("" 계산 불가)
	UsesLeft     int         `json:"usesLeft"`     // 평균 차감액 기준 남은 사용 횟수 (0 = 계산 불가 또는 부족)
	Low          bool        `json:"low"`          // 잔액이 평균 1회 차감액보다 적다 → 재충전 필요
	Expired      bool        `json:"expired"`      // 유효기간이 지났다
}

// PrepaidHistory 는 선불권 한 장의 충전·사용 내역 (최근 것부터).
type PrepaidHistory struct {
	Charges []PrepaidCharge `json:"charges"`
	Uses    []PrepaidUse    `json:"uses"`
}
