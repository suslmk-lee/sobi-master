package store

import "testing"

// 오늘을 2026-10-31 로 고정하고 카드·급여·잔액·계좌 정기결제가 어우러진 계획을 검증한다.
// (TEST_DATABASE_URL 미설정 시 자동 스킵 — 테이블을 TRUNCATE 하므로 실제 DB 를 가리키면 안 된다)
func TestBuildCashPlan(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE subscriptions, subscription_posts, cash_balances, app_settings, custom_holidays RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	var salaryCat int64
	if err := st.queryRow(`SELECT id FROM categories WHERE name='급여' AND parent_id IS NULL`).Scan(&salaryCat); err != nil {
		t.Fatal(err)
	}
	card, err := st.SavePaymentMethod(PaymentMethod{Name: "A카드", Type: "card", BillingDay: 1, CycleStartDay: 1})
	if err != nil {
		t.Fatal(err)
	}
	add := func(date string, amount int64, dir string, pm, cat *int64, paidAt string) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{Date: date, Amount: amount, Direction: dir,
			Merchant: "x", PaymentMethodID: pm, CategoryID: cat, PaidAt: paidAt}); err != nil {
			t.Fatal(err)
		}
	}
	// 급여: 7~9월 300만 + 이번 달 10/8(10일이 토요일, 9일 한글날)
	for _, d := range []string{"2026-07-10", "2026-08-10", "2026-09-10", "2026-10-08"} {
		add(d, 3000000, "income", nil, &salaryCat, "")
	}
	// 카드: 이용기간 9/19~10/18 (결제일 11/1 일요일 → 11/2 출금)
	add("2026-09-18", 50000, "expense", &card.ID, nil, "")            // 기간 밖(이전)
	add("2026-09-25", 100000, "expense", &card.ID, nil, "2026-10-28") // 기간 안, 이미 갚음
	add("2026-10-10", 200000, "expense", &card.ID, nil, "")           // 기간 안
	add("2026-10-19", 70000, "expense", &card.ID, nil, "")            // 다음 청구(10/19~11/18)
	// 잔액 10/20 기준 100만원, 이후 변동
	if _, err := st.AddCashBalance("2026-10-20", 1000000, "테스트"); err != nil {
		t.Fatal(err)
	}
	add("2026-10-22", 30000, "expense", nil, nil, "") // 계좌에서 나간 지출
	add("2026-10-25", 20000, "income", nil, nil, "")  // 부수입
	// 계좌로 나가는 정기결제(월세, 매달 5일)
	if _, err := st.SaveSubscription(Subscription{Name: "월세", Amount: 500000, Cycle: "monthly", BillingDay: 5, Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCashSettings(CashSettings{SalaryDay: 10, Reserve: 50000}); err != nil {
		t.Fatal(err)
	}

	plan, err := st.BuildCashPlan(date("2026-10-31"), nil)
	if err != nil {
		t.Fatal(err)
	}
	eq := func(name string, got, want interface{}) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %v want %v", name, got, want)
		}
	}
	eq("SalaryAuto", plan.SalaryAuto, int64(3000000))
	eq("SalaryBasis", plan.SalaryBasis, "최근 3개월 급여 평균")
	eq("NextSalary", plan.NextSalary, "2026-11-10")
	eq("BalanceAdjust", plan.BalanceAdjust, int64(-110000))
	eq("BalanceNow", plan.BalanceNow, int64(890000))
	eq("NextDue", plan.NextDue, "2026-11-02")
	eq("NextDueTotal", plan.NextDueTotal, int64(200000))
	if len(plan.NextDueBills) != 1 {
		t.Fatalf("NextDueBills %d", len(plan.NextDueBills))
	}
	b := plan.NextDueBills[0]
	eq("bill.Total", b.Total, int64(300000))
	eq("bill.Prepaid", b.Prepaid, int64(100000))
	eq("bill.Period", b.PeriodStart+"~"+b.PeriodEnd, "2026-09-19~2026-10-18")
	eq("bill.Closed", b.Closed, true)
	eq("FollowingDue", plan.FollowingDue, "2026-12-01")
	eq("FollowingTotal", plan.FollowingTotal, int64(70000))
	// 첫 사용일 9/18 → 일평균은 44일(9/18~10/31)로 나눈다: 7만 + 42만/44 × 18일
	eq("FollowingProjected", plan.FollowingProjected, int64(241818))
	eq("AfterNextDue", plan.AfterNextDue, int64(690000))

	wantDates := []string{"2026-11-02", "2026-11-05", "2026-11-10", "2026-12-01", "2026-12-07", "2026-12-10"} // 12/5 월세는 토요일이라 12/7
	wantKinds := []string{"card", "subscription", "salary", "card", "subscription", "salary"}
	wantBal := []int64{690000, 190000, 3190000, 2948182, 2448182, 5448182}
	if len(plan.Events) != len(wantDates) {
		t.Fatalf("이벤트 %d개: %+v", len(plan.Events), plan.Events)
	}
	for i, e := range plan.Events {
		if e.Date != wantDates[i] || e.Kind != wantKinds[i] || e.Balance != wantBal[i] {
			t.Errorf("event %d: %+v want %s %s %d", i, e, wantDates[i], wantKinds[i], wantBal[i])
		}
	}
	eq("Horizon", plan.Horizon, "2026-12-10")
	eq("LowestBalance", plan.LowestBalance, int64(190000))
	eq("LowestDate", plan.LowestDate, "2026-11-05")
	eq("SafeToSpend", plan.SafeToSpend, int64(140000))
	eq("DaysToSalary", plan.DaysToSalary, 10)
	eq("DailyAllowance", plan.DailyAllowance, int64(14000))

	// 잔액 기록이 없으면 0 에서 출발하고 Balance 는 nil
	if _, err := st.db.Exec(`TRUNCATE cash_balances`); err != nil {
		t.Fatal(err)
	}
	plan, err = st.BuildCashPlan(date("2026-10-31"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Balance != nil || plan.BalanceNow != 0 {
		t.Errorf("잔액 없음인데 Balance=%v Now=%d", plan.Balance, plan.BalanceNow)
	}
}

func TestCashSettingsAndBalances(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE cash_balances, app_settings RESTART IDENTITY`); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetCashSettings()
	if err != nil || got != (CashSettings{SalaryDay: 10}) {
		t.Fatalf("기본값: %+v %v", got, err)
	}
	want := CashSettings{SalaryDay: 25, SalaryAmount: 3500000, Reserve: 100000}
	if err := st.SaveCashSettings(want); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetCashSettings(); got != want {
		t.Fatalf("저장값: %+v", got)
	}
	if err := st.SaveCashSettings(CashSettings{SalaryDay: 32}); err == nil {
		t.Error("월급일 32 가 통과함")
	}
	if _, err := st.AddCashBalance("2026/10/01", 1, ""); err == nil {
		t.Error("잘못된 날짜 형식이 통과함")
	}
	a, _ := st.AddCashBalance("2026-10-01", 100, "a")
	b, _ := st.AddCashBalance("2026-10-01", 200, "b")
	c, _ := st.AddCashBalance("2026-09-01", 300, "c")
	list, _ := st.ListCashBalances()
	if len(list) != 3 || list[0].ID != b.ID || list[1].ID != a.ID || list[2].ID != c.ID {
		t.Fatalf("정렬: %+v", list)
	}
	if err := st.DeleteCashBalance(b.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := st.latestCashBalance(); l == nil || l.ID != a.ID {
		t.Fatalf("최신 기록: %+v", l)
	}
}
