package store

import (
	"testing"
	"time"

	"sobi/internal/holiday"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNthWeekday(t *testing.T) {
	cases := []struct {
		y       int
		m       time.Month
		nth     int
		wd      time.Weekday
		want    string
		comment string
	}{
		{2026, 10, 1, time.Friday, "2026-10-02", "첫 금요일"},
		{2026, 10, 2, time.Friday, "2026-10-09", "둘째 금요일"},
		{2026, 10, 5, time.Friday, "2026-10-30", "5번째가 있는 달"},
		{2026, 2, 5, time.Sunday, "2026-02-22", "5번째 없으면 마지막"},
		{2026, 10, -1, time.Friday, "2026-10-30", "마지막"},
		{2026, 2, -1, time.Friday, "2026-02-27", "2월 마지막"},
		{2026, 11, 1, time.Sunday, "2026-11-01", "1일이 곧 그 요일"},
		{2026, 11, -1, time.Monday, "2026-11-30", "말일이 곧 그 요일"},
		{2026, 11, 5, time.Monday, "2026-11-30", "5번째 월요일"},
	}
	for _, c := range cases {
		if got := nthWeekday(c.y, c.m, c.nth, c.wd).Format("2006-01-02"); got != c.want {
			t.Errorf("%s: got %s, want %s", c.comment, got, c.want)
		}
	}
}

func TestChargeDate(t *testing.T) {
	cal := holiday.New(nil)
	cases := []struct {
		name         string
		sub          Subscription
		ym           string
		planned, act string
		reason       string
	}{
		{"기본값(필드 비움) = 결제일·다음 영업일", Subscription{BillingDay: 24}, "2026-10", "2026-10-24", "2026-10-26", "주말 → 다음 영업일"},
		{"day/next 평일", Subscription{BillingDay: 20, DayRule: "day", HolidayRule: "next"}, "2026-10", "2026-10-20", "2026-10-20", ""},
		{"day/next 추석", Subscription{BillingDay: 25, DayRule: "day", HolidayRule: "next"}, "2026-09", "2026-09-25", "2026-09-28", "공휴일(추석) → 다음 영업일"},
		{"day/prev 추석", Subscription{BillingDay: 25, DayRule: "day", HolidayRule: "prev"}, "2026-09", "2026-09-25", "2026-09-23", "공휴일(추석) → 이전 영업일"},
		{"day/none 추석", Subscription{BillingDay: 25, DayRule: "day", HolidayRule: "none"}, "2026-09", "2026-09-25", "2026-09-25", ""},
		{"day/prev 토요일", Subscription{BillingDay: 24, DayRule: "day", HolidayRule: "prev"}, "2026-10", "2026-10-24", "2026-10-23", "주말 → 이전 영업일"},
		{"day/none 일요일", Subscription{BillingDay: 25, DayRule: "day", HolidayRule: "none"}, "2026-10", "2026-10-25", "2026-10-25", ""},
		{"day/next 개천절(토)+대체공휴일", Subscription{BillingDay: 3, HolidayRule: "next"}, "2026-10", "2026-10-03", "2026-10-06", "공휴일(개천절) → 다음 영업일"},
		{"day/prev 개천절(토)", Subscription{BillingDay: 3, HolidayRule: "prev"}, "2026-10", "2026-10-03", "2026-10-02", "공휴일(개천절) → 이전 영업일"},
		{"말일 보정 31일 → 2월 28일(토→다음)", Subscription{BillingDay: 31}, "2026-02", "2026-02-28", "2026-03-03", "주말 → 다음 영업일"},
		{"next 가 다음 달로 넘어감", Subscription{BillingDay: 31}, "2026-10", "2026-10-31", "2026-11-02", "주말 → 다음 영업일"},
		{"prev 가 전달로 넘어감(1일 일요일)", Subscription{BillingDay: 1, HolidayRule: "prev"}, "2026-11", "2026-11-01", "2026-10-30", "주말 → 이전 영업일"},
		{"prev 가 전달로 넘어감(신정 목 → 수)", Subscription{BillingDay: 1, HolidayRule: "prev"}, "2026-01", "2026-01-01", "2025-12-31", "공휴일(신정) → 이전 영업일"},
		{"weekday 둘째 금요일, 한글날 next", Subscription{DayRule: "weekday", WeekNth: 2, Weekday: 5, HolidayRule: "next"}, "2026-10", "2026-10-09", "2026-10-12", "공휴일(한글날) → 다음 영업일"},
		{"weekday 둘째 금요일, 한글날 prev", Subscription{DayRule: "weekday", WeekNth: 2, Weekday: 5, HolidayRule: "prev"}, "2026-10", "2026-10-09", "2026-10-08", "공휴일(한글날) → 이전 영업일"},
		{"weekday 마지막 금요일", Subscription{DayRule: "weekday", WeekNth: -1, Weekday: 5, HolidayRule: "next"}, "2026-10", "2026-10-30", "2026-10-30", ""},
		{"weekday 첫째 일요일 prev → 전달", Subscription{DayRule: "weekday", WeekNth: 1, Weekday: 0, HolidayRule: "prev"}, "2026-11", "2026-11-01", "2026-10-30", "주말 → 이전 영업일"},
		{"weekday 첫째 토요일 none", Subscription{DayRule: "weekday", WeekNth: 1, Weekday: 6, HolidayRule: "none"}, "2026-10", "2026-10-03", "2026-10-03", ""},
		{"weekday 5번째 없음 → 마지막", Subscription{DayRule: "weekday", WeekNth: 5, Weekday: 0, HolidayRule: "none"}, "2026-02", "2026-02-22", "2026-02-22", ""},
		{"연 결제(결제 월 10월, 9일 한글날)", Subscription{Cycle: "yearly", BillingMonth: 10, BillingDay: 9}, "2026-10", "2026-10-09", "2026-10-12", "공휴일(한글날) → 다음 영업일"},
		{"연 결제 weekday", Subscription{Cycle: "yearly", BillingMonth: 10, DayRule: "weekday", WeekNth: 1, Weekday: 5}, "2026-10", "2026-10-02", "2026-10-02", ""},
		{"잘못된 규칙 값은 기본값으로", Subscription{BillingDay: 24, DayRule: "x", HolidayRule: "y"}, "2026-10", "2026-10-24", "2026-10-26", "주말 → 다음 영업일"},
	}
	for _, c := range cases {
		planned, actual, err := c.sub.ChargeDate(c.ym, cal)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := planned.Format("2006-01-02"); got != c.planned {
			t.Errorf("%s: planned %s, want %s", c.name, got, c.planned)
		}
		if got := actual.Format("2006-01-02"); got != c.act {
			t.Errorf("%s: actual %s, want %s", c.name, got, c.act)
		}
		if got := c.sub.ShiftReason(planned, actual, cal); got != c.reason {
			t.Errorf("%s: reason %q, want %q", c.name, got, c.reason)
		}
	}
	if _, _, err := (Subscription{}).ChargeDate("bad", cal); err == nil {
		t.Error("잘못된 ym 은 오류여야 한다")
	}
}

func TestNormalizeSchedule(t *testing.T) {
	s := Subscription{DayRule: "zzz", HolidayRule: "zzz", WeekNth: 9, Weekday: 9}
	s.normalizeSchedule()
	if s.DayRule != "day" || s.HolidayRule != "next" || s.WeekNth != 1 || s.Weekday != 5 {
		t.Errorf("기본값으로 돌아가지 않음: %+v", s)
	}
	s = Subscription{DayRule: "weekday", HolidayRule: "prev", WeekNth: -1, Weekday: 0}
	s.normalizeSchedule()
	if s.DayRule != "weekday" || s.HolidayRule != "prev" || s.WeekNth != -1 || s.Weekday != 0 {
		t.Errorf("유효한 값이 바뀜: %+v", s)
	}
}
