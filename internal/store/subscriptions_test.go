package store

import "testing"

// 요금 변경 예약: 변경 월 이전에는 현재 요금, 그 달부터는 새 요금.
func TestSubscriptionAmountAt(t *testing.T) {
	s := Subscription{Amount: 29000, NextAmount: 39000, NextAmountYM: "2026-11"}
	cases := []struct {
		ym   string
		want int64
	}{
		{"2026-08", 29000},
		{"2026-10", 29000},
		{"2026-11", 39000}, // 변경 월 당월부터 적용
		{"2026-12", 39000},
		{"2027-01", 39000},
	}
	for _, c := range cases {
		if got := s.AmountAt(c.ym); got != c.want {
			t.Errorf("AmountAt(%s) = %d, 기대 %d", c.ym, got, c.want)
		}
	}

	// 예약이 없으면 항상 현재 요금
	none := Subscription{Amount: 17000}
	if got := none.AmountAt("2030-01"); got != 17000 {
		t.Errorf("예약 없음: AmountAt = %d, 기대 17000", got)
	}
}

// 시작·종료 월 안에서만 결제 예정으로 잡힌다. EndYM 은 "마지막으로 빠지는 달".
func TestSubscriptionDueInMonthly(t *testing.T) {
	// 2026-09 부터 2개월(09, 10)만 결제하고 종료
	s := Subscription{Active: true, Cycle: "monthly", StartYM: "2026-09", EndYM: "2026-10"}
	cases := []struct {
		ym   string
		want bool
	}{
		{"2026-08", false}, // 시작 전
		{"2026-09", true},  // 시작 월
		{"2026-10", true},  // 종료 월 — 이 달까지는 빠진다
		{"2026-11", false}, // 종료 다음 달
	}
	for _, c := range cases {
		if got := s.DueIn(c.ym); got != c.want {
			t.Errorf("DueIn(%s) = %v, 기대 %v", c.ym, got, c.want)
		}
	}
}

func TestSubscriptionDueInEdges(t *testing.T) {
	// 기간 제한 없음 → 항상 예정
	open := Subscription{Active: true, Cycle: "monthly"}
	if !open.DueIn("2019-01") || !open.DueIn("2099-12") {
		t.Error("기간 제한이 없으면 언제나 예정이어야 한다")
	}
	// 꺼 두면 기간과 무관하게 예정 아님
	off := Subscription{Active: false, Cycle: "monthly"}
	if off.DueIn("2026-08") {
		t.Error("꺼 둔 정기결제가 예정으로 잡혔다")
	}
	// 시작만 지정
	fromOnly := Subscription{Active: true, Cycle: "monthly", StartYM: "2026-09"}
	if fromOnly.DueIn("2026-08") || !fromOnly.DueIn("2026-09") {
		t.Error("시작 월만 지정한 경우가 어긋난다")
	}
	// 종료만 지정
	toOnly := Subscription{Active: true, Cycle: "monthly", EndYM: "2026-09"}
	if !toOnly.DueIn("2026-09") || toOnly.DueIn("2026-10") {
		t.Error("종료 월만 지정한 경우가 어긋난다")
	}
	// 형식이 깨진 값에 패닉하지 않아야 한다
	if open.DueIn("") || open.DueIn("2026") {
		t.Error("짧은 ym 은 예정이 아니어야 한다")
	}
}

// 연 결제는 결제 월에만 빠진다.
func TestSubscriptionDueInYearly(t *testing.T) {
	s := Subscription{Active: true, Cycle: "yearly", BillingMonth: 3}
	if s.DueIn("2026-02") || s.DueIn("2026-04") {
		t.Error("연 결제가 결제 월이 아닌 달에 예정으로 잡혔다")
	}
	if !s.DueIn("2026-03") || !s.DueIn("2027-03") {
		t.Error("연 결제가 결제 월에 예정으로 잡히지 않았다")
	}
	// 결제 월이 안 잡혀 있으면 아무 달도 예정이 아니다
	bad := Subscription{Active: true, Cycle: "yearly", BillingMonth: 0}
	for m := 1; m <= 12; m++ {
		if bad.DueIn("2026-" + pad2m(m)) {
			t.Errorf("결제 월 미지정인데 %d월이 예정으로 잡혔다", m)
		}
	}
}

func pad2m(m int) string {
	if m < 10 {
		return "0" + string(rune('0'+m))
	}
	return string(rune('0'+m/10)) + string(rune('0'+m%10))
}

// 월 환산: 연 결제는 12로 나눠 월 결제와 비교 가능하게 한다.
func TestSubscriptionMonthlyEquivalent(t *testing.T) {
	monthly := Subscription{Amount: 17000, Cycle: "monthly"}
	if got := monthly.MonthlyEquivalent("2026-08"); got != 17000 {
		t.Errorf("월 결제 환산 = %d, 기대 17000", got)
	}
	yearly := Subscription{Amount: 120000, Cycle: "yearly", BillingMonth: 3}
	if got := yearly.MonthlyEquivalent("2026-08"); got != 10000 {
		t.Errorf("연 결제 환산 = %d, 기대 10000", got)
	}
	// 요금 변경 예약도 환산에 반영된다
	changing := Subscription{Amount: 29000, Cycle: "monthly", NextAmount: 39000, NextAmountYM: "2026-11"}
	if got := changing.MonthlyEquivalent("2026-11"); got != 39000 {
		t.Errorf("변경 후 환산 = %d, 기대 39000", got)
	}
}

func TestSubscriptionEndsInAndMatchTarget(t *testing.T) {
	s := Subscription{EndYM: "2026-10"}
	if !s.EndsIn("2026-10") || s.EndsIn("2026-09") {
		t.Error("EndsIn 이 마지막 결제 달을 제대로 못 짚는다")
	}
	if (Subscription{}).EndsIn("2026-10") {
		t.Error("종료 월이 없는데 EndsIn 이 참이다")
	}

	// 대조 대상: 가맹점이 있으면 그것, 없으면 이름
	withMerchant := Subscription{Name: "구글 제미나이", Merchant: "GOOGLE *GEMINI"}
	if withMerchant.MatchTarget() != "GOOGLE *GEMINI" {
		t.Errorf("MatchTarget = %q", withMerchant.MatchTarget())
	}
	nameOnly := Subscription{Name: "넷플릭스", Merchant: "   "}
	if nameOnly.MatchTarget() != "넷플릭스" {
		t.Errorf("가맹점이 공백이면 이름을 써야 한다: %q", nameOnly.MatchTarget())
	}
}
