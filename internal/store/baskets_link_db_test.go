package store

import (
	"testing"
	"time"
)

// 바구니 2차: 카테고리 연결 → 연결 후보(정해진 지출 이름 먼저, '아님', 연결하면 빠짐) → 적립액 제안
// → 예산에서 바구니 몫 빼기 → 월 요약의 바구니 지출.
func TestBasketsLinkDB(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE baskets, budgets, subscriptions, subscription_posts RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	old := nowFunc
	t.Cleanup(func() { nowFunc = old })
	nowFunc = func() time.Time { return time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC) }

	cats, err := st.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	catID := map[string]int64{}
	for _, c := range cats {
		catID[c.Name] = c.ID
	}
	gift, med, etc := catID["회비/경조사"], catID["의료비"], catID["기타지출"]
	if gift == 0 || med == 0 || etc == 0 {
		t.Fatalf("기본 카테고리: %v", catID)
	}

	reserve, err := st.SaveBasket(Basket{Name: "생활 예비비", Kind: BasketReserve, Active: true, CategoryIDs: []int64{gift, gift}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reserve.CategoryIDs) != 1 || reserve.CategoryIDs[0] != gift {
		t.Fatalf("카테고리 연결: %v", reserve.CategoryIDs)
	}
	sched, err := st.SaveBasket(Basket{Name: "정해진 연간 지출", Kind: BasketScheduled, Active: true,
		Items: []BasketItem{{Name: "재산세", Month: 7, Amount: 300000}, {Name: "자동차세(연납)", Month: 1, Amount: 450000}}}, 0)
	if err != nil {
		t.Fatal(err)
	}

	add := func(date string, amount int64, merchant string, cat int64) int64 {
		t.Helper()
		id, err := st.AddTransaction(Transaction{Date: date, Amount: amount, Direction: "expense", Merchant: merchant, CategoryID: &cat, Source: "manual"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	wedding := add("2026-11-08", 100000, "축의금", gift)
	add("2026-11-10", 30000, "동아리 회비", gift)     // 작은 지출 — 후보 아님
	tax := add("2026-11-12", 450000, "자동차세 연납", etc) // 정해진 지출 이름 — 카테고리와 상관없이
	add("2026-11-11", 200000, "정형외과", med)          // 의료비를 이은 바구니가 없음
	add("2026-05-20", 500000, "부의금", gift)           // 90일 전 — 후보 아님, 제안에는 들어감
	add("2025-10-01", 900000, "오래된 축의금", gift)       // 12개월 전 — 제안에서도 빠짐

	cands, err := st.BasketCandidates(date("2026-11-14"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 || cands[0].TransactionID != tax || cands[0].BasketID != sched.ID || cands[0].ItemName != "자동차세(연납)" ||
		cands[1].TransactionID != wedding || cands[1].BasketID != reserve.ID {
		t.Fatalf("후보: %+v", cands)
	}

	// '아님'으로 지우거나 바구니에 연결하면 후보에서 빠진다
	if err := st.DismissBasketCandidate(wedding); err != nil {
		t.Fatal(err)
	}
	if err := st.DismissBasketCandidate(wedding); err != nil {
		t.Fatal("두 번 지우기", err)
	}
	if _, err := st.AddBasketEntry(BasketEntry{BasketID: sched.ID, Kind: BasketSpend, Amount: 450000, TransactionID: &tax}); err != nil {
		t.Fatal(err)
	}
	if cands, _ = st.BasketCandidates(date("2026-11-14")); len(cands) != 0 {
		t.Fatalf("남은 후보: %+v", cands)
	}

	// 적립액 제안: 지난 12개월 이은 카테고리의 큰 지출(축의금 10만 + 부의금 50만) ÷ 12 → 만 원 올림
	m, n, err := st.SuggestBasketMonthly([]int64{gift}, date("2026-11-14"))
	if err != nil || m != 50000 || n != 2 {
		t.Fatalf("제안: %d원 %d건 %v", m, n, err)
	}
	book, _ := st.BasketBook(date("2026-11-14"))
	for _, b := range book {
		if b.Basket.ID == reserve.ID && b.SuggestMonthly != 50000 {
			t.Fatalf("바구니 화면 제안: %d", b.SuggestMonthly)
		}
	}

	// 예산: 축의금을 바구니에서 냈으면 그달 경조사 예산 사용에서 빠진다
	if err := st.SetBudget(gift, "*", 120000); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddBasketEntry(BasketEntry{BasketID: reserve.ID, Kind: BasketSpend, Amount: 100000, TransactionID: &wedding}); err != nil {
		t.Fatal(err)
	}
	bs, err := st.BudgetStatuses(2026, 11)
	if err != nil || len(bs) != 1 || bs[0].Spent != 30000 || bs[0].Over {
		t.Fatalf("예산: %+v %v", bs, err)
	}

	// 월 요약: 지출 합은 그대로, 그중 바구니에서 낸 돈을 따로
	sum, err := st.MonthlySummary(2026, 11)
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalExpense != 780000 || sum.BasketExpense != 550000 {
		t.Fatalf("월 요약: 지출 %d · 바구니 %d", sum.TotalExpense, sum.BasketExpense)
	}
}
