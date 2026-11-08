package store

import "testing"

// 이름을 바꿔도 거래 연결은 그대로여야 하고, 같은 자리(같은 주 아래 / 주끼리)에서만 이름이 겹치면 안 된다.
// (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestRenameCategory(t *testing.T) {
	st := openTestStore(t)

	cats, _ := st.ListCategories()
	food := findCatID(cats, "식비")
	tele := findCatID(cats, "통신비")
	cafe, err := st.AddCategory("카페", "expense", food)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddCategory("배달", "expense", food); err != nil {
		t.Fatal(err)
	}
	txID, err := st.AddTransaction(Transaction{Date: "2026-11-01", Amount: 4500, Direction: "expense",
		Merchant: "카페A", CategoryID: &cafe.ID})
	if err != nil {
		t.Fatal(err)
	}

	// 부 이름 바꾸기 (앞뒤 공백은 잘라 낸다)
	if err := st.RenameCategory(cafe.ID, "  커피/음료 "); err != nil {
		t.Fatal(err)
	}
	cats, _ = st.ListCategories()
	var got *Category
	for i := range cats {
		if cats[i].ID == cafe.ID {
			got = &cats[i]
		}
	}
	if got == nil || got.Name != "커피/음료" || got.FullName != "식비 > 커피/음료" {
		t.Fatalf("이름이 바뀌지 않음: %+v", got)
	}
	// 거래는 같은 카테고리에 그대로 붙어 있다
	txs, err := st.CategoryMonthlyTx("2026-11", food, BreakSub, "커피/음료")
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 || txs[0].ID != txID {
		t.Errorf("이름을 바꾼 뒤 거래 연결: %+v", txs)
	}

	// 같은 주 아래 같은 이름은 안 됨, 다른 주 아래면 됨
	if err := st.RenameCategory(cafe.ID, "배달"); err == nil {
		t.Error("같은 주 아래 이름 중복이 통과함")
	}
	if _, err := st.AddCategory("요금", "expense", tele); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameCategory(cafe.ID, "요금"); err != nil {
		t.Errorf("다른 주 아래 같은 이름은 허용돼야 함: %v", err)
	}
	// 주끼리 이름 중복은 안 됨, 자기 이름 그대로 저장은 됨, 빈 이름은 안 됨
	if err := st.RenameCategory(food, "통신비"); err == nil {
		t.Error("주 카테고리 이름 중복이 통과함")
	}
	if err := st.RenameCategory(food, "식비"); err != nil {
		t.Errorf("자기 이름 그대로 저장이 막힘: %v", err)
	}
	if err := st.RenameCategory(food, "  "); err == nil {
		t.Error("빈 이름이 통과함")
	}
	if err := st.RenameCategory(999999, "없음"); err != ErrNotFound {
		t.Errorf("없는 id: %v", err)
	}
}
