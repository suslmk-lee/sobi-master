package classifier

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"sobi/internal/store"
)

// refMatch 는 Matcher 도입 이전의 Classifier.Match 알고리즘을 그대로 옮긴 것이다.
// 리팩터링이 판정 결과를 바꾸지 않았는지 대조하는 기준(oracle)으로만 쓴다.
func refMatch(rules []store.Rule, merchant string, amount int64) *store.Rule {
	nm := normalize(merchant)
	if nm == "" {
		return nil
	}
	var best *store.Rule
	for i := range rules {
		r := rules[i]
		nr := normalize(r.Merchant)
		if nr == "" {
			continue
		}
		if !strings.Contains(nm, nr) && !strings.Contains(nr, nm) {
			continue
		}
		if amount < r.AmountMin || amount > r.AmountMax {
			continue
		}
		if best == nil || (r.AmountMax-r.AmountMin) < (best.AmountMax-best.AmountMin) {
			best = &rules[i]
		}
	}
	return best
}

func ruleID(r *store.Rule) int64 {
	if r == nil {
		return 0
	}
	return r.ID
}

// 대표적인 상황(부분일치 양방향, 금액 경계, 가장 좁은 구간 선택, 빈 가맹점)에서
// Matcher 가 기존 알고리즘과 같은 규칙을 고르는지 본다.
func TestMatcherMatchesReferenceAlgorithm(t *testing.T) {
	rules := []store.Rule{
		{ID: 1, Merchant: "LG유플러스", AmountMin: 30000, AmountMax: 70000},
		{ID: 2, Merchant: "LG유플러스", AmountMin: 60000, AmountMax: 68000}, // 더 좁은 구간
		{ID: 3, Merchant: "스타벅스", AmountMin: 4000, AmountMax: 6000},
		{ID: 4, Merchant: "", AmountMin: 0, AmountMax: 1000000},          // 빈 가맹점 규칙은 무시돼야 함
		{ID: 5, Merchant: "쿠팡", AmountMin: 10000, AmountMax: 10000},      // 단일 금액
		{ID: 6, Merchant: "  스타 벅스  ", AmountMin: 4500, AmountMax: 5500}, // 공백 정규화 + 더 좁음
	}
	m := NewMatcher(rules)

	cases := []struct {
		merchant string
		amount   int64
	}{
		{"LG유플러스", 65890},      // 2번(좁은 구간)이 이겨야 함
		{"LG유플러스", 38500},      // 1번만 해당
		{"LG유플러스 자동결제", 65890}, // 규칙명이 거래명에 포함
		{"LG", 65890},          // 거래명이 규칙명에 포함(역방향)
		{"스타벅스강남점", 5000},      // 3번 vs 6번 → 6번이 좁음
		{"스타벅스강남점", 4200},      // 6번 구간 밖 → 3번
		{"쿠팡", 10000},          // 경계 정확히 일치
		{"쿠팡", 10001},          // 경계 밖
		{"쿠팡", 9999},           // 경계 밖
		{"", 5000},             // 빈 가맹점 → nil
		{"   ", 5000},          // 공백만 → nil
		{"전혀다른곳", 5000},        // 매칭 없음
		{"스타벅스", 4000},         // 하한 경계
		{"스타벅스", 6000},         // 상한 경계
		{"lg유플러스", 65890},      // 대소문자 무시
		{"L G 유 플 러 스", 65890}, // 공백 제거 후 일치
	}

	for _, c := range cases {
		want := refMatch(rules, c.merchant, c.amount)
		got := m.Match(c.merchant, c.amount)
		if ruleID(want) != ruleID(got) {
			t.Errorf("Match(%q, %d) = 규칙 %d, 기존 알고리즘은 규칙 %d",
				c.merchant, c.amount, ruleID(got), ruleID(want))
		}
	}
}

// 무작위 규칙·거래 조합으로 기존 알고리즘과 결과가 갈리는 입력이 없는지 훑는다.
func TestMatcherEquivalenceRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(20260829))
	names := []string{"LG유플러스", "스타벅스", "쿠팡", "GS25", "배달의민족", "", "  ", "네이버페이", "lg", "스타"}

	for trial := 0; trial < 300; trial++ {
		n := rng.Intn(12)
		rules := make([]store.Rule, n)
		for i := range rules {
			lo := int64(rng.Intn(100000))
			hi := lo + int64(rng.Intn(50000))
			rules[i] = store.Rule{
				ID:        int64(i + 1),
				Merchant:  names[rng.Intn(len(names))],
				AmountMin: lo,
				AmountMax: hi,
			}
		}
		m := NewMatcher(rules)
		for probe := 0; probe < 30; probe++ {
			merchant := names[rng.Intn(len(names))]
			if rng.Intn(3) == 0 {
				merchant += fmt.Sprintf("%d호점", rng.Intn(50))
			}
			amount := int64(rng.Intn(150000))
			want := refMatch(rules, merchant, amount)
			got := m.Match(merchant, amount)
			if ruleID(want) != ruleID(got) {
				t.Fatalf("trial %d: Match(%q, %d) = 규칙 %d, 기존 알고리즘은 규칙 %d\n규칙: %+v",
					trial, merchant, amount, ruleID(got), ruleID(want), rules)
			}
		}
	}
}

// Matcher 가 고른 규칙은 원본 슬라이스의 값과 같아야 한다(포인터로 넘겨 Apply 가 읽으므로).
func TestMatcherReturnsRuleContents(t *testing.T) {
	mid, cid := int64(7), int64(9)
	rules := []store.Rule{
		{ID: 42, Merchant: "쿠팡", AmountMin: 1000, AmountMax: 20000, MemberID: &mid, CategoryID: &cid, Label: "아빠 · 생활용품"},
	}
	got := NewMatcher(rules).Match("쿠팡", 15000)
	if got == nil {
		t.Fatal("규칙을 찾지 못했다")
	}
	if got.ID != 42 || got.Label != "아빠 · 생활용품" {
		t.Errorf("규칙 내용이 다르다: %+v", *got)
	}
	if got.MemberID == nil || *got.MemberID != 7 || got.CategoryID == nil || *got.CategoryID != 9 {
		t.Errorf("귀속자/카테고리가 다르다: %+v", *got)
	}
}

// RuleMatchesNorm 은 RuleMatches 와 판정이 같아야 한다(정규화만 미리 해 둔 것).
func TestRuleMatchesNormEquivalentToRuleMatches(t *testing.T) {
	rules := []store.Rule{
		{ID: 1, Merchant: "LG유플러스", AmountMin: 30000, AmountMax: 70000},
		{ID: 2, Merchant: "", AmountMin: 0, AmountMax: 100},
		{ID: 3, Merchant: "  스타 벅스 ", AmountMin: 4000, AmountMax: 6000},
	}
	merchants := []string{"LG유플러스", "lg유플러스 자동결제", "스타벅스", "", "   ", "쿠팡"}
	amounts := []int64{0, 50, 100, 4000, 5000, 6000, 65890, 999999}

	for _, r := range rules {
		nr := Normalize(r.Merchant)
		for _, mch := range merchants {
			for _, amt := range amounts {
				want := RuleMatches(r, mch, amt)
				got := RuleMatchesNorm(r, nr, Normalize(mch), amt)
				if want != got {
					t.Errorf("규칙 %d, 가맹점 %q, 금액 %d: RuleMatchesNorm=%v, RuleMatches=%v",
						r.ID, mch, amt, got, want)
				}
			}
		}
	}
}

// 빈 규칙 목록에서도 안전해야 한다.
func TestMatcherEmptyRules(t *testing.T) {
	if got := NewMatcher(nil).Match("쿠팡", 1000); got != nil {
		t.Errorf("빈 규칙 목록인데 %+v 를 돌려줬다", *got)
	}
	if got := NewMatcher([]store.Rule{}).Match("쿠팡", 1000); got != nil {
		t.Errorf("빈 규칙 목록인데 %+v 를 돌려줬다", *got)
	}
}
