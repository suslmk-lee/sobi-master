package main

import (
	"testing"

	"sobi/internal/store"
)

// 거래에서 정기결제 초안을 만들 때: 그 거래 달은 이미 결제됐으니 "다음 달"부터 추적해야 하고,
// 연말 거래는 해를 넘겨야 한다. 분류(카테고리·귀속자·결제수단)는 거래 것을 물려받는다.
// 같은 가맹점이 이미 걸려 있으면 실제 대조와 같은 기준으로 중복을 알려야 한다.
// (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestDraftSubscriptionFromTx(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.st.DB().Exec(`TRUNCATE subscriptions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	cats, _ := a.st.ListCategories()
	mems, _ := a.st.ListMembers()
	tele := categoryID(cats, "통신비")
	dad := memberID(mems, "아빠")

	add := func(date, merchant, dir string, amt int64) int64 {
		t.Helper()
		id, err := a.st.AddTransaction(store.Transaction{
			Date: date, Amount: amt, Direction: dir, Merchant: merchant,
			CategoryID: &tele, MemberID: &dad,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	t.Run("다음 달 시작·분류 승계", func(t *testing.T) {
		id := add("2026-09-25", "LG유플러스 자동결제", "expense", 55000)
		r, err := a.DraftSubscriptionFromTx(id)
		if err != nil {
			t.Fatal(err)
		}
		d := r.Draft
		if d.StartYM != "2026-10" {
			t.Errorf("시작 월 %q, want 2026-10 (거래 달은 이미 결제됨)", d.StartYM)
		}
		if d.BillingDay != 25 || d.Amount != 55000 || d.Cycle != "monthly" || !d.Active {
			t.Errorf("초안 값이 거래와 다름: %+v", d)
		}
		if d.CategoryID == nil || *d.CategoryID != tele || d.MemberID == nil || *d.MemberID != dad {
			t.Errorf("분류가 승계되지 않음: cat=%v mem=%v", d.CategoryID, d.MemberID)
		}
		if d.ID != 0 {
			t.Errorf("초안은 저장 전이어야 함: id=%d", d.ID)
		}
		if len(r.Duplicates) != 0 {
			t.Errorf("등록된 게 없는데 중복이 나옴: %+v", r.Duplicates)
		}
	})

	t.Run("12월 거래는 해를 넘긴다", func(t *testing.T) {
		id := add("2026-12-31", "넷플릭스", "expense", 17000)
		r, err := a.DraftSubscriptionFromTx(id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Draft.StartYM != "2027-01" || r.Draft.BillingDay != 31 {
			t.Errorf("연말 롤오버 실패: start=%q day=%d", r.Draft.StartYM, r.Draft.BillingDay)
		}
	})

	t.Run("지출이 아니면 거절", func(t *testing.T) {
		id := add("2026-09-10", "급여", "income", 3000000)
		if _, err := a.DraftSubscriptionFromTx(id); err == nil {
			t.Fatal("수입 거래로 정기결제 초안이 만들어짐")
		}
	})

	t.Run("같은 가맹점 정기결제가 있으면 중복으로 알림", func(t *testing.T) {
		// 대조 가맹점을 짧게 적어 둔 기존 정기결제 — 실제 대조처럼 부분 일치로 잡혀야 한다
		if _, err := a.st.SaveSubscription(store.Subscription{
			Name: "휴대폰 요금", Merchant: "LG유플러스", Amount: 50000, Cycle: "monthly", BillingDay: 25, Active: true,
		}); err != nil {
			t.Fatal(err)
		}
		id := add("2026-10-25", "LG유플러스 자동결제", "expense", 55000)
		r, err := a.DraftSubscriptionFromTx(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Duplicates) != 1 || r.Duplicates[0].Name != "휴대폰 요금" {
			t.Fatalf("부분 일치 중복을 못 잡음: %+v", r.Duplicates)
		}
	})
}
