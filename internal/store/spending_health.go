package store

import (
	"fmt"
	"time"
)

// 지출 성격(필수·선택·낭비)과 낭비율.
//
// 계산은 전부 dash 스키마의 뷰 안에 있다. 앱(SpendingHealth)과 개인 대시보드(읽기 전용 역할 dashboard_ro,
// docs/dashboard_ro.sql)가 같은 뷰를 읽으므로 두 곳의 숫자가 항상 같다. Go 로 계산식을 다시 만들지 않는다.
//
//	낭비액 = 낭비 카테고리 지출 + 선택 카테고리의 예산 초과분
//	낭비율 = 낭비액 ÷ 그달 총지출 (총지출은 MonthlySummary.TotalExpense 와 같은 기준)
//
// 거래 한 건의 성격 (위에서부터 먼저 맞는 것):
//  1. 부 카테고리의 spend_class
//  2. 주 카테고리의 spend_class
//  3. 고정비(부 또는 주)이거나 정기결제에 연결된 거래 → 필수 (구성원 분석 spendKind 와 같은 조건)
//  4. 그 밖 → 미지정
//
// 금액은 예산과 같이 바구니에서 낸 몫을 뺀 나머지로 센다. 바구니 몫은 미리 떼어 둔 돈이라 낭비로 보지 않고
// basket 칸에 따로 둔다. 그래서 항상 essential + discretionary + waste + unassigned + basket == total_expense.
//
// 예산 초과분은 같은 돈을 두 번 세지 않는다. 예산 화면의 사용액은 주 예산에 부 지출까지 합산하지만, 여기서는
// '선택' 거래 하나를 가장 가까운 예산 하나(부에 예산이 있으면 부, 없으면 주)에만 묶고
// 예산마다 max(0, 묶인 금액 − 예산액)을 더한다. 필수·낭비·미지정 거래는 초과분에 넣지 않는다.

// 지출 성격 값 (categories.spend_class)
const (
	SpendUnset         = ""
	SpendEssential     = "essential"
	SpendDiscretionary = "discretionary"
	SpendWaste         = "waste"
)

// dashExpenseCTE 는 지출 거래 한 건마다 달·금액·바구니 몫·카테고리·성격을 붙인다 (x).
// be_amount 는 바구니에서 낸 몫으로 0 이하다 (basket_entries 의 spend 는 음수로 저장).
const dashExpenseCTE = `x AS (
  SELECT substr(t.date, 1, 7) AS ym, t.amount, COALESCE(be.amount, 0) AS be_amount,
         t.category_id, c.parent_id,
         CASE WHEN cp.id IS NULL THEN c.name ELSE cp.name || ' > ' || c.name END AS cat_name,
         NULLIF(btrim(COALESCE(t.merchant, '')), '') AS merchant,
         CASE
           WHEN COALESCE(c.spend_class, '') <> '' THEN c.spend_class
           WHEN COALESCE(cp.spend_class, '') <> '' THEN cp.spend_class
           WHEN COALESCE(c.fixed_cost, 0) = 1 OR COALESCE(cp.fixed_cost, 0) = 1
             OR EXISTS (SELECT 1 FROM subscription_posts sp WHERE sp.transaction_id = t.id) THEN 'essential'
           ELSE 'unassigned'
         END AS class
  FROM transactions t
  LEFT JOIN categories c ON c.id = t.category_id
  LEFT JOIN categories cp ON cp.id = c.parent_id
  LEFT JOIN basket_entries be ON be.transaction_id = t.id AND be.kind = 'spend'
  WHERE t.direction = 'expense'
)`

// dashBudgetCTE 는 달마다 카테고리별 유효 예산을 고른다 (eff). 월 전용 예산이 기본('*')보다 우선한다
// (BudgetStatuses 와 같은 규칙). 달 목록은 거래가 있는 달과 월 전용 예산이 있는 달이다.
const dashBudgetCTE = `bm AS (
  SELECT DISTINCT substr(date, 1, 7) AS ym FROM transactions
  UNION
  SELECT ym FROM budgets WHERE ym <> '*'
),
eff AS (
  SELECT DISTINCT ON (bm.ym, b.category_id) bm.ym, b.category_id, b.amount, b.ym <> '*' AS override
  FROM bm JOIN budgets b ON b.ym = bm.ym OR b.ym = '*'
  ORDER BY bm.ym, b.category_id, (b.ym <> '*') DESC
)`

// dashBoundCTE 는 '선택' 거래를 가장 가까운 예산 하나에 묶은 합계 (bound). x 와 eff 가 앞에 있어야 한다.
const dashBoundCTE = `bound AS (
  SELECT x.ym, COALESCE(bs.category_id, bp.category_id) AS budget_id,
         MAX(COALESCE(bs.amount, bp.amount)) AS budget_amount,
         SUM(x.amount + x.be_amount) AS spent
  FROM x
  LEFT JOIN eff bs ON bs.ym = x.ym AND bs.category_id = x.category_id
  LEFT JOIN eff bp ON bp.ym = x.ym AND bp.category_id = x.parent_id
  WHERE x.class = 'discretionary' AND COALESCE(bs.category_id, bp.category_id) IS NOT NULL
  GROUP BY 1, 2
)`

// dashSchema 는 대시보드와 앱이 같이 읽는 집계 뷰. fullSchema 맨 끝에 붙는다.
//
// 뷰는 처음에 넷을 한꺼번에 지우고 다시 만든다. CREATE OR REPLACE VIEW 는 컬럼을 빼거나 순서를 바꾸면 실패해
// 앱 시작이 막힌다. 뷰마다 지우고 만들기를 번갈아 하면, 한 뷰가 다른 뷰를 참조하게 됐을 때 두 번째 적용에서
// DROP 이 실패한다. 스키마(dash) 자체는 지우지 않는다 — 지우면 dashboard_ro 에 준 권한이 함께 사라진다.
// 다시 만든 뷰에도 ALTER DEFAULT PRIVILEGES(docs/dashboard_ro.sql)로 읽기 권한이 자동으로 붙는다.
//
// 뷰는 소유자(앱이 접속하는 역할) 권한으로 원본 표를 읽는다. security_invoker 는 켜지 않는다 —
// 그래야 dashboard_ro 가 원본 표 권한 없이 뷰만 읽을 수 있다.
const dashSchema = `
CREATE SCHEMA IF NOT EXISTS dash;
DROP VIEW IF EXISTS dash.waste_top_monthly, dash.spending_health_monthly,
                    dash.budget_status_monthly, dash.month_summary;

-- 월 요약: MonthlySummary 의 TotalIncome / TotalExpense / TotalTransfer / BasketExpense / UnclassifiedCount
CREATE VIEW dash.month_summary AS
SELECT substr(t.date, 1, 7) AS ym,
       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'income'), 0)::bigint AS income,
       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'expense'), 0)::bigint AS expense,
       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'transfer'), 0)::bigint AS transfer,
       COALESCE(-SUM(be.amount) FILTER (WHERE t.direction = 'expense'), 0)::bigint AS basket_expense,
       COUNT(*) FILTER (WHERE t.member_id IS NULL OR t.category_id IS NULL) AS unclassified_count
FROM transactions t
LEFT JOIN basket_entries be ON be.transaction_id = t.id AND be.kind = 'spend'
GROUP BY 1;

-- 예산 현황: BudgetStatuses(year, month) 와 같다. 주 예산은 부 지출까지 합산하고, 바구니에서 낸 몫은 뺀다.
CREATE VIEW dash.budget_status_monthly AS
WITH ` + dashBudgetCTE + `,
spent AS (
  SELECT substr(t.date, 1, 7) AS ym, k.category_id, SUM(t.amount + COALESCE(be.amount, 0)) AS spent
  FROM transactions t
  JOIN categories c ON c.id = t.category_id
  LEFT JOIN basket_entries be ON be.transaction_id = t.id AND be.kind = 'spend'
  CROSS JOIN LATERAL (VALUES (c.id), (c.parent_id)) AS k(category_id)
  WHERE t.direction = 'expense' AND k.category_id IS NOT NULL
  GROUP BY 1, 2
)
SELECT e.ym, e.category_id, c.name AS category, e.amount,
       COALESCE(s.spent, 0)::bigint AS spent,
       (e.amount - COALESCE(s.spent, 0))::bigint AS remaining,
       COALESCE(s.spent, 0) > e.amount AS "over",
       e.override
FROM eff e
JOIN categories c ON c.id = e.category_id
LEFT JOIN spent s ON s.ym = e.ym AND s.category_id = e.category_id;

-- 지출 건강도: 성격별 금액과 낭비율. 거래가 있는 달마다 한 줄.
CREATE VIEW dash.spending_health_monthly AS
WITH ` + dashExpenseCTE + `,
` + dashBudgetCTE + `,
` + dashBoundCTE + `,
agg AS (
  SELECT ym,
         SUM(amount) AS total_expense,
         COALESCE(SUM(amount + be_amount) FILTER (WHERE class = 'essential'), 0) AS essential,
         COALESCE(SUM(amount + be_amount) FILTER (WHERE class = 'discretionary'), 0) AS discretionary,
         COALESCE(SUM(amount + be_amount) FILTER (WHERE class = 'waste'), 0) AS waste,
         COALESCE(SUM(amount + be_amount) FILTER (WHERE class = 'unassigned'), 0) AS unassigned,
         COALESCE(SUM(-be_amount), 0) AS basket
  FROM x GROUP BY ym
),
overage AS (
  SELECT ym, SUM(GREATEST(spent - budget_amount, 0)) AS amount FROM bound GROUP BY ym
),
h AS (
  SELECT m.ym,
         COALESCE(a.total_expense, 0) AS total_expense,
         COALESCE(a.essential, 0) AS essential,
         COALESCE(a.discretionary, 0) AS discretionary,
         COALESCE(a.waste, 0) AS waste,
         COALESCE(a.unassigned, 0) AS unassigned,
         COALESCE(a.basket, 0) AS basket,
         COALESCE(o.amount, 0) AS over_budget
  FROM (SELECT DISTINCT substr(date, 1, 7) AS ym FROM transactions) m
  LEFT JOIN agg a ON a.ym = m.ym
  LEFT JOIN overage o ON o.ym = m.ym
)
SELECT ym,
       total_expense::bigint AS total_expense,
       essential::bigint AS essential,
       discretionary::bigint AS discretionary,
       waste::bigint AS waste,
       unassigned::bigint AS unassigned,
       basket::bigint AS basket,
       over_budget::bigint AS discretionary_over_budget,
       (waste + over_budget)::bigint AS waste_amount,
       CASE WHEN total_expense > 0 THEN ((waste + over_budget) / total_expense)::float8 ELSE 0 END AS waste_rate,
       CASE WHEN total_expense > 0 THEN (discretionary / total_expense)::float8 ELSE 0 END AS discretionary_rate,
       CASE WHEN total_expense > 0 THEN (unassigned / total_expense)::float8 ELSE 0 END AS unassigned_rate
FROM h;

-- 낭비 상위 5: 카테고리(낭비 카테고리 + 선택 카테고리의 예산 초과분)와 가맹점(낭비 거래).
CREATE VIEW dash.waste_top_monthly AS
WITH ` + dashExpenseCTE + `,
` + dashBudgetCTE + `,
` + dashBoundCTE + `,
items AS (
  SELECT ym, 'category' AS kind, cat_name AS name, SUM(amount + be_amount) AS amount
  FROM x WHERE class = 'waste' GROUP BY 1, 3
  UNION ALL
  SELECT b.ym, 'category', c.name || ' (예산 초과)', b.spent - b.budget_amount
  FROM bound b JOIN categories c ON c.id = b.budget_id
  WHERE b.spent > b.budget_amount
  UNION ALL
  SELECT ym, 'merchant', COALESCE(merchant, '(가맹점 없음)'), SUM(amount + be_amount)
  FROM x WHERE class = 'waste' GROUP BY 1, 3
),
ranked AS (
  SELECT ym, kind, name, amount::bigint AS amount,
         ROW_NUMBER() OVER (PARTITION BY ym, kind ORDER BY amount DESC, name) AS rank
  FROM items WHERE amount > 0
)
SELECT ym, kind, name, amount, rank FROM ranked WHERE rank <= 5;
`

// SpendingHealth 는 한 달의 지출 건강도(낭비율과 그 구성). 금액은 원, 비율은 0~1.
type SpendingHealth struct {
	YM                      string        `json:"ym"` // "2026-11"
	TotalExpense            int64         `json:"totalExpense"`
	Essential               int64         `json:"essential"`
	Discretionary           int64         `json:"discretionary"`
	Waste                   int64         `json:"waste"`
	Unassigned              int64         `json:"unassigned"`
	Basket                  int64         `json:"basket"`
	DiscretionaryOverBudget int64         `json:"discretionaryOverBudget"`
	WasteAmount             int64         `json:"wasteAmount"`
	WasteRate               float64       `json:"wasteRate"`
	DiscretionaryRate       float64       `json:"discretionaryRate"`
	UnassignedRate          float64       `json:"unassignedRate"` // 크면 낭비율을 믿기 어렵다는 표시
	PrevWasteRate           float64       `json:"prevWasteRate"`  // 전월
	Trend                   []WastePoint  `json:"trend"`          // 최근 6개월 (오래된 것부터, 거래 없는 달은 0)
	TopWaste                []NamedAmount `json:"topWaste"`       // Kind: category | merchant, 각각 많은 순 5개까지
}

// WastePoint 는 낭비율 추이의 한 점.
type WastePoint struct {
	YM        string  `json:"ym"`
	WasteRate float64 `json:"wasteRate"`
}

// healthTrendMonths 는 추이에 보여 줄 달 수 (그달 포함).
const healthTrendMonths = 6

// SpendingHealth 는 그달의 지출 건강도를 dash 뷰에서 읽는다.
func (s *Store) SpendingHealth(year, month int) (SpendingHealth, error) {
	cur := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	ym := cur.Format("2006-01")
	from := cur.AddDate(0, -(healthTrendMonths - 1), 0).Format("2006-01")
	out := SpendingHealth{YM: ym, Trend: []WastePoint{}, TopWaste: []NamedAmount{}}

	rowsByYM := map[string]SpendingHealth{}
	err := parallel(
		func() error {
			rows, err := s.query(`
SELECT ym, total_expense, essential, discretionary, waste, unassigned, basket,
       discretionary_over_budget, waste_amount, waste_rate, discretionary_rate, unassigned_rate
FROM dash.spending_health_monthly WHERE ym >= ? AND ym <= ?`, from, ym)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var h SpendingHealth
				if err := rows.Scan(&h.YM, &h.TotalExpense, &h.Essential, &h.Discretionary, &h.Waste, &h.Unassigned,
					&h.Basket, &h.DiscretionaryOverBudget, &h.WasteAmount, &h.WasteRate, &h.DiscretionaryRate,
					&h.UnassignedRate); err != nil {
					return err
				}
				rowsByYM[h.YM] = h
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.query(`SELECT name, kind, amount FROM dash.waste_top_monthly WHERE ym = ? ORDER BY kind, rank`, ym)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var na NamedAmount
				if err := rows.Scan(&na.Name, &na.Kind, &na.Amount); err != nil {
					return err
				}
				out.TopWaste = append(out.TopWaste, na)
			}
			return rows.Err()
		},
	)
	if err != nil {
		return out, fmt.Errorf("지출 건강도를 읽지 못했습니다: %w", err)
	}

	if h, ok := rowsByYM[ym]; ok {
		top := out.TopWaste
		out = h
		out.TopWaste = top
	}
	out.Trend = make([]WastePoint, 0, healthTrendMonths)
	for i := healthTrendMonths - 1; i >= 0; i-- {
		m := cur.AddDate(0, -i, 0).Format("2006-01")
		out.Trend = append(out.Trend, WastePoint{YM: m, WasteRate: rowsByYM[m].WasteRate})
	}
	out.PrevWasteRate = rowsByYM[cur.AddDate(0, -1, 0).Format("2006-01")].WasteRate
	return out, nil
}
