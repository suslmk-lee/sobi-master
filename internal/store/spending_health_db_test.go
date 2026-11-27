package store

import (
	"math"
	"path/filepath"
	"testing"

	"database/sql"
)

// healthFixture 는 2026-10 에 성격이 섞인 지출을 넣는다. 예상 값 (원):
//
//	총지출 1,000,000 = 필수 230,000 + 선택 490,000 + 낭비 80,000 + 미지정 50,000 + 바구니 150,000
//	예산 초과: 식비(월 전용 280,000) ← 식비 직접 250,000 + 바구니 뺀 50,000 = 300,000 → 20,000
//	           외식(기본 100,000)     ← 외식 150,000                           → 50,000
//	  (배달은 낭비, 장보기는 필수라 식비 초과분에 넣지 않는다 — 넣으면 같은 돈을 두 번 센다)
//	낭비액 = 80,000 + 70,000 = 150,000 → 낭비율 0.15
func healthFixture(t *testing.T, st *Store) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	cats, err := st.ListCategories()
	must(err)
	food := findCatID(cats, "식비")
	culture := findCatID(cats, "문화/여가")
	phone := findCatID(cats, "통신비")
	misc := findCatID(cats, "기타지출")
	living := findCatID(cats, "생활용품")
	salary := findCatID(cats, "급여")
	invest := findCatID(cats, "투자이체")
	sub := func(name string) int64 {
		c, err := st.AddCategory(name, "expense", food)
		must(err)
		return c.ID
	}
	delivery, grocery, dining := sub("배달"), sub("장보기"), sub("외식")

	must(st.SetCategorySpendClass(food, SpendDiscretionary))
	must(st.SetCategorySpendClass(delivery, SpendWaste))
	must(st.SetCategorySpendClass(grocery, SpendEssential))
	must(st.SetCategoryFixed(misc, true)) // 성격은 미지정이지만 고정비 → 필수

	must(st.SetBudget(food, "*", 300000))
	must(st.SetBudget(food, "2026-10", 280000)) // 월 전용이 기본보다 우선
	must(st.SetBudget(dining, "*", 100000))     // 외식은 자기 예산에만 묶인다

	add := func(date string, amount int64, dir string, cat int64, merchant string) int64 {
		t.Helper()
		tx := Transaction{Date: date, Amount: amount, Direction: dir, Merchant: merchant}
		if cat != 0 {
			tx.CategoryID = &cat
		}
		id, err := st.AddTransaction(tx)
		must(err)
		return id
	}
	add("2026-10-02", 250000, "expense", food, "마트")
	add("2026-10-03", 80000, "expense", delivery, "배민")
	add("2026-10-04", 120000, "expense", grocery, "이마트")
	add("2026-10-05", 150000, "expense", dining, "식당")
	add("2026-10-06", 40000, "expense", culture, "영화관")
	add("2026-10-07", 60000, "expense", phone, "통신사")
	add("2026-10-08", 30000, "expense", misc, "관리")
	linked := add("2026-10-09", 20000, "expense", living, "정수기")
	add("2026-10-10", 25000, "expense", living, "다이소")
	add("2026-10-11", 25000, "expense", 0, "모름")
	big := add("2026-10-12", 200000, "expense", food, "명절장보기")
	add("2026-10-25", 3000000, "income", salary, "회사")
	add("2026-10-26", 500000, "transfer", invest, "증권")
	add("2026-09-15", 10000, "expense", food, "마트") // 전월: 기본 예산 300,000 안 → 낭비 0
	add("2026-08-25", 3000000, "income", salary, "회사") // 지출이 없는 달

	s, err := st.SaveSubscription(Subscription{Name: "정수기", Amount: 20000, Cycle: "monthly", BillingDay: 9, Active: true})
	must(err)
	if ok, err := st.LinkSubscriptionTx(s.ID, linked); err != nil || !ok {
		t.Fatalf("정기결제 연결: %v %v", ok, err)
	}
	b, err := st.SaveBasket(Basket{Name: "명절 바구니", Kind: BasketReserve, Active: true}, 1000000)
	must(err)
	if _, err := st.AddBasketEntry(BasketEntry{BasketID: b.ID, Kind: BasketSpend, Amount: 150000, Date: "2026-10-12", TransactionID: &big}); err != nil {
		t.Fatal(err)
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSpendingHealthDB(t *testing.T) {
	st := openTestStore(t)
	healthFixture(t, st)

	h, err := st.SpendingHealth(2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := SpendingHealth{YM: "2026-10", TotalExpense: 1000000, Essential: 230000, Discretionary: 490000,
		Waste: 80000, Unassigned: 50000, Basket: 150000, DiscretionaryOverBudget: 70000, WasteAmount: 150000}
	if h.YM != want.YM || h.TotalExpense != want.TotalExpense || h.Essential != want.Essential ||
		h.Discretionary != want.Discretionary || h.Waste != want.Waste || h.Unassigned != want.Unassigned ||
		h.Basket != want.Basket || h.DiscretionaryOverBudget != want.DiscretionaryOverBudget || h.WasteAmount != want.WasteAmount {
		t.Fatalf("구성\n got %+v\nwant %+v", h, want)
	}
	if h.Essential+h.Discretionary+h.Waste+h.Unassigned+h.Basket != h.TotalExpense {
		t.Error("성격별 합계가 총지출과 다름")
	}
	if !near(h.WasteRate, 0.15) || !near(h.DiscretionaryRate, 0.49) || !near(h.UnassignedRate, 0.05) {
		t.Errorf("비율 %v %v %v", h.WasteRate, h.DiscretionaryRate, h.UnassignedRate)
	}
	if h.PrevWasteRate != 0 {
		t.Errorf("전월 낭비율 %v", h.PrevWasteRate)
	}
	if len(h.Trend) != 6 || h.Trend[0].YM != "2026-05" || h.Trend[5].YM != "2026-10" || !near(h.Trend[5].WasteRate, 0.15) {
		t.Errorf("추이 %+v", h.Trend)
	}

	// 낭비 상위: 카테고리(낭비 + 예산 초과분)와 가맹점
	var cats, merchants []NamedAmount
	for _, na := range h.TopWaste {
		if na.Kind == "category" {
			cats = append(cats, na)
		} else {
			merchants = append(merchants, na)
		}
	}
	wantCats := []NamedAmount{
		{Name: "식비 > 배달", Kind: "category", Amount: 80000},
		{Name: "외식 (예산 초과)", Kind: "category", Amount: 50000},
		{Name: "식비 (예산 초과)", Kind: "category", Amount: 20000},
	}
	if len(cats) != len(wantCats) {
		t.Fatalf("상위 카테고리 %+v", cats)
	}
	for i := range wantCats {
		if cats[i] != wantCats[i] {
			t.Errorf("상위 카테고리 %d: %+v, want %+v", i, cats[i], wantCats[i])
		}
	}
	if len(merchants) != 1 || merchants[0].Name != "배민" || merchants[0].Amount != 80000 {
		t.Errorf("상위 가맹점 %+v", merchants)
	}

	// 지출이 없는 달(수입만): 0 으로 나누지 않고 0
	if h8, err := st.SpendingHealth(2026, 8); err != nil || h8.TotalExpense != 0 || h8.WasteRate != 0 {
		t.Errorf("지출 없는 달: %+v %v", h8, err)
	}
	// 거래가 없는 달: 빈 값, 목록은 nil 이 아닌 빈 배열
	h0, err := st.SpendingHealth(2027, 1)
	if err != nil || h0.YM != "2027-01" || h0.TotalExpense != 0 || h0.TopWaste == nil || len(h0.Trend) != 6 {
		t.Errorf("빈 달: %+v %v", h0, err)
	}
}

// 대시보드가 읽는 뷰는 앱 화면과 같은 숫자를 내야 한다.
func TestDashViewsMatchApp(t *testing.T) {
	st := openTestStore(t)
	healthFixture(t, st)

	for _, m := range []int{9, 10} {
		ym := "2026-" + map[int]string{9: "09", 10: "10"}[m]
		sum, err := st.MonthlySummary(2026, m)
		if err != nil {
			t.Fatal(err)
		}
		var inc, exp, tr, basket, unclass int64
		if err := st.queryRow(`SELECT income, expense, transfer, basket_expense, unclassified_count FROM dash.month_summary WHERE ym = ?`, ym).
			Scan(&inc, &exp, &tr, &basket, &unclass); err != nil {
			t.Fatal(err)
		}
		if inc != sum.TotalIncome || exp != sum.TotalExpense || tr != sum.TotalTransfer || basket != sum.BasketExpense || unclass != int64(sum.UnclassifiedCount) {
			t.Errorf("%s 월 요약: 뷰 %d/%d/%d/%d/%d, 앱 %+v", ym, inc, exp, tr, basket, unclass, sum)
		}
		h, _ := st.SpendingHealth(2026, m)
		if h.TotalExpense != sum.TotalExpense {
			t.Errorf("%s 총지출: 건강도 %d, 월 요약 %d", ym, h.TotalExpense, sum.TotalExpense)
		}

		app, err := st.BudgetStatuses(2026, m)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := st.query(`SELECT category_id, category, amount, spent, remaining, "over", override FROM dash.budget_status_monthly WHERE ym = ?`, ym)
		if err != nil {
			t.Fatal(err)
		}
		view := map[int64]BudgetStatus{}
		for rows.Next() {
			var b BudgetStatus
			if err := rows.Scan(&b.CategoryID, &b.Category, &b.Amount, &b.Spent, &b.Remaining, &b.Over, &b.Override); err != nil {
				t.Fatal(err)
			}
			view[b.CategoryID] = b
		}
		rows.Close()
		if len(view) != len(app) {
			t.Errorf("%s 예산 행 수: 뷰 %d, 앱 %d", ym, len(view), len(app))
		}
		for _, a := range app {
			v := view[a.CategoryID]
			if v.Category != a.Category || v.Amount != a.Amount || v.Spent != a.Spent || v.Remaining != a.Remaining || v.Over != a.Over || v.Override != a.Override {
				t.Errorf("%s 예산 %s: 뷰 %+v, 앱 %+v", ym, a.Category, v, a)
			}
		}
	}
}

// 기본 분류: 새 DB(seedDefaults)에서도 채워지고, 부는 자기 값이 없으면 주의 값을 받는다.
func TestSpendClassDefaultsAndValidation(t *testing.T) {
	st := openTestStore(t)
	cats, _ := st.ListCategories()
	byName := map[string]Category{}
	for _, c := range cats {
		byName[c.FullName] = c
	}
	for name, want := range map[string]string{"통신비": SpendEssential, "대출상환": SpendEssential, "의료비": SpendEssential,
		"교육비": SpendEssential, "문화/여가": SpendDiscretionary, "식비": SpendUnset} {
		if got := byName[name].SpendClass; got != want {
			t.Errorf("%s 기본 분류 %q, want %q", name, got, want)
		}
	}
	food := byName["식비"].ID
	if err := st.SetCategorySpendClass(food, "luxury"); err == nil {
		t.Error("모르는 값을 받음")
	}
	if err := st.SetCategorySpendClass(999999, SpendWaste); err != ErrNotFound {
		t.Errorf("없는 카테고리: %v", err)
	}
	if err := st.SetCategorySpendClass(food, SpendWaste); err != nil {
		t.Fatal(err)
	}
	snack, err := st.AddCategory("간식", "expense", food)
	if err != nil {
		t.Fatal(err)
	}
	cats, _ = st.ListCategories()
	for _, c := range cats {
		if c.ID == snack.ID && (c.SpendClass != "" || c.ParentSpendClass != SpendWaste) {
			t.Errorf("부 카테고리: %+v", c)
		}
	}
}

// 스키마를 다시 적용해도(뷰 지우고 다시 만들기) 실패하지 않는다 — 실패하면 앱이 시작하지 못한다.
func TestDashSchemaReapply(t *testing.T) {
	st := openTestStore(t)
	for i := 0; i < 2; i++ {
		if err := st.migrate(); err != nil {
			t.Fatalf("%d번째 적용: %v", i+1, err)
		}
	}
	var n int
	if err := st.queryRow(`SELECT COUNT(*) FROM information_schema.views WHERE table_schema = 'dash'`).Scan(&n); err != nil || n != 4 {
		t.Errorf("dash 뷰 %d개 (%v)", n, err)
	}
}

// 백업 파일에 지출 성격이 들어간다.
func TestBackupKeepsSpendClass(t *testing.T) {
	st := openTestStore(t)
	cats, _ := st.ListCategories()
	food := findCatID(cats, "식비")
	if err := st.SetCategorySpendClass(food, SpendDiscretionary); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.db")
	if _, err := st.BackupToSQLite(path); err != nil {
		t.Fatal(err)
	}
	lite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lite.Close()
	var v string
	if err := lite.QueryRow(`SELECT spend_class FROM categories WHERE id = ?`, food).Scan(&v); err != nil || v != SpendDiscretionary {
		t.Errorf("백업의 지출 성격 %q (%v)", v, err)
	}
}
