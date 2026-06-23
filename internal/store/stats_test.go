package store

import "testing"

// findSeries 는 이름으로 시리즈 한 개를 찾는다(없으면 nil).
func findSeries(series []Series, name string) *Series {
	for i := range series {
		if series[i].Name == name {
			return &series[i]
		}
	}
	return nil
}

func findCatID(cats []Category, name string) int64 {
	for _, c := range cats {
		if c.Name == name {
			return c.ID
		}
	}
	return 0
}

func TestTopNSeriesMergesRest(t *testing.T) {
	series := []Series{
		{Name: "a", Total: 100, Values: []int64{100, 0}},
		{Name: "b", Total: 90, Values: []int64{40, 50}},
		{Name: "c", Total: 30, Values: []int64{10, 20}},
		{Name: "d", Total: 20, Values: []int64{20, 0}},
	}
	out := topNSeries(series, 2, 2)
	if len(out) != 3 {
		t.Fatalf("상위 2개 + 기타 = 3개 기대, 실제 %d개", len(out))
	}
	if out[2].Name != "기타" {
		t.Fatalf("마지막은 기타여야 함, 실제 %q", out[2].Name)
	}
	if out[2].Total != 50 {
		t.Fatalf("기타 합계 50(=30+20) 기대, 실제 %d", out[2].Total)
	}
	if out[2].Values[0] != 30 || out[2].Values[1] != 20 {
		t.Fatalf("기타 일별값 [30,20] 기대, 실제 %v", out[2].Values)
	}
}

func TestTopNSeriesNoMergeWhenFew(t *testing.T) {
	series := []Series{{Name: "a", Total: 1, Values: []int64{1}}}
	out := topNSeries(series, 7, 1)
	if len(out) != 1 || out[0].Name != "a" {
		t.Fatalf("개수가 n 이하면 그대로 반환해야 함, 실제 %v", out)
	}
}

func TestCumulativeFillHoldsLast(t *testing.T) {
	pts := []DayPoint{{Day: 1, Amount: 10}, {Day: 2, Amount: 5}, {Day: 3, Amount: 0}}
	got := cumulativeFill(pts, 5)
	want := []int64{10, 15, 15, 15, 15} // 3일까지 누적, 이후 마지막값 유지
	if len(got) != len(want) {
		t.Fatalf("길이 %d 기대, 실제 %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: %d 기대, 실제 %d (전체 %v)", i, want[i], got[i], got)
		}
	}
}

// TestStatsQueries 는 통계 쿼리 5종을 실제 Postgres 에서 실행해 SQL 유효성과
// 핵심 집계값을 검증한다. (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestStatsQueries(t *testing.T) {
	st := openTestStore(t)

	card, err := st.SavePaymentMethod(PaymentMethod{
		Name: "테스트카드", Type: "card", Issuer: "신한카드",
		CycleStartDay: 1, PerfTarget: 300000, Color: "#123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	cats, _ := st.ListCategories()
	mems, _ := st.ListMembers()
	food := findCatID(cats, "식비")
	tele := findCatID(cats, "통신비")
	dad := mems[0].ID

	add := func(d string, amt, catID int64) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{
			Date: d, Amount: amt, Direction: "expense", Merchant: "가맹점",
			PaymentMethodID: &card.ID, CategoryID: &catID, MemberID: &dad,
		}); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-06-01", 50000, food)
	add("2026-06-02", 100000, tele)
	add("2026-06-10", 70000, food)
	add("2026-05-01", 30000, food) // 전월/추이용

	const y, m = 2026, 6

	// 1) 일별 차원별 — 결제수단
	pmD, err := st.DailyByDimension(y, m, "paymentMethod")
	if err != nil {
		t.Fatalf("DailyByDimension(paymentMethod): %v", err)
	}
	if pmD.Days != 30 {
		t.Errorf("6월 Days=%d, want 30", pmD.Days)
	}
	s := findSeries(pmD.Series, "테스트카드")
	if s == nil {
		t.Fatalf("테스트카드 시리즈 없음: %+v", pmD.Series)
	}
	if s.Color != "#123456" {
		t.Errorf("색 %q, want #123456", s.Color)
	}
	if s.Values[0] != 50000 || s.Values[1] != 100000 || s.Values[9] != 70000 {
		t.Errorf("일별값 오류: 1일=%d 2일=%d 10일=%d", s.Values[0], s.Values[1], s.Values[9])
	}
	if s.Total != 220000 {
		t.Errorf("Total=%d, want 220000", s.Total)
	}
	// 카테고리/귀속자 차원도 실행만 검증
	if _, err := st.DailyByDimension(y, m, "member"); err != nil {
		t.Fatalf("DailyByDimension(member): %v", err)
	}
	if _, err := st.DailyByDimension(y, m, "category"); err != nil {
		t.Fatalf("DailyByDimension(category): %v", err)
	}

	// 2) 카드 실적기간 누적 진행
	paces, err := st.CardPaces(date("2026-06-15"))
	if err != nil {
		t.Fatalf("CardPaces: %v", err)
	}
	if len(paces) != 1 {
		t.Fatalf("페이스 카드 수=%d, want 1", len(paces))
	}
	p := paces[0]
	if p.Spent != 220000 || p.Days != 30 || p.Elapsed != 15 {
		t.Errorf("Spent=%d Days=%d Elapsed=%d, want 220000/30/15", p.Spent, p.Days, p.Elapsed)
	}
	if p.Projected != 440000 || p.Achieved || !p.OnTrack {
		t.Errorf("Projected=%d Achieved=%v OnTrack=%v, want 440000/false/true", p.Projected, p.Achieved, p.OnTrack)
	}

	// 3) 전월 동일시점 누적 비교
	cmp, err := st.CumulativeCompare(y, m, date("2026-06-15"))
	if err != nil {
		t.Fatalf("CumulativeCompare: %v", err)
	}
	if cmp.CompareDay != 15 {
		t.Errorf("CompareDay=%d, want 15", cmp.CompareDay)
	}
	if cmp.Current[14] != 220000 {
		t.Errorf("이번달 15일 누적=%d, want 220000", cmp.Current[14])
	}
	if cmp.Previous[14] != 30000 {
		t.Errorf("전월 15일 누적=%d, want 30000", cmp.Previous[14])
	}

	// 4) 요일별 평균 지출
	wd, err := st.WeekdayAverages(y, m)
	if err != nil {
		t.Fatalf("WeekdayAverages: %v", err)
	}
	if len(wd) != 7 {
		t.Fatalf("요일 칸 수=%d, want 7", len(wd))
	}
	var wdTotal int64
	for _, w := range wd {
		wdTotal += w.Total
	}
	if wdTotal != 220000 {
		t.Errorf("요일 합계=%d, want 220000", wdTotal)
	}

	// 5) 카테고리별 6개월 추이
	ct, err := st.CategoryTrend(y, m, 6)
	if err != nil {
		t.Fatalf("CategoryTrend: %v", err)
	}
	if len(ct.Months) != 6 || ct.Months[5] != "2026-06" {
		t.Errorf("Months=%v, 마지막 want 2026-06", ct.Months)
	}
	cs := findSeries(ct.Series, "식비")
	if cs == nil {
		t.Fatalf("식비 시리즈 없음: %+v", ct.Series)
	}
	if cs.Values[5] != 120000 || cs.Values[4] != 30000 {
		t.Errorf("식비 6월=%d 5월=%d, want 120000/30000", cs.Values[5], cs.Values[4])
	}

	// 6) 카드별 카테고리 구성
	cardCat, err := st.CardCategoryBreakdown(y, m)
	if err != nil {
		t.Fatalf("CardCategoryBreakdown: %v", err)
	}
	if len(cardCat) != 1 {
		t.Fatalf("결제수단 수=%d, want 1(테스트카드)", len(cardCat))
	}
	cc := cardCat[0]
	if cc.Card != "테스트카드" || cc.Total != 220000 {
		t.Errorf("Card=%q Total=%d, want 테스트카드/220000", cc.Card, cc.Total)
	}
	if cc.Color != "#123456" {
		t.Errorf("Color=%q, want #123456", cc.Color)
	}
	// 카테고리 내림차순: 통신비(100000) > 식비(50000+70000=120000)? → 식비가 더 큼
	if len(cc.Categories) != 2 || cc.Categories[0].Name != "식비" || cc.Categories[0].Amount != 120000 {
		t.Errorf("카테고리 1순위 식비/120000 기대, 실제 %+v", cc.Categories)
	}

	// 7) 예산: 식비 예산 10만 → 6월 지출 12만이므로 초과
	if err := st.SetBudget(food, "*", 100000); err != nil {
		t.Fatal(err)
	}
	bsts, err := st.BudgetStatuses(y, m)
	if err != nil {
		t.Fatalf("BudgetStatuses: %v", err)
	}
	if len(bsts) != 1 {
		t.Fatalf("예산 수=%d, want 1", len(bsts))
	}
	if bsts[0].Spent != 120000 || !bsts[0].Over || bsts[0].Remaining != -20000 || bsts[0].Override {
		t.Errorf("예산 현황 오류: %+v", bsts[0])
	}

	// 7-b) 6월 전용 예산 20만 → 오버라이드 우선, 더 이상 초과 아님
	if err := st.SetBudget(food, "2026-06", 200000); err != nil {
		t.Fatal(err)
	}
	bsts, err = st.BudgetStatuses(y, m)
	if err != nil {
		t.Fatalf("BudgetStatuses(override): %v", err)
	}
	if bsts[0].Amount != 200000 || bsts[0].Over || !bsts[0].Override {
		t.Errorf("월 전용 예산 우선 실패: %+v", bsts[0])
	}

	// 8) 연간 요약: 6월 지출 220000, 5월 30000, 연 합계 250000
	ys, err := st.YearSummary(y)
	if err != nil {
		t.Fatalf("YearSummary: %v", err)
	}
	if ys.Months[5].Expense != 220000 || ys.Months[4].Expense != 30000 {
		t.Errorf("연간 월별 지출 오류: 6월=%d 5월=%d", ys.Months[5].Expense, ys.Months[4].Expense)
	}
	if ys.TotalExpense != 250000 {
		t.Errorf("연 지출 합계=%d, want 250000", ys.TotalExpense)
	}

	// 9) 귀속자별 분석 (모든 거래가 아빠)
	mc, err := st.MemberCategoryBreakdown(y, m)
	if err != nil {
		t.Fatalf("MemberCategoryBreakdown: %v", err)
	}
	if len(mc) != 1 || mc[0].Card != "아빠" || mc[0].Total != 220000 {
		t.Errorf("귀속자 카테고리 집계 오류: %+v", mc)
	}
	if len(mc[0].Categories) == 0 || mc[0].Categories[0].Name != "식비" || mc[0].Categories[0].Amount != 120000 {
		t.Errorf("귀속자 카테고리 1순위 식비/120000 기대: %+v", mc[0].Categories)
	}

	ms, err := st.MemberStats(y, m)
	if err != nil {
		t.Fatalf("MemberStats: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("귀속자 요약 수=%d, want 1", len(ms))
	}
	if ms[0].Member != "아빠" || ms[0].Total != 220000 || ms[0].Share != 100 || ms[0].Prev != 30000 {
		t.Errorf("귀속자 요약 오류: %+v", ms[0])
	}
	if ms[0].TopCategory != "식비" || ms[0].TopCategoryAmount != 120000 || ms[0].TopMerchant != "가맹점" {
		t.Errorf("귀속자 최다 항목 오류: %+v", ms[0])
	}

	mt, err := st.MemberTrend(y, m, 6)
	if err != nil {
		t.Fatalf("MemberTrend: %v", err)
	}
	ds := findSeries(mt.Series, "아빠")
	if ds == nil || ds.Values[5] != 220000 || ds.Values[4] != 30000 {
		t.Errorf("귀속자 추이 오류: %+v", mt.Series)
	}
}
