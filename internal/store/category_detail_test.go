package store

import "testing"

func TestCategoryDetailRange(t *testing.T) {
	today := date("2026-11-08")
	cases := []struct {
		period                        string
		from, to, prevFrom, prevTo    string
		label, prevLabel, budgetMonth string
		anchorY, anchorM              int
	}{
		{"month", "2026-11-01", "2026-11-30", "2026-10-01", "2026-10-31", "11월", "전월", "2026-11", 2026, 11},
		{"prev", "2026-10-01", "2026-10-31", "2026-09-01", "2026-09-30", "10월", "전월", "2026-10", 2026, 10},
		{"recent30", "2026-10-10", "2026-11-08", "2026-09-10", "2026-10-09", "최근 30일", "이전 30일", "", 2026, 11},
	}
	for _, c := range cases {
		r := categoryDetailRange(2026, 11, c.period, today)
		got := []string{fmtDay(r.From), fmtDay(r.To), fmtDay(r.PrevFrom), fmtDay(r.PrevTo), r.Label, r.PrevLabel, r.BudgetMonth}
		want := []string{c.from, c.to, c.prevFrom, c.prevTo, c.label, c.prevLabel, c.budgetMonth}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: 항목 %d got %q want %q", c.period, i, got[i], want[i])
			}
		}
		if r.AnchorY != c.anchorY || r.AnchorM != c.anchorM {
			t.Errorf("%s: 기준 달 %d-%d", c.period, r.AnchorY, r.AnchorM)
		}
	}
	// 1월의 전월은 전년 12월
	if r := categoryDetailRange(2026, 1, "prev", today); fmtDay(r.From) != "2025-12-01" || r.BudgetMonth != "2025-12" {
		t.Errorf("1월 전월: %s %s", fmtDay(r.From), r.BudgetMonth)
	}
}

// 상세 막대의 한 줄을 눌러 여는 거래 목록은 그 막대 금액과 합계가 같아야 한다.
// 막대(집계)와 목록이 같은 기간·같은 이름 식을 쓰는지 네 기준 모두 확인한다. (DB 없으면 자동 스킵)
func TestCategoryDetailTx(t *testing.T) {
	st := openTestStore(t)

	cats, _ := st.ListCategories()
	food := findCatID(cats, "식비")
	other := findCatID(cats, "통신비")
	cafe, err := st.AddCategory("카페", "expense", food)
	if err != nil {
		t.Fatal(err)
	}
	mom, err := st.AddMember("엄마")
	if err != nil {
		t.Fatal(err)
	}
	card, err := st.SavePaymentMethod(PaymentMethod{Name: "A카드", Type: "card"})
	if err != nil {
		t.Fatal(err)
	}
	add := func(d, merchant string, amt, catID int64, member, pm *int64) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{Date: d, Amount: amt, Direction: "expense", Merchant: merchant,
			CategoryID: &catID, MemberID: member, PaymentMethodID: pm}); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-11-02", "카페A", 5000, cafe.ID, &mom.ID, &card.ID)
	add("2026-11-03", "카페A", 4500, cafe.ID, nil, &card.ID)
	add("2026-11-04", "식당B", 12000, food, &mom.ID, nil)        // 주에 바로 단 지출
	add("2026-11-05", "", 3000, food, nil, nil)                // 가맹점 없음
	add("2026-10-30", "카페A", 7000, cafe.ID, &mom.ID, &card.ID) // 이번 달 밖
	add("2026-11-06", "통신사", 50000, other, &mom.ID, &card.ID)  // 다른 카테고리

	today := date("2026-11-08")
	for _, period := range []string{"month", "recent30", "prev"} {
		d, err := st.CategoryDetail(2026, 11, food, period, today, 6)
		if err != nil {
			t.Fatal(err)
		}
		bars := map[string][]NamedAmount{
			BreakMember: d.ByMember, BreakPayment: d.ByPayment, BreakMerchant: d.TopMerchants, BreakSub: d.BySub,
		}
		for by, rows := range bars {
			var barSum int64
			for _, r := range rows {
				txs, err := st.CategoryDetailTx(2026, 11, food, period, today, by, r.Name)
				if err != nil {
					t.Fatal(err)
				}
				var sum int64
				for _, tx := range txs {
					sum += tx.Amount
				}
				if sum != r.Amount {
					t.Errorf("%s/%s %q: 목록 합계 %d, 막대 %d", period, by, r.Name, sum, r.Amount)
				}
				barSum += r.Amount
			}
			if barSum != d.Total {
				t.Errorf("%s/%s: 막대 합계 %d, 총액 %d", period, by, barSum, d.Total)
			}
		}
	}

	// 이번 달 엄마: 카페A 5,000 + 식당B 12,000 (10/30 건과 통신비는 빠짐)
	txs, err := st.CategoryDetailTx(2026, 11, food, "month", today, BreakMember, "엄마")
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 2 || txs[0].Merchant != "카페A" || txs[1].Merchant != "식당B" {
		t.Errorf("엄마 이번 달 내역: %+v", txs)
	}
	// 최근 30일이면 10/30 건까지 들어온다
	if txs, _ := st.CategoryDetailTx(2026, 11, food, "recent30", today, BreakMember, "엄마"); len(txs) != 3 {
		t.Errorf("엄마 최근 30일 %d건, want 3", len(txs))
	}
}
