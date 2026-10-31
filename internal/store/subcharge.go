package store

import (
	"fmt"
	"time"

	"sobi/internal/holiday"
)

// 정기결제 결제일 계산의 단일 출처. 자동 등록·정기결제 화면·자금 계획이 모두 이걸 쓴다.

// 결제일 규칙 값.
const (
	DayRuleDay     = "day"
	DayRuleWeekday = "weekday"

	HolidayNext = "next"
	HolidayPrev = "prev"
	HolidayNone = "none"
)

// normalizeSchedule 은 결제일 규칙 필드를 허용 값으로 맞춘다.
func (s *Subscription) normalizeSchedule() {
	if s.DayRule != DayRuleWeekday {
		s.DayRule = DayRuleDay
	}
	if s.HolidayRule != HolidayPrev && s.HolidayRule != HolidayNone {
		s.HolidayRule = HolidayNext
	}
	if s.WeekNth != -1 && (s.WeekNth < 1 || s.WeekNth > 5) {
		s.WeekNth = 1
	}
	if s.Weekday < 0 || s.Weekday > 6 {
		s.Weekday = 5
	}
}

// nthWeekday 는 (year, month) 의 nth 번째 weekday. nth=-1 은 마지막.
// 그 달에 nth 번째가 없으면(5번째 등) 마지막 것을 돌려준다.
func nthWeekday(year int, month time.Month, nth int, weekday time.Weekday) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	if nth == -1 {
		d := last
		for d.Weekday() != weekday {
			d = d.AddDate(0, 0, -1)
		}
		return d
	}
	if nth < 1 {
		nth = 1
	}
	offset := (int(weekday) - int(first.Weekday()) + 7) % 7
	d := first.AddDate(0, 0, offset+7*(nth-1))
	for d.After(last) {
		d = d.AddDate(0, 0, -7)
	}
	return d
}

// ChargeDate 는 ym("YYYY-MM") 달의 결제일. planned 는 규칙대로 정한 원래 날짜, actual 은
// 휴일 처리(HolidayRule)를 거친 실제 날짜다. 다음 영업일로 밀면 다음 달로, 이전 영업일로
// 당기면 이전 달로 넘어갈 수 있다. 연 결제는 ym 을 결제 월로 넘긴다(DueIn 이 거른다).
func (s Subscription) ChargeDate(ym string, cal *holiday.Calendar) (planned, actual time.Time, err error) {
	first, err := time.Parse("2006-01", ym)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	s.normalizeSchedule()
	if s.DayRule == DayRuleWeekday {
		planned = nthWeekday(first.Year(), first.Month(), s.WeekNth, time.Weekday(s.Weekday))
	} else {
		last := first.AddDate(0, 1, -1).Day()
		day := s.BillingDay
		if day < 1 {
			day = 1
		}
		if day > last {
			day = last
		}
		planned = time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)
	}
	switch {
	case s.HolidayRule == HolidayNone || cal == nil:
		actual = planned
	case s.HolidayRule == HolidayPrev:
		actual = cal.PrevBusinessDay(planned)
	default:
		actual = cal.NextBusinessDay(planned)
	}
	return planned, actual, nil
}

// ShiftReason 은 결제일이 밀린(당겨진) 까닭을 짧게. 안 밀렸으면 "".
// 예: "주말 → 다음 영업일", "공휴일(추석) → 이전 영업일".
func (s Subscription) ShiftReason(planned, actual time.Time, cal *holiday.Calendar) string {
	if actual.Equal(planned) || cal == nil {
		return ""
	}
	dir := "다음 영업일"
	if s.HolidayRule == HolidayPrev {
		dir = "이전 영업일"
	}
	if h, name := cal.Holiday(planned); h {
		if name == "" {
			return "공휴일 → " + dir
		}
		return fmt.Sprintf("공휴일(%s) → %s", name, dir)
	}
	return "주말 → " + dir
}
