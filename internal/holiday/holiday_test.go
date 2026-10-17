package holiday

import (
	"strings"
	"testing"
	"time"
)

func day(s string) time.Time {
	d, err := time.Parse(layout, s)
	if err != nil {
		panic(err)
	}
	return d
}

func isWeekend(d time.Time) bool {
	return d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
}

// 표를 손으로 적었으니 대체공휴일 규칙에 비춰 스스로 검증한다.
// 표가 틀리면 결제일이 하루 어긋나 엉뚱한 날짜로 거래가 생긴다.

// 대체공휴일은 반드시 평일이어야 하고, 바로 앞날은 주말이거나 공휴일이어야 한다
// (그래야 "밀려서 생긴" 날이다).
func TestSubstituteHolidaysAreWeekdaysAfterAnOffDay(t *testing.T) {
	cal := New(nil)
	for date, name := range builtin {
		if !strings.HasPrefix(name, "대체공휴일") {
			continue
		}
		d := day(date)
		if isWeekend(d) {
			t.Errorf("%s %s 가 주말임", date, name)
		}
		prev := d.AddDate(0, 0, -1)
		if !isWeekend(prev) {
			if h, _ := cal.Holiday(prev); !h {
				t.Errorf("%s %s 의 전날(%s)이 평일 업무일 — 대체될 까닭이 없음", date, name, prev.Format(layout))
			}
		}
	}
}

// 대체공휴일 대상(삼일절·어린이날·광복절·개천절·한글날·성탄절)이 주말이면
// 다음 업무일에 대체공휴일이 있어야 한다. 신정·현충일은 대상이 아니라 없어야 한다.
func TestFixedHolidaySubstitutionRule(t *testing.T) {
	subject := map[string]bool{
		"03-01": true, "05-05": true, "08-15": true, "10-03": true, "10-09": true, "12-25": true,
		"01-01": false, "06-06": false,
	}
	for year := range coveredYears {
		for md, needs := range subject {
			d := day(itoa4(year) + "-" + md)
			if !isWeekend(d) {
				continue
			}
			// 그 주말 뒤 첫 평일
			next := d.AddDate(0, 0, 1)
			for isWeekend(next) {
				next = next.AddDate(0, 0, 1)
			}
			name := builtin[next.Format(layout)]
			got := strings.HasPrefix(name, "대체공휴일")
			if needs && !got {
				t.Errorf("%d-%s(%s)가 주말인데 %s 에 대체공휴일이 없음", year, md, d.Weekday(), next.Format(layout))
			}
			if !needs && got {
				t.Errorf("%d-%s 는 대체공휴일 대상이 아닌데 %s 에 대체공휴일이 있음", year, md, next.Format(layout))
			}
		}
	}
}

// 설날·추석은 사흘 연휴(가운데가 당일)이고, 연휴에 일요일이 끼면 바로 뒤 평일이 대체공휴일이다.
// 토요일만 낀 경우는 대체공휴일이 없다.
func TestLunarHolidayBlocks(t *testing.T) {
	for date, name := range builtin {
		if name != "설날" && name != "추석" {
			continue
		}
		d := day(date)
		before, after := d.AddDate(0, 0, -1), d.AddDate(0, 0, 1)
		for _, x := range []time.Time{before, after} {
			if !strings.Contains(builtin[x.Format(layout)], name) {
				t.Errorf("%s %s 연휴의 %s 가 빠짐", date, name, x.Format(layout))
			}
		}
		hasSunday := false
		for _, x := range []time.Time{before, d, after} {
			if x.Weekday() == time.Sunday {
				hasSunday = true
			}
		}
		next := after.AddDate(0, 0, 1)
		for isWeekend(next) {
			next = next.AddDate(0, 0, 1)
		}
		gotSub := strings.HasPrefix(builtin[next.Format(layout)], "대체공휴일")
		if hasSunday && !gotSub {
			t.Errorf("%s %s 연휴에 일요일이 있는데 %s 대체공휴일이 없음", date, name, next.Format(layout))
		}
		if !hasSunday && gotSub && !strings.Contains(builtin[next.Format(layout)], name) {
			t.Errorf("%s %s 연휴에 일요일이 없는데 대체공휴일이 있음", date, name)
		}
	}
}

func TestNextBusinessDay(t *testing.T) {
	cal := New(nil)
	cases := []struct {
		in, want, why string
	}{
		{"2026-10-20", "2026-10-20", "평일은 그대로"},
		{"2026-10-24", "2026-10-26", "토요일 → 월요일"},
		{"2026-10-25", "2026-10-26", "일요일 → 월요일"},
		{"2026-10-03", "2026-10-06", "개천절(토) → 대체공휴일(월) 건너 화요일"},
		{"2026-09-24", "2026-09-28", "추석 연휴(목~토) → 월요일"},
		{"2026-02-16", "2026-02-19", "설 연휴(월~수) → 목요일"},
		{"2026-01-31", "2026-02-02", "달이 바뀌어도 밀린다(토 → 다음 달 월)"},
		{"2026-12-25", "2026-12-28", "성탄절(금) → 월요일"},
		{"2027-02-05", "2027-02-09", "설 연휴 + 대체공휴일 → 화요일"},
	}
	for _, c := range cases {
		if got := cal.NextBusinessDay(day(c.in)).Format(layout); got != c.want {
			t.Errorf("%s: %s → %s, want %s", c.why, c.in, got, c.want)
		}
	}
}

// 사용자가 임시공휴일을 추가하거나, 표가 틀린 날을 업무일로 정정할 수 있어야 한다.
func TestOverrides(t *testing.T) {
	cal := New([]Override{
		{Date: "2026-10-20", Name: "임시공휴일", IsHoliday: true},
		{Date: "2026-10-09", Name: "", IsHoliday: false}, // 한글날을 업무일로 정정
	})
	if got := cal.NextBusinessDay(day("2026-10-20")).Format(layout); got != "2026-10-21" {
		t.Errorf("추가한 임시공휴일이 반영 안 됨: %s", got)
	}
	if !cal.IsBusinessDay(day("2026-10-09")) {
		t.Error("업무일로 정정한 날이 여전히 공휴일")
	}
	ys := cal.Year(2026)
	var found, off bool
	for _, e := range ys {
		if e.Date == "2026-10-20" && !e.Builtin {
			found = true
		}
		if e.Date == "2026-10-09" && e.Off {
			off = true
		}
	}
	if !found || !off {
		t.Errorf("연도 목록에 추가/정정이 안 보임: found=%v off=%v", found, off)
	}
}

// 표가 없는 해는 양력 고정 공휴일만이라도 적용하고, Covered 로 그 사실을 알린다.
func TestUncoveredYearFallsBackToFixedSolar(t *testing.T) {
	cal := New(nil)
	if Covered(2030) {
		t.Fatal("2030 은 표가 없어야 함")
	}
	if h, name := cal.Holiday(day("2030-08-15")); !h || name != "광복절" {
		t.Errorf("표 없는 해에 양력 고정 공휴일이 빠짐: %v %q", h, name)
	}
	if h, _ := cal.Holiday(day("2030-02-04")); h {
		t.Error("표 없는 해에 임의 날짜가 공휴일로 잡힘")
	}
}
