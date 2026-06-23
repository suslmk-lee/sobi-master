package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// ---- 귀속자 ----

func (s *Store) ListMembers() ([]Member, error) {
	rows, err := s.query(`SELECT id, name FROM members ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AddMember(name string) (Member, error) {
	var id int64
	err := s.queryRow(`INSERT INTO members(name) VALUES(?) RETURNING id`, strings.TrimSpace(name)).Scan(&id)
	if err != nil {
		return Member{}, err
	}
	return Member{ID: id, Name: name}, nil
}

func (s *Store) DeleteMember(id int64) error {
	_, err := s.exec(`DELETE FROM members WHERE id=?`, id)
	return err
}

// ---- 카테고리 ----

func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.query(`SELECT id, name, kind FROM categories ORDER BY kind, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) AddCategory(name, kind string) (Category, error) {
	var id int64
	err := s.queryRow(`INSERT INTO categories(name, kind) VALUES(?,?) RETURNING id`, strings.TrimSpace(name), kind).Scan(&id)
	if err != nil {
		return Category{}, err
	}
	return Category{ID: id, Name: name, Kind: kind}, nil
}

func (s *Store) DeleteCategory(id int64) error {
	_, err := s.exec(`DELETE FROM categories WHERE id=?`, id)
	return err
}

// ---- 결제수단 ----

func (s *Store) ListPaymentMethods() ([]PaymentMethod, error) {
	rows, err := s.query(`SELECT id, name, type, issuer, billing_day, cycle_start_day, perf_target, color FROM payment_methods ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PaymentMethod{}
	for rows.Next() {
		var p PaymentMethod
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Issuer, &p.BillingDay, &p.CycleStartDay, &p.PerfTarget, &p.Color); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) AddPaymentMethod(name, typ string) (PaymentMethod, error) {
	return s.SavePaymentMethod(PaymentMethod{Name: name, Type: typ, CycleStartDay: 1})
}

// SavePaymentMethod 는 ID 가 있으면 갱신, 없으면 새로 만든다. 카드 상세 정보 포함.
func (s *Store) SavePaymentMethod(p PaymentMethod) (PaymentMethod, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.CycleStartDay < 1 || p.CycleStartDay > 31 {
		p.CycleStartDay = 1
	}
	if p.BillingDay < 0 || p.BillingDay > 31 {
		p.BillingDay = 0
	}
	if p.ID > 0 {
		_, err := s.exec(`UPDATE payment_methods SET name=?, type=?, issuer=?, billing_day=?, cycle_start_day=?, perf_target=?, color=? WHERE id=?`,
			p.Name, p.Type, strings.TrimSpace(p.Issuer), p.BillingDay, p.CycleStartDay, p.PerfTarget, p.Color, p.ID)
		return p, err
	}
	err := s.queryRow(`INSERT INTO payment_methods(name, type, issuer, billing_day, cycle_start_day, perf_target, color) VALUES(?,?,?,?,?,?,?) RETURNING id`,
		p.Name, p.Type, strings.TrimSpace(p.Issuer), p.BillingDay, p.CycleStartDay, p.PerfTarget, p.Color).Scan(&p.ID)
	return p, err
}

func (s *Store) DeletePaymentMethod(id int64) error {
	_, err := s.exec(`DELETE FROM payment_methods WHERE id=?`, id)
	return err
}

// ---- 거래 ----

const txSelect = `
SELECT t.id, t.date, t.amount, t.direction, t.merchant, t.memo,
       t.member_id, t.category_id, t.payment_method_id, t.source, t.auto_classified,
       COALESCE(m.name, ''), COALESCE(c.name, ''), COALESCE(p.name, '')
FROM transactions t
LEFT JOIN members m ON m.id = t.member_id
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN payment_methods p ON p.id = t.payment_method_id
`

func (s *Store) scanTx(rows *sql.Rows) (Transaction, error) {
	var t Transaction
	var mid, cid, pid sql.NullInt64
	var auto int
	err := rows.Scan(&t.ID, &t.Date, &t.Amount, &t.Direction, &t.Merchant, &t.Memo,
		&mid, &cid, &pid, &t.Source, &auto,
		&t.MemberName, &t.CategoryName, &t.PaymentMethodName)
	if err != nil {
		return t, err
	}
	t.MemberID = scanNullableID(mid)
	t.CategoryID = scanNullableID(cid)
	t.PaymentMethodID = scanNullableID(pid)
	t.AutoClassified = auto == 1
	return t, nil
}

// ListTransactions 는 필터 조건으로 거래를 조회한다.
// From/To 가 있으면 날짜 범위로, 없으면 Month("YYYY-MM") 로 거른다. 빈 값/0 은 미적용.
func (s *Store) ListTransactions(f TxFilter) ([]Transaction, error) {
	q := txSelect
	args := []interface{}{}
	conds := []string{}
	if f.From != "" || f.To != "" {
		if f.From != "" {
			conds = append(conds, `t.date >= ?`)
			args = append(args, f.From)
		}
		if f.To != "" {
			conds = append(conds, `t.date <= ?`)
			args = append(args, f.To)
		}
	} else if f.Month != "" {
		conds = append(conds, `t.date LIKE ?`)
		args = append(args, f.Month+"%")
	}
	if f.UnclassifiedOnly {
		conds = append(conds, `(t.member_id IS NULL OR t.category_id IS NULL)`)
	}
	if f.Query != "" {
		conds = append(conds, `(t.merchant ILIKE ? OR t.memo ILIKE ?)`)
		like := "%" + likeEscape(f.Query) + "%"
		args = append(args, like, like)
	}
	if f.AmountMin > 0 {
		conds = append(conds, `t.amount >= ?`)
		args = append(args, f.AmountMin)
	}
	if f.AmountMax > 0 {
		conds = append(conds, `t.amount <= ?`)
		args = append(args, f.AmountMax)
	}
	if f.Direction != "" {
		conds = append(conds, `t.direction = ?`)
		args = append(args, f.Direction)
	}
	if f.MemberID > 0 {
		conds = append(conds, `t.member_id = ?`)
		args = append(args, f.MemberID)
	}
	if f.CategoryID > 0 {
		conds = append(conds, `t.category_id = ?`)
		args = append(args, f.CategoryID)
	}
	if f.PaymentMethodID > 0 {
		conds = append(conds, `t.payment_method_id = ?`)
		args = append(args, f.PaymentMethodID)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	switch f.Sort {
	case "date_asc":
		q += ` ORDER BY t.date ASC, t.id ASC`
	case "amount_desc":
		q += ` ORDER BY t.amount DESC, t.date DESC`
	case "amount_asc":
		q += ` ORDER BY t.amount ASC, t.date DESC`
	default:
		q += ` ORDER BY t.date DESC, t.id DESC`
	}
	rows, err := s.query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Transaction{}
	for rows.Next() {
		t, err := s.scanTx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) AddTransaction(t Transaction) (int64, error) {
	if t.Direction == "" {
		t.Direction = "expense"
	}
	var id int64
	err := s.queryRow(`
INSERT INTO transactions(date, amount, direction, merchant, memo, member_id, category_id, payment_method_id, source, auto_classified)
VALUES(?,?,?,?,?,?,?,?,?,?) RETURNING id`,
		t.Date, t.Amount, t.Direction, strings.TrimSpace(t.Merchant), t.Memo,
		nullableID(t.MemberID), nullableID(t.CategoryID), nullableID(t.PaymentMethodID),
		t.Source, boolToInt(t.AutoClassified)).Scan(&id)
	return id, err
}

func (s *Store) UpdateTransaction(t Transaction) error {
	_, err := s.exec(`
UPDATE transactions SET date=?, amount=?, direction=?, merchant=?, memo=?,
	member_id=?, category_id=?, payment_method_id=?, auto_classified=?
WHERE id=?`,
		t.Date, t.Amount, t.Direction, strings.TrimSpace(t.Merchant), t.Memo,
		nullableID(t.MemberID), nullableID(t.CategoryID), nullableID(t.PaymentMethodID),
		boolToInt(t.AutoClassified), t.ID)
	return err
}

func (s *Store) DeleteTransaction(id int64) error {
	_, err := s.exec(`DELETE FROM transactions WHERE id=?`, id)
	return err
}

// GetTransaction 은 단건 조회(일괄 삭제 취소용 백업에 사용).
func (s *Store) GetTransaction(id int64) (Transaction, error) {
	rows, err := s.query(txSelect+" WHERE t.id=?", id)
	if err != nil {
		return Transaction{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Transaction{}, ErrNotFound
	}
	return s.scanTx(rows)
}

// DeleteTransactions 는 여러 거래를 한 번에 삭제한다.
func (s *Store) DeleteTransactions(ids []int64) error {
	for _, id := range ids {
		if _, err := s.exec(`DELETE FROM transactions WHERE id=?`, id); err != nil {
			return err
		}
	}
	return nil
}

// HasTransaction 은 import 중복 방지용: 같은 날짜+금액+가맹점+방향 거래가 이미 있는지 본다.
func (s *Store) HasTransaction(date string, amount int64, merchant, direction string) (bool, error) {
	var n int
	err := s.queryRow(`SELECT COUNT(*) FROM transactions WHERE date=? AND amount=? AND merchant=? AND direction=?`,
		date, amount, strings.TrimSpace(merchant), direction).Scan(&n)
	return n > 0, err
}

// ---- 규칙 ----

func (s *Store) ListRules() ([]Rule, error) {
	rows, err := s.query(`SELECT id, merchant, amount_min, amount_max, member_id, category_id, label FROM rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		var mid, cid sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Merchant, &r.AmountMin, &r.AmountMax, &mid, &cid, &r.Label); err != nil {
			return nil, err
		}
		r.MemberID = scanNullableID(mid)
		r.CategoryID = scanNullableID(cid)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddRule(r Rule) (int64, error) {
	var id int64
	err := s.queryRow(`INSERT INTO rules(merchant, amount_min, amount_max, member_id, category_id, label) VALUES(?,?,?,?,?,?) RETURNING id`,
		strings.TrimSpace(r.Merchant), r.AmountMin, r.AmountMax, nullableID(r.MemberID), nullableID(r.CategoryID), r.Label).Scan(&id)
	return id, err
}

func (s *Store) UpdateRule(r Rule) error {
	_, err := s.exec(`UPDATE rules SET merchant=?, amount_min=?, amount_max=?, member_id=?, category_id=?, label=? WHERE id=?`,
		strings.TrimSpace(r.Merchant), r.AmountMin, r.AmountMax, nullableID(r.MemberID), nullableID(r.CategoryID), r.Label, r.ID)
	return err
}

func (s *Store) DeleteRule(id int64) error {
	_, err := s.exec(`DELETE FROM rules WHERE id=?`, id)
	return err
}

// ---- 월별 집계 ----

func (s *Store) MonthlySummary(year, month int) (MonthlySummary, error) {
	sum := MonthlySummary{Year: year, Month: month}
	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"

	err := s.queryRow(`
SELECT
	COALESCE(SUM(CASE WHEN direction='income'   THEN amount END), 0),
	COALESCE(SUM(CASE WHEN direction='expense'  THEN amount END), 0),
	COALESCE(SUM(CASE WHEN direction='transfer' THEN amount END), 0)
FROM transactions WHERE date LIKE ?`, prefix).
		Scan(&sum.TotalIncome, &sum.TotalExpense, &sum.TotalTransfer)
	if err != nil {
		return sum, err
	}

	group := func(query string) ([]NamedAmount, error) {
		rows, err := s.query(query, prefix)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []NamedAmount{}
		for rows.Next() {
			var na NamedAmount
			if err := rows.Scan(&na.Name, &na.Kind, &na.Amount); err != nil {
				return nil, err
			}
			out = append(out, na)
		}
		return out, rows.Err()
	}

	if sum.ByMember, err = group(`
SELECT COALESCE(m.name, '미지정'), t.direction, SUM(t.amount)
FROM transactions t LEFT JOIN members m ON m.id = t.member_id
WHERE t.date LIKE ? AND t.direction='expense'
GROUP BY m.name, t.direction ORDER BY SUM(t.amount) DESC`); err != nil {
		return sum, err
	}
	if sum.ByCategory, err = group(`
SELECT COALESCE(c.name, '미분류'), t.direction, SUM(t.amount)
FROM transactions t LEFT JOIN categories c ON c.id = t.category_id
WHERE t.date LIKE ?
GROUP BY c.name, t.direction ORDER BY SUM(t.amount) DESC`); err != nil {
		return sum, err
	}
	if sum.ByPaymentMethod, err = group(`
SELECT COALESCE(p.name, '미지정'), t.direction, SUM(t.amount)
FROM transactions t LEFT JOIN payment_methods p ON p.id = t.payment_method_id
WHERE t.date LIKE ? AND t.direction='expense'
GROUP BY p.name, t.direction ORDER BY SUM(t.amount) DESC`); err != nil {
		return sum, err
	}

	err = s.queryRow(`
SELECT COUNT(*) FROM transactions
WHERE date LIKE ? AND (member_id IS NULL OR category_id IS NULL)`, prefix).
		Scan(&sum.UnclassifiedCount)
	return sum, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
