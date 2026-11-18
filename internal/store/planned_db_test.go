package store

import (
	"path/filepath"
	"testing"
	"time"
)

// 예정 지출: 저장 → 후보(금액 · 날짜 가까운 것 먼저) → 냈어요(바구니 꺼냄 함께) → 중복 연결 막기 → 안 냄 · 되돌리기
// → 바구니 화면의 예정 → 백업.
func TestPlannedDB(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE baskets, planned_expenses RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	old := nowFunc
	t.Cleanup(func() { nowFunc = old })
	nowFunc = func() time.Time { return time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC) }

	if _, err := st.SavePlannedExpense(PlannedExpense{Date: "2026-12-05", Amount: 100000}); err == nil {
		t.Fatal("이름 없이 저장됨")
	}
	if _, err := st.SavePlannedExpense(PlannedExpense{Date: "12/5", Name: "x", Amount: 1}); err == nil {
		t.Fatal("날짜 형식이 틀렸는데 저장됨")
	}
	reserve, err := st.SaveBasket(Basket{Name: "생활 예비비", Kind: BasketReserve, Active: true}, 300000)
	if err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	main, err := st.SavePlannedExpense(PlannedExpense{Date: "2026-12-05", Name: "김○○ 결혼식 축의금", Amount: 100000, BasketID: &zero})
	if err != nil || main.BasketID != nil || main.Status != PlannedOpen {
		t.Fatalf("생활비에서 낼 예정: %+v %v", main, err)
	}
	fromBasket, err := st.SavePlannedExpense(PlannedExpense{Date: "2026-12-12", Name: "박○○ 결혼식 축의금", Amount: 100000, BasketID: &reserve.ID})
	if err != nil || fromBasket.BasketName != "생활 예비비" {
		t.Fatalf("바구니에서 낼 예정: %+v %v", fromBasket, err)
	}

	// 바구니 화면: 잔액 30만 원 중 예정 10만 원
	book, err := st.BasketBook(date("2026-11-19"))
	if err != nil || len(book) != 1 || book[0].PlannedTotal != 100000 || len(book[0].Planned) != 1 {
		t.Fatalf("바구니 예정: %+v %v", book, err)
	}

	// 후보: 예정일 앞뒤 2주 지출, 금액이 같은 것 먼저
	add := func(date string, amount int64, merchant string) int64 {
		t.Helper()
		id, err := st.AddTransaction(Transaction{Date: date, Amount: amount, Direction: "expense", Merchant: merchant, Source: "manual"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	add("2026-12-06", 32000, "점심")
	gift := add("2026-12-13", 100000, "축의금")
	add("2026-11-01", 100000, "너무 이른 거래") // 범위 밖
	cands, err := st.PlannedCandidates(fromBasket.ID)
	if err != nil || len(cands) != 2 || cands[0].ID != gift {
		t.Fatalf("후보: %+v %v", cands, err)
	}

	// 냈어요: 끝나고, 바구니 꺼냄도 남는다(생활비 통장으로 옮겨 옴)
	done, err := st.SettlePlannedExpense(fromBasket.ID, gift, true)
	if err != nil || done.Status != PlannedDone || done.TxMerchant != "축의금" {
		t.Fatalf("냈어요: %+v %v", done, err)
	}
	entries, _ := st.BasketEntries(reserve.ID)
	var spend *BasketEntry
	for i := range entries {
		if entries[i].Kind == BasketSpend {
			spend = &entries[i]
		}
	}
	if spend == nil || spend.Amount != -100000 || !spend.Moved || spend.TransactionID == nil || *spend.TransactionID != gift {
		t.Fatalf("바구니 꺼냄: %+v", entries)
	}
	if _, err := st.SettlePlannedExpense(fromBasket.ID, gift, true); err == nil {
		t.Fatal("끝낸 예정을 다시 연결")
	}
	if _, err := st.SettlePlannedExpense(main.ID, gift, false); err == nil {
		t.Fatal("한 거래를 두 예정에 연결")
	}
	if book, _ = st.BasketBook(date("2026-11-19")); book[0].PlannedTotal != 0 || book[0].Balance != 200000 {
		t.Fatalf("낸 뒤 바구니: 예정 %d 잔액 %d", book[0].PlannedTotal, book[0].Balance)
	}

	// 안 냄 → 되돌리기
	if err := st.CancelPlannedExpense(main.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SavePlannedExpense(PlannedExpense{ID: main.ID, Date: "2026-12-06", Name: "고침", Amount: 1}); err == nil {
		t.Fatal("끝낸 예정을 고침")
	}
	if err := st.CancelPlannedExpense(main.ID, true); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListPlannedExpenses(date("2026-11-19"), 60)
	if err != nil || len(list) != 2 || list[0].Status != PlannedOpen || list[1].Status != PlannedDone {
		t.Fatalf("목록: %+v %v", list, err)
	}

	counts, err := st.BackupToSQLite(filepath.Join(t.TempDir(), "b.db"))
	if err != nil || counts["planned_expenses"] != 2 {
		t.Fatalf("백업: %+v %v", counts, err)
	}
}
