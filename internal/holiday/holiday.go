// Package holiday 는 한국 공휴일과 "다음 업무일" 계산을 맡는다.
//
// 정기결제 자동 등록에서 결제일이 주말·공휴일이면 다음 업무일로 미룰 때 쓴다.
// 음력 공휴일(설·추석·부처님오신날)과 대체공휴일·선거일·임시공휴일은 규칙으로
// 계산할 수 없거나 해마다 정해지므로 연도별 표로 둔다. 표에 없는 해는 양력 고정
// 공휴일만 적용하고, 그 사실을 Covered 로 알린다. 표가 틀렸거나 임시공휴일이
// 새로 생기면 사용자가 설정에서 날짜를 추가·해제(Override)해 바로잡는다.
package holiday

import (
	"sort"
	"time"
)

const layout = "2006-01-02"

// builtin 은 연도별 공휴일 표(대체공휴일·선거일·임시공휴일 포함).
// 근로자의 날(5/1)은 법정 공휴일이 아니라 넣지 않았다 — 은행 휴무라 결제일이
// 밀리는 경우가 있으면 설정에서 추가한다.
var builtin = map[string]string{
	// 2025
	"2025-01-01": "신정",
	"2025-01-27": "임시공휴일",
	"2025-01-28": "설날 연휴",
	"2025-01-29": "설날",
	"2025-01-30": "설날 연휴",
	"2025-03-01": "삼일절",
	"2025-03-03": "대체공휴일(삼일절)",
	"2025-05-05": "어린이날·부처님오신날",
	"2025-05-06": "대체공휴일(어린이날·부처님오신날)",
	"2025-06-03": "대통령 선거일",
	"2025-06-06": "현충일",
	"2025-08-15": "광복절",
	"2025-10-03": "개천절",
	"2025-10-05": "추석 연휴",
	"2025-10-06": "추석",
	"2025-10-07": "추석 연휴",
	"2025-10-08": "대체공휴일(추석)",
	"2025-10-09": "한글날",
	"2025-12-25": "성탄절",

	// 2026
	"2026-01-01": "신정",
	"2026-02-16": "설날 연휴",
	"2026-02-17": "설날",
	"2026-02-18": "설날 연휴",
	"2026-03-01": "삼일절",
	"2026-03-02": "대체공휴일(삼일절)",
	"2026-05-05": "어린이날",
	"2026-05-24": "부처님오신날",
	"2026-05-25": "대체공휴일(부처님오신날)",
	"2026-06-03": "전국동시지방선거일",
	"2026-06-06": "현충일",
	"2026-08-15": "광복절",
	"2026-08-17": "대체공휴일(광복절)",
	"2026-09-24": "추석 연휴",
	"2026-09-25": "추석",
	"2026-09-26": "추석 연휴",
	"2026-10-03": "개천절",
	"2026-10-05": "대체공휴일(개천절)",
	"2026-10-09": "한글날",
	"2026-12-25": "성탄절",

	// 2027
	"2027-01-01": "신정",
	"2027-02-05": "설날 연휴",
	"2027-02-06": "설날",
	"2027-02-07": "설날 연휴",
	"2027-02-08": "대체공휴일(설날)",
	"2027-03-01": "삼일절",
	"2027-05-05": "어린이날",
	"2027-05-13": "부처님오신날",
	"2027-06-06": "현충일",
	"2027-08-15": "광복절",
	"2027-08-16": "대체공휴일(광복절)",
	"2027-09-14": "추석 연휴",
	"2027-09-15": "추석",
	"2027-09-16": "추석 연휴",
	"2027-10-03": "개천절",
	"2027-10-04": "대체공휴일(개천절)",
	"2027-10-09": "한글날",
	"2027-10-11": "대체공휴일(한글날)",
	"2027-12-25": "성탄절",
	"2027-12-27": "대체공휴일(성탄절)",
}

// coveredYears 는 builtin 표가 온전히(음력·대체공휴일까지) 갖춘 해.
var coveredYears = map[int]bool{2025: true, 2026: true, 2027: true}

// fixedSolar 는 표가 없는 해에도 적용하는 양력 고정 공휴일("MM-DD").
var fixedSolar = map[string]string{
	"01-01": "신정",
	"03-01": "삼일절",
	"05-05": "어린이날",
	"06-06": "현충일",
	"08-15": "광복절",
	"10-03": "개천절",
	"10-09": "한글날",
	"12-25": "성탄절",
}

// Override 는 사용자가 달력에 덧붙이거나 고친 날 하루.
// IsHoliday=true 면 공휴일로 추가(임시공휴일 등), false 면 표에 있어도 업무일로 본다(정정).
type Override struct {
	Date      string `json:"date"` // "YYYY-MM-DD"
	Name      string `json:"name"`
	IsHoliday bool   `json:"isHoliday"`
}

// Calendar 는 내장 표 + 사용자 정정을 합친 달력.
type Calendar struct {
	over map[string]Override
}

// New 는 사용자 정정 목록으로 달력을 만든다. 정정이 없으면 nil 을 넘겨도 된다.
func New(overrides []Override) *Calendar {
	c := &Calendar{over: map[string]Override{}}
	for _, o := range overrides {
		c.over[o.Date] = o
	}
	return c
}

// Covered 는 그 해의 음력 공휴일·대체공휴일까지 내장 표에 있는지.
// false 면 양력 고정 공휴일만 적용되므로 화면에서 알려야 한다.
func Covered(year int) bool { return coveredYears[year] }

// Holiday 는 d 가 공휴일이면 (true, 이름). 사용자 정정이 내장 표보다 우선한다.
func (c *Calendar) Holiday(d time.Time) (bool, string) {
	key := d.Format(layout)
	if o, ok := c.over[key]; ok {
		return o.IsHoliday, o.Name
	}
	if name, ok := builtin[key]; ok {
		return true, name
	}
	if !Covered(d.Year()) {
		if name, ok := fixedSolar[key[5:]]; ok {
			return true, name
		}
	}
	return false, ""
}

// IsBusinessDay 는 주말도 공휴일도 아닌 날인지.
func (c *Calendar) IsBusinessDay(d time.Time) bool {
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	h, _ := c.Holiday(d)
	return !h
}

// NextBusinessDay 는 d 가 업무일이면 d, 아니면 그 뒤 첫 업무일.
// 결제일을 밀다 보면 달이 바뀔 수 있다(예: 31일이 토요일이면 다음 달 2일).
func (c *Calendar) NextBusinessDay(d time.Time) time.Time {
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30 && !c.IsBusinessDay(d); i++ { // 30일 이상 연휴는 없다 — 무한 루프 방지
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// Reason 은 d 가 업무일이 아닌 까닭("토요일", "추석" 등). 업무일이면 "".
func (c *Calendar) Reason(d time.Time) string {
	if h, name := c.Holiday(d); h {
		if name == "" {
			return "공휴일"
		}
		return name
	}
	switch d.Weekday() {
	case time.Saturday:
		return "토요일"
	case time.Sunday:
		return "일요일"
	}
	return ""
}

// Entry 는 화면 표시용 공휴일 한 건.
type Entry struct {
	Date    string `json:"date"`
	Name    string `json:"name"`
	Builtin bool   `json:"builtin"` // 내장 표에서 온 것(false 면 사용자가 추가)
	Off     bool   `json:"off"`     // 내장 공휴일이지만 사용자가 업무일로 정정함
}

// Year 는 그 해의 공휴일 목록(내장 + 사용자 추가, 정정 표시 포함)을 날짜순으로 돌려준다.
func (c *Calendar) Year(year int) []Entry {
	out := []Entry{}
	seen := map[string]bool{}
	add := func(date, name string) {
		e := Entry{Date: date, Name: name, Builtin: true}
		if o, ok := c.over[date]; ok && !o.IsHoliday {
			e.Off = true
		}
		out = append(out, e)
		seen[date] = true
	}
	if Covered(year) {
		for date, name := range builtin {
			if date[:4] == itoa4(year) {
				add(date, name)
			}
		}
	} else {
		for md, name := range fixedSolar {
			add(itoa4(year)+"-"+md, name)
		}
	}
	for date, o := range c.over {
		if date[:4] == itoa4(year) && o.IsHoliday && !seen[date] {
			out = append(out, Entry{Date: date, Name: o.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func itoa4(y int) string {
	return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006")
}
