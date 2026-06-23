// Package classifier 는 반복 거래 핑거프린트(가맹점명 + 금액 구간)로
// 거래의 귀속자/카테고리를 자동 분류한다.
//
// 예: "LG유플러스" 자동결제가 65,890 / 38,500 / 29,700원 3건으로 들어와도
// 한 번 라벨링해 두면 다음 달부터 금액 구간으로 누구 요금인지 구분된다.
package classifier

import (
	"strings"

	"sobi/internal/store"
)

// Tolerance 는 금액 매칭 허용 비율. 요금이 ±8% 안에서 변동해도 같은 규칙으로 본다.
const Tolerance = 0.08

type Classifier struct {
	st *store.Store
}

func New(st *store.Store) *Classifier { return &Classifier{st: st} }

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), "")
}

// Normalize 는 가맹점명 정규화 함수의 외부 공개판(고정비 매칭 등에 사용).
func Normalize(s string) string { return normalize(s) }

// RuleMatches 는 규칙이 (가맹점, 금액)에 들어맞는지 본다. Match 와 동일한 기준.
func RuleMatches(r store.Rule, merchant string, amount int64) bool {
	nm := normalize(merchant)
	nr := normalize(r.Merchant)
	if nm == "" || nr == "" {
		return false
	}
	if !strings.Contains(nm, nr) && !strings.Contains(nr, nm) {
		return false
	}
	return amount >= r.AmountMin && amount <= r.AmountMax
}

// Match 는 거래(가맹점, 금액)에 들어맞는 규칙을 찾는다.
// 가맹점명은 정규화 후 부분 일치, 금액은 [amountMin, amountMax] 구간 일치.
// 여러 규칙이 맞으면 금액 구간이 가장 좁은(=가장 구체적인) 규칙을 고른다.
func (c *Classifier) Match(merchant string, amount int64) (*store.Rule, error) {
	rules, err := c.st.ListRules()
	if err != nil {
		return nil, err
	}
	nm := normalize(merchant)
	if nm == "" {
		return nil, nil
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
	return best, nil
}

// Apply 는 미분류 필드에만 규칙 값을 채운다. 사용자가 이미 지정한 값은 건드리지 않는다.
func Apply(t *store.Transaction, r *store.Rule) {
	if r == nil {
		return
	}
	if t.MemberID == nil && r.MemberID != nil {
		id := *r.MemberID
		t.MemberID = &id
		t.AutoClassified = true
	}
	if t.CategoryID == nil && r.CategoryID != nil {
		id := *r.CategoryID
		t.CategoryID = &id
		t.AutoClassified = true
	}
}

// Learn 은 사용자가 분류를 확정한 거래에서 규칙을 만들거나 갱신한다.
// 같은 가맹점에 겹치는 금액 구간 규칙이 있으면 그 규칙을 새 분류로 덮어쓰고,
// 없으면 금액 ±Tolerance 구간의 새 규칙을 만든다.
func (c *Classifier) Learn(t store.Transaction) error {
	if t.MemberID == nil && t.CategoryID == nil {
		return nil
	}
	nm := normalize(t.Merchant)
	if nm == "" {
		return nil
	}
	min := int64(float64(t.Amount) * (1 - Tolerance))
	max := int64(float64(t.Amount)*(1+Tolerance)) + 1

	rules, err := c.st.ListRules()
	if err != nil {
		return err
	}
	for _, r := range rules {
		if normalize(r.Merchant) != nm {
			continue
		}
		// 금액 구간이 겹치면 같은 반복 거래로 보고 갱신
		if t.Amount >= r.AmountMin && t.Amount <= r.AmountMax ||
			(min <= r.AmountMax && max >= r.AmountMin) {
			r.MemberID = t.MemberID
			r.CategoryID = t.CategoryID
			r.Label = ruleLabel(t)
			return c.st.UpdateRule(r)
		}
	}
	_, err = c.st.AddRule(store.Rule{
		Merchant:   strings.TrimSpace(t.Merchant),
		AmountMin:  min,
		AmountMax:  max,
		MemberID:   t.MemberID,
		CategoryID: t.CategoryID,
		Label:      ruleLabel(t),
	})
	return err
}

func ruleLabel(t store.Transaction) string {
	parts := []string{}
	if t.MemberName != "" {
		parts = append(parts, t.MemberName)
	}
	if t.CategoryName != "" {
		parts = append(parts, t.CategoryName)
	}
	return strings.Join(parts, " · ")
}
