package store

import (
	"testing"
	"time"

	"sobi/internal/holiday"
)

func TestStatementStartDayDefault(t *testing.T) {
	cases := []struct {
		name string
		pm   PaymentMethod
		want int
	}{
		{"결제일 1 기본", PaymentMethod{BillingDay: 1, CycleStartDay: 1}, 19},
		{"결제일 25 기본", PaymentMethod{BillingDay: 25, CycleStartDay: 1}, 12},
		{"결제일 14 기본", PaymentMethod{BillingDay: 14, CycleStartDay: 1}, 1},
		{"결제일 15 기본", PaymentMethod{BillingDay: 15, CycleStartDay: 1}, 2},
		{"결제일 31 기본", PaymentMethod{BillingDay: 31, CycleStartDay: 1}, 18},
		{"실적 시작일 사용", PaymentMethod{BillingDay: 1, CycleStartDay: 20}, 20},
		{"명시값 우선", PaymentMethod{BillingDay: 1, CycleStartDay: 20, StatementStartDay: 5}, 5},
	}
	for _, c := range cases {
		if got := StatementStartDay(c.pm); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestBillPeriod(t *testing.T) {
	cases := []struct {
		due        string
		s          int
		start, end string
	}{
		{"2026-11-01", 19, "2026-09-19", "2026-10-18"},
		{"2026-11-01", 1, "2026-09-01", "2026-09-30"},
		{"2026-11-01", 20, "2026-09-20", "2026-10-19"},
		{"2026-11-25", 12, "2026-10-12", "2026-11-11"},
		{"2026-12-01", 19, "2026-10-19", "2026-11-18"},
		// 2월: 결제일 28(말일) S=14 → 한계 2/18, 주기 2/14~3/13 은 넘으니 1/14~2/13
		{"2027-02-28", 14, "2027-01-14", "2027-02-13"},
		// S=31 은 달 말일로 줄어든다
		{"2027-03-01", 31, "2026-12-31", "2027-01-30"},
	}
	for _, c := range cases {
		s, e := billPeriod(date(c.due), c.s)
		if fmtDay(s) != c.start || fmtDay(e) != c.end {
			t.Errorf("due %s S=%d: got %s~%s want %s~%s", c.due, c.s, fmtDay(s), fmtDay(e), c.start, c.end)
		}
		if e.After(date(c.due).AddDate(0, 0, -billCloseGapDays)) {
			t.Errorf("due %s S=%d: 이용기간 끝 %s 이 결제일−10일을 넘음", c.due, c.s, fmtDay(e))
		}
	}
}

func TestNominalDueClampsToMonthEnd(t *testing.T) {
	if got := fmtDay(nominalDue(2027, time.February, 31)); got != "2027-02-28" {
		t.Errorf("평년 2월: %s", got)
	}
	if got := fmtDay(nominalDue(2028, time.February, 30)); got != "2028-02-29" {
		t.Errorf("윤년 2월: %s", got)
	}
	if got := fmtDay(nominalDue(2026, time.April, 31)); got != "2026-04-30" {
		t.Errorf("4월: %s", got)
	}
}

func TestDueDateShiftsOverWeekendAndHoliday(t *testing.T) {
	cal := holiday.New(nil)
	cases := []struct{ nominal, want string }{
		{"2026-11-01", "2026-11-02"}, // 일요일 → 월요일
		{"2026-10-03", "2026-10-06"}, // 토요일(개천절) + 대체공휴일 10/5 → 10/6
		{"2026-05-05", "2026-05-06"}, // 어린이날(화) → 수
		{"2026-12-01", "2026-12-01"}, // 화요일
	}
	for _, c := range cases {
		if got := fmtDay(cal.NextBusinessDay(date(c.nominal))); got != c.want {
			t.Errorf("%s → %s, want %s", c.nominal, got, c.want)
		}
	}
}

func TestSalaryDatePrevBusinessDay(t *testing.T) {
	cal := holiday.New(nil)
	// 2026-10-10 은 토요일 → 금요일 9일은 한글날(공휴일) → 8일(목)
	if got := fmtDay(salaryDate(cal, 2026, time.October, 10)); got != "2026-10-08" {
		t.Errorf("2026-10: %s", got)
	}
	// 2026-11-10 은 화요일
	if got := fmtDay(salaryDate(cal, 2026, time.November, 10)); got != "2026-11-10" {
		t.Errorf("2026-11: %s", got)
	}
	// 2026-01-10 은 토요일 → 9일 금요일
	if got := fmtDay(salaryDate(cal, 2026, time.January, 10)); got != "2026-01-09" {
		t.Errorf("2026-01: %s", got)
	}
	// 말일 보정: 31일 월급일, 2027-02 → 28일(일요일) → 26일(금)
	if got := fmtDay(salaryDate(cal, 2027, time.February, 31)); got != "2027-02-26" {
		t.Errorf("2027-02: %s", got)
	}
}

func TestNextSalaryDate(t *testing.T) {
	cal := holiday.New(nil)
	cases := []struct {
		today    string
		received bool
		want     string
	}{
		{"2026-10-31", false, "2026-11-10"},
		{"2026-11-03", false, "2026-11-10"},
		{"2026-11-10", false, "2026-11-10"}, // 오늘이 입금일, 아직 안 받음
		{"2026-11-10", true, "2026-12-10"},  // 받았으면 다음 달
		{"2026-11-11", false, "2026-12-10"},
		{"2026-10-01", false, "2026-10-08"}, // 10일이 토요일+한글날 → 8일
	}
	for _, c := range cases {
		if got := fmtDay(nextSalaryDate(cal, date(c.today), 10, c.received)); got != c.want {
			t.Errorf("today %s received=%v: got %s want %s", c.today, c.received, got, c.want)
		}
	}
}

func TestProjectedBill(t *testing.T) {
	// 지금까지 100,000 + 최근 90일 900,000 (일 10,000) × 남은 5일 = 150,000
	if got := projectedBill(100000, 900000, 90, 5); got != 150000 {
		t.Errorf("got %d", got)
	}
	// 남은 날이 없으면 그대로
	if got := projectedBill(100000, 900000, 90, 0); got != 100000 {
		t.Errorf("got %d", got)
	}
}

func TestSpendWindowDays(t *testing.T) {
	today := date("2026-11-08")
	cases := []struct {
		first string // "" 이면 사용 기록 없음
		want  int
	}{
		{"", spendAvgDays},              // 기록 없음 → 기본 90일
		{"2026-01-01", spendAvgDays},    // 오래 쓴 카드 → 최근 90일
		{"2026-08-11", spendAvgDays},    // 딱 90일째(8/11~11/8)
		{"2026-08-12", 89},              // 89일
		{"2026-09-18", 52},              // 9/18~11/8
		{"2026-10-30", minSpendAvgDays}, // 열흘 쓴 새 카드 → 30일보다 짧게 나누지 않음
		{"2026-11-08", minSpendAvgDays}, // 오늘 처음 씀
	}
	for _, c := range cases {
		var first time.Time
		if c.first != "" {
			first = date(c.first)
		}
		if got := spendWindowDays(today, first); got != c.want {
			t.Errorf("first=%q: got %d want %d", c.first, got, c.want)
		}
	}
	// 새 카드(30일 하한): 10일 동안 883,700 → 883,700/30 × 남은 11일 = 324,023 (90일로 나누면 108,007)
	if got := projectedBill(883700, 883700, spendWindowDays(today, date("2026-10-30")), 11); got != 883700+324023 {
		t.Errorf("새 카드 예상: got %d", got)
	}
}
