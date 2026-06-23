package main

import (
	"context"
	"os"
	"testing"

	"sobi/internal/classifier"
	"sobi/internal/store"
)

// newTestApp 은 TEST_DATABASE_URL(Postgres) 로 접속한 깨끗한 App 을 만든다.
// 환경변수가 없으면 테스트를 건너뛴다. (ensure() 는 a.st 가 있으면 그대로 사용한다)
func newTestApp(t *testing.T) *App {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL 미설정 — DB 테스트 건너뜀")
	}
	st, err := store.OpenAt(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.DB().Exec(`TRUNCATE members, categories, payment_methods, rules, transactions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureDefaults(); err != nil {
		t.Fatal(err)
	}
	return &App{ctx: context.Background(), st: st, cl: classifier.New(st)}
}

func memberID(ms []store.Member, name string) int64 {
	for _, m := range ms {
		if m.Name == name {
			return m.ID
		}
	}
	return 0
}

func categoryID(cs []store.Category, name string) int64 {
	for _, c := range cs {
		if c.Name == name {
			return c.ID
		}
	}
	return 0
}

// TestManualAddRuleWins 는 수동 등록 시 학습된 규칙이 폼 기본 귀속자를 이기고
// 자동 분류하며, 그 과정에서 규칙이 새로 추가/오염되지 않음을 검증한다.
func TestManualAddRuleWins(t *testing.T) {
	a := newTestApp(t)
	mems, _ := a.st.ListMembers()
	cats, _ := a.st.ListCategories()
	dad := memberID(mems, "아빠")
	mom := memberID(mems, "엄마")
	food := categoryID(cats, "식비")

	// 1) "스타벅스 → 엄마·식비" 를 카테고리까지 골라 등록 → 규칙이 학습된다.
	if _, err := a.AddTransaction(store.Transaction{
		Date: "2026-06-01", Amount: 5000, Direction: "expense",
		Merchant: "스타벅스", MemberID: &mom, CategoryID: &food,
	}); err != nil {
		t.Fatal(err)
	}
	rules, _ := a.st.ListRules()
	if len(rules) != 1 {
		t.Fatalf("학습 후 규칙 1개 기대, 실제 %d", len(rules))
	}

	// 2) 기본 귀속자(아빠)로 같은 가맹점·유사 금액(±8% 내) 수동 등록.
	id, err := a.AddTransaction(store.Transaction{
		Date: "2026-06-15", Amount: 5100, Direction: "expense",
		Merchant: "스타벅스", MemberID: &dad, // 폼 기본값
	})
	if err != nil {
		t.Fatal(err)
	}

	txs, _ := a.st.ListTransactions(store.TxFilter{Month: "2026-06"})
	var got *store.Transaction
	for i := range txs {
		if txs[i].ID == id {
			got = &txs[i]
		}
	}
	if got == nil {
		t.Fatal("등록한 거래를 찾지 못함")
	}
	if got.MemberID == nil || *got.MemberID != mom {
		t.Errorf("귀속자=%v, want 엄마(%d) — 규칙이 기본 아빠를 이겨야 함", got.MemberID, mom)
	}
	if got.CategoryID == nil || *got.CategoryID != food {
		t.Errorf("카테고리=%v, want 식비(%d) — 규칙으로 자동 분류돼야 함", got.CategoryID, food)
	}
	if !got.AutoClassified {
		t.Error("AutoClassified=true 기대 (규칙으로 자동 분류)")
	}

	// 3) 규칙이 추가되거나 오염되지 않아야 한다 (여전히 1개, 엄마·식비).
	rules, _ = a.st.ListRules()
	if len(rules) != 1 {
		t.Errorf("규칙 수=%d, want 1 — 규칙 매칭 등록은 학습하지 않아야 함", len(rules))
	}
	if rules[0].MemberID == nil || *rules[0].MemberID != mom {
		t.Errorf("규칙 귀속자가 오염됨: %v, want 엄마(%d)", rules[0].MemberID, mom)
	}
}

// TestManualAddNoLearnFromDefaultOnly 는 규칙이 없고 카테고리도 안 고른 채
// 기본 귀속자만으로 등록하면 규칙을 학습하지 않음을 검증한다(잡음 규칙 방지).
func TestManualAddNoLearnFromDefaultOnly(t *testing.T) {
	a := newTestApp(t)
	mems, _ := a.st.ListMembers()
	dad := memberID(mems, "아빠")

	if _, err := a.AddTransaction(store.Transaction{
		Date: "2026-06-02", Amount: 12000, Direction: "expense",
		Merchant: "편의점", MemberID: &dad, // 카테고리 없음
	}); err != nil {
		t.Fatal(err)
	}
	rules, _ := a.st.ListRules()
	if len(rules) != 0 {
		t.Errorf("규칙 수=%d, want 0 — 기본 귀속자만으로는 학습하면 안 됨", len(rules))
	}
}
