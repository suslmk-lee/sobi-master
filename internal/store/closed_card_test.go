package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestPaymentMethodIsClosed(t *testing.T) {
	cases := []struct {
		closed, today string
		want          bool
	}{
		{"", "2026-10-31", false},
		{"2026-10-31", "2026-10-31", true},
		{"2026-10-01", "2026-10-31", true},
		{"2026-11-15", "2026-10-31", false}, // 미래 해지일은 그날까지 사용 중
	}
	for _, c := range cases {
		if got := (PaymentMethod{ClosedAt: c.closed}).IsClosed(c.today); got != c.want {
			t.Errorf("IsClosed(%q,%q)=%v want %v", c.closed, c.today, got, c.want)
		}
	}
}

func countIDs(sts []CardStatus, ids ...int64) int {
	n := 0
	for _, s := range sts {
		for _, id := range ids {
			if s.Card.ID == id {
				n++
			}
		}
	}
	return n
}

func hasCard(sts []CardStatus, id int64) bool { return countIDs(sts, id) == 1 }

func TestClosedCardDB(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE subscriptions, subscription_posts, cash_balances, app_settings, custom_holidays RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	// 저장·검증 왕복
	if _, err := st.SavePaymentMethod(PaymentMethod{Name: "나쁜날짜", Type: "card", ClosedAt: "2026/10/01"}); err == nil {
		t.Error("잘못된 해지일이 저장됨")
	}
	old, err := st.SavePaymentMethod(PaymentMethod{Name: "옛카드", Type: "card", BillingDay: 1, CycleStartDay: 1,
		PerfTarget: 300000, CashbackBp: 100, ClosedAt: "2026-09-01"})
	if err != nil {
		t.Fatal(err)
	}
	cur, err := st.SavePaymentMethod(PaymentMethod{Name: "현카드", Type: "card", BillingDay: 1, CycleStartDay: 1, PerfTarget: 300000})
	if err != nil {
		t.Fatal(err)
	}
	pms, _ := st.ListPaymentMethods()
	for _, p := range pms {
		if p.ID == old.ID && p.ClosedAt != "2026-09-01" {
			t.Errorf("closedAt 왕복 실패: %q", p.ClosedAt)
		}
	}
	if err := st.SetPaymentMethodClosed(old.ID, "bad"); err == nil {
		t.Error("잘못된 날짜 허용")
	}
	if err := st.SetPaymentMethodClosed(999999, "2026-10-01"); err == nil {
		t.Error("없는 결제수단 허용")
	}

	add := func(d string, amt int64, pm int64) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{Date: d, Amount: amt, Direction: "expense", Merchant: "x", PaymentMethodID: &pm}); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-10-05", 40000, old.ID)
	add("2026-10-06", 10000, cur.ID)

	today := date("2026-10-31")
	sts, err := st.CardStatuses(today)
	if err != nil {
		t.Fatal(err)
	}
	if countIDs(sts, old.ID, cur.ID) != 1 || !hasCard(sts, cur.ID) {
		t.Errorf("CardStatuses 가 해지 카드를 포함: %+v", sts)
	}
	paces, err := st.CardPaces(today)
	if err != nil {
		t.Fatal(err)
	}
	if len(paces) != 1 || paces[0].Card.ID != cur.ID {
		t.Errorf("CardPaces 가 해지 카드를 포함")
	}
	rep, err := st.Cashback(2026, 10, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Cards) != 0 || rep.HasCashbackCard {
		t.Errorf("Cashback 이 해지 카드를 포함: %+v", rep.Cards)
	}

	// 해지 취소 → 다시 포함, 해지 → 제외
	if err := st.SetPaymentMethodClosed(old.ID, ""); err != nil {
		t.Fatal(err)
	}
	if sts, _ = st.CardStatuses(today); countIDs(sts, old.ID, cur.ID) != 2 {
		t.Errorf("해지 취소 후 %d장", len(sts))
	}
	if err := st.SetPaymentMethodClosed(old.ID, "2026-10-20"); err != nil {
		t.Fatal(err)
	}
	// 미래 해지일은 아직 사용 중
	if sts, _ = st.CardStatuses(date("2026-10-10")); countIDs(sts, old.ID, cur.ID) != 2 {
		t.Errorf("미래 해지일인데 제외됨: %d장", len(sts))
	}

	// 거래 연결 유지(이름 포함)
	txs, err := st.ListTransactions(TxFilter{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tx := range txs {
		if tx.PaymentMethodID != nil && *tx.PaymentMethodID == old.ID {
			found = true
			if tx.PaymentMethodName != "옛카드" {
				t.Errorf("거래의 결제수단 이름 %q", tx.PaymentMethodName)
			}
		}
	}
	if !found {
		t.Error("해지 뒤 거래 연결이 사라짐")
	}

	// 자금 계획: 해지 카드의 0원 청구서는 빠지고, 값이 있는 청구서는 남는다
	plan, err := st.BuildCashPlan(date("2026-10-31"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var oldBills, curBills int
	for _, b := range plan.NextDueBills {
		switch b.CardID {
		case old.ID:
			oldBills++
			if b.Total == 0 || b.Projected != b.Due {
				t.Errorf("해지 카드 청구서 이상: %+v", b)
			}
		case cur.ID:
			curBills++
		}
	}
	if oldBills != 1 || curBills != 1 {
		t.Errorf("다음 결제일 청구서: 해지 %d, 현 %d (want 1,1)", oldBills, curBills)
	}
	for _, e := range plan.Events {
		if e.Kind == "card" && e.RefID == old.ID && e.Amount == 0 {
			t.Errorf("0원 이벤트: %+v", e)
		}
	}

	// 백업에 closed_at 포함
	path := filepath.Join(t.TempDir(), "b.db")
	if _, err := st.BackupToSQLite(path); err != nil {
		t.Fatal(err)
	}
	lite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lite.Close()
	var closed string
	if err := lite.QueryRow(`SELECT closed_at FROM payment_methods WHERE id=?`, old.ID).Scan(&closed); err != nil {
		t.Fatal(err)
	}
	if closed != "2026-10-20" {
		t.Errorf("백업 closed_at=%q", closed)
	}
}
