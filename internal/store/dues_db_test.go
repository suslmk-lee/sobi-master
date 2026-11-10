package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// 회비 저장: 기준(paid_through_at/tx)은 서버가 정하고, "어디까지 냈나"를 바꿀 때만 다시 잡는다.
func TestDuesSaveAndLinks(t *testing.T) {
	st := openTestStore(t)
	old := nowFunc
	t.Cleanup(func() { nowFunc = old })
	nowFunc = func() time.Time { return time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC) }

	before, err := st.AddTransaction(Transaction{Date: "2026-11-11", Amount: 30000, Direction: "expense", Merchant: "김총무", Source: "import"})
	if err != nil {
		t.Fatal(err)
	}

	// 검증
	if _, err := st.SaveSubscription(Subscription{Name: "동아리", Kind: KindDues, Amount: 30000, Active: true}); err == nil {
		t.Fatal("어디까지 냈나 없이 저장됨")
	}
	if _, err := st.SaveSubscription(Subscription{Name: "동아리", Kind: KindDues, PaidThrough: "2026-06", Active: true}); err == nil {
		t.Fatal("회비 0원으로 저장됨")
	}

	sub, err := st.SaveSubscription(Subscription{
		Name: "동아리", Merchant: "김총무", Kind: KindDues, Amount: 30000, BillingDay: 10,
		PaidThrough: "2026-06", Active: true, AutoPost: true, Cycle: "yearly", StartYM: "2026-01",
		PaidThroughAt: "1999-01-01", PaidThroughTx: 999, // 클라이언트 값은 무시
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSubscription(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindDues || got.PaidThrough != "2026-06" || got.PaidThroughAt != "2026-11-11" || got.PaidThroughTx != before {
		t.Fatalf("기준: %+v", got)
	}
	if got.AutoPost || got.Cycle != "monthly" || got.StartYM != "" {
		t.Fatalf("회비 정리: auto=%v cycle=%s start=%s", got.AutoPost, got.Cycle, got.StartYM)
	}
	if got.DueIn("2026-11") {
		t.Fatal("회비가 정기결제 대조에 들어감")
	}

	// 납부 기록: 거래 + 넣음 연결
	txID, err := st.AddDuesPayment(sub.ID, "2026-11-12", 90000, "7~9월분")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := st.GetTransaction(txID)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Merchant != "김총무" || tx.Amount != 90000 || tx.Direction != "expense" || tx.Memo != "7~9월분" {
		t.Fatalf("납부 거래: %+v", tx)
	}
	links, err := st.DuesLinks()
	if err != nil {
		t.Fatal(err)
	}
	if l := links[txID]; l.State != DuesLinkInclude || l.SubscriptionID != sub.ID {
		t.Fatalf("연결: %+v", links)
	}

	// 이름만 고치면 기준은 그대로
	nowFunc = func() time.Time { return time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC) }
	got.Name = "동아리 동기모임"
	if _, err := st.SaveSubscription(got); err != nil {
		t.Fatal(err)
	}
	again, _ := st.GetSubscription(sub.ID)
	if again.PaidThroughAt != "2026-11-11" || again.PaidThroughTx != before {
		t.Fatalf("이름만 고쳤는데 기준이 바뀜: %+v", again)
	}
	// "어디까지 냈나"를 바꾸면 기준을 지금으로
	again.PaidThrough = "2026-09"
	if _, err := st.SaveSubscription(again); err != nil {
		t.Fatal(err)
	}
	moved, _ := st.GetSubscription(sub.ID)
	if moved.PaidThroughAt != "2026-11-20" || moved.PaidThroughTx != txID {
		t.Fatalf("기준 이동: %+v", moved)
	}

	// 회비 아님 / 되돌리기
	other, _ := st.AddTransaction(Transaction{Date: "2026-11-21", Amount: 45000, Direction: "expense", Merchant: "김총무"})
	if err := st.SetDuesLink(sub.ID, other, DuesLinkExclude); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDuesLink(sub.ID, other, "bogus"); err == nil {
		t.Fatal("잘못된 상태가 저장됨")
	}
	inc, _ := st.AddTransaction(Transaction{Date: "2026-11-21", Amount: 1000, Direction: "income", Merchant: "김총무"})
	if err := st.SetDuesLink(sub.ID, inc, DuesLinkInclude); err == nil {
		t.Fatal("수입 거래가 회비로 연결됨")
	}
	byTx, err := st.DuesLinksFor([]int64{other, txID})
	if err != nil || len(byTx) != 2 || byTx[other].State != DuesLinkExclude {
		t.Fatalf("DuesLinksFor: %+v %v", byTx, err)
	}
	if err := st.ClearDuesLink(other); err != nil {
		t.Fatal(err)
	}
	links, _ = st.DuesLinks()
	if _, ok := links[other]; ok {
		t.Fatal("되돌리기 실패")
	}

	// 거래를 지우면 연결도 지워지고, 되살린 거래에 연결을 복원할 수 있다
	saved := byTx[txID]
	if err := st.DeleteTransaction(txID); err != nil {
		t.Fatal(err)
	}
	links, _ = st.DuesLinks()
	if _, ok := links[txID]; ok {
		t.Fatal("거래를 지웠는데 연결이 남음")
	}
	newID, _ := st.AddTransaction(tx)
	if err := st.RestoreDuesLink(newID, saved); err != nil {
		t.Fatal(err)
	}
	links, _ = st.DuesLinks()
	if links[newID].State != DuesLinkInclude {
		t.Fatalf("복원 실패: %+v", links)
	}

	// 일반 정기결제로 바꾸면 회비 값이 지워진다
	moved.Kind = ""
	if _, err := st.SaveSubscription(moved); err != nil {
		t.Fatal(err)
	}
	plain, _ := st.GetSubscription(sub.ID)
	if plain.Kind != "" || plain.PaidThrough != "" || plain.PaidThroughTx != 0 {
		t.Fatalf("일반 정기결제: %+v", plain)
	}
	if err := st.SetDuesLink(sub.ID, other, DuesLinkInclude); err == nil {
		t.Fatal("회비가 아닌데 연결됨")
	}
}

func TestDuesBackup(t *testing.T) {
	st := openTestStore(t)
	sub, err := st.SaveSubscription(Subscription{Name: "아파트", Kind: KindDues, Amount: 20000, PaidThrough: "2026-10", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDuesPayment(sub.ID, "2026-11-12", 20000, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "b.db")
	counts, err := st.BackupToSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if counts["dues_links"] != 1 || counts["subscriptions"] != 1 {
		t.Fatalf("counts: %+v", counts)
	}
	lite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lite.Close()
	var kind, through string
	if err := lite.QueryRow(`SELECT kind, paid_through FROM subscriptions`).Scan(&kind, &through); err != nil {
		t.Fatal(err)
	}
	if kind != KindDues || through != "2026-10" {
		t.Fatalf("백업: %s %s", kind, through)
	}
}
