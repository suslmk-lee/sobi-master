package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// 교통카드: 저장 정리 → 출금일이 지나면 어림값 기록(한 번만) → 실제 금액으로 고침 → 안 탄 달 → 자금 계획에서 빠짐 → 백업.
func TestTransitDB(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE subscriptions, subscription_posts, custom_holidays RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	old := nowFunc
	t.Cleanup(func() { nowFunc = old })
	nowFunc = func() time.Time { return time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC) }

	// 결제수단: K뱅크 카드는 결제일 없이(카드값 계산에서 빠짐), 비교용으로 계좌 하나
	kbank, err := st.AddPaymentMethod("K뱅크 K-패스", "card")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveSubscription(Subscription{Name: "K-패스", Kind: KindTransit, Active: true}); err == nil {
		t.Fatal("예상 요금 없이 저장됨")
	}
	sub, err := st.SaveSubscription(Subscription{
		Name: "K-패스", Kind: KindTransit, Amount: 60000, BillingDay: 10, DayRule: "day", HolidayRule: "next",
		PaymentMethodID: &kbank.ID, Active: true, Cycle: "yearly", StartYM: "2026-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSubscription(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindTransit || got.Cycle != "monthly" || got.StartYM != "" || !got.AutoPost || got.AutoSince != "2026-11" {
		t.Fatalf("저장 정리: %+v", got)
	}
	if got.DueIn("2026-11") {
		t.Fatal("교통카드가 일반 정기결제 대조·자동 등록에 들어감")
	}

	// 출금일(11/10) 전에는 아무것도 기록하지 않는다
	if n, err := st.RunTransit(date("2026-11-05")); err != nil || n != 0 {
		t.Fatalf("출금일 전: %d %v", n, err)
	}
	// 지나면 어림값(설정한 60,000원, 환급 모름)으로 한 번만
	if n, err := st.RunTransit(date("2026-11-14")); err != nil || n != 1 {
		t.Fatalf("자동 기록: %d %v", n, err)
	}
	if n, err := st.RunTransit(date("2026-11-14")); err != nil || n != 0 {
		t.Fatalf("두 번 기록: %d %v", n, err)
	}
	months, err := st.TransitMonths(sub.ID)
	if err != nil || len(months) != 1 {
		t.Fatalf("기록: %v %+v", err, months)
	}
	m := months[0]
	if m.YM != "2026-11" || m.Date != "2026-11-10" || m.Fare != 60000 || m.Refund != 0 || !m.Estimated || m.TransactionID == nil {
		t.Fatalf("어림 기록: %+v", m)
	}
	tx, err := st.GetTransaction(*m.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Amount != 60000 || tx.Direction != "expense" || tx.Source != SourceTransit || tx.Date != "2026-11-10" ||
		tx.PaymentMethodID == nil || *tx.PaymentMethodID != kbank.ID {
		t.Fatalf("어림 거래: %+v", tx)
	}

	// 실제 금액으로 고침: 실부담 한 건, 추정 표시가 사라진다
	if _, err := st.SaveTransitMonth(sub.ID, "2026-11", 62350, 70000); err == nil {
		t.Fatal("환급이 요금보다 큰데 저장됨")
	}
	if _, err := st.SaveTransitMonth(sub.ID, "2026-12", 1, 0); err == nil {
		t.Fatal("오지 않은 달이 저장됨")
	}
	saved, err := st.SaveTransitMonth(sub.ID, "2026-11", 62350, 12470)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Net != 49880 || saved.TransactionID == nil || *saved.TransactionID != *m.TransactionID {
		t.Fatalf("고친 기록: %+v", saved)
	}
	if tx, _ = st.GetTransaction(*saved.TransactionID); tx.Amount != 49880 || tx.Memo != "요금 62,350원 · 환급 12,470원" {
		t.Fatalf("고친 거래: %d %q", tx.Amount, tx.Memo)
	}
	months, _ = st.TransitMonths(sub.ID)
	if months[0].Estimated {
		t.Fatal("고쳤는데 추정으로 남음")
	}

	// 지난 달(자동 기록 시작 전)도 직접 넣을 수 있다 — 10/10 은 토요일이라 10/12
	oct, err := st.SaveTransitMonth(sub.ID, "2026-10", 58000, 11600)
	if err != nil || oct.Date != "2026-10-12" {
		t.Fatalf("지난 달: %+v %v", oct, err)
	}

	// 안 탄 달: 거래를 지우고 기록만 남긴다 → 다시 넣으면 거래를 새로 만든다
	zero, err := st.SaveTransitMonth(sub.ID, "2026-11", 0, 0)
	if err != nil || zero.TransactionID != nil {
		t.Fatalf("안 탄 달: %+v %v", zero, err)
	}
	if _, err := st.GetTransaction(*saved.TransactionID); err == nil {
		t.Fatal("안 탄 달인데 거래가 남음")
	}
	again, err := st.SaveTransitMonth(sub.ID, "2026-11", 62350, 12470)
	if err != nil || again.TransactionID == nil {
		t.Fatalf("다시 기록: %+v %v", again, err)
	}
	// 사용자가 거래내역에서 지워도 기록은 남고(자동 기록이 다시 만들지 않는다), 이름 표시는 사라진다
	if err := st.DeleteTransaction(*again.TransactionID); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.RunTransit(date("2026-11-14")); n != 0 {
		t.Fatal("지운 달을 다시 자동 기록함")
	}
	if names, _ := st.TransitTxNames(); len(names) != 1 { // 10월분만
		t.Fatalf("이름 표시: %v", names)
	}

	// 자금 계획: 교통카드 거래는 다른 통장에서 빠지므로 잔액 계산에 들어가지 않는다(계좌로 골라도)
	bank, err := st.AddPaymentMethod("K뱅크 통장", "bank")
	if err != nil {
		t.Fatal(err)
	}
	sub.PaymentMethodID = &bank.ID
	if sub, err = st.SaveSubscription(sub); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveTransitMonth(sub.ID, "2026-09", 50000, 10000); err != nil {
		t.Fatal(err)
	}
	adj, err := st.balanceAdjust("2026-08-31", date("2026-11-14"), nil)
	if err != nil || adj != 0 {
		t.Fatalf("잔액 보정에 교통카드가 들어감: %d %v", adj, err)
	}

	// 다시 켤 때 쉬는 동안의 달을 한꺼번에 채우지 않는다
	nowFunc = func() time.Time { return time.Date(2027, 2, 3, 9, 0, 0, 0, time.UTC) }
	sub.Active = false
	if sub, err = st.SaveSubscription(sub); err != nil {
		t.Fatal(err)
	}
	sub.Active = true
	if sub, err = st.SaveSubscription(sub); err != nil {
		t.Fatal(err)
	}
	if sub.AutoSince != "2027-02" {
		t.Fatalf("다시 켠 뒤 시작 월: %s", sub.AutoSince)
	}

	// 백업에 월 기록이 들어간다
	path := filepath.Join(t.TempDir(), "b.db")
	counts, err := st.BackupToSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if counts["transit_months"] != 3 {
		t.Fatalf("백업 counts: %+v", counts)
	}
	lite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lite.Close()
	var fare, refund int64
	if err := lite.QueryRow(`SELECT fare, refund FROM transit_months WHERE ym='2026-10'`).Scan(&fare, &refund); err != nil || fare != 58000 || refund != 11600 {
		t.Fatalf("백업 내용: %d %d %v", fare, refund, err)
	}
}
