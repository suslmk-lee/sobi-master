package store

import "testing"

// TestCashbackRates 는 기본/추가 적립 계산과 기한 판정을 검증한다.
// (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestCashbackRates(t *testing.T) {
	st := openTestStore(t)

	// 결제액 1% 기본, 5일 내 납부 시 1% 추가
	card, err := st.SavePaymentMethod(PaymentMethod{
		Name: "줍줍카드", Type: "card", CycleStartDay: 1,
		CashbackBp: 100, CashbackBonusBp: 100, CashbackBonusDays: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	add := func(date string, amt int64, paidAt string) int64 {
		t.Helper()
		id, err := st.AddTransaction(Transaction{
			Date: date, Amount: amt, Direction: "expense", Merchant: "가맹점",
			PaymentMethodID: &card.ID, PaidAt: paidAt,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	// 오늘을 6/15 로 두고 판정한다
	today := date("2026-06-15")
	earnedID := add("2026-06-05", 100000, "2026-06-08") // 3일 만에 납부 → 추가 획득
	missedID := add("2026-06-01", 50000, "2026-06-10")  // 9일 만에 납부 → 기한 초과
	pendingID := add("2026-06-13", 30000, "")           // 미납부, 기한 6/18 → 아직 가능
	staleID := add("2026-06-02", 20000, "")             // 미납부, 기한 6/07 → 놓침

	rep, err := st.Cashback(2026, 6, today)
	if err != nil {
		t.Fatalf("Cashback: %v", err)
	}
	if !rep.HasCashbackCard || len(rep.Cards) != 1 {
		t.Fatalf("카드 집계 오류: %+v", rep.Cards)
	}
	c := rep.Cards[0]

	// 기본 적립 = 전체 결제액의 1% = (100000+50000+30000+20000) * 1% = 2000
	if c.TotalAmount != 200000 || c.TotalBase != 2000 {
		t.Errorf("기본 적립 오류: amount=%d base=%d, want 200000/2000", c.TotalAmount, c.TotalBase)
	}
	// 추가 획득 = 기한 내 납부한 10만의 1% = 1000
	if c.TotalBonus != 1000 {
		t.Errorf("추가 적립=%d, want 1000", c.TotalBonus)
	}
	// 놓친 추가분 = 기한 초과 납부(50000) + 기한 지난 미납부(20000) 의 1% = 500 + 200
	if c.MissedBonus != 700 {
		t.Errorf("놓친 추가분=%d, want 700", c.MissedBonus)
	}
	// 아직 받을 수 있는 추가분 = 미납부 30000 의 1% = 300
	if c.PendingBonus != 300 {
		t.Errorf("남은 추가분=%d, want 300", c.PendingBonus)
	}

	byID := map[int64]CashbackItem{}
	for _, it := range c.Items {
		byID[it.TxID] = it
	}
	if got := byID[earnedID]; got.Status != CashbackEarned || got.Bonus != 1000 || got.PaidInDays != 3 {
		t.Errorf("획득 건 오류: %+v", got)
	}
	if got := byID[missedID]; got.Status != CashbackMissed || got.Bonus != 0 {
		t.Errorf("기한 초과 납부 건 오류: %+v", got)
	}
	if got := byID[pendingID]; got.Status != CashbackPending || got.DaysLeft != 3 || got.Deadline != "2026-06-18" {
		t.Errorf("기한 남은 건 오류: %+v", got)
	}
	if got := byID[staleID]; got.Status != CashbackMissed {
		t.Errorf("기한 지난 미납부 건 오류: %+v", got)
	}

	// Pending 목록에는 아직 받을 수 있는 건만, 기한 급한 순으로
	if len(rep.Pending) != 1 || rep.Pending[0].TxID != pendingID {
		t.Errorf("Pending 목록 오류: %+v", rep.Pending)
	}

	// 납부 처리하면 추가분을 받은 것으로 바뀐다
	if err := st.SetPaidAt([]int64{pendingID}, "2026-06-15"); err != nil {
		t.Fatal(err)
	}
	rep, err = st.Cashback(2026, 6, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Pending) != 0 {
		t.Errorf("납부 처리 후에도 Pending 남음: %+v", rep.Pending)
	}
	if rep.Cards[0].TotalBonus != 1300 {
		t.Errorf("납부 처리 후 추가 적립=%d, want 1300", rep.Cards[0].TotalBonus)
	}
}

// TestCashbackNoBonusCard 는 추가 적립이 없는 카드도 기본 적립만 정상 집계되는지 본다.
func TestCashbackNoBonusCard(t *testing.T) {
	st := openTestStore(t)
	card, err := st.SavePaymentMethod(PaymentMethod{
		Name: "기본만카드", Type: "card", CycleStartDay: 1, CashbackBp: 50, // 0.5%
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTransaction(Transaction{
		Date: "2026-06-05", Amount: 100000, Direction: "expense",
		Merchant: "가맹점", PaymentMethodID: &card.ID,
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := st.Cashback(2026, 6, date("2026-06-15"))
	if err != nil {
		t.Fatal(err)
	}
	c := rep.Cards[0]
	if c.TotalBase != 500 || c.TotalBonus != 0 || c.PendingBonus != 0 || c.MissedBonus != 0 {
		t.Errorf("기본만 카드 집계 오류: %+v", c)
	}
	if len(rep.Pending) != 0 {
		t.Errorf("추가 적립 없는 카드는 Pending 이 없어야 함: %+v", rep.Pending)
	}
}
