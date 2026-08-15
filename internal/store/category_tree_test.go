package store

import "testing"

// TestCategoryHierarchy 는 주/부 2단 계층의 생성·이동·가드를 검증한다.
func TestCategoryHierarchy(t *testing.T) {
	st := openTestStore(t)

	cats, _ := st.ListCategories()
	food := findCatID(cats, "식비")
	tele := findCatID(cats, "통신비")

	// 부 추가: 종류는 주(지출)를 따른다
	sub, err := st.AddCategory("배달", "income", food) // kind 를 일부러 다르게 줘도
	if err != nil {
		t.Fatal(err)
	}
	if sub.Kind != "expense" {
		t.Errorf("부 kind=%q, want expense (주를 따라야 함)", sub.Kind)
	}
	if sub.ParentID == nil || *sub.ParentID != food {
		t.Errorf("부 parent=%v, want %d", sub.ParentID, food)
	}

	// 부의 부는 금지 (2단까지)
	if _, err := st.AddCategory("치킨", "expense", sub.ID); err == nil {
		t.Error("부 아래에 부를 만들면 오류여야 함")
	}

	// 같은 이름을 다른 주 아래에 두는 것은 허용
	if _, err := st.AddCategory("배달", "expense", tele); err != nil {
		t.Errorf("다른 주 아래 같은 이름은 허용돼야 함: %v", err)
	}

	// ListCategories: 부는 parent/fullName 이 채워지고 주 그룹 뒤에 온다
	cats, err = st.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	var found *Category
	for i := range cats {
		if cats[i].ID == sub.ID {
			found = &cats[i]
		}
	}
	if found == nil {
		t.Fatal("추가한 부를 찾지 못함")
	}
	if found.Parent != "식비" || found.FullName != "식비 > 배달" {
		t.Errorf("부 표시 오류: parent=%q fullName=%q", found.Parent, found.FullName)
	}

	// 주로 승격 후 다시 다른 주 아래로 이동
	if err := st.SetCategoryParent(sub.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := st.SetCategoryParent(sub.ID, tele); err == nil {
		t.Error("통신비 아래에 같은 이름의 부가 이미 있으므로 오류여야 함")
	}
	// 자기 자신을 상위로 지정 금지
	if err := st.SetCategoryParent(sub.ID, sub.ID); err == nil {
		t.Error("자기 자신을 상위로 지정하면 오류여야 함")
	}
}

// TestCategoryRollup 은 부에 붙은 지출이 주 기준 집계·예산에 합산되는지 검증한다.
func TestCategoryRollup(t *testing.T) {
	st := openTestStore(t)

	cats, _ := st.ListCategories()
	mems, _ := st.ListMembers()
	food := findCatID(cats, "식비")
	dad := mems[0].ID

	deliv, err := st.AddCategory("배달", "expense", food)
	if err != nil {
		t.Fatal(err)
	}

	// 주에 직접 1건, 부에 2건
	add := func(catID, amt int64) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{
			Date: "2026-06-05", Amount: amt, Direction: "expense", Merchant: "가게",
			CategoryID: &catID, MemberID: &dad,
		}); err != nil {
			t.Fatal(err)
		}
	}
	add(food, 10000)
	add(deliv.ID, 20000)
	add(deliv.ID, 30000)

	const y, m = 2026, 6

	// 주 기준 월 집계: 식비 = 60000 (직접 10000 + 배달 50000)
	sum, err := st.MonthlySummary(y, m)
	if err != nil {
		t.Fatal(err)
	}
	var foodTotal int64
	for _, na := range sum.ByCategory {
		if na.Name == "식비" {
			foodTotal = na.Amount
		}
		if na.Name == "배달" {
			t.Errorf("주 기준 집계에 부 이름이 나오면 안 됨: %+v", na)
		}
	}
	if foodTotal != 60000 {
		t.Errorf("주 롤업 식비=%d, want 60000", foodTotal)
	}

	// 부 기준 추이에는 "식비 > 배달" 이 별도로 나온다
	subTrend, err := st.CategoryTrend(y, m, 6, CatLevelSub)
	if err != nil {
		t.Fatal(err)
	}
	if s := findSeries(subTrend.Series, "식비 > 배달"); s == nil || s.Values[5] != 50000 {
		t.Errorf("부 기준 추이 오류: %+v", subTrend.Series)
	}
	// 주 기준 추이에는 합산된 "식비" 만
	mainTrend, err := st.CategoryTrend(y, m, 6, CatLevelMain)
	if err != nil {
		t.Fatal(err)
	}
	if s := findSeries(mainTrend.Series, "식비"); s == nil || s.Values[5] != 60000 {
		t.Errorf("주 기준 추이 오류: %+v", mainTrend.Series)
	}

	// 주 예산은 하위 부 지출까지 합산해 판정
	if err := st.SetBudget(food, "*", 50000); err != nil {
		t.Fatal(err)
	}
	bs, err := st.BudgetStatuses(y, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 || bs[0].Spent != 60000 || !bs[0].Over {
		t.Errorf("주 예산 롤업 판정 오류: %+v", bs)
	}

	// 주 상세 드릴다운: 총액 합산 + 부별 구성(직접/배달)
	d, err := st.CategoryDetail(y, m, food, "month", date("2026-06-15"), 6)
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsMain || d.Total != 60000 {
		t.Errorf("주 상세 오류: isMain=%v total=%d", d.IsMain, d.Total)
	}
	if len(d.BySub) != 2 || d.BySub[0].Name != "배달" || d.BySub[0].Amount != 50000 {
		t.Errorf("부별 구성 오류: %+v", d.BySub)
	}

	// 부 상세는 자기 것만 (표시 이름은 전체 경로)
	ds, err := st.CategoryDetail(y, m, deliv.ID, "month", date("2026-06-15"), 6)
	if err != nil {
		t.Fatal(err)
	}
	if ds.IsMain || ds.Total != 50000 || ds.Category != "식비 > 배달" {
		t.Errorf("부 상세 오류: %+v", ds)
	}

	// 카테고리 필터: 주를 고르면 하위 부 거래까지 조회된다
	txs, err := st.ListTransactions(TxFilter{Month: "2026-06", CategoryID: food})
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 3 {
		t.Errorf("주 필터 거래 수=%d, want 3(직접1+부2)", len(txs))
	}
}

// TestSplitSlashCategories 는 "A/B" 이름을 A 가 실제 주 카테고리일 때만 분리하는지 검증한다.
func TestSplitSlashCategories(t *testing.T) {
	st := openTestStore(t)

	// "식비" 는 시드에 있음 → 분리 대상. "회비" 는 없음 → 유지.
	if _, err := st.AddCategory("식비/배달", "expense", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddCategory("회비/경조사", "expense", 0); err != nil {
		t.Fatal(err)
	}

	if err := st.splitSlashCategories(); err != nil {
		t.Fatalf("splitSlashCategories: %v", err)
	}

	cats, err := st.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	var split, kept *Category
	for i := range cats {
		switch cats[i].FullName {
		case "식비 > 배달":
			split = &cats[i]
		case "회비/경조사":
			kept = &cats[i]
		}
	}
	if split == nil {
		t.Error(`"식비/배달" 이 "식비 > 배달" 로 분리되지 않음`)
	}
	if kept == nil || kept.ParentID != nil {
		t.Error(`"회비/경조사" 는 그대로 유지돼야 함 ("회비" 카테고리가 없으므로)`)
	}

	// 멱등: 다시 실행해도 변화 없음
	if err := st.splitSlashCategories(); err != nil {
		t.Fatal(err)
	}
}
