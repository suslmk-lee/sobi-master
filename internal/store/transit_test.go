package store

import (
	"testing"

	"sobi/internal/holiday"
)

func TestTransitEstimate(t *testing.T) {
	sub := Subscription{Kind: KindTransit, Amount: 60000}
	// 확정한 달이 없으면 설정한 예상 요금, 환급은 모름(0)
	if f, r, b := TransitEstimate(sub, nil); f != 60000 || r != 0 || b != 0 {
		t.Errorf("기록 없음: %d %d %d", f, r, b)
	}
	months := []TransitMonth{
		{YM: "2026-07", Fare: 90000, Refund: 18000},                  // 4번째 — 평균에서 빠짐
		{YM: "2026-08", Fare: 0},                                     // 안 탄 달 — 빠짐
		{YM: "2026-09", Fare: 60000, Refund: 12000},
		{YM: "2026-10", Fare: 66000, Refund: 13200},
		{YM: "2026-11", Fare: 54000, Refund: 10800, Estimated: false},
		{YM: "2026-12", Fare: 99999, Refund: 1, Estimated: true}, // 추정 — 빠짐
	}
	f, r, b := TransitEstimate(sub, months)
	if b != 3 || f != 60000 || r != 12000 {
		t.Errorf("최근 확정 3달 평균: fare=%d refund=%d basis=%d", f, r, b)
	}
}

func TestBuildTransitStatus(t *testing.T) {
	cal := holiday.New(nil)
	sub := Subscription{ID: 1, Kind: KindTransit, Amount: 60000, Cycle: "monthly", BillingDay: 10,
		DayRule: "day", HolidayRule: "next", Active: true}
	months := []TransitMonth{
		{YM: "2026-10", Date: "2026-10-12", Fare: 61000, Refund: 12200, Net: 48800},
		{YM: "2026-11", Date: "2026-11-10", Fare: 62000, Refund: 12400, Net: 49600, Estimated: true},
	}
	st := BuildTransitStatus(sub, months, date("2026-11-14"), cal)
	if st.NextYM != "2026-12" || st.NextDate != "2026-12-10" {
		t.Errorf("다음 출금: %s %s", st.NextYM, st.NextDate)
	}
	if st.Pending != 1 || len(st.Months) != 2 || st.Months[0].YM != "2026-11" {
		t.Errorf("기록: pending=%d %+v", st.Pending, st.Months)
	}
	if st.Basis != 1 || st.EstFare != 61000 || st.EstRefund != 12200 {
		t.Errorf("어림: %d %d %d", st.EstFare, st.EstRefund, st.Basis)
	}

	// 이번 달 출금일 전이면 이번 달이 다음 출금. 10/10 은 토요일 → 다음 영업일 10/12(월)
	st = BuildTransitStatus(sub, nil, date("2026-10-05"), cal)
	if st.NextYM != "2026-10" || st.NextDate != "2026-10-12" {
		t.Errorf("주말 출금일: %s %s", st.NextYM, st.NextDate)
	}
	// 출금일이 지났는데 아직 기록이 없으면(자동 기록 전) 그 달은 건너뛰고 다음 달
	st = BuildTransitStatus(sub, nil, date("2026-10-20"), cal)
	if st.NextYM != "2026-11" {
		t.Errorf("지난 출금일: %s", st.NextYM)
	}
	// 꺼 두면 다음 출금이 없다
	sub.Active = false
	if st = BuildTransitStatus(sub, months, date("2026-11-14"), cal); st.NextYM != "" {
		t.Errorf("꺼 둔 교통카드: %s", st.NextYM)
	}
}

func TestTransitMemo(t *testing.T) {
	if got := transitMemo(62350, 12470, false); got != "요금 62,350원 · 환급 12,470원" {
		t.Errorf("메모: %q", got)
	}
	if got := transitMemo(1234567, 0, true); got != "요금 1,234,567원 · 환급 0원 · 추정(실제 금액으로 고쳐 주세요)" {
		t.Errorf("추정 메모: %q", got)
	}
	if commaInt(-1500) != "-1,500" || commaInt(999) != "999" {
		t.Error("commaInt")
	}
}
