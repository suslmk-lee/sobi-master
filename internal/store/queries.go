package store

import (
	"database/sql"
	"fmt"
	"strconv"
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

// ListCategories 는 주/부 계층 순서로 돌려준다: 주 그룹별로 묶이고, 각 그룹은
// 주가 먼저 오고 그 뒤에 부가 이름순으로 따라온다. (드롭다운에 그대로 뿌릴 수 있는 순서)
func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.query(`
SELECT c.id, c.name, c.kind, c.parent_id, COALESCE(p.name, '')
FROM categories c LEFT JOIN categories p ON p.id = c.parent_id
ORDER BY COALESCE(p.kind, c.kind), COALESCE(p.name, c.name),
         (c.parent_id IS NOT NULL), c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		var pid sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &pid, &c.Parent); err != nil {
			return nil, err
		}
		c.ParentID = scanNullableID(pid)
		c.FullName = c.Name
		if c.Parent != "" {
			c.FullName = c.Parent + " > " + c.Name
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddCategory 는 카테고리를 추가한다. parentID>0 이면 그 주의 부로 만들고 kind 는 주를 따른다.
// 계층은 2단까지만 허용한다(부의 부 금지).
func (s *Store) AddCategory(name, kind string, parentID int64) (Category, error) {
	name = strings.TrimSpace(name)
	c := Category{Name: name, Kind: kind}
	var parent interface{}
	if parentID > 0 {
		var pKind string
		var pParent sql.NullInt64
		if err := s.queryRow(`SELECT kind, parent_id FROM categories WHERE id=?`, parentID).
			Scan(&pKind, &pParent); err != nil {
			return Category{}, fmt.Errorf("상위 카테고리를 찾을 수 없습니다: %w", err)
		}
		if pParent.Valid {
			return Category{}, fmt.Errorf("부 카테고리 아래에 다시 부를 만들 수 없습니다 (2단까지)")
		}
		c.Kind = pKind // 부는 주의 종류를 따른다
		pid := parentID
		c.ParentID = &pid
		parent = parentID
	}
	var id int64
	err := s.queryRow(`INSERT INTO categories(name, kind, parent_id) VALUES(?,?,?) RETURNING id`,
		name, c.Kind, parent).Scan(&id)
	if err != nil {
		return Category{}, err
	}
	c.ID = id
	return c, nil
}

// SetCategoryParent 는 카테고리를 다른 주 아래로 옮긴다(parentID=0 이면 주로 승격).
// 2단 계층을 지키기 위해 부가 있는 카테고리는 다른 주 아래로 옮길 수 없다.
func (s *Store) SetCategoryParent(id, parentID int64) error {
	if id == parentID {
		return fmt.Errorf("자기 자신을 상위로 지정할 수 없습니다")
	}
	if parentID == 0 {
		_, err := s.exec(`UPDATE categories SET parent_id=NULL WHERE id=?`, id)
		return err
	}
	var childCount int
	if err := s.queryRow(`SELECT COUNT(*) FROM categories WHERE parent_id=?`, id).Scan(&childCount); err != nil {
		return err
	}
	if childCount > 0 {
		return fmt.Errorf("이 카테고리에는 부 카테고리가 있어 다른 주 아래로 옮길 수 없습니다 (2단까지)")
	}
	var pKind string
	var pParent sql.NullInt64
	if err := s.queryRow(`SELECT kind, parent_id FROM categories WHERE id=?`, parentID).
		Scan(&pKind, &pParent); err != nil {
		return fmt.Errorf("상위 카테고리를 찾을 수 없습니다: %w", err)
	}
	if pParent.Valid {
		return fmt.Errorf("부 카테고리를 상위로 지정할 수 없습니다 (2단까지)")
	}
	// 부는 주의 종류(수입/지출/이체)를 따른다
	_, err := s.exec(`UPDATE categories SET parent_id=?, kind=? WHERE id=?`, parentID, pKind, id)
	return err
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

// 카테고리는 부일 때 "주 > 부" 전체 경로로 보여준다(부 이름만으로는 모호하므로).
const txSelect = `
SELECT t.id, t.date, t.amount, t.direction, t.merchant, t.memo,
       t.member_id, t.category_id, t.payment_method_id, t.source, t.auto_classified,
       COALESCE(m.name, ''),
       CASE WHEN c.id IS NULL THEN ''
            WHEN cp.name IS NULL THEN c.name
            ELSE cp.name || ' > ' || c.name END,
       COALESCE(p.name, '')
FROM transactions t
LEFT JOIN members m ON m.id = t.member_id
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
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

// sortColumn 은 정렬 키("<컬럼>_<asc|desc>")를 안전한 정렬식으로 매핑한다.
// isDate 는 날짜 정렬 여부(보조 정렬 방식이 달라진다). 화이트리스트만 허용해 주입 위험 없음.
// 이름 컬럼은 NULL(미분류/미지정)을 항상 뒤로 보낸다.
func sortColumn(sort string) (orderExpr, dir string, isDate bool) {
	dir = "DESC"
	key := sort
	if strings.HasSuffix(sort, "_asc") {
		dir = "ASC"
		key = strings.TrimSuffix(sort, "_asc")
	} else if strings.HasSuffix(sort, "_desc") {
		key = strings.TrimSuffix(sort, "_desc")
	}
	switch key {
	case "amount":
		return "t.amount " + dir, dir, false
	case "direction":
		return "t.direction " + dir, dir, false
	case "merchant":
		return "t.merchant " + dir, dir, false
	case "member":
		return "m.name " + dir + " NULLS LAST", dir, false
	case "category":
		// 주 그룹 → 그 안의 부 순서로 정렬
		return "COALESCE(cp.name, c.name) " + dir + " NULLS LAST, c.name " + dir + " NULLS LAST", dir, false
	case "payment":
		return "p.name " + dir + " NULLS LAST", dir, false
	default:
		return "t.date " + dir, dir, true
	}
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
		// 주 카테고리를 고르면 그 하위 부 거래까지 포함한다
		conds = append(conds, `t.category_id IN (SELECT id FROM categories WHERE id=? OR parent_id=?)`)
		args = append(args, f.CategoryID, f.CategoryID)
	}
	if f.PaymentMethodID > 0 {
		conds = append(conds, `t.payment_method_id = ?`)
		args = append(args, f.PaymentMethodID)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	// 정렬: "<컬럼>_<asc|desc>". 미분류(NULL) 이름은 항상 뒤로 보낸다.
	orderExpr, dir, isDate := sortColumn(f.Sort)
	q += " ORDER BY " + orderExpr
	if isDate {
		q += ", t.id " + dir
	} else { // 동일 값일 때 최신 거래가 먼저 오도록 보조 정렬
		q += ", t.date DESC, t.id DESC"
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

// idChunk 는 IN (...) 절에 한 번에 넣을 ID 개수. Postgres 의 파라미터 상한(65535)에
// 걸리지 않도록 넉넉히 잘라 보낸다.
const idChunk = 1000

// idPlaceholders 는 IN (...) 에 쓸 "?,?,?" 와 인자 슬라이스를 만든다.
func idPlaceholders(ids []int64) (string, []interface{}) {
	ph := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return strings.Join(ph, ","), args
}

// DeleteTransactions 는 여러 거래를 한 번에 삭제한다(ID 묶음당 쿼리 1회).
func (s *Store) DeleteTransactions(ids []int64) error {
	for start := 0; start < len(ids); start += idChunk {
		end := start + idChunk
		if end > len(ids) {
			end = len(ids)
		}
		ph, args := idPlaceholders(ids[start:end])
		if _, err := s.exec(`DELETE FROM transactions WHERE id IN (`+ph+`)`, args...); err != nil {
			return err
		}
	}
	return nil
}

// GetTransactions 는 여러 거래를 ID 묶음당 쿼리 1회로 읽는다(일괄 처리용).
// 존재하지 않는 ID 는 결과에서 빠지므로, 호출 측에서 ID 순서가 필요하면 직접 정렬한다.
func (s *Store) GetTransactions(ids []int64) ([]Transaction, error) {
	out := make([]Transaction, 0, len(ids))
	for start := 0; start < len(ids); start += idChunk {
		end := start + idChunk
		if end > len(ids) {
			end = len(ids)
		}
		ph, args := idPlaceholders(ids[start:end])
		rows, err := s.query(txSelect+" WHERE t.id IN ("+ph+")", args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			t, err := s.scanTx(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// MerchantSuggestions 는 query 를 포함하는 과거 가맹점들을, 각 가맹점의 가장 최근
// 거래(메모/귀속자/카테고리/결제수단/구분)와 함께 최근순으로 돌려준다(자동완성용).
func (s *Store) MerchantSuggestions(query string, limit int) ([]MerchantSuggestion, error) {
	if limit < 1 {
		limit = 8
	}
	like := "%" + likeEscape(query) + "%"
	rows, err := s.query(`
SELECT sub.merchant, sub.memo, sub.direction, sub.member_id, sub.category_id, sub.payment_method_id
FROM (
	SELECT DISTINCT ON (merchant)
	       merchant, memo, direction, member_id, category_id, payment_method_id, date, id
	FROM transactions
	WHERE merchant ILIKE ? AND merchant <> ''
	ORDER BY merchant, date DESC, id DESC
) sub
ORDER BY sub.date DESC, sub.id DESC
LIMIT ?`, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MerchantSuggestion{}
	for rows.Next() {
		var m MerchantSuggestion
		var mid, cid, pid sql.NullInt64
		if err := rows.Scan(&m.Merchant, &m.Memo, &m.Direction, &mid, &cid, &pid); err != nil {
			return nil, err
		}
		m.MemberID = scanNullableID(mid)
		m.CategoryID = scanNullableID(cid)
		m.PaymentMethodID = scanNullableID(pid)
		out = append(out, m)
	}
	return out, rows.Err()
}

// HasTransaction 은 import 중복 방지용: 같은 날짜+금액+가맹점+방향 거래가 이미 있는지 본다.
// 여러 행을 한꺼번에 확인할 때는 ExistingTxKeys 로 키 집합을 한 번에 읽는다.
func (s *Store) HasTransaction(date string, amount int64, merchant, direction string) (bool, error) {
	var n int
	err := s.queryRow(`SELECT COUNT(*) FROM transactions WHERE date=? AND amount=? AND merchant=? AND direction=?`,
		date, amount, strings.TrimSpace(merchant), direction).Scan(&n)
	return n > 0, err
}

// TxKey 는 import 중복 판정 기준(날짜+금액+가맹점+방향)을 한 문자열로 묶는다.
// 가맹점명은 저장 시와 같게 trim 한다. 구분자는 데이터에 나올 수 없는 NUL 을 쓴다.
func TxKey(date string, amount int64, merchant, direction string) string {
	return date + "\x00" + strconv.FormatInt(amount, 10) + "\x00" +
		strings.TrimSpace(merchant) + "\x00" + direction
}

// ExistingTxKeys 는 [from, to] 기간 거래의 중복 판정 키 집합을 쿼리 1회로 읽는다.
// 중복 판정 키에 날짜가 들어가므로, CSV 에 들어 있는 날짜 범위만 읽으면
// 행마다 HasTransaction 을 부르는 것과 결과가 같다.
func (s *Store) ExistingTxKeys(from, to string) (map[string]struct{}, error) {
	rows, err := s.query(
		`SELECT date, amount, merchant, direction FROM transactions WHERE date >= ? AND date <= ?`,
		from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := map[string]struct{}{}
	for rows.Next() {
		var date, merchant, direction string
		var amount int64
		if err := rows.Scan(&date, &amount, &merchant, &direction); err != nil {
			return nil, err
		}
		keys[TxKey(date, amount, merchant, direction)] = struct{}{}
	}
	return keys, rows.Err()
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
	// 카테고리는 주 기준으로 합산한다(부 지출은 상위 주에 포함)
	if sum.ByCategory, err = group(`
SELECT COALESCE(cp.name, c.name, '미분류'), t.direction, SUM(t.amount)
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
WHERE t.date LIKE ?
GROUP BY 1, t.direction ORDER BY SUM(t.amount) DESC`); err != nil {
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
