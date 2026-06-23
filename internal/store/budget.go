package store

import "fmt"

// budgetDefault 은 "매월 기본" 예산을 나타내는 ym 값.
const budgetDefault = "*"

// Budget 은 카테고리 예산. Ym 이 "*" 면 매월 기본, "YYYY-MM" 이면 해당 월 전용.
type Budget struct {
	CategoryID int64  `json:"categoryId"`
	Category   string `json:"category"`
	Kind       string `json:"kind"`
	Ym         string `json:"ym"`
	Amount     int64  `json:"amount"`
}

// BudgetStatus 는 특정 월의 예산 대비 실제 지출 현황.
type BudgetStatus struct {
	CategoryID int64  `json:"categoryId"`
	Category   string `json:"category"`
	Amount     int64  `json:"amount"`    // 유효 예산(월 전용 우선, 없으면 기본)
	Spent      int64  `json:"spent"`     // 해당 월 지출
	Remaining  int64  `json:"remaining"` // 남은 예산(초과 시 음수)
	Pct        int    `json:"pct"`       // 사용률(%)
	Over       bool   `json:"over"`      // 초과 여부
	Override   bool   `json:"override"`  // 이 달 전용 예산이 적용됐는지
}

// ListBudgets 는 주어진 ym("*" 또는 "YYYY-MM")으로 설정된 예산 목록을 돌려준다.
func (s *Store) ListBudgets(ym string) ([]Budget, error) {
	if ym == "" {
		ym = budgetDefault
	}
	rows, err := s.query(`
SELECT b.category_id, c.name, c.kind, b.ym, b.amount
FROM budgets b JOIN categories c ON c.id = b.category_id
WHERE b.ym = ?
ORDER BY c.name`, ym)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Budget{}
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.CategoryID, &b.Category, &b.Kind, &b.Ym, &b.Amount); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// SetBudget 은 (카테고리, ym) 예산을 추가/수정한다. amount<=0 이면 삭제한다.
func (s *Store) SetBudget(categoryID int64, ym string, amount int64) error {
	if ym == "" {
		ym = budgetDefault
	}
	if amount <= 0 {
		return s.DeleteBudget(categoryID, ym)
	}
	_, err := s.exec(`
INSERT INTO budgets(category_id, ym, amount) VALUES(?,?,?)
ON CONFLICT (category_id, ym) DO UPDATE SET amount = EXCLUDED.amount`, categoryID, ym, amount)
	return err
}

func (s *Store) DeleteBudget(categoryID int64, ym string) error {
	if ym == "" {
		ym = budgetDefault
	}
	_, err := s.exec(`DELETE FROM budgets WHERE category_id=? AND ym=?`, categoryID, ym)
	return err
}

// BudgetStatuses 는 해당 월의 예산별 사용 현황을 사용률 내림차순으로 돌려준다.
// 월 전용 예산이 있으면 그것을, 없으면 매월 기본 예산을 적용한다.
func (s *Store) BudgetStatuses(year, month int) ([]BudgetStatus, error) {
	target := fmt.Sprintf("%04d-%02d", year, month)

	// 1) 유효 예산 결정(월 전용 > 기본)
	type eff struct {
		name     string
		amount   int64
		override bool
	}
	byCat := map[int64]*eff{}
	rows, err := s.query(`
SELECT b.category_id, c.name, b.ym, b.amount
FROM budgets b JOIN categories c ON c.id = b.category_id
WHERE b.ym = ? OR b.ym = ?`, budgetDefault, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, amt int64
		var name, ym string
		if err := rows.Scan(&cid, &name, &ym, &amt); err != nil {
			return nil, err
		}
		isOverride := ym == target
		cur := byCat[cid]
		if cur == nil || (isOverride && !cur.override) {
			byCat[cid] = &eff{name: name, amount: amt, override: isOverride}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(byCat) == 0 {
		return []BudgetStatus{}, nil
	}

	// 2) 해당 월 카테고리별 지출
	spent := map[int64]int64{}
	srows, err := s.query(`
SELECT category_id, SUM(amount) FROM transactions
WHERE date LIKE ? AND direction='expense' AND category_id IS NOT NULL
GROUP BY category_id`, target+"%")
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var cid, amt int64
		if err := srows.Scan(&cid, &amt); err != nil {
			return nil, err
		}
		spent[cid] = amt
	}
	if err := srows.Err(); err != nil {
		return nil, err
	}

	out := make([]BudgetStatus, 0, len(byCat))
	for cid, e := range byCat {
		st := BudgetStatus{
			CategoryID: cid, Category: e.name, Amount: e.amount,
			Spent: spent[cid], Override: e.override,
		}
		st.Remaining = st.Amount - st.Spent
		st.Over = st.Spent > st.Amount
		if st.Amount > 0 {
			st.Pct = int(st.Spent * 100 / st.Amount)
		}
		out = append(out, st)
	}
	// 사용률 내림차순(초과/임박이 위로)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Pct > out[j-1].Pct; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}
