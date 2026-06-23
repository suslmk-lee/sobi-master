package store

import (
	"fmt"
	"sort"
	"time"
)

// Series 는 라벨 1개에 대한 숫자 시계열(일별·월별 공용).
// Values 의 의미(일/월)는 호출 맥락(DailyByDimension/CategoryTrend)이 정한다.
type Series struct {
	Name   string  `json:"name"`
	Color  string  `json:"color"` // 지정 색(없으면 빈 문자열 → 프론트가 팔레트로 대체)
	Total  int64   `json:"total"`
	Values []int64 `json:"values"`
}

// topNSeries 는 합계 내림차순으로 정렬된 series 에서 상위 n개만 남기고
// 나머지를 "기타" 한 줄로 합친다. length 는 각 Values 의 길이.
func topNSeries(series []Series, n, length int) []Series {
	if len(series) <= n {
		return series
	}
	rest := Series{Name: "기타", Values: make([]int64, length)}
	for _, s := range series[n:] {
		rest.Total += s.Total
		for i := range s.Values {
			rest.Values[i] += s.Values[i]
		}
	}
	out := make([]Series, 0, n+1)
	out = append(out, series[:n]...)
	out = append(out, rest)
	return out
}

// DailyByDimension 은 통계 탭의 멀티라인 일별 차트용 응답.
// 각 Series 의 Values 길이는 Days (index 0 = 1일).
type DailyByDimension struct {
	Days   int      `json:"days"`
	Series []Series `json:"series"`
}

// DailyByDimension 은 해당 월의 일별 지출을 차원(paymentMethod|member|category)별로
// 분해해 돌려준다. 상위 7개 + 기타로 묶는다.
func (s *Store) DailyByDimension(year, month int, dimension string) (DailyByDimension, error) {
	days := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	out := DailyByDimension{Days: days, Series: []Series{}}

	var nameExpr, colorExpr, join string
	switch dimension {
	case "member":
		nameExpr = "COALESCE(m.name, '(미지정)')"
		colorExpr = "''"
		join = "LEFT JOIN members m ON m.id = t.member_id"
	case "category":
		nameExpr = "COALESCE(c.name, '(미분류)')"
		colorExpr = "''"
		join = "LEFT JOIN categories c ON c.id = t.category_id"
	default: // paymentMethod
		nameExpr = "COALESCE(pm.name, '(미지정)')"
		colorExpr = "COALESCE(pm.color, '')"
		join = "LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id"
	}

	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"
	// nameExpr/colorExpr/join 은 위 switch 의 고정 문자열뿐이라 주입 위험 없음.
	q := fmt.Sprintf(`
SELECT %s AS name, %s AS color, CAST(substr(t.date,9,2) AS INTEGER) AS day, SUM(t.amount)
FROM transactions t %s
WHERE t.date LIKE ? AND t.direction='expense'
GROUP BY 1, 2, 3`, nameExpr, colorExpr, join)

	rows, err := s.query(q, prefix)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	type acc struct {
		color string
		total int64
		daily []int64
	}
	byName := map[string]*acc{}
	for rows.Next() {
		var name, color string
		var day int
		var amt int64
		if err := rows.Scan(&name, &color, &day, &amt); err != nil {
			return out, err
		}
		a := byName[name]
		if a == nil {
			a = &acc{daily: make([]int64, days)}
			byName[name] = a
		}
		if a.color == "" && color != "" {
			a.color = color
		}
		if day >= 1 && day <= days {
			a.daily[day-1] += amt
		}
		a.total += amt
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	series := make([]Series, 0, len(byName))
	for name, a := range byName {
		series = append(series, Series{Name: name, Color: a.color, Total: a.total, Values: a.daily})
	}
	sort.Slice(series, func(i, j int) bool {
		if series[i].Total != series[j].Total {
			return series[i].Total > series[j].Total
		}
		return series[i].Name < series[j].Name
	})
	out.Series = topNSeries(series, 7, days)
	return out, nil
}

// YearSummary 는 한 해의 월별 추이 + 연 합계 + 전년 대비 + 카테고리별 지출.
type YearSummary struct {
	Year          int           `json:"year"`
	Months        []MonthPoint  `json:"months"` // 1월~12월
	TotalIncome   int64         `json:"totalIncome"`
	TotalExpense  int64         `json:"totalExpense"`
	TotalTransfer int64         `json:"totalTransfer"`
	PrevIncome    int64         `json:"prevIncome"`
	PrevExpense   int64         `json:"prevExpense"`
	PrevTransfer  int64         `json:"prevTransfer"`
	ByCategory    []NamedAmount `json:"byCategory"` // 연간 지출 카테고리별(내림차순)
}

// yearTotals 는 한 해의 수입/지출/이체 합계.
func (s *Store) yearTotals(year int) (inc, exp, tr int64, err error) {
	prefix := fmt.Sprintf("%04d", year) + "-%"
	err = s.queryRow(`
SELECT
	COALESCE(SUM(CASE WHEN direction='income'   THEN amount END), 0),
	COALESCE(SUM(CASE WHEN direction='expense'  THEN amount END), 0),
	COALESCE(SUM(CASE WHEN direction='transfer' THEN amount END), 0)
FROM transactions WHERE date LIKE ?`, prefix).Scan(&inc, &exp, &tr)
	return
}

// YearSummary 는 연간 보기용 집계.
func (s *Store) YearSummary(year int) (YearSummary, error) {
	out := YearSummary{Year: year, Months: make([]MonthPoint, 12), ByCategory: []NamedAmount{}}
	for i := 0; i < 12; i++ {
		out.Months[i] = MonthPoint{Month: fmt.Sprintf("%04d-%02d", year, i+1)}
	}

	prefix := fmt.Sprintf("%04d", year) + "-%"
	rows, err := s.query(`
SELECT CAST(substr(date,6,2) AS INTEGER), direction, SUM(amount)
FROM transactions WHERE date LIKE ? GROUP BY 1, 2`, prefix)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var mo int
		var dir string
		var amt int64
		if err := rows.Scan(&mo, &dir, &amt); err != nil {
			return out, err
		}
		if mo < 1 || mo > 12 {
			continue
		}
		switch dir {
		case "income":
			out.Months[mo-1].Income = amt
			out.TotalIncome += amt
		case "expense":
			out.Months[mo-1].Expense = amt
			out.TotalExpense += amt
		case "transfer":
			out.Months[mo-1].Transfer = amt
			out.TotalTransfer += amt
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	pi, pe, pt, err := s.yearTotals(year - 1)
	if err != nil {
		return out, err
	}
	out.PrevIncome, out.PrevExpense, out.PrevTransfer = pi, pe, pt

	crows, err := s.query(`
SELECT COALESCE(c.name,'(미분류)'), SUM(t.amount)
FROM transactions t LEFT JOIN categories c ON c.id = t.category_id
WHERE t.date LIKE ? AND t.direction='expense'
GROUP BY 1 ORDER BY SUM(t.amount) DESC`, prefix)
	if err != nil {
		return out, err
	}
	defer crows.Close()
	for crows.Next() {
		var na NamedAmount
		na.Kind = "expense"
		if err := crows.Scan(&na.Name, &na.Amount); err != nil {
			return out, err
		}
		out.ByCategory = append(out.ByCategory, na)
	}
	return out, crows.Err()
}

// CardCategory 는 결제수단 한 개의 카테고리별 지출 구성(적층 막대용).
type CardCategory struct {
	Card       string        `json:"card"`
	Color      string        `json:"color"`
	Total      int64         `json:"total"`
	Categories []NamedAmount `json:"categories"` // 금액 내림차순
}

// groupCategoryBreakdown 은 해당 월의 지출을 한 차원(nameExpr/colorExpr/join)별로,
// 그 안에서 카테고리별로 집계한다. 그룹은 총액 내림차순, 각 카테고리도 금액 내림차순.
// nameExpr/colorExpr/join 은 호출부의 고정 문자열뿐이라 주입 위험 없음.
func (s *Store) groupCategoryBreakdown(year, month int, nameExpr, colorExpr, join string) ([]CardCategory, error) {
	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"
	q := fmt.Sprintf(`
SELECT %s, %s, COALESCE(c.name,'(미분류)'), SUM(t.amount)
FROM transactions t
%s
LEFT JOIN categories c ON c.id = t.category_id
WHERE t.date LIKE ? AND t.direction='expense'
GROUP BY 1, 2, 3`, nameExpr, colorExpr, join)
	rows, err := s.query(q, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type acc struct {
		color string
		total int64
		cats  []NamedAmount
	}
	order := []string{}
	byGroup := map[string]*acc{}
	for rows.Next() {
		var name, color, cat string
		var amt int64
		if err := rows.Scan(&name, &color, &cat, &amt); err != nil {
			return nil, err
		}
		a := byGroup[name]
		if a == nil {
			a = &acc{color: color}
			byGroup[name] = a
			order = append(order, name)
		}
		if a.color == "" && color != "" {
			a.color = color
		}
		a.total += amt
		a.cats = append(a.cats, NamedAmount{Name: cat, Kind: "expense", Amount: amt})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]CardCategory, 0, len(byGroup))
	for _, name := range order {
		a := byGroup[name]
		sort.Slice(a.cats, func(i, j int) bool {
			if a.cats[i].Amount != a.cats[j].Amount {
				return a.cats[i].Amount > a.cats[j].Amount
			}
			return a.cats[i].Name < a.cats[j].Name
		})
		out = append(out, CardCategory{Card: name, Color: a.color, Total: a.total, Categories: a.cats})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Card < out[j].Card
	})
	return out, nil
}

// CardCategoryBreakdown 은 해당 월의 지출을 결제수단별 카테고리 구성으로 집계한다.
func (s *Store) CardCategoryBreakdown(year, month int) ([]CardCategory, error) {
	return s.groupCategoryBreakdown(year, month,
		"COALESCE(pm.name,'(미지정)')", "COALESCE(pm.color,'')",
		"LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id")
}

// MemberCategoryBreakdown 은 해당 월의 지출을 귀속자별 카테고리 구성으로 집계한다.
func (s *Store) MemberCategoryBreakdown(year, month int) ([]CardCategory, error) {
	return s.groupCategoryBreakdown(year, month,
		"COALESCE(m.name,'(미지정)')", "''",
		"LEFT JOIN members m ON m.id = t.member_id")
}

// CardPace 는 카드 한 장의 현재 실적기간 누적 사용 추이(목표 대비 페이스).
type CardPace struct {
	Card        PaymentMethod `json:"card"`
	PeriodStart string        `json:"periodStart"`
	PeriodEnd   string        `json:"periodEnd"`
	Target      int64         `json:"target"`
	Days        int           `json:"days"`       // 실적기간 총 일수
	Elapsed     int           `json:"elapsed"`    // 시작일~오늘 경과 일수(1-based, 기간 종료 후엔 Days)
	Spent       int64         `json:"spent"`      // 현재까지 누적
	Projected   int64         `json:"projected"`  // 현재 페이스 기준 기간말 예상액
	Achieved    bool          `json:"achieved"`   // 현재 누적이 이미 목표 충족
	OnTrack     bool          `json:"onTrack"`    // 예상액이 목표 이상(달성 페이스)
	Cumulative  []int64       `json:"cumulative"` // 1일차~경과일 일별 누적(length = Elapsed)
}

// dailyInRange 는 결제수단 한 개의 [start, end] 기간 내 일별 지출을
// 기간 시작일 기준 오프셋 배열(index 0 = start 당일)로 돌려준다.
func (s *Store) dailyInRange(pmID int64, start, end time.Time) ([]int64, error) {
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	daily := make([]int64, days)
	rows, err := s.query(`
SELECT date, SUM(amount) FROM transactions
WHERE payment_method_id=? AND direction='expense' AND date >= ? AND date <= ?
GROUP BY date`, pmID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dstr string
		var amt int64
		if err := rows.Scan(&dstr, &amt); err != nil {
			return nil, err
		}
		d, perr := time.ParseInLocation("2006-01-02", dstr, start.Location())
		if perr != nil {
			continue
		}
		off := int(d.Sub(start).Hours() / 24)
		if off >= 0 && off < days {
			daily[off] += amt
		}
	}
	return daily, rows.Err()
}

// CardPaces 는 실적 목표(perf_target>0)가 설정된 카드 각각의 현재 실적기간
// 누적 사용 추이와 목표 대비 페이스를 돌려준다.
func (s *Store) CardPaces(today time.Time) ([]CardPace, error) {
	pms, err := s.ListPaymentMethods()
	if err != nil {
		return nil, err
	}
	out := []CardPace{}
	for _, pm := range pms {
		if pm.Type != "card" || pm.PerfTarget <= 0 {
			continue
		}
		start, end := perfPeriod(today, pm.CycleStartDay)
		daily, err := s.dailyInRange(pm.ID, start, end)
		if err != nil {
			return nil, err
		}
		days := len(daily)
		elapsed := int(today.Sub(start).Hours()/24) + 1
		if elapsed < 0 {
			elapsed = 0
		}
		if elapsed > days {
			elapsed = days
		}

		cum := make([]int64, elapsed)
		var run int64
		for i := 0; i < elapsed; i++ {
			run += daily[i]
			cum[i] = run
		}
		var projected int64
		if elapsed > 0 {
			projected = run * int64(days) / int64(elapsed)
		}
		out = append(out, CardPace{
			Card:        pm,
			PeriodStart: start.Format("2006-01-02"),
			PeriodEnd:   end.Format("2006-01-02"),
			Target:      pm.PerfTarget,
			Days:        days,
			Elapsed:     elapsed,
			Spent:       run,
			Projected:   projected,
			Achieved:    run >= pm.PerfTarget,
			OnTrack:     projected >= pm.PerfTarget,
			Cumulative:  cum,
		})
	}
	return out, nil
}

// CumCompare 는 이번 달과 전월의 일별 누적 지출 비교.
type CumCompare struct {
	Days       int     `json:"days"`       // 비교 x축 길이 = max(이번달 말일, 전월 말일)
	CompareDay int     `json:"compareDay"` // 동일시점 기준일(현재 달이면 오늘, 아니면 말일)
	CurMonth   string  `json:"curMonth"`   // YYYY-MM
	PrevMonth  string  `json:"prevMonth"`
	Current    []int64 `json:"current"`  // 이번 달 일별 누적(length Days, 말일 이후 마지막값 유지)
	Previous   []int64 `json:"previous"` // 전월 일별 누적(length Days)
}

// cumulativeFill 은 일별 금액을 누적으로 바꾸고, points 길이를 넘는 날은 마지막 누적값을 유지한다.
func cumulativeFill(points []DayPoint, length int) []int64 {
	out := make([]int64, length)
	var run int64
	for i := 0; i < length; i++ {
		if i < len(points) {
			run += points[i].Amount
		}
		out[i] = run
	}
	return out
}

// CumulativeCompare 는 (year, month) 와 그 전월의 일별 누적 지출을 같은 길이로 맞춰 돌려준다.
func (s *Store) CumulativeCompare(year, month int, today time.Time) (CumCompare, error) {
	cur, err := s.DailyExpenses(year, month)
	if err != nil {
		return CumCompare{}, err
	}
	py, pm := year, month-1
	if pm == 0 {
		py, pm = year-1, 12
	}
	prev, err := s.DailyExpenses(py, pm)
	if err != nil {
		return CumCompare{}, err
	}

	curLast, prevLast := len(cur), len(prev)
	days := curLast
	if prevLast > days {
		days = prevLast
	}

	compareDay := curLast
	if year == today.Year() && month == int(today.Month()) {
		if today.Day() < compareDay {
			compareDay = today.Day()
		}
	}

	return CumCompare{
		Days:       days,
		CompareDay: compareDay,
		CurMonth:   fmt.Sprintf("%04d-%02d", year, month),
		PrevMonth:  fmt.Sprintf("%04d-%02d", py, pm),
		Current:    cumulativeFill(cur, days),
		Previous:   cumulativeFill(prev, days),
	}, nil
}

// WeekdayAvg 는 요일별 평균 지출. Avg 는 그 요일이 해당 월에 등장한 날 수(Count)로 나눈 값.
type WeekdayAvg struct {
	Weekday int   `json:"weekday"` // 0=일 ... 6=토
	Total   int64 `json:"total"`
	Count   int   `json:"count"`
	Avg     int64 `json:"avg"`
}

// WeekdayAverages 는 해당 월의 요일별 평균 지출을 돌려준다(일~토 7칸 고정).
func (s *Store) WeekdayAverages(year, month int) ([]WeekdayAvg, error) {
	out := make([]WeekdayAvg, 7)
	for i := range out {
		out[i].Weekday = i
	}

	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	for d := 0; d < lastDay; d++ {
		wd := int(first.AddDate(0, 0, d).Weekday())
		out[wd].Count++
	}

	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"
	rows, err := s.query(`
SELECT EXTRACT(DOW FROM date::date)::int AS dow, SUM(amount)
FROM transactions
WHERE date LIKE ? AND direction='expense'
GROUP BY 1`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dow int
		var amt int64
		if err := rows.Scan(&dow, &amt); err != nil {
			return nil, err
		}
		if dow >= 0 && dow <= 6 {
			out[dow].Total = amt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Count > 0 {
			out[i].Avg = out[i].Total / int64(out[i].Count)
		}
	}
	return out, nil
}

// CategoryTrend 는 (year, month)를 마지막으로 하는 최근 n개월의 카테고리별 월별 지출 추이.
type CategoryTrend struct {
	Months []string `json:"months"` // ["2026-01", ...] 오래된→최신
	Series []Series `json:"series"` // 상위 카테고리, 각 Values 길이 = len(Months)
}

// seriesTrend 는 최근 n개월의 한 차원(nameExpr/join)별 월별 지출 추이를 돌려준다.
func (s *Store) seriesTrend(year, month, n int, nameExpr, join string) (CategoryTrend, error) {
	if n < 1 {
		n = 6
	}
	months := make([]string, n)
	idx := map[string]int{}
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	for i := n - 1; i >= 0; i-- {
		key := t.Format("2006-01")
		months[i] = key
		idx[key] = i
		t = t.AddDate(0, -1, 0)
	}
	out := CategoryTrend{Months: months, Series: []Series{}}
	first, last := months[0], months[n-1]

	q := fmt.Sprintf(`
SELECT %s AS name, substr(t.date,1,7) AS ym, SUM(t.amount)
FROM transactions t %s
WHERE substr(t.date,1,7) >= ? AND substr(t.date,1,7) <= ? AND t.direction='expense'
GROUP BY 1, 2`, nameExpr, join)
	rows, err := s.query(q, first, last)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	type acc struct {
		total   int64
		monthly []int64
	}
	byName := map[string]*acc{}
	for rows.Next() {
		var name, ym string
		var amt int64
		if err := rows.Scan(&name, &ym, &amt); err != nil {
			return out, err
		}
		a := byName[name]
		if a == nil {
			a = &acc{monthly: make([]int64, n)}
			byName[name] = a
		}
		if i, ok := idx[ym]; ok {
			a.monthly[i] += amt
			a.total += amt
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	series := make([]Series, 0, len(byName))
	for name, a := range byName {
		series = append(series, Series{Name: name, Total: a.total, Values: a.monthly})
	}
	sort.Slice(series, func(i, j int) bool {
		if series[i].Total != series[j].Total {
			return series[i].Total > series[j].Total
		}
		return series[i].Name < series[j].Name
	})
	out.Series = topNSeries(series, 6, n)
	return out, nil
}

// CategoryTrend 는 상위 6개 카테고리 + 기타의 최근 n개월 월별 지출 추이.
func (s *Store) CategoryTrend(year, month, n int) (CategoryTrend, error) {
	return s.seriesTrend(year, month, n,
		"COALESCE(c.name,'(미분류)')", "LEFT JOIN categories c ON c.id = t.category_id")
}

// MemberTrend 는 귀속자별 최근 n개월 월별 지출 추이.
func (s *Store) MemberTrend(year, month, n int) (CategoryTrend, error) {
	return s.seriesTrend(year, month, n,
		"COALESCE(m.name,'(미지정)')", "LEFT JOIN members m ON m.id = t.member_id")
}

// MemberStat 은 귀속자 한 명의 해당 월 지출 요약.
type MemberStat struct {
	Member            string `json:"member"`
	Total             int64  `json:"total"`
	Prev              int64  `json:"prev"`  // 전월 지출
	Share             int    `json:"share"` // 이번 달 총지출 대비 비중(%)
	TopCategory       string `json:"topCategory"`
	TopCategoryAmount int64  `json:"topCategoryAmount"`
	TopMerchant       string `json:"topMerchant"`
	TopMerchantAmount int64  `json:"topMerchantAmount"`
}

// memberTotals 는 해당 월 귀속자별 지출 합계 맵.
func (s *Store) memberTotals(year, month int) (map[string]int64, error) {
	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"
	rows, err := s.query(`
SELECT COALESCE(m.name,'(미지정)'), SUM(t.amount)
FROM transactions t LEFT JOIN members m ON m.id = t.member_id
WHERE t.date LIKE ? AND t.direction='expense'
GROUP BY 1`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var name string
		var amt int64
		if err := rows.Scan(&name, &amt); err != nil {
			return nil, err
		}
		out[name] = amt
	}
	return out, rows.Err()
}

// MemberStats 는 귀속자별 지출 요약(총액·비중·전월 대비·최다 카테고리/가맹점)을 총액순으로 돌려준다.
func (s *Store) MemberStats(year, month int) ([]MemberStat, error) {
	mcb, err := s.MemberCategoryBreakdown(year, month)
	if err != nil {
		return nil, err
	}
	py, pm := year, month-1
	if pm == 0 {
		py, pm = year-1, 12
	}
	prev, err := s.memberTotals(py, pm)
	if err != nil {
		return nil, err
	}

	// 귀속자별 최다 가맹점
	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"
	mrows, err := s.query(`
SELECT COALESCE(m.name,'(미지정)'),
       CASE WHEN t.merchant='' THEN '(내용 없음)' ELSE t.merchant END, SUM(t.amount)
FROM transactions t LEFT JOIN members m ON m.id = t.member_id
WHERE t.date LIKE ? AND t.direction='expense'
GROUP BY 1, 2`, prefix)
	if err != nil {
		return nil, err
	}
	defer mrows.Close()
	type ma struct {
		name string
		amt  int64
	}
	topMerchant := map[string]ma{}
	for mrows.Next() {
		var member, merchant string
		var amt int64
		if err := mrows.Scan(&member, &merchant, &amt); err != nil {
			return nil, err
		}
		if cur, ok := topMerchant[member]; !ok || amt > cur.amt {
			topMerchant[member] = ma{merchant, amt}
		}
	}
	if err := mrows.Err(); err != nil {
		return nil, err
	}

	var grand int64
	for _, g := range mcb {
		grand += g.Total
	}

	out := make([]MemberStat, 0, len(mcb))
	for _, g := range mcb {
		st := MemberStat{Member: g.Card, Total: g.Total, Prev: prev[g.Card]}
		if len(g.Categories) > 0 {
			st.TopCategory = g.Categories[0].Name
			st.TopCategoryAmount = g.Categories[0].Amount
		}
		if tm, ok := topMerchant[g.Card]; ok {
			st.TopMerchant = tm.name
			st.TopMerchantAmount = tm.amt
		}
		if grand > 0 {
			st.Share = int(g.Total * 100 / grand)
		}
		out = append(out, st)
	}
	return out, nil
}
