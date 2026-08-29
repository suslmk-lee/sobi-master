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
	return RuleMatchesNorm(r, normalize(r.Merchant), normalize(merchant), amount)
}

// RuleMatchesNorm 은 RuleMatches 와 같은 판정을 하되 정규화된 가맹점명을 그대로 받는다.
// 규칙 × 거래를 전부 대조할 때(고정비 추적 등) 정규화 재계산을 피하려고 쓴다.
func RuleMatchesNorm(r store.Rule, normRule, normMerchant string, amount int64) bool {
	if normRule == "" || normMerchant == "" {
		return false
	}
	if !strings.Contains(normMerchant, normRule) && !strings.Contains(normRule, normMerchant) {
		return false
	}
	return amount >= r.AmountMin && amount <= r.AmountMax
}

// Matcher 는 규칙 목록의 스냅샷이다. 가맹점명을 미리 정규화해 두므로 여러 거래를
// 연속으로 분류할 때 규칙 재조회(DB 왕복)도, 정규화 재계산도 일어나지 않는다.
// 스냅샷이므로 만든 뒤 바뀐 규칙은 반영되지 않는다 — 규칙을 바꾸는 루프에는 쓰지 않는다.
type Matcher struct {
	rules []store.Rule
	norm  []string // norm[i] 는 rules[i].Merchant 의 정규화 결과
}

// NewMatcher 는 이미 읽어 둔 규칙 목록으로 스냅샷을 만든다.
func NewMatcher(rules []store.Rule) *Matcher {
	m := &Matcher{rules: rules, norm: make([]string, len(rules))}
	for i, r := range rules {
		m.norm[i] = normalize(r.Merchant)
	}
	return m
}

// Matcher 는 규칙을 한 번 읽어 스냅샷을 만든다. 여러 건을 분류하는 루프에서 쓴다.
func (c *Classifier) Matcher() (*Matcher, error) {
	rules, err := c.st.ListRules()
	if err != nil {
		return nil, err
	}
	return NewMatcher(rules), nil
}

// Match 는 거래(가맹점, 금액)에 들어맞는 규칙을 찾는다.
// 가맹점명은 정규화 후 부분 일치, 금액은 [amountMin, amountMax] 구간 일치.
// 여러 규칙이 맞으면 금액 구간이 가장 좁은(=가장 구체적인) 규칙을 고른다.
func (m *Matcher) Match(merchant string, amount int64) *store.Rule {
	nm := normalize(merchant)
	if nm == "" {
		return nil
	}
	var best *store.Rule
	for i := range m.rules {
		r := m.rules[i]
		if !RuleMatchesNorm(r, m.norm[i], nm, amount) {
			continue
		}
		if best == nil || (r.AmountMax-r.AmountMin) < (best.AmountMax-best.AmountMin) {
			best = &m.rules[i]
		}
	}
	return best
}

// Match 는 규칙을 읽어 한 건을 분류한다(단건 호출용).
// 여러 건을 처리할 때는 Matcher 로 스냅샷을 한 번만 만들어 재사용한다.
func (c *Classifier) Match(merchant string, amount int64) (*store.Rule, error) {
	m, err := c.Matcher()
	if err != nil {
		return nil, err
	}
	return m.Match(merchant, amount), nil
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

// LearnSession 은 규칙 목록을 한 번만 읽어 두고 여러 거래를 연속으로 학습한다.
// 학습으로 추가·갱신된 규칙은 메모리 목록에도 즉시 반영되므로, 앞선 학습 결과가
// 뒤따르는 학습에 그대로 보인다(매번 다시 읽는 것과 결과가 같다).
type LearnSession struct {
	st    *store.Store
	rules []store.Rule
}

// NewLearnSession 은 규칙을 한 번 읽어 학습 세션을 연다.
func (c *Classifier) NewLearnSession() (*LearnSession, error) {
	rules, err := c.st.ListRules()
	if err != nil {
		return nil, err
	}
	return &LearnSession{st: c.st, rules: rules}, nil
}

// Learn 은 사용자가 분류를 확정한 거래에서 규칙을 만들거나 갱신한다.
// 같은 가맹점에 겹치는 금액 구간 규칙이 있으면 그 규칙을 새 분류로 덮어쓰고,
// 없으면 금액 ±Tolerance 구간의 새 규칙을 만든다.
func (s *LearnSession) Learn(t store.Transaction) error {
	if t.MemberID == nil && t.CategoryID == nil {
		return nil
	}
	nm := normalize(t.Merchant)
	if nm == "" {
		return nil
	}
	min := int64(float64(t.Amount) * (1 - Tolerance))
	max := int64(float64(t.Amount)*(1+Tolerance)) + 1

	for i := range s.rules {
		r := s.rules[i]
		if normalize(r.Merchant) != nm {
			continue
		}
		// 금액 구간이 겹치면 같은 반복 거래로 보고 갱신
		if t.Amount >= r.AmountMin && t.Amount <= r.AmountMax ||
			(min <= r.AmountMax && max >= r.AmountMin) {
			r.MemberID = t.MemberID
			r.CategoryID = t.CategoryID
			r.Label = ruleLabel(t)
			if err := s.st.UpdateRule(r); err != nil {
				return err
			}
			s.rules[i] = r
			return nil
		}
	}
	nr := store.Rule{
		Merchant:   strings.TrimSpace(t.Merchant),
		AmountMin:  min,
		AmountMax:  max,
		MemberID:   t.MemberID,
		CategoryID: t.CategoryID,
		Label:      ruleLabel(t),
	}
	id, err := s.st.AddRule(nr)
	if err != nil {
		return err
	}
	nr.ID = id
	s.rules = append(s.rules, nr)
	return nil
}

// Learn 은 규칙을 읽어 한 건을 학습한다(단건 호출용).
// 여러 건을 학습할 때는 NewLearnSession 으로 세션을 열어 재사용한다.
func (c *Classifier) Learn(t store.Transaction) error {
	ls, err := c.NewLearnSession()
	if err != nil {
		return err
	}
	return ls.Learn(t)
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
