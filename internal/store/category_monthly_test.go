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
