package classifier

import (
	"os"
	"testing"

	"sobi/internal/store"
)

// U+ 자동결제 3건(금액만 다름)을 한 번 라벨링하면
// 다음 달 비슷한 금액이 같은 사람으로 자동 분류되는지 검증한다.
func TestUPlusScenario(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL 미설정 — DB 테스트 건너뜀")
	}
	st, err := store.OpenAt(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.DB().Exec(`TRUNCATE members, categories, payment_methods, rules, transactions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureDefaults(); err != nil {
		t.Fatal(err)
	}
	cl := New(st)

	members, _ := st.ListMembers()
	cats, _ := st.ListCategories()
	var dad, kid, telecom int64
	for _, m := range members {
		if m.Name == "아빠" {
			dad = m.ID
		}
		if m.Name == "아이" {
			kid = m.ID
		}
	}
	for _, c := range cats {
		if c.Name == "통신비" {
			telecom = c.ID
		}
	}

	// 5월: 아빠가 두 건을 직접 라벨링
	for _, tc := range []struct {
		amount int64
		member int64
	}{
		{65890, dad},
		{38500, kid},
	} {
		tx := store.Transaction{
			Date: "2026-05-15", Amount: tc.amount, Direction: "expense",
			Merchant: "LG유플러스", MemberID: &tc.member, CategoryID: &telecom,
		}
		if err := cl.Learn(tx); err != nil {
			t.Fatal(err)
		}
	}

	// 6월: 요금이 조금 달라져도(±8% 이내) 올바른 사람으로 매칭되어야 한다
	cases := []struct {
		amount int64
		want   int64
		desc   string
	}{
		{65890, dad, "동일 금액 → 아빠"},
		{67100, dad, "약간 오른 요금 → 아빠"},
		{38500, kid, "동일 금액 → 아이"},
		{37900, kid, "약간 내린 요금 → 아이"},
	}
	for _, c := range cases {
		rule, err := cl.Match("LG유플러스", c.amount)
		if err != nil {
			t.Fatal(err)
		}
		if rule == nil {
			t.Fatalf("%s: 규칙 매칭 실패", c.desc)
		}
		if rule.MemberID == nil || *rule.MemberID != c.want {
			t.Errorf("%s: 잘못된 귀속자", c.desc)
		}
	}

	// 전혀 다른 금액(인터넷 요금 등 새 회선)은 매칭되지 않아야 한다
	rule, _ := cl.Match("LG유플러스", 29700)
	if rule != nil {
		t.Error("범위 밖 금액이 기존 규칙에 매칭됨")
	}

	// 같은 거래를 다시 학습하면 규칙이 늘어나지 않아야 한다(갱신)
	tx := store.Transaction{
		Date: "2026-06-15", Amount: 65890, Direction: "expense",
		Merchant: "LG유플러스", MemberID: &dad, CategoryID: &telecom,
	}
	if err := cl.Learn(tx); err != nil {
		t.Fatal(err)
	}
	rules, _ := st.ListRules()
	if len(rules) != 2 {
		t.Errorf("규칙 수 = %d, want 2 (중복 생성됨)", len(rules))
	}
}
