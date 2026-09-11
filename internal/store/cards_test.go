package store

import (
	"os"
	"testing"
	"time"
)

// openTestStore 는 TEST_DATABASE_URL(Postgres)로 접속해 깨끗한 상태에서 시작한다.
// 환경변수가 없으면 테스트를 건너뛴다.
func openTestStore(t *testing.T) *Store {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL 미설정 — DB 테스트 건너뜀")
	}
	st, err := OpenAt(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.db.Exec(`TRUNCATE members, categories, payment_methods, rules, transactions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureDefaults(); err != nil {
		t.Fatal(err)
	}
	return st
}

func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestPerfPeriod(t *testing.T) {
	cases := []struct {
		today    string
		startDay int
		start    string
		end      string
	}{
		// 매월 1일 시작 → 그 달 1일~말일
		{"2026-06-10", 1, "2026-06-01", "2026-06-30"},
		// 15일 시작, 오늘이 10일 → 전월 15일~당월 14일
		{"2026-06-10", 15, "2026-05-15", "2026-06-14"},
		// 15일 시작, 오늘이 20일 → 당월 15일~익월 14일
		{"2026-06-20", 15, "2026-06-15", "2026-07-14"},
		// 31일 시작인데 2월 → 말일로 줄여서 계산
		{"2026-03-01", 31, "2026-02-28", "2026-03-30"},
	}
	for _, c := range cases {
		s, e := perfPeriod(date(c.today), c.startDay)
		if s.Format("2006-01-02") != c.start || e.Format("2006-01-02") != c.end {
			t.Errorf("perfPeriod(%s, %d) = %s~%s, want %s~%s",
				c.today, c.startDay, s.Format("2006-01-02"), e.Format("2006-01-02"), c.start, c.end)
		}
	}
}

func TestCardStatus(t *testing.T) {
	st := openTestStore(t)

	card, err := st.SavePaymentMethod(PaymentMethod{
		Name: "신한 딥드림", Type: "card", Issuer: "신한카드",
		BillingDay: 25, CycleStartDay: 1, PerfTarget: 300000,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, amt := range []int64{120000, 100000} {
		if _, err := st.AddTransaction(Transaction{
			Date: "2026-06-05", Amount: amt, Direction: "expense",
			Merchant: "이마트", PaymentMethodID: &card.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// 기간 밖 거래는 집계에서 빠져야 한다
	if _, err := st.AddTransaction(Transaction{
		Date: "2026-05-20", Amount: 999999, Direction: "expense",
		Merchant: "이마트", PaymentMethodID: &card.ID,
	}); err != nil {
		t.Fatal(err)
	}

	sts, err := st.CardStatuses(date("2026-06-10"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sts) == 0 {
		t.Fatal("카드 현황 없음")
	}
	var got *CardStatus
	for i := range sts {
		if sts[i].Card.ID == card.ID {
			got = &sts[i]
		}
	}
	if got == nil {
		t.Fatal("등록한 카드를 찾지 못함")
	}
	if got.Spent != 220000 {
		t.Errorf("Spent = %d, want 220000", got.Spent)
	}
	if got.Achieved || got.Remaining != 80000 {
		t.Errorf("Achieved=%v Remaining=%d, want false/80000", got.Achieved, got.Remaining)
	}

	bd, err := st.CardBreakdown(card.ID, got.PeriodStart, got.PeriodEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(bd.ByMerchant) != 1 || bd.ByMerchant[0].Name != "이마트" || bd.ByMerchant[0].Amount != 220000 {
		t.Errorf("가맹점 집계 오류: %+v", bd.ByMerchant)
	}
}

// TestCardPerfExcludesMarked 는 "실적 제외" 표시된 결제가 카드 실적에서만 빠지고
// 일반 지출 집계에는 그대로 잡히는지 검증한다. (TEST_DATABASE_URL 미설정 시 스킵)
func TestCardPerfExcludesMarked(t *testing.T) {
	st := openTestStore(t)

	card, err := st.SavePaymentMethod(PaymentMethod{
		Name: "실적테스트카드", Type: "card", CycleStartDay: 1, PerfTarget: 300000,
	})
	if err != nil {
		t.Fatal(err)
	}
	add := func(amt int64, exclude bool) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{
			Date: "2026-06-05", Amount: amt, Direction: "expense", Merchant: "가맹점",
			PaymentMethodID: &card.ID, ExcludePerf: exclude,
		}); err != nil {
			t.Fatal(err)
		}
	}
	add(200000, false) // 실적 포함
	add(150000, true)  // 세금 등 — 실적 제외

	sts, err := st.CardStatuses(date("2026-06-10"))
	if err != nil {
		t.Fatal(err)
	}
	var got *CardStatus
	for i := range sts {
		if sts[i].Card.ID == card.ID {
			got = &sts[i]
		}
	}
	if got == nil {
		t.Fatal("등록한 카드를 찾지 못함")
	}
	// 실적에는 20만만 잡히고, 제외분 15만은 따로 보고된다
	if got.Spent != 200000 || got.Excluded != 150000 {
		t.Errorf("Spent=%d Excluded=%d, want 200000/150000", got.Spent, got.Excluded)
	}
	// 20만 < 목표 30만 → 미달 (제외분을 더하면 35만이라 잘못 달성 처리될 수 있음)
	if got.Achieved || got.Remaining != 100000 {
		t.Errorf("Achieved=%v Remaining=%d, want false/100000", got.Achieved, got.Remaining)
	}

	// 일반 지출 집계에는 제외분도 포함돼야 한다 (실제로 나간 돈이므로)
	sum, err := st.MonthlySummary(2026, 6)
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalExpense != 350000 {
		t.Errorf("월 지출 합계=%d, want 350000 (실적 제외분 포함)", sum.TotalExpense)
	}

	// 실적 페이스(누적)도 제외분을 빼야 한다
	paces, err := st.CardPaces(date("2026-06-10"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paces {
		if p.Card.ID == card.ID && p.Spent != 200000 {
			t.Errorf("CardPace Spent=%d, want 200000", p.Spent)
		}
	}

	// 일괄 해제하면 실적에 다시 포함된다
	txs, err := st.ListTransactions(TxFilter{Month: "2026-06"})
	if err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	for _, tx := range txs {
		ids = append(ids, tx.ID)
	}
	if err := st.SetExcludePerf(ids, false); err != nil {
		t.Fatal(err)
	}
	sts, _ = st.CardStatuses(date("2026-06-10"))
	for i := range sts {
		if sts[i].Card.ID == card.ID {
			if sts[i].Spent != 350000 || sts[i].Excluded != 0 {
				t.Errorf("해제 후 Spent=%d Excluded=%d, want 350000/0", sts[i].Spent, sts[i].Excluded)
			}
		}
	}
}
