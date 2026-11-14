package store

import (
	"database/sql"
	"regexp"
	"strings"
	"time"
)

// ---- 목돈 바구니: 카테고리 연결 · 연결 후보 · 적립액 제안 ----
//
// 바구니에 카테고리를 이어 두면(생활 예비비 ← 회비/경조사, 부모님 병원비 ← 의료비)
//   - 그 카테고리의 큰 지출(basketBigSpend 이상) 중 아직 어느 바구니에도 연결하지 않은 것을 '연결 후보'로 보여 준다.
//     정해진 연간 지출은 항목 이름(재산세 등)이 거래 이름에 들어 있으면 후보다. '아님'으로 지운 거래는 다시 보이지 않는다.
//   - 지난 12개월 그 카테고리의 큰 지출로 예비비 월 적립액을 제안한다.
// 정기결제 · 회비 · 교통카드로 이미 잡힌 거래는 정해진 돈이라 후보·제안에서 뺀다.

// basketBigSpend 는 바구니 후보·제안으로 볼 지출의 최소 금액(축의금 5만 원부터).
const basketBigSpend = 50000

// basketCandidateDays 는 연결 후보를 찾는 기간(오늘부터 거꾸로).
const basketCandidateDays = 90

const basketLinkSchema = `
CREATE TABLE IF NOT EXISTS basket_categories (
	basket_id   BIGINT NOT NULL REFERENCES baskets(id) ON DELETE CASCADE,
	category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
	PRIMARY KEY (basket_id, category_id)
);
CREATE TABLE IF NOT EXISTS basket_dismissed (
	transaction_id BIGINT PRIMARY KEY REFERENCES transactions(id) ON DELETE CASCADE,
	created_at     TEXT NOT NULL DEFAULT ''
);
`

// basketRegularFilter 는 정기결제 · 회비 · 교통카드로 잡힌 거래를 빼는 조건. t 는 transactions 별칭.
const basketRegularFilter = `NOT EXISTS (SELECT 1 FROM subscription_posts sp WHERE sp.transaction_id = t.id)
  AND NOT EXISTS (SELECT 1 FROM dues_links dl WHERE dl.transaction_id = t.id AND dl.state = 'include')
  AND ` + transitTxFilter

// BasketCandidate 는 바구니에서 낸 것 같은 지출 하나.
type BasketCandidate struct {
	TransactionID int64  `json:"transactionId"`
	Date          string `json:"date"`
	Amount        int64  `json:"amount"`
	Merchant      string `json:"merchant"`
	Memo          string `json:"memo"`
	CategoryName  string `json:"categoryName"`
	PaymentMethodID *int64 `json:"paymentMethodId"`
	BasketID      int64  `json:"basketId"`
	BasketName    string `json:"basketName"`
	// 정해진 연간 지출 항목 이름으로 찾았으면 그 항목(꺼낼 때 마감을 미리 고른다)
	ItemID   int64  `json:"itemId"`
	ItemName string `json:"itemName"`
	Reason   string `json:"reason"` // 화면에 보일 까닭("회비/경조사 지출", "재산세 이름")
}

// basketItemKey 는 항목 이름에서 괄호 설명을 뺀 비교용 이름("자동차세(연납)" → "자동차세").
var parenRe = regexp.MustCompile(`\(.*?\)`)

func basketItemKey(name string) string {
	return strings.TrimSpace(parenRe.ReplaceAllString(name, ""))
}

// basketCategorySet 은 바구니에 이은 카테고리와 그 하위 부까지(주 카테고리를 이으면 부 지출도 들어온다).
func (s *Store) basketCategorySet(ids []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, id := range ids {
		out[id] = true
	}
	rows, err := s.query(`SELECT id, parent_id FROM categories WHERE parent_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var parent sql.NullInt64
		if err := rows.Scan(&id, &parent); err != nil {
			return nil, err
		}
		if parent.Valid && out[parent.Int64] {
			out[id] = true
		}
	}
	return out, rows.Err()
}

type basketTxRow struct {
	ID         int64
	Date       string
	Amount     int64
	Merchant   string
	Memo       string
	CategoryID int64
	Category   string
	PM         *int64
}

// basketTxRows 는 [from, to] 사이 큰 지출 중 정기 지출이 아닌 것. unlinked 면 바구니에 연결했거나 '아님'으로 지운 것도 뺀다.
func (s *Store) basketTxRows(from, to string, unlinked bool) ([]basketTxRow, error) {
	q := `
SELECT t.id, t.date, t.amount, t.merchant, t.memo, COALESCE(t.category_id, 0),
       COALESCE(cp.name || ' > ' || c.name, c.name, ''), t.payment_method_id
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
WHERE t.direction = 'expense' AND t.amount >= ? AND t.date >= ? AND t.date <= ? AND ` + basketRegularFilter
	if unlinked {
		q += `
  AND NOT EXISTS (SELECT 1 FROM basket_entries be WHERE be.transaction_id = t.id)
  AND NOT EXISTS (SELECT 1 FROM basket_dismissed bd WHERE bd.transaction_id = t.id)`
	}
	rows, err := s.query(q+` ORDER BY t.date DESC, t.id DESC`, basketBigSpend, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []basketTxRow
	for rows.Next() {
		var r basketTxRow
		var pm sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Date, &r.Amount, &r.Merchant, &r.Memo, &r.CategoryID, &r.Category, &pm); err != nil {
			return nil, err
		}
		r.PM = scanNullableID(pm)
		out = append(out, r)
	}
	return out, rows.Err()
}

// BasketCandidates 는 바구니에 연결할 만한 지출(최근 90일). 거래 하나는 한 바구니에만 — 정해진 지출 이름이 먼저.
func (s *Store) BasketCandidates(today time.Time) ([]BasketCandidate, error) {
	out := []BasketCandidate{}
	baskets, err := s.ListBaskets()
	if err != nil {
		return out, err
	}
	type catBasket struct {
		b    Basket
		cats map[int64]bool
	}
	var byCat []catBasket
	hasAny := false
	for _, b := range baskets {
		if !b.Active {
			continue
		}
		set, err := s.basketCategorySet(b.CategoryIDs)
		if err != nil {
			return out, err
		}
		byCat = append(byCat, catBasket{b, set})
		hasAny = hasAny || len(set) > 0 || len(b.Items) > 0
	}
	if !hasAny {
		return out, nil
	}
	to := fmtDay(dayOf(today))
	from := fmtDay(dayOf(today).AddDate(0, 0, -basketCandidateDays))
	rows, err := s.basketTxRows(from, to, true)
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		c := BasketCandidate{TransactionID: r.ID, Date: r.Date, Amount: r.Amount, Merchant: r.Merchant, Memo: r.Memo,
			CategoryName: r.Category, PaymentMethodID: r.PM}
		text := r.Merchant + " " + r.Memo
		found := false
		for _, cb := range byCat { // 정해진 지출 항목 이름
			for _, it := range cb.b.Items {
				if k := basketItemKey(it.Name); k != "" && strings.Contains(text, k) {
					c.BasketID, c.BasketName, c.ItemID, c.ItemName, c.Reason = cb.b.ID, cb.b.Name, it.ID, it.Name, k+" 이름"
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found && r.CategoryID != 0 { // 이은 카테고리
			for _, cb := range byCat {
				if cb.cats[r.CategoryID] {
					c.BasketID, c.BasketName, c.Reason = cb.b.ID, cb.b.Name, r.Category+" 지출"
					found = true
					break
				}
			}
		}
		if found {
			out = append(out, c)
		}
	}
	return out, nil
}

// DismissBasketCandidate 는 후보를 '아님'으로 지운다(다시 보이지 않는다).
func (s *Store) DismissBasketCandidate(txID int64) error {
	_, err := s.exec(`INSERT INTO basket_dismissed(transaction_id, created_at) VALUES(?,?) ON CONFLICT (transaction_id) DO NOTHING`,
		txID, nowFunc().Format("2006-01-02 15:04:05"))
	return err
}

// SuggestBasketMonthly 는 이은 카테고리의 지난 12개월 큰 지출(정기 지출 제외) 합 ÷ 12 를 만 원 올림으로 제안한다.
// 몇 건을 봤는지도 돌려준다(0 건이면 제안 없음).
func (s *Store) SuggestBasketMonthly(categoryIDs []int64, today time.Time) (monthly int64, count int, err error) {
	set, err := s.basketCategorySet(categoryIDs)
	if err != nil || len(set) == 0 {
		return 0, 0, err
	}
	to := fmtDay(dayOf(today))
	from := fmtDay(dayOf(today).AddDate(-1, 0, 1))
	rows, err := s.basketTxRows(from, to, false)
	if err != nil {
		return 0, 0, err
	}
	var sum int64
	for _, r := range rows {
		if set[r.CategoryID] {
			sum += r.Amount
			count++
		}
	}
	if count == 0 {
		return 0, 0, nil
	}
	return ceilDiv(sum, 12*10000) * 10000, count, nil
}

// saveBasketCategories 는 바구니에 이은 카테고리를 넘긴 목록으로 맞춘다(트랜잭션 안에서).
func saveBasketCategories(tx *sql.Tx, basketID int64, ids []int64) error {
	if _, err := tx.Exec(pq(`DELETE FROM basket_categories WHERE basket_id=?`), basketID); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		if _, err := tx.Exec(pq(`INSERT INTO basket_categories(basket_id, category_id) VALUES(?,?)`), basketID, id); err != nil {
			return err
		}
	}
	return nil
}

// basketCategoryIDs 는 바구니 id → 이은 카테고리 id 목록.
func (s *Store) basketCategoryIDs() (map[int64][]int64, error) {
	rows, err := s.query(`SELECT basket_id, category_id FROM basket_categories ORDER BY category_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]int64{}
	for rows.Next() {
		var b, c int64
		if err := rows.Scan(&b, &c); err != nil {
			return nil, err
		}
		out[b] = append(out[b], c)
	}
	return out, rows.Err()
}

// basketSpentIn 은 그달 바구니에서 낸 지출 합(평소 지출과 나눠 보이려고). prefix 는 "YYYY-MM%".
func (s *Store) basketSpentIn(prefix string) (int64, error) {
	var out int64
	err := s.queryRow(`
SELECT COALESCE(-SUM(e.amount), 0) FROM basket_entries e JOIN transactions t ON t.id = e.transaction_id
WHERE e.kind = 'spend' AND t.direction = 'expense' AND t.date LIKE ?`, prefix).Scan(&out)
	return out, err
}
