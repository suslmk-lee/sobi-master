package store

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)

// 구성원 분석: 한 사람(귀속자)의 소비를 요약·추이·습관으로 본다.
//
// 지출을 세 갈래로 나눈다.
//   - 고정비: 고정비 표시 카테고리(주에 켜면 그 부까지) 또는 정기결제에 연결된 거래
//   - 큰 지출: 고정비가 아니면서 건당 bigSpendAmount 이상
//   - 생활 소비: 나머지
//
// 총액은 보험 연납·병원비 같은 한 번의 큰 지출에 크게 흔들리므로 "소비가 늘었나"는 생활 소비로 판단한다.

// bigSpendAmount 는 '큰 지출'로 따로 떼어 보는 건당 금액 기준(원).
const bigSpendAmount = 200000

// memberFrom 은 구성원 분석 집계의 FROM 절: 거래 + 카테고리(부 c, 주 cp) + 정기결제 연결 여부(sp).
const memberFrom = `
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
LEFT JOIN (SELECT DISTINCT transaction_id FROM subscription_posts) sp ON sp.transaction_id = t.id`

// spendKind 는 거래 한 건을 fixed | big | living 으로 나누는 식. linked 는 정기결제 연결 여부 조건.
func spendKind(linked string) string {
	return fmt.Sprintf(`CASE
  WHEN COALESCE(c.fixed_cost, 0) = 1 OR COALESCE(cp.fixed_cost, 0) = 1 OR %s THEN 'fixed'
  WHEN t.amount >= %d THEN 'big'
  ELSE 'living' END`, linked, bigSpendAmount)
}

var (
	// spendKindExpr 는 memberFrom 과 함께 쓴다(sp 조인).
	spendKindExpr = spendKind("sp.transaction_id IS NOT NULL")
	// spendKindTxExpr 는 txSelect(거래 목록)와 함께 쓴다 — sp 조인이 없어 EXISTS 로 본다.
	spendKindTxExpr = spendKind("EXISTS (SELECT 1 FROM subscription_posts sp2 WHERE sp2.transaction_id = t.id)")
)

// 구성원 분석의 항목 기준(월 × 항목 표, 거래 목록 되짚기).
const (
	MemberByCategory = "category" // 주 카테고리
	MemberByMerchant = "merchant" // 가맹점
	MemberByPayment  = "payment"  // 결제수단
	MemberByKind     = "kind"     // fixed | big | living
	MemberByAll      = "all"      // 기간 전체
)

// memberNameExpr 는 기준별 항목 이름 식. 집계와 거래 목록이 같은 식을 써야 금액이 맞는다.
// pmAlias 는 payment_methods 별칭(집계 pm, 거래 목록 p).
func memberNameExpr(by, pmAlias string) string {
	switch by {
	case MemberByMerchant:
		return detailNameExpr(BreakMerchant, pmAlias)
	case MemberByPayment:
		return detailNameExpr(BreakPayment, pmAlias)
	default:
		return nameCategory
	}
}

type MemberChip struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Total int64  `json:"total"` // 기간 지출
}

type MemberMonth struct {
	Month   string `json:"month"` // YYYY-MM
	Living  int64  `json:"living"`
	Fixed   int64  `json:"fixed"`
	Big     int64  `json:"big"`
	Partial bool   `json:"partial"` // 아직 끝나지 않은 달
	// 진행 중인 달이면 월말 예상(생활 소비·전체). 막대 위에 예상분을 덧그려 다른 달과 높이를 견주게 한다.
	ProjLiving int64 `json:"projLiving"`
	ProjTotal  int64 `json:"projTotal"`
}

type MemberCatDelta struct {
	Name    string `json:"name"`    // 주 카테고리
	Current int64  `json:"current"` // 기간 지출
	Avg     int64  `json:"avg"`     // 비교 기간 평균
}

type NamedCount struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
	Count  int    `json:"count"`
}

type MemberBigItem struct {
	ID       int64  `json:"id"`
	Date     string `json:"date"`
	Merchant string `json:"merchant"`
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
}

// MemberAnalysis 는 구성원 분석 화면의 요약·추이·구성 데이터.
type MemberAnalysis struct {
	MemberID  int64  `json:"memberId"`
	Member    string `json:"member"`
	Label     string `json:"label"`     // "10월", "최근 30일"
	PrevLabel string `json:"prevLabel"` // "전월", "전월 같은 날까지", "이전 30일"
	From      string `json:"from"`      // 기간 (거래 목록이 같은 범위를 쓰도록 돌려준다)
	To        string `json:"to"`

	Total       int64 `json:"total"`
	Prev        int64 `json:"prev"`
	Living      int64 `json:"living"`
	TxCount     int   `json:"txCount"`
	LivingCount int   `json:"livingCount"`
	FamilyTotal int64 `json:"familyTotal"` // 같은 기간 가족 전체 지출

	// 직전 3개 같은 길이 기간의 평균(기록이 시작되기 전 기간은 뺀다). AvgWindows 가 0 이면 비교할 기록 없음.
	Avg3       int64 `json:"avg3"`
	LivingAvg3 int64 `json:"livingAvg3"`
	AvgWindows int   `json:"avgWindows"`

	Members []MemberChip  `json:"members"`
	Months  []MemberMonth `json:"months"` // 기준 달까지 6개월

	// 생활 소비 흐름: 끝난 달 중 최근 3개월 평균과 그 앞 3개월 평균(기록이 있는 달만)
	RecentLabel string `json:"recentLabel"`
	RecentAvg   int64  `json:"recentAvg"`
	PriorLabel  string `json:"priorLabel"`
	PriorAvg    int64  `json:"priorAvg"`

	BigItems   []MemberBigItem  `json:"bigItems"`
	CatChanges []MemberCatDelta `json:"catChanges"`
	ByPayment  []NamedCount     `json:"byPayment"`

	// 반복 소비의 기본 카테고리: 최근 90일 이 사람이 가장 자주 쓴 카테고리(0 이면 없음)
	HabitCategoryID int64 `json:"habitCategoryId"`

	// 진행 중인 달(기간=이번 달, 오늘이 말일 전)은 며칠치 표본이라 한 건에 %가 크게 흔들린다.
	// 그래서 % 대신 월말 예상과 '같은 날까지 평소 범위'로 본다.
	InProgress  bool `json:"inProgress"`
	ElapsedDays int  `json:"elapsedDays"` // 기간 시작부터 오늘까지(오늘 포함)
	PeriodDays  int  `json:"periodDays"`  // 기간 전체 일수
	// 월말 예상: 생활 소비는 남은 날을 평소 속도와 이번 달 속도를 섞어 채우고(지난 날이 많을수록 이번 달 쪽),
	// 고정비는 카테고리별로 직전 달들의 중앙값과 이미 빠진 금액 중 큰 값, 큰 지출은 이미 쓴 것만(늘려 잡지 않음).
	ProjLiving int64 `json:"projLiving"`
	ProjFixed  int64 `json:"projFixed"`
	ProjBig    int64 `json:"projBig"`
	ProjTotal  int64 `json:"projTotal"`
	// 직전 3개월 '한 달 전체' 평균 — 월말 예상과 견준다
	FullAvg3       int64 `json:"fullAvg3"`
	FullLivingAvg3 int64 `json:"fullLivingAvg3"`
	// 비교 기간들(진행 중이면 같은 날까지)의 생활 소비 최소~최대 — %보다 덜 흔들리는 '평소 범위'
	RangeLivingMin int64 `json:"rangeLivingMin"`
	RangeLivingMax int64 `json:"rangeLivingMax"`
}

// memberRange 는 분석 기간을 정한다. categoryDetailRange 와 같되, 진행 중인 달(period=month)은
// 비교 기간(전월)을 같은 날까지로 잘라 "지난달 이맘때"와 견준다. 반환하는 end 는 오늘을 넘지 않는다.
func memberRange(year, month int, period string, today time.Time) (r detailRange, end time.Time, sameDay bool) {
	r = categoryDetailRange(year, month, period, today)
	today = dayOf(today)
	end = r.To
	if today.Before(end) && !today.Before(r.From) {
		end = today
		if period != "recent30" {
			sameDay = true
			span := int(end.Sub(r.From).Hours() / 24)
			if cut := r.PrevFrom.AddDate(0, 0, span); cut.Before(r.PrevTo) {
				r.PrevTo = cut
			}
			r.PrevLabel = "전월 같은 날까지"
		}
	}
	return r, end, sameDay
}

// compareWindows 는 '최근 3개월 평균'을 낼 직전 3개 기간. 기간과 같은 길이로 맞춘다.
//   - 달 기준(month|prev): 직전 세 달의 1일부터 같은 일수만큼 (진행 중인 달이면 같은 날까지)
//   - 최근 30일: 그 앞의 30일 세 묶음
//
// dataStart 이전에 끝나는 기간은 기록이 없어 평균을 깎으므로 뺀다.
func compareWindows(r detailRange, period string, end, dataStart time.Time) [][2]time.Time {
	var out [][2]time.Time
	span := int(end.Sub(r.From).Hours() / 24)
	// 기간이 달 전체면 비교도 달 전체 — 일수로 맞추면 30일 달 기준에서 31일 달의 말일이 빠진다
	wholeMonth := !end.Before(r.To)
	for k := 1; k <= 3; k++ {
		var a, b time.Time
		if period == "recent30" {
			a, b = r.From.AddDate(0, 0, -30*k), end.AddDate(0, 0, -30*k)
		} else {
			y, m := r.AnchorY, r.AnchorM
			for i := 0; i < k; i++ {
				y, m = prevMonthOf(y, m)
			}
			a = firstOfMonth(y, m)
			b = a.AddDate(0, 0, span)
			if last := lastOfMonth(y, m); wholeMonth || b.After(last) {
				b = last
			}
		}
		if !dataStart.IsZero() && b.Before(dataStart) {
			continue
		}
		out = append(out, [2]time.Time{a, b})
	}
	return out
}

// trendSplit 는 생활 소비 흐름 판정용으로 끝난 달들을 최근 3개월과 그 앞 3개월로 나눈다.
// months 는 오래된→최신 순, dataStartYM 이전 달(기록 전)은 쓰지 않는다. 각 묶음은 인덱스 목록.
func trendSplit(months []MemberMonth, dataStartYM string) (recent, prior []int) {
	var done []int
	for i, m := range months {
		if !m.Partial && m.Month >= dataStartYM {
			done = append(done, i)
		}
	}
	if len(done) > 3 {
		recent = done[len(done)-3:]
		prior = done[:len(done)-3]
		if len(prior) > 3 {
			prior = prior[len(prior)-3:]
		}
	} else {
		recent = done
	}
	return recent, prior
}

// monthSpanLabel 은 연속한 달 묶음의 표시("8~10월", 한 달이면 "10월").
func monthSpanLabel(months []MemberMonth, idx []int) string {
	if len(idx) == 0 {
		return ""
	}
	first := months[idx[0]].Month
	last := months[idx[len(idx)-1]].Month
	fm, lm := monthNum(first), monthNum(last)
	if first == last {
		return fmt.Sprintf("%d월", lm)
	}
	return fmt.Sprintf("%d~%d월", fm, lm)
}

func monthNum(ym string) int {
	var y, m int
	fmt.Sscanf(ym, "%d-%d", &y, &m)
	return m
}

// spendSums 는 한 사람의 기간 지출을 종류별로 합한다.
type spendSums struct {
	Total, Living, Fixed, Big int64
	Count, LivingCount        int
}

func (s *Store) memberSpend(memberID int64, from, to time.Time) (spendSums, error) {
	var out spendSums
	rows, err := s.query(`
SELECT kind, COUNT(*), SUM(amount) FROM (
  SELECT `+spendKindExpr+` AS kind, t.amount `+memberFrom+`
  WHERE t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?
) x GROUP BY kind`, memberID, fmtDay(from), fmtDay(to))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var n int
		var amt int64
		if err := rows.Scan(&kind, &n, &amt); err != nil {
			return out, err
		}
		out.Total += amt
		out.Count += n
		switch kind {
		case "living":
			out.Living, out.LivingCount = amt, n
		case "fixed":
			out.Fixed = amt
		case "big":
			out.Big = amt
		}
	}
	return out, rows.Err()
}

// memberByMainCat 은 기간 지출을 주 카테고리별로 합한다.
func (s *Store) memberByMainCat(memberID int64, from, to time.Time) (map[string]int64, error) {
	rows, err := s.query(`
SELECT `+nameCategory+`, SUM(t.amount)
FROM transactions t `+joinCategory+`
WHERE t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?
GROUP BY 1`, memberID, fmtDay(from), fmtDay(to))
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

// dataStart 는 기록이 시작된 날(가장 이른 거래일). 거래가 없으면 영.
func (s *Store) dataStart() (time.Time, error) {
	var d sql.NullString
	if err := s.queryRow(`SELECT MIN(date) FROM transactions`).Scan(&d); err != nil {
		return time.Time{}, err
	}
	if !d.Valid {
		return time.Time{}, nil
	}
	t, err := time.Parse(dateLayout, d.String)
	if err != nil {
		return time.Time{}, nil
	}
	return t, nil
}

// MemberAnalysis 는 구성원 한 사람의 기간(period: month|prev|recent30) 분석.
func (s *Store) MemberAnalysis(year, month int, memberID int64, period string, today time.Time) (MemberAnalysis, error) {
	today = dayOf(today)
	r, end, sameDay := memberRange(year, month, period, today)
	a := MemberAnalysis{
		MemberID: memberID, Label: r.Label, PrevLabel: r.PrevLabel,
		From: fmtDay(r.From), To: fmtDay(end),
		Members: []MemberChip{}, Months: []MemberMonth{}, BigItems: []MemberBigItem{},
		CatChanges: []MemberCatDelta{}, ByPayment: []NamedCount{},
	}
	if err := s.queryRow(`SELECT name FROM members WHERE id=?`, memberID).Scan(&a.Member); err != nil {
		if err == sql.ErrNoRows {
			return a, ErrNotFound
		}
		return a, err
	}

	// 구성원 선택 칩: 모든 구성원의 같은 기간 지출
	rows, err := s.query(`
SELECT m.id, m.name, COALESCE(SUM(t.amount), 0)
FROM members m
LEFT JOIN transactions t ON t.member_id = m.id AND t.direction = 'expense' AND t.date >= ? AND t.date <= ?
GROUP BY m.id, m.name ORDER BY m.id`, a.From, a.To)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var c MemberChip
		if err := rows.Scan(&c.ID, &c.Name, &c.Total); err != nil {
			rows.Close()
			return a, err
		}
		a.Members = append(a.Members, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return a, err
	}

	// 요약
	cur, err := s.memberSpend(memberID, r.From, end)
	if err != nil {
		return a, err
	}
	a.Total, a.Living, a.TxCount, a.LivingCount = cur.Total, cur.Living, cur.Count, cur.LivingCount
	prev, err := s.memberSpend(memberID, r.PrevFrom, r.PrevTo)
	if err != nil {
		return a, err
	}
	a.Prev = prev.Total
	if err := s.queryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions
WHERE direction = 'expense' AND date >= ? AND date <= ?`, a.From, a.To).Scan(&a.FamilyTotal); err != nil {
		return a, err
	}

	// 직전 3개 기간 평균 (총액·생활 소비·주 카테고리별)
	start, err := s.dataStart()
	if err != nil {
		return a, err
	}
	windows := compareWindows(r, period, end, start)
	a.AvgWindows = len(windows)
	avgCat := map[string]int64{}
	for i, w := range windows {
		sp, err := s.memberSpend(memberID, w[0], w[1])
		if err != nil {
			return a, err
		}
		a.Avg3 += sp.Total
		a.LivingAvg3 += sp.Living
		if i == 0 || sp.Living < a.RangeLivingMin {
			a.RangeLivingMin = sp.Living
		}
		if sp.Living > a.RangeLivingMax {
			a.RangeLivingMax = sp.Living
		}
		byCat, err := s.memberByMainCat(memberID, w[0], w[1])
		if err != nil {
			return a, err
		}
		for k, v := range byCat {
			avgCat[k] += v
		}
	}
	if n := int64(len(windows)); n > 0 {
		a.Avg3 /= n
		a.LivingAvg3 /= n
		for k := range avgCat {
			avgCat[k] /= n
		}
	}

	// 무엇이 늘었나: 주 카테고리별 기간 vs 평균, 증감이 큰 순
	curCat, err := s.memberByMainCat(memberID, r.From, end)
	if err != nil {
		return a, err
	}
	names := map[string]bool{}
	for k := range curCat {
		names[k] = true
	}
	if len(windows) > 0 {
		for k := range avgCat {
			names[k] = true
		}
	}
	for k := range names {
		if curCat[k] == 0 && avgCat[k] == 0 {
			continue
		}
		a.CatChanges = append(a.CatChanges, MemberCatDelta{Name: k, Current: curCat[k], Avg: avgCat[k]})
	}
	sort.Slice(a.CatChanges, func(i, j int) bool {
		di := abs64(a.CatChanges[i].Current - a.CatChanges[i].Avg)
		dj := abs64(a.CatChanges[j].Current - a.CatChanges[j].Avg)
		if di != dj {
			return di > dj
		}
		return a.CatChanges[i].Name < a.CatChanges[j].Name
	})
	if len(a.CatChanges) > 10 {
		a.CatChanges = a.CatChanges[:10]
	}

	// 월별 추이: 기준 달까지 6개월, 종류별
	const nMonths = 6
	idx := map[string]int{}
	ty, tm := r.AnchorY, r.AnchorM
	a.Months = make([]MemberMonth, nMonths)
	for i := nMonths - 1; i >= 0; i-- {
		ym := fmt.Sprintf("%04d-%02d", ty, tm)
		a.Months[i] = MemberMonth{Month: ym, Partial: monthInProgress(ty, tm, today)}
		idx[ym] = i
		ty, tm = prevMonthOf(ty, tm)
	}
	mrows, err := s.query(`
SELECT ym, kind, SUM(amount) FROM (
  SELECT substr(t.date, 1, 7) AS ym, `+spendKindExpr+` AS kind, t.amount `+memberFrom+`
  WHERE t.direction = 'expense' AND t.member_id = ? AND substr(t.date, 1, 7) >= ? AND substr(t.date, 1, 7) <= ?
) x GROUP BY ym, kind`, memberID, a.Months[0].Month, a.Months[nMonths-1].Month)
	if err != nil {
		return a, err
	}
	for mrows.Next() {
		var ym, kind string
		var amt int64
		if err := mrows.Scan(&ym, &kind, &amt); err != nil {
			mrows.Close()
			return a, err
		}
		i, ok := idx[ym]
		if !ok {
			continue
		}
		switch kind {
		case "living":
			a.Months[i].Living = amt
		case "fixed":
			a.Months[i].Fixed = amt
		case "big":
			a.Months[i].Big = amt
		}
	}
	mrows.Close()
	if err := mrows.Err(); err != nil {
		return a, err
	}
	startYM := ""
	if !start.IsZero() {
		startYM = start.Format("2006-01")
	}
	// 기록이 시작되기 전 달은 0 이 아니라 '모름'이므로 막대에서 뺀다(기준 달은 남긴다)
	for len(a.Months) > 1 && a.Months[0].Month < startYM {
		a.Months = a.Months[1:]
	}
	recent, prior := trendSplit(a.Months, startYM)

	// 진행 중인 달: 월말 예상
	if sameDay {
		if err := s.projectMonth(&a, memberID, r, period, end, start, cur); err != nil {
			return a, err
		}
	}
	a.RecentLabel, a.RecentAvg = monthSpanLabel(a.Months, recent), avgLiving(a.Months, recent)
	a.PriorLabel, a.PriorAvg = monthSpanLabel(a.Months, prior), avgLiving(a.Months, prior)

	// 기간의 큰 지출 (금액순 5건)
	brows, err := s.query(`
SELECT id, date, merchant, cat, amount FROM (
  SELECT t.id, t.date, t.merchant, COALESCE(cp.name || ' > ' || c.name, c.name, '(미분류)') AS cat, t.amount,
         `+spendKindExpr+` AS kind `+memberFrom+`
  WHERE t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?
) x WHERE kind = 'big' ORDER BY amount DESC, date LIMIT 5`, memberID, a.From, a.To)
	if err != nil {
		return a, err
	}
	for brows.Next() {
		var b MemberBigItem
		if err := brows.Scan(&b.ID, &b.Date, &b.Merchant, &b.Category, &b.Amount); err != nil {
			brows.Close()
			return a, err
		}
		a.BigItems = append(a.BigItems, b)
	}
	brows.Close()
	if err := brows.Err(); err != nil {
		return a, err
	}

	// 결제수단별
	prows, err := s.query(`
SELECT `+memberNameExpr(MemberByPayment, "pm")+`, SUM(t.amount), COUNT(*)
FROM transactions t `+joinPayment+`
WHERE t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?
GROUP BY 1 ORDER BY 2 DESC`, memberID, a.From, a.To)
	if err != nil {
		return a, err
	}
	for prows.Next() {
		var n NamedCount
		if err := prows.Scan(&n.Name, &n.Amount, &n.Count); err != nil {
			prows.Close()
			return a, err
		}
		a.ByPayment = append(a.ByPayment, n)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return a, err
	}

	// 반복 소비 기본 카테고리: 최근 90일 가장 자주 쓴 카테고리
	var habit sql.NullInt64
	err = s.queryRow(`
SELECT category_id FROM transactions
WHERE direction = 'expense' AND member_id = ? AND category_id IS NOT NULL AND date >= ? AND date <= ?
GROUP BY category_id ORDER BY COUNT(*) DESC, category_id LIMIT 1`,
		memberID, fmtDay(today.AddDate(0, 0, -89)), fmtDay(today)).Scan(&habit)
	if err != nil && err != sql.ErrNoRows {
		return a, err
	}
	a.HabitCategoryID = habit.Int64
	return a, nil
}

// monthInProgress 는 (y, m) 달이 오늘 기준으로 아직 끝나지 않았는지(오늘이 그 달 1일~말일 전날). 말일이면 끝난 달로 본다.
func monthInProgress(y, m int, today time.Time) bool {
	return !today.Before(firstOfMonth(y, m)) && today.Before(lastOfMonth(y, m))
}

// projectMonth 는 진행 중인 달의 월말 예상과 직전 달들의 한 달 전체 평균을 채운다.
func (s *Store) projectMonth(a *MemberAnalysis, memberID int64, r detailRange, period string,
	end, dataStart time.Time, cur spendSums) error {
	a.InProgress = true
	a.ElapsedDays = dayDiff(r.From, end) + 1
	a.PeriodDays = dayDiff(r.From, r.To) + 1

	// 직전 달들 '전체' (기록 시작 전 달은 뺀다)
	full := compareWindows(r, period, r.To, dataStart)
	var histLiving int64
	histDays := 0
	fixedWins := make([]map[string]int64, 0, len(full))
	for _, w := range full {
		sp, err := s.memberSpend(memberID, w[0], w[1])
		if err != nil {
			return err
		}
		a.FullAvg3 += sp.Total
		a.FullLivingAvg3 += sp.Living
		histLiving += sp.Living
		histDays += dayDiff(w[0], w[1]) + 1
		fc, err := s.memberFixedByCat(memberID, w[0], w[1])
		if err != nil {
			return err
		}
		fixedWins = append(fixedWins, fc)
	}
	if n := int64(len(full)); n > 0 {
		a.FullAvg3 /= n
		a.FullLivingAvg3 /= n
	}
	curFixed, err := s.memberFixedByCat(memberID, r.From, end)
	if err != nil {
		return err
	}
	a.ProjLiving = projectLiving(cur.Living, a.ElapsedDays, a.PeriodDays, histLiving, histDays)
	a.ProjFixed = projectFixed(curFixed, fixedWins)
	a.ProjBig = cur.Big
	a.ProjTotal = a.ProjLiving + a.ProjFixed + a.ProjBig
	if n := len(a.Months); n > 0 && a.Months[n-1].Partial {
		a.Months[n-1].ProjLiving, a.Months[n-1].ProjTotal = a.ProjLiving, a.ProjTotal
	}
	return nil
}

// memberFixedByCat 은 기간의 고정비를 주 카테고리별로 합한다.
func (s *Store) memberFixedByCat(memberID int64, from, to time.Time) (map[string]int64, error) {
	rows, err := s.query(`
SELECT name, SUM(amount) FROM (
  SELECT `+nameCategory+` AS name, t.amount, `+spendKindExpr+` AS kind `+memberFrom+`
  WHERE t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?
) x WHERE kind = 'fixed' GROUP BY name`, memberID, fmtDay(from), fmtDay(to))
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

// projectLiving 은 생활 소비의 월말 예상: 지금까지 + 남은 날 × 하루 속도.
// 하루 속도는 이번 달 속도와 평소 속도(histLiving / histDays)를 지난 날 비율로 섞는다 —
// 월초에는 며칠치 표본보다 평소 속도를 믿고, 월말로 갈수록 이번 달 실제 속도를 믿는다.
// 평소 기록이 없으면 이번 달 속도만 쓴다.
func projectLiving(living int64, elapsed, periodDays int, histLiving int64, histDays int) int64 {
	remaining := periodDays - elapsed
	if remaining <= 0 || elapsed <= 0 {
		return living
	}
	rate := float64(living) / float64(elapsed)
	if histDays > 0 {
		w := float64(elapsed) / float64(periodDays)
		rate = w*rate + (1-w)*float64(histLiving)/float64(histDays)
	}
	return living + int64(math.Round(rate*float64(remaining)))
}

// projectFixed 는 고정비의 월말 예상: 카테고리마다 직전 달들의 중앙값과 이번 달 이미 빠진 금액 중 큰 값.
// 보험 연납처럼 한 달만 튄 금액에 끌려가지 않게 평균 대신 중앙값을 쓴다.
func projectFixed(cur map[string]int64, wins []map[string]int64) int64 {
	names := map[string]bool{}
	for k := range cur {
		names[k] = true
	}
	for _, w := range wins {
		for k := range w {
			names[k] = true
		}
	}
	var sum int64
	for name := range names {
		vals := make([]int64, len(wins))
		for i, w := range wins {
			vals[i] = w[name]
		}
		v := cur[name]
		if m := median64(vals); m > v {
			v = m
		}
		sum += v
	}
	return sum
}

func median64(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]int64(nil), vals...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// dayDiff 는 a 에서 b 까지의 날 수(같은 날이면 0).
func dayDiff(a, b time.Time) int { return int(b.Sub(a).Hours() / 24) }

func avgLiving(months []MemberMonth, idx []int) int64 {
	if len(idx) == 0 {
		return 0
	}
	var sum int64
	for _, i := range idx {
		sum += months[i].Living
	}
	return sum / int64(len(idx))
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---- 반복 소비 ----

type HabitMonth struct {
	Month   string `json:"month"`
	Count   int    `json:"count"`
	Total   int64  `json:"total"`
	Partial bool   `json:"partial"`
}

type HabitPlace struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Total int64  `json:"total"`
	Last  string `json:"last"` // 기간 안 마지막 방문일
	New   bool   `json:"new"`  // 이 사람이 이 카테고리로 처음 간 날이 기간 안
}

// MemberHabit 은 한 사람이 한 카테고리(회사점심 등)에 반복해서 쓰는 모습:
// 월별 횟수·합계(6개월), 기간 안 자주 간 곳, 요일별 횟수(6개월).
type MemberHabit struct {
	CategoryID  int64        `json:"categoryId"`
	Category    string       `json:"category"` // "식비 > 회사점심"
	Months      []HabitMonth `json:"months"`
	Places      []HabitPlace `json:"places"`
	Weekday     []int        `json:"weekday"` // 일~토 7칸
	WeekdayFrom string       `json:"weekdayFrom"`
	WeekdayTo   string       `json:"weekdayTo"`
	// 진행 중인 달이면 오늘까지 횟수와 전월 같은 날까지 횟수 (한 달 전체끼리 견주지 않도록)
	InProgress       bool `json:"inProgress"`
	ElapsedDays      int  `json:"elapsedDays"`
	SameDayCount     int  `json:"sameDayCount"`
	SameDayPrevCount int  `json:"sameDayPrevCount"`
}

func (s *Store) MemberHabit(year, month int, memberID, categoryID int64, period string, today time.Time) (MemberHabit, error) {
	today = dayOf(today)
	r, end, sameDay := memberRange(year, month, period, today)
	h := MemberHabit{CategoryID: categoryID, Months: []HabitMonth{}, Places: []HabitPlace{}, Weekday: make([]int, 7)}

	var parent sql.NullString
	if err := s.queryRow(`
SELECT c.name, p.name FROM categories c LEFT JOIN categories p ON p.id = c.parent_id WHERE c.id = ?`,
		categoryID).Scan(&h.Category, &parent); err != nil {
		if err == sql.ErrNoRows {
			return h, ErrNotFound
		}
		return h, err
	}
	if parent.Valid {
		h.Category = parent.String + " > " + h.Category
	}

	// 월별 횟수·합계 (기준 달까지 6개월)
	const nMonths = 6
	idx := map[string]int{}
	ty, tm := r.AnchorY, r.AnchorM
	h.Months = make([]HabitMonth, nMonths)
	for i := nMonths - 1; i >= 0; i-- {
		ym := fmt.Sprintf("%04d-%02d", ty, tm)
		h.Months[i] = HabitMonth{Month: ym, Partial: monthInProgress(ty, tm, today)}
		idx[ym] = i
		ty, tm = prevMonthOf(ty, tm)
	}
	first, last := h.Months[0].Month, h.Months[nMonths-1].Month
	rows, err := s.query(`
SELECT substr(t.date, 1, 7), COUNT(*), SUM(t.amount) FROM transactions t
WHERE `+catScopeT+` AND t.direction = 'expense' AND t.member_id = ?
  AND substr(t.date, 1, 7) >= ? AND substr(t.date, 1, 7) <= ?
GROUP BY 1`, categoryID, categoryID, memberID, first, last)
	if err != nil {
		return h, err
	}
	for rows.Next() {
		var ym string
		var n int
		var amt int64
		if err := rows.Scan(&ym, &n, &amt); err != nil {
			rows.Close()
			return h, err
		}
		if i, ok := idx[ym]; ok {
			h.Months[i].Count, h.Months[i].Total = n, amt
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return h, err
	}
	if sameDay {
		h.InProgress = true
		h.ElapsedDays = dayDiff(r.From, end) + 1
		count := func(from, to time.Time) (int, error) {
			var n int
			err := s.queryRow(`SELECT COUNT(*) FROM transactions t
WHERE `+catScopeT+` AND t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?`,
				categoryID, categoryID, memberID, fmtDay(from), fmtDay(to)).Scan(&n)
			return n, err
		}
		if h.SameDayCount, err = count(r.From, end); err != nil {
			return h, err
		}
		if h.SameDayPrevCount, err = count(r.PrevFrom, r.PrevTo); err != nil {
			return h, err
		}
	}

	// 기록 시작 전 달은 막대에서 뺀다(MemberAnalysis 와 같은 규칙)
	start, err := s.dataStart()
	if err != nil {
		return h, err
	}
	if !start.IsZero() {
		startYM := start.Format("2006-01")
		for len(h.Months) > 1 && h.Months[0].Month < startYM {
			h.Months = h.Months[1:]
		}
	}

	// 요일별 횟수 (같은 6개월, 오늘까지)
	wFrom := h.Months[0].Month + "-01"
	wTo := fmtDay(lastOfMonth(r.AnchorY, r.AnchorM))
	if t := fmtDay(today); t < wTo {
		wTo = t
	}
	h.WeekdayFrom, h.WeekdayTo = wFrom, wTo
	drows, err := s.query(`
SELECT t.date, COUNT(*) FROM transactions t
WHERE `+catScopeT+` AND t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?
GROUP BY t.date`, categoryID, categoryID, memberID, wFrom, wTo)
	if err != nil {
		return h, err
	}
	for drows.Next() {
		var d string
		var n int
		if err := drows.Scan(&d, &n); err != nil {
			drows.Close()
			return h, err
		}
		if t, err := time.Parse(dateLayout, d); err == nil {
			h.Weekday[int(t.Weekday())] += n
		}
	}
	drows.Close()
	if err := drows.Err(); err != nil {
		return h, err
	}

	// 기간 안 자주 간 곳 + 처음 간 곳 표시
	merchant := memberNameExpr(MemberByMerchant, "")
	firstSeen := map[string]string{}
	frows, err := s.query(`
SELECT `+merchant+`, MIN(t.date) FROM transactions t
WHERE `+catScopeT+` AND t.direction = 'expense' AND t.member_id = ? AND t.date <= ?
GROUP BY 1`, categoryID, categoryID, memberID, fmtDay(end))
	if err != nil {
		return h, err
	}
	for frows.Next() {
		var name, d string
		if err := frows.Scan(&name, &d); err != nil {
			frows.Close()
			return h, err
		}
		firstSeen[name] = d
	}
	frows.Close()
	if err := frows.Err(); err != nil {
		return h, err
	}
	from := fmtDay(r.From)
	prow, err := s.query(`
SELECT `+merchant+`, COUNT(*), SUM(t.amount), MAX(t.date) FROM transactions t
WHERE `+catScopeT+` AND t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?
GROUP BY 1 ORDER BY 2 DESC, 3 DESC LIMIT 15`, categoryID, categoryID, memberID, from, fmtDay(end))
	if err != nil {
		return h, err
	}
	for prow.Next() {
		var p HabitPlace
		if err := prow.Scan(&p.Name, &p.Count, &p.Total, &p.Last); err != nil {
			prow.Close()
			return h, err
		}
		p.New = firstSeen[p.Name] >= from
		h.Places = append(h.Places, p)
	}
	prow.Close()
	return h, prow.Err()
}

// MemberMonthly 는 한 사람의 "월 × 항목" 표(by: category|merchant|payment), 기준 달까지 n개월.
func (s *Store) MemberMonthly(year, month, n int, memberID int64, by string) (CategoryTrend, error) {
	join := joinCategory
	switch by {
	case MemberByPayment:
		join = joinPayment
	case MemberByMerchant:
		join = ""
	}
	out, err := s.seriesTrendWhere(year, month, n, memberNameExpr(by, "pm"), join, 0,
		"t.member_id = ?", []interface{}{memberID})
	if err != nil {
		return out, err
	}
	// 기록 시작 전 달은 빈 칸이 아니라 '모름'이라 표에서 뺀다(월별 추이와 같은 규칙)
	start, err := s.dataStart()
	if err != nil || start.IsZero() {
		return out, err
	}
	startYM := start.Format("2006-01")
	cut := 0
	for cut < len(out.Months)-1 && out.Months[cut] < startYM {
		cut++
	}
	if cut > 0 {
		out.Months = out.Months[cut:]
		for i := range out.Series {
			out.Series[i].Values = out.Series[i].Values[cut:]
		}
	}
	return out, nil
}

// MemberTx 는 구성원 분석의 막대·칸 하나에 해당하는 거래 목록.
// by 는 category|merchant|payment|kind|all, key 는 화면에 찍힌 이름(kind 면 fixed|big|living).
// categoryID > 0 이면 그 카테고리(주면 하위 부 포함)로 더 좁힌다(반복 소비의 가게 목록).
func (s *Store) MemberTx(memberID int64, from, to, by, key string, categoryID int64) ([]Transaction, error) {
	q := txSelect + `WHERE t.direction = 'expense' AND t.member_id = ? AND t.date >= ? AND t.date <= ?`
	args := []interface{}{memberID, from, to}
	switch by {
	case MemberByAll:
	case MemberByKind:
		q += ` AND ` + spendKindTxExpr + ` = ?`
		args = append(args, key)
	default:
		q += ` AND ` + memberNameExpr(by, "p") + ` = ?`
		args = append(args, key)
	}
	if categoryID > 0 {
		q += ` AND ` + catScopeT
		args = append(args, categoryID, categoryID)
	}
	q += ` ORDER BY t.date, t.id`
	return s.scanTxList(q, args...)
}
