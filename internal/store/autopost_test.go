package store

import (
	"testing"
	"time"
)

// 같은 (정기결제, 월) 은 두 번 기록되지 않아야 한다 — 두 PC 가 동시에 돌려도.
// (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestPostSubscriptionClaimsOnce(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE subscriptions, subscription_posts RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	sub, err := st.SaveSubscription(Subscription{Name: "구독", Amount: 1000, Cycle: "monthly", BillingDay: 1, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	tx := Transaction{Date: "2026-10-01", Amount: 1000, Direction: "expense", Merchant: "구독"}

	posted, id1, err := st.PostSubscription(sub.ID, "2026-10", tx, 0)
	if err != nil || !posted || id1 == 0 {
		t.Fatalf("첫 기록 실패: posted=%v id=%d err=%v", posted, id1, err)
	}
	posted, id2, err := st.PostSubscription(sub.ID, "2026-10", tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if posted || id2 != 0 {
		t.Fatalf("같은 달이 두 번 기록됨: posted=%v id=%d", posted, id2)
	}
	var n int
	if err := st.queryRow(`SELECT COUNT(*) FROM transactions WHERE source=?`, SourceSubscription).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("거래가 %d건 — 중복 생성", n)
	}

	// 동시에 여러 번 시도해도 한 번만 성공한다
	done := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		go func() {
			p, _, err := st.PostSubscription(sub.ID, "2026-11", tx, 0)
			done <- err == nil && p
		}()
	}
	wins := 0
	for i := 0; i < 8; i++ {
		if <-done {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("동시 시도 8번 중 %d번 성공 — 정확히 1번이어야 함", wins)
	}
}

// 자동 등록 시작 달은 서버가 정한다. 켠 시점 이전으로 소급하지 않고,
// 수정할 때마다 밀리지 않으며, 끄면 비워진다.
func TestResolveAutoSince(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE subscriptions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	old := nowFunc
	t.Cleanup(func() { nowFunc = old })
	nowFunc = func() time.Time { return time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC) }

	// 시작 월이 과거여도 이번 달부터
	s, err := st.SaveSubscription(Subscription{Name: "a", Amount: 1, BillingDay: 1, StartYM: "2026-01", Active: true, AutoPost: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.AutoSince != "2026-10" {
		t.Errorf("과거 시작 월인데 소급함: %q", s.AutoSince)
	}
	// 시작 월이 미래면 그 달부터
	f, err := st.SaveSubscription(Subscription{Name: "b", Amount: 1, BillingDay: 1, StartYM: "2026-11", Active: true, AutoPost: true})
	if err != nil {
		t.Fatal(err)
	}
	if f.AutoSince != "2026-11" {
		t.Errorf("미래 시작 월이 반영 안 됨: %q", f.AutoSince)
	}

	// 다음 달에 수정해도 원래 시작 달을 지킨다(그 사이 달이 빠지면 안 됨)
	nowFunc = func() time.Time { return time.Date(2026, 12, 2, 9, 0, 0, 0, time.UTC) }
	s.Amount = 2
	s.AutoSince = "2099-01" // 클라이언트가 보낸 값은 무시해야 한다
	s2, err := st.SaveSubscription(s)
	if err != nil {
		t.Fatal(err)
	}
	if s2.AutoSince != "2026-10" {
		t.Errorf("수정했더니 시작 달이 바뀜: %q", s2.AutoSince)
	}

	// 끄면 비우고, 다시 켜면 그때부터 센다(꺼 둔 기간은 소급하지 않음)
	s2.AutoPost = false
	s3, _ := st.SaveSubscription(s2)
	if s3.AutoSince != "" {
		t.Errorf("껐는데 시작 달이 남음: %q", s3.AutoSince)
	}
	s3.AutoPost = true
	s4, _ := st.SaveSubscription(s3)
	if s4.AutoSince != "2026-12" {
		t.Errorf("다시 켠 달부터여야 함: %q", s4.AutoSince)
	}

	// 목록 조회에도 그대로 실려야 한다
	list, _ := st.ListSubscriptions()
	for _, x := range list {
		if x.ID == s4.ID && (!x.AutoPost || x.AutoSince != "2026-12") {
			t.Errorf("조회 결과에 자동 등록 값이 안 실림: %+v", x)
		}
	}
}
