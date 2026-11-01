package main

import (
	"testing"

	"sobi/internal/store"
)

func newPrepaidTestApp(t *testing.T) *App {
	a := newTestApp(t)
	if _, err := a.st.DB().Exec(`TRUNCATE prepaid_passes RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	return a
}

// 삭제 취소는 새 ID 로 거래를 다시 만들지만 선불권 충전 연결은 되살아나야 한다(단건·일괄 모두).
func TestUndoDeleteRestoresPrepaidLink(t *testing.T) {
	a := newPrepaidTestApp(t)
	p, err := a.SavePrepaidPass(store.PrepaidPass{Name: "OO헤어", Merchant: "OO헤어", DiscountMode: "bonus", BonusBp: 1000, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	check := func(label string, wantCount int) {
		t.Helper()
		sts, err := a.ListPrepaidStatuses()
		if err != nil || len(sts) != 1 {
			t.Fatalf("%s: %v %+v", label, err, sts)
		}
		if sts[0].ChargeCount != wantCount || sts[0].Balance != int64(wantCount)*11000 {
			t.Errorf("%s: 충전 %d건/잔액 %d, want %d건/%d", label, sts[0].ChargeCount, sts[0].Balance, wantCount, wantCount*11000)
		}
	}

	c1, err := a.AddPrepaidCharge(p.ID, "2026-06-01", 10000, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	// 단건 삭제 → 취소
	if err := a.DeleteTransaction(c1.TransactionID); err != nil {
		t.Fatal(err)
	}
	check("삭제 직후", 0)
	if n, err := a.UndoDelete(); err != nil || n != 1 {
		t.Fatalf("undo: %d %v", n, err)
	}
	check("단건 취소 후", 1)

	// 일괄 삭제(연결 없는 거래 포함) → 취소
	h, _ := a.GetPrepaidHistory(p.ID)
	plain, _ := a.st.AddTransaction(store.Transaction{Date: "2026-06-02", Amount: 500, Direction: "expense", Merchant: "편의점"})
	if err := a.BatchDelete([]int64{h.Charges[0].TransactionID, plain}); err != nil {
		t.Fatal(err)
	}
	check("일괄 삭제 직후", 0)
	if n, err := a.UndoDelete(); err != nil || n != 2 {
		t.Fatalf("undo: %d %v", n, err)
	}
	check("일괄 취소 후", 1)
	if _, err := a.UndoDelete(); err != nil { // 두 번째 취소는 아무것도 안 함
		t.Fatal(err)
	}
	check("재취소 후", 1)
}

// 선불권 충전은 분류 규칙을 학습하지 않고, 후보는 가맹점 매칭(merchantMatch)을 쓴다.
func TestPrepaidChargeNoLearnAndSuggest(t *testing.T) {
	a := newPrepaidTestApp(t)
	cats, _ := a.st.ListCategories()
	food := categoryID(cats, "식비")
	p, _ := a.SavePrepaidPass(store.PrepaidPass{Name: "헤어", Merchant: "OO헤어", CategoryID: &food, Active: true})
	if _, err := a.AddPrepaidCharge(p.ID, "2026-06-01", 10000, 0, 0, ""); err != nil {
		t.Fatal(err)
	}
	if rules, _ := a.st.ListRules(); len(rules) != 0 {
		t.Errorf("규칙 %d개 — 선불권 충전에서 학습하면 안 됨", len(rules))
	}
	id, _ := a.st.AddTransaction(store.Transaction{Date: "2026-06-05", Amount: 30000, Direction: "expense", Merchant: "OO헤어 강남점"})
	a.st.AddTransaction(store.Transaction{Date: "2026-06-06", Amount: 3000, Direction: "expense", Merchant: "편의점"})
	sug, err := a.SuggestPrepaidCharges(p.ID)
	if err != nil || len(sug) != 1 || sug[0].ID != id {
		t.Fatalf("suggest: %v %+v", err, sug)
	}
}
