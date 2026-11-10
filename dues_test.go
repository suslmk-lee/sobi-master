package main

import (
	"testing"
	"time"

	"sobi/internal/store"
)

func TestAssignDues(t *testing.T) {
	apt := store.Subscription{ID: 1, Name: "아파트 모임", Merchant: "아파트친목회", Amount: 20000,
		Kind: store.KindDues, PaidThrough: "2026-10", PaidThroughAt: "2026-11-11", PaidThroughTx: 100}
	club := store.Subscription{ID: 2, Name: "동아리", Merchant: "김총무", Amount: 30000,
		Kind: store.KindDues, PaidThrough: "2026-06", PaidThroughAt: "2026-11-11", PaidThroughTx: 100}
	tx := func(id int64, date, merchant string, amount int64) store.Transaction {
		return store.Transaction{ID: id, Date: date, Merchant: merchant, Amount: amount, Direction: "expense"}
	}
	txs := []store.Transaction{
		tx(90, "2026-11-11", "아파트친목회", 20000),  // 기준 때 이미 있던 거래 — "10월분까지"에 들어 있다
		tx(101, "2026-10-30", "아파트친목회", 20000), // 기준 뒤에 가져온 지난 거래(날짜가 기준 전) — 세지 않음
		tx(102, "2026-11-12", "아파트친목회", 20000), // 회비
		tx(103, "2026-11-13", "김총무", 90000),     // 석 달치 몰아서
		tx(104, "2026-11-14", "김총무", 45000),     // 회식비? 금액이 안 맞음 → 확인 필요
		tx(105, "2026-11-15", "김총무", 60000),     // 회비 아님으로 뺀 거래
		tx(106, "2026-11-16", "김총무", 15000),     // 금액은 안 맞지만 직접 넣은 거래
		tx(107, "2026-11-17", "다른 가게", 30000),   // 받는 사람이 다름
		{ID: 108, Date: "2026-11-18", Merchant: "김총무", Amount: 30000, Direction: "income"},
		{ID: 109, Date: "2026-11-19", Merchant: "아파트친목회", Amount: 20000, Direction: "expense",
			SubscriptionName: "다른 정기결제"}, // 이미 정기결제에 묶인 거래
	}
	links := map[int64]store.DuesLink{
		105: {SubscriptionID: 2, TransactionID: 105, State: store.DuesLinkExclude},
		106: {SubscriptionID: 2, TransactionID: 106, State: store.DuesLinkInclude},
	}
	got := assignDues([]store.Subscription{club, apt}, txs, links)

	type p struct {
		id     int64
		state  string
		manual bool
	}
	flat := func(ps []store.DuesPayment) []p {
		var out []p
		for _, x := range ps {
			out = append(out, p{x.TransactionID, x.State, x.Manual})
		}
		return out
	}
	check := func(name string, got, want []p) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %+v want %+v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s[%d]: got %+v want %+v", name, i, got[i], want[i])
			}
		}
	}
	check("아파트", flat(got[1]), []p{{102, store.DuesCounted, false}})
	check("동아리", flat(got[2]), []p{
		{103, store.DuesCounted, false},
		{104, store.DuesReview, false},
		{105, store.DuesExcluded, true},
		{106, store.DuesCounted, true},
	})
}

func TestAssignDuesOverlap(t *testing.T) {
	// 받는 사람 이름이 같은 회비 둘: 금액이 맞는 쪽으로, 둘 다 맞으면 먼저 만든 쪽으로
	a := store.Subscription{ID: 5, Merchant: "홍길동", Amount: 20000, Kind: store.KindDues, PaidThrough: "2026-10", PaidThroughAt: "2026-11-01"}
	b := store.Subscription{ID: 3, Merchant: "홍길동", Amount: 30000, Kind: store.KindDues, PaidThrough: "2026-10", PaidThroughAt: "2026-11-01"}
	txs := []store.Transaction{
		{ID: 1, Date: "2026-11-02", Merchant: "홍길동", Amount: 40000, Direction: "expense"}, // a 만 맞음
		{ID: 2, Date: "2026-11-03", Merchant: "홍길동", Amount: 60000, Direction: "expense"}, // 둘 다 맞음 → b(id 3)
		{ID: 3, Date: "2026-11-04", Merchant: "홍길동", Amount: 7000, Direction: "expense"},  // 둘 다 안 맞음 → b 의 확인 필요
	}
	got := assignDues([]store.Subscription{a, b}, txs, nil)
	if len(got[5]) != 1 || got[5][0].TransactionID != 1 {
		t.Fatalf("a: %+v", got[5])
	}
	if len(got[3]) != 2 || got[3][0].TransactionID != 2 || got[3][1].State != store.DuesReview {
		t.Fatalf("b: %+v", got[3])
	}
}

func TestDuesAmountFits(t *testing.T) {
	s := store.Subscription{Amount: 20000, NextAmount: 25000, NextAmountYM: "2027-01"}
	for _, c := range []struct {
		amount int64
		ok     bool
	}{
		{20000, true}, {60000, true}, {25000, true}, {45000, false}, {720000, true}, {740000, false}, {0, false},
	} {
		if got := duesAmountFits(s, store.Transaction{Date: "2026-11-01", Amount: c.amount}); got != c.ok {
			t.Errorf("%d: got %v", c.amount, got)
		}
	}
}

// 회비 전체 흐름: 등록 → 가져온 이체 자동 반영·확인 필요 → 회비 아님/되돌리기 → 거래내역 표시 →
// 정기결제 화면에서 빠짐 → 자금 계획 출금 → 납부 기록 삭제·되돌리기.
func TestDuesFlow(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	ym := func(n int) string {
		return time.Date(now.Year(), now.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
	}
	today := now.Format("2006-01-02")

	pms, err := a.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	var bank int64
	for _, p := range pms {
		if p.Type == "bank" {
			bank = p.ID
		}
	}
	club, err := a.SaveSubscription(store.Subscription{
		Name: "동아리", Merchant: "김총무", Kind: store.KindDues, Amount: 30000, BillingDay: 1,
		HolidayRule: store.HolidayNone, PaidThrough: ym(-5), PaymentMethodID: &bank, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveSubscription(store.Subscription{Name: "넷플릭스", Amount: 17000, BillingDay: 5, Active: true}); err != nil {
		t.Fatal(err)
	}

	add := func(merchant string, amount int64) int64 {
		id, err := a.st.AddTransaction(store.Transaction{Date: today, Amount: amount, Direction: "expense", Merchant: merchant, Source: "import"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	lump := add("김총무", 60000) // 두 달치
	odd := add("김총무", 45000)  // 확인 필요
	add("넷플릭스", 17000)

	book := func() store.DuesStatus {
		t.Helper()
		list, err := a.ListDues()
		if err != nil || len(list) != 1 {
			t.Fatalf("ListDues: %+v %v", list, err)
		}
		return list[0]
	}
	d := book()
	// 두 달치가 채워져 ym(-3)까지. 남은 ym(-2), ym(-1), 이번 달 — 이번 달은 1일이 납부일이라 오늘이 1일이면 아직 미납 아님
	wantOverdue := 3
	if now.Day() == 1 {
		wantOverdue = 2
	}
	if d.PaidTotal != 60000 || d.PaidThrough != ym(-3) || d.OverdueMonths != wantOverdue || d.Overdue != int64(wantOverdue)*30000 {
		t.Fatalf("장부: total=%d through=%s overdue=%d %d", d.PaidTotal, d.PaidThrough, d.OverdueMonths, d.Overdue)
	}
	states := map[int64]string{}
	for _, p := range d.Payments {
		states[p.TransactionID] = p.State
	}
	if states[lump] != store.DuesCounted || states[odd] != store.DuesReview || len(states) != 2 {
		t.Fatalf("거래 판정: %+v", states)
	}

	// 거래내역: 장부에 들어간 거래에만 회비 이름
	txs, err := a.ListTransactions(store.TxFilter{Month: now.Format("2006-01")})
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range txs {
		want := ""
		if tx.ID == lump {
			want = "동아리"
		}
		if tx.DuesName != want {
			t.Errorf("거래 %d(%s) DuesName=%q", tx.ID, tx.Merchant, tx.DuesName)
		}
	}

	// 회비 아님 → 장부에서 빠짐, 되돌리면 다시
	if err := a.SetDuesLink(club.ID, lump, store.DuesLinkExclude); err != nil {
		t.Fatal(err)
	}
	if d := book(); d.PaidTotal != 0 {
		t.Fatalf("회비 아님 후 total=%d", d.PaidTotal)
	}
	if err := a.ClearDuesLink(lump); err != nil {
		t.Fatal(err)
	}
	// 확인 필요 거래를 회비로 넣기
	if err := a.SetDuesLink(club.ID, odd, store.DuesLinkInclude); err != nil {
		t.Fatal(err)
	}
	if d := book(); d.PaidTotal != 105000 || d.Credit != 15000 {
		t.Fatalf("넣기 후 total=%d credit=%d", d.PaidTotal, d.Credit)
	}

	// 정기결제 화면에는 회비가 없다
	sm, err := a.GetSubscriptionMonth(now.Year(), int(now.Month()))
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range sm.Items {
		if it.Sub.Kind == store.KindDues {
			t.Fatal("정기결제 목록에 회비가 나옴")
		}
	}

	// 자금 계획: 다음 납부일(다음 달 1일)에 밀린 돈 + 그 달 몫
	d = book()
	plan, err := a.GetCashPlan()
	if err != nil {
		t.Fatal(err)
	}
	var dues []store.CashEvent
	for _, e := range plan.Events {
		if e.Kind == "dues" {
			dues = append(dues, e)
		}
	}
	if len(dues) == 0 || dues[0].RefID != club.ID || dues[0].Date != d.NextDate || dues[0].Amount != -(d.Overdue+d.NextDue) {
		t.Fatalf("자금 계획 회비: %+v (장부 %+v)", dues, d)
	}

	// 납부 기록을 지웠다 되돌리면 연결도 돌아온다
	if err := a.AddDuesPayment(club.ID, today, 15000, "반달치"); err != nil {
		t.Fatal(err)
	}
	d = book()
	var rec int64
	for _, p := range d.Payments {
		if p.Amount == 15000 && p.Manual {
			rec = p.TransactionID
		}
	}
	if rec == 0 || d.PaidTotal != 120000 {
		t.Fatalf("납부 기록: %+v", d)
	}
	if err := a.DeleteTransaction(rec); err != nil {
		t.Fatal(err)
	}
	if d := book(); d.PaidTotal != 105000 {
		t.Fatalf("삭제 후 total=%d", d.PaidTotal)
	}
	if _, err := a.UndoDelete(); err != nil {
		t.Fatal(err)
	}
	if d := book(); d.PaidTotal != 120000 {
		t.Fatalf("되돌린 뒤 total=%d (연결 복원 안 됨)", d.PaidTotal)
	}
}
