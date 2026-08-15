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
	ct, err := st.CategoryTrend(y, m, 6, CatLevelMain)
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

	// 10) 가맹점 자동완성: "가맹" 검색 → "가맹점"의 최근 거래(6/10, 식비/아빠) 반환
	sg, err := st.MerchantSuggestions("가맹", 8)
	if err != nil {
		t.Fatalf("MerchantSuggestions: %v", err)
	}
	if len(sg) != 1 || sg[0].Merchant != "가맹점" {
		t.Fatalf("자동완성 결과 오류: %+v", sg)
	}
	if sg[0].CategoryID == nil || *sg[0].CategoryID != food || sg[0].MemberID == nil || *sg[0].MemberID != dad {
		t.Errorf("자동완성 최근값(식비/아빠) 오류: %+v", sg[0])
	}

	// 11) 컬럼 정렬: 금액 오름차순이면 최소 금액(30000=5월 건)이 맨 앞
	byAmt, err := st.ListTransactions(TxFilter{Sort: "amount_asc"})
	if err != nil {
		t.Fatalf("ListTransactions(amount_asc): %v", err)
	}
	if len(byAmt) < 2 || byAmt[0].Amount > byAmt[1].Amount {
		t.Errorf("금액 오름차순 정렬 실패: %+v", byAmt)
	}
	if byAmt[0].Amount != 30000 {
		t.Errorf("금액 오름차순 첫 건=%d, want 30000", byAmt[0].Amount)
	}
	// 카테고리 정렬도 SQL 오류 없이 동작해야 한다
	if _, err := st.ListTransactions(TxFilter{Sort: "category_desc"}); err != nil {
		t.Fatalf("ListTransactions(category_desc): %v", err)
	}

	// 12) 카테고리 상세 드릴다운 (식비: 6월 120000, 5월 30000, 전체의 54%)
	cd, err := st.CategoryDetail(y, m, food, "month", date("2026-06-15"), 6)
	if err != nil {
		t.Fatalf("CategoryDetail: %v", err)
	}
	if cd.Category != "식비" || cd.Total != 120000 || cd.Prev != 30000 {
		t.Errorf("카테고리 상세 오류: %+v", cd)
	}
	if cd.Label != "6월" || cd.PrevLabel != "전월" {
		t.Errorf("기간 라벨 오류: %q / %q", cd.Label, cd.PrevLabel)
	}
	if cd.Share != 54 { // 120000 / 220000 = 54%
		t.Errorf("카테고리 비중=%d, want 54", cd.Share)
	}

	// 12-b) 전월 기간: 5월(식비 30000), 예산 없으니 0
	cdPrev, err := st.CategoryDetail(y, m, food, "prev", date("2026-06-15"), 6)
	if err != nil {
		t.Fatalf("CategoryDetail(prev): %v", err)
	}
	if cdPrev.Total != 30000 || cdPrev.Label != "5월" {
		t.Errorf("전월 기간 오류: total=%d label=%q", cdPrev.Total, cdPrev.Label)
	}

	// 12-c) 최근 30일(오늘=6/15 → 5/17~6/15): 식비 6/1(50000)+6/10(70000)=120000, 5/1은 범위 밖. 예산 미적용
	cd30, err := st.CategoryDetail(y, m, food, "recent30", date("2026-06-15"), 6)
	if err != nil {
		t.Fatalf("CategoryDetail(recent30): %v", err)
	}
	if cd30.Label != "최근 30일" || cd30.Total != 120000 || cd30.Budget != 0 {
		t.Errorf("최근 30일 오류: label=%q total=%d budget=%d", cd30.Label, cd30.Total, cd30.Budget)
	}
	if len(cd.ByMember) == 0 || cd.ByMember[0].Name != "아빠" {
		t.Errorf("카테고리 귀속자 분해 오류: %+v", cd.ByMember)
	}
	if cd.Trend[5] != 120000 || cd.Trend[4] != 30000 {
		t.Errorf("카테고리 추이 오류: 6월=%d 5월=%d", cd.Trend[5], cd.Trend[4])
	}
	// 예산(앞에서 식비 6월 오버라이드 200000 설정됨) → 120000/200000 = 60%, 미초과
	if cd.Budget != 200000 || cd.BudgetPct != 60 || cd.Over {
		t.Errorf("카테고리 예산 오류: budget=%d pct=%d over=%v", cd.Budget, cd.BudgetPct, cd.Over)
	}

	// 13) 카테고리별 귀속자 구성 (식비 그룹 안에 아빠 120000)
	cm, err := st.CategoryMemberBreakdown(y, m, CatLevelMain)
	if err != nil {
		t.Fatalf("CategoryMemberBreakdown: %v", err)
	}
	var foodGroup *CardCategory
	for i := range cm {
		if cm[i].Card == "식비" {
			foodGroup = &cm[i]
		}
	}
	if foodGroup == nil || foodGroup.Total != 120000 || foodGroup.Categories[0].Name != "아빠" {
		t.Errorf("카테고리별 귀속자 구성 오류: %+v", cm)
	}

	// 14) 히트맵 매트릭스: 전체 카테고리 반환(묶음 없음), 식비 6월 120000
	mtx, err := st.CategoryMatrix(y, m, 6, CatLevelMain)
	if err != nil {
		t.Fatalf("CategoryMatrix: %v", err)
	}
	fs := findSeries(mtx.Series, "식비")
	if fs == nil || fs.Values[5] != 120000 {
		t.Errorf("히트맵 매트릭스 오류: %+v", mtx.Series)
	}
}
