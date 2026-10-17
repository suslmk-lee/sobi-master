package store

import "testing"

// 주 카테고리를 고르면 하위 부 지출까지 끌어와야 하고, 주에 바로 단 지출은
// 부와 섞이지 않게 "(주 직접)"으로 구분돼야 한다. 그래야 어느 달에 무엇 때문에
// 늘었는지 표에서 바로 읽힌다. (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestCategoryMonthly(t *testing.T) {
	st := openTestStore(t)

	cats, _ := st.ListCategories()
	care := findCatID(cats, "식비") // 기본 카테고리 중 아무 주나 사용
	sub, err := st.AddCategory("요양원", "expense", care)
	if err != nil {
		t.Fatal(err)
	}
	other := findCatID(cats, "통신비")

	add := func(d, merchant string, amt, catID int64) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{
			Date: d, Amount: amt, Direction: "expense", Merchant: merchant,
			CategoryID: &catID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-05-10", "요양원A", 800000, sub.ID)
	add("2026-06-10", "요양원A", 800000, sub.ID)
	add("2026-06-20", "약국B", 50000, sub.ID)
	add("2026-06-25", "기타지출", 30000, care) // 주에 바로 단 건
	add("2026-06-05", "무관한가게", 90000, other)

	t.Run("가맹점 기준", func(t *testing.T) {
		got, err := st.CategoryMonthly(2026, 6, 2, care, BreakMerchant)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Months) != 2 || got.Months[0] != "2026-05" || got.Months[1] != "2026-06" {
			t.Fatalf("월 목록 %v", got.Months)
		}
		a := findSeries(got.Series, "요양원A")
		if a == nil {
			t.Fatalf("요양원A 없음: %+v", got.Series)
		}
		if a.Values[0] != 800000 || a.Values[1] != 800000 || a.Total != 1600000 {
			t.Errorf("요양원A 월별 %v 합계 %d", a.Values, a.Total)
		}
		if b := findSeries(got.Series, "약국B"); b == nil || b.Values[1] != 50000 {
			t.Errorf("약국B 6월 금액이 맞지 않음: %+v", b)
		}
		// 다른 카테고리 지출은 섞이면 안 된다
		if x := findSeries(got.Series, "무관한가게"); x != nil {
			t.Errorf("다른 카테고리 가맹점이 섞임: %+v", x)
		}
		// 총액 내림차순 정렬
		if got.Series[0].Name != "요양원A" {
			t.Errorf("정렬이 총액순이 아님: %s 가 먼저", got.Series[0].Name)
		}
	})

	t.Run("부 카테고리 기준", func(t *testing.T) {
		got, err := st.CategoryMonthly(2026, 6, 2, care, BreakSub)
		if err != nil {
			t.Fatal(err)
		}
		s := findSeries(got.Series, "요양원")
		if s == nil || s.Total != 1650000 {
			t.Fatalf("부 '요양원' 합계가 맞지 않음: %+v", s)
		}
		// 주에 바로 단 지출은 부와 섞이지 않고 따로 잡혀야 한다
		direct := findSeries(got.Series, "식비 (주 직접)")
		if direct == nil || direct.Values[1] != 30000 {
			t.Fatalf("주 직접 지출이 분리되지 않음: %+v", got.Series)
		}
	})

	t.Run("부를 직접 고르면 그 부만", func(t *testing.T) {
		got, err := st.CategoryMonthly(2026, 6, 2, sub.ID, BreakMerchant)
		if err != nil {
			t.Fatal(err)
		}
		if findSeries(got.Series, "기타지출") != nil {
			t.Errorf("부를 골랐는데 주 직접 지출이 섞임: %+v", got.Series)
		}
		var sum int64
		for _, s := range got.Series {
			sum += s.Total
		}
		if sum != 1650000 {
			t.Errorf("부 합계 %d, want 1650000", sum)
		}
	})
}

// 전월 대비 비교는 한쪽 달에만 있는 카테고리도 신규/없어짐으로 실어야 하고,
// 금액순이 아니라 증감이 큰 순으로 와야 "무엇이 총액을 흔들었나"가 위에 온다.
// 또 행 합계가 총계와 어긋나면 화면의 숫자가 서로 안 맞는다. (DB 없으면 자동 스킵)
func TestCategoryCompare(t *testing.T) {
	st := openTestStore(t)

	cats, _ := st.ListCategories()
	food := findCatID(cats, "식비")
	tele := findCatID(cats, "통신비")
	sub, err := st.AddCategory("배달", "expense", food)
	if err != nil {
		t.Fatal(err)
	}

	add := func(d string, amt, catID int64) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{
			Date: d, Amount: amt, Direction: "expense", Merchant: "가맹점", CategoryID: &catID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// 식비: 8월 10만(주) → 9월 10만(주) + 50만(부 배달) = 증감 +50만 (가장 큰 변화)
	add("2026-08-05", 100000, food)
	add("2026-09-05", 100000, food)
	add("2026-09-06", 500000, sub.ID)
	// 통신비: 8월에만 → 없어짐
	add("2026-08-07", 70000, tele)

	got, err := st.CategoryCompare(2026, 9, CatLevelMain)
	if err != nil {
		t.Fatal(err)
	}
	if got.Month != "2026-09" || got.PrevMonth != "2026-08" {
		t.Fatalf("월 라벨 %s / %s", got.Month, got.PrevMonth)
	}

	var sumCur, sumPrev int64
	for _, r := range got.Rows {
		sumCur += r.Current
		sumPrev += r.Prev
	}
	if sumCur != got.Total || sumPrev != got.PrevTotal {
		t.Errorf("행 합계가 총계와 다름: %d/%d vs %d/%d", sumCur, sumPrev, got.Total, got.PrevTotal)
	}

	// 증감 큰 순 — 식비(+50만)가 통신비(-7만)보다 앞
	if len(got.Rows) < 2 || got.Rows[0].Name != "식비" {
		t.Fatalf("증감순 정렬이 아님: %+v", got.Rows)
	}
	if f := got.Rows[0]; f.Prev != 100000 || f.Current != 600000 || f.Delta != 500000 {
		t.Errorf("주 기준 합산이 틀림(부 배달 포함돼야 함): %+v", f)
	}

	// 이번 달에 사라진 카테고리도 실려야 한다
	var gone *CategoryDelta
	for i := range got.Rows {
		if got.Rows[i].Name == "통신비" {
			gone = &got.Rows[i]
		}
	}
	if gone == nil {
		t.Fatalf("전월에만 있던 카테고리가 빠짐: %+v", got.Rows)
	}
	if !gone.Gone || gone.Pct != -100 || gone.Current != 0 {
		t.Errorf("없어짐 표시가 잘못됨: %+v", gone)
	}

	// 부 기준에서는 배달이 따로 잡힌다
	subCmp, err := st.CategoryCompare(2026, 9, CatLevelSub)
	if err != nil {
		t.Fatal(err)
	}
	var delivery *CategoryDelta
	for i := range subCmp.Rows {
		if subCmp.Rows[i].Name == "식비 > 배달" {
			delivery = &subCmp.Rows[i]
		}
	}
	if delivery == nil || !delivery.IsNew || delivery.Current != 500000 {
		t.Fatalf("부 기준에서 '식비 > 배달' 신규가 안 보임: %+v", subCmp.Rows)
	}
	if subCmp.Total != got.Total {
		t.Errorf("주/부 기준 총계가 다름: %d vs %d", subCmp.Total, got.Total)
	}
}
