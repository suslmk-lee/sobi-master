package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ---- 예정 지출 ----
//
// 정기는 아니지만 날짜와 금액을 미리 아는 일회성 지출(결혼식 축의금, 돌잔치, 수리 예약 등)을 미리 적어 둔다.
//
//   - 생활비 통장에서 낼 돈(BasketID 없음): 자금 계획 출금 일정에 그날 넣는다.
//   - 바구니에서 낼 돈: 자금 계획 잔액은 그대로, 바구니 화면에 '예정'으로 미리 빼 보여 준다.
//
// 날짜가 지나도 실제 거래와 연결하기 전까지는 계획에 남긴다(기록을 안 했다고 계획에서 빠지면 잔액 예상이 낙관적이 된다).
// 연결하면(냈어요) 끝 — 그 거래가 잔액 계산에 들어가므로 두 번 세지 않는다. 바구니에서 낸 돈이면 바구니 꺼냄도 함께 남긴다.
// 안 냈으면 '안 냄'으로 끝낸다.

const (
	PlannedOpen      = "planned"
	PlannedDone      = "done"
	PlannedCancelled = "cancelled"
)

// plannedMatchDays 는 '냈어요' 후보를 찾는 범위(예정일 앞뒤).
const plannedMatchDays = 14

const plannedSchema = `
CREATE TABLE IF NOT EXISTS planned_expenses (
	id             BIGSERIAL PRIMARY KEY,
	date           TEXT NOT NULL,
	name           TEXT NOT NULL,
	amount         BIGINT NOT NULL,
	category_id    BIGINT REFERENCES categories(id) ON DELETE SET NULL,
	member_id      BIGINT REFERENCES members(id) ON DELETE SET NULL,
	basket_id      BIGINT REFERENCES baskets(id) ON DELETE SET NULL,
	memo           TEXT NOT NULL DEFAULT '',
	status         TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','done','cancelled')),
	transaction_id BIGINT REFERENCES transactions(id) ON DELETE SET NULL,
	created_at     TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_planned_tx ON planned_expenses(transaction_id) WHERE transaction_id IS NOT NULL;
`

// PlannedExpense 는 예정 지출 하나.
type PlannedExpense struct {
	ID            int64  `json:"id"`
	Date          string `json:"date"` // 예정일
	Name          string `json:"name"`
	Amount        int64  `json:"amount"`
	CategoryID    *int64 `json:"categoryId"`
	MemberID      *int64 `json:"memberId"`
	BasketID      *int64 `json:"basketId"` // 바구니에서 낼 돈(없으면 생활비 통장)
	Memo          string `json:"memo"`
	Status        string `json:"status"`
	TransactionID *int64 `json:"transactionId"`
	// 조회 편의용
	BasketName   string `json:"basketName"`
	CategoryName string `json:"categoryName"`
	TxDate       string `json:"txDate"`
	TxAmount     int64  `json:"txAmount"`
	TxMerchant   string `json:"txMerchant"`
}

const plannedSelect = `
SELECT p.id, p.date, p.name, p.amount, p.category_id, p.member_id, p.basket_id, p.memo, p.status, p.transaction_id,
       COALESCE(b.name, ''), COALESCE(cp.name || ' > ' || c.name, c.name, ''),
       COALESCE(t.date, ''), COALESCE(t.amount, 0), COALESCE(t.merchant, '')
FROM planned_expenses p
LEFT JOIN baskets b ON b.id = p.basket_id
LEFT JOIN categories c ON c.id = p.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
LEFT JOIN transactions t ON t.id = p.transaction_id`

func scanPlanned(rows *sql.Rows) ([]PlannedExpense, error) {
	defer rows.Close()
	out := []PlannedExpense{}
	for rows.Next() {
		var p PlannedExpense
		var cid, mid, bid, tid sql.NullInt64
		if err := rows.Scan(&p.ID, &p.Date, &p.Name, &p.Amount, &cid, &mid, &bid, &p.Memo, &p.Status, &tid,
			&p.BasketName, &p.CategoryName, &p.TxDate, &p.TxAmount, &p.TxMerchant); err != nil {
			return nil, err
		}
		p.CategoryID, p.MemberID, p.BasketID, p.TransactionID = scanNullableID(cid), scanNullableID(mid), scanNullableID(bid), scanNullableID(tid)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListPlannedExpenses 는 아직 끝나지 않은 예정 지출 전체와, 끝낸 것 중 최근 doneDays 일 안의 것(예정일 순).
func (s *Store) ListPlannedExpenses(today time.Time, doneDays int) ([]PlannedExpense, error) {
	since := fmtDay(dayOf(today).AddDate(0, 0, -doneDays))
	rows, err := s.query(plannedSelect+` WHERE p.status = 'planned' OR p.date >= ? ORDER BY p.date, p.id`, since)
	if err != nil {
		return nil, err
	}
	return scanPlanned(rows)
}

func (s *Store) getPlanned(id int64) (PlannedExpense, error) {
	rows, err := s.query(plannedSelect+` WHERE p.id = ?`, id)
	if err != nil {
		return PlannedExpense{}, err
	}
	out, err := scanPlanned(rows)
	if err != nil {
		return PlannedExpense{}, err
	}
	if len(out) == 0 {
		return PlannedExpense{}, ErrNotFound
	}
	return out[0], nil
}

// SavePlannedExpense 는 예정 지출을 추가하거나 고친다(끝난 건 고칠 수 없다 — 연결을 풀고 고친다).
func (s *Store) SavePlannedExpense(p PlannedExpense) (PlannedExpense, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return p, fmt.Errorf("무슨 지출인지 이름을 입력하세요")
	}
	if _, err := time.Parse(dateLayout, p.Date); err != nil {
		return p, fmt.Errorf("날짜를 고르세요")
	}
	if p.Amount <= 0 {
		return p, fmt.Errorf("금액을 입력하세요")
	}
	if p.BasketID != nil && *p.BasketID == 0 {
		p.BasketID = nil
	}
	args := []interface{}{p.Date, p.Name, p.Amount, nullableID(p.CategoryID), nullableID(p.MemberID), nullableID(p.BasketID), strings.TrimSpace(p.Memo)}
	if p.ID == 0 {
		err := s.queryRow(`
INSERT INTO planned_expenses(date, name, amount, category_id, member_id, basket_id, memo, created_at)
VALUES(?,?,?,?,?,?,?,?) RETURNING id`, append(args, nowFunc().Format("2006-01-02 15:04:05"))...).Scan(&p.ID)
		if err != nil {
			return p, err
		}
	} else {
		res, err := s.exec(`
UPDATE planned_expenses SET date=?, name=?, amount=?, category_id=?, member_id=?, basket_id=?, memo=?
WHERE id=? AND status='planned'`, append(args, p.ID)...)
		if err != nil {
			return p, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return p, fmt.Errorf("이미 끝낸 예정 지출은 고칠 수 없습니다")
		}
	}
	return s.getPlanned(p.ID)
}

// DeletePlannedExpense 는 예정 지출을 지운다(연결한 거래와 바구니 꺼냄은 그대로 남는다).
func (s *Store) DeletePlannedExpense(id int64) error {
	_, err := s.exec(`DELETE FROM planned_expenses WHERE id=?`, id)
	return err
}

// CancelPlannedExpense 는 '안 냄'으로 끝낸다. 되돌리면(reopen) 다시 예정으로.
func (s *Store) CancelPlannedExpense(id int64, reopen bool) error {
	q := `UPDATE planned_expenses SET status='cancelled' WHERE id=? AND status='planned'`
	if reopen {
		q = `UPDATE planned_expenses SET status='planned', transaction_id=NULL WHERE id=? AND status='cancelled'`
	}
	res, err := s.exec(q, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PlannedCandidates 는 '냈어요'로 연결할 만한 지출: 예정일 앞뒤 2주의 지출 중 다른 예정 지출에 묶이지 않은 것,
// 금액이 가까운 것 먼저(같으면 날짜가 가까운 것). 최대 10개.
func (s *Store) PlannedCandidates(id int64) ([]Transaction, error) {
	p, err := s.getPlanned(id)
	if err != nil {
		return nil, err
	}
	d, err := time.Parse(dateLayout, p.Date)
	if err != nil {
		return nil, err
	}
	rows, err := s.query(`
SELECT t.id FROM transactions t
WHERE t.direction = 'expense' AND t.date >= ? AND t.date <= ?
  AND NOT EXISTS (SELECT 1 FROM planned_expenses pe WHERE pe.transaction_id = t.id AND pe.id <> ?)`,
		fmtDay(d.AddDate(0, 0, -plannedMatchDays)), fmtDay(d.AddDate(0, 0, plannedMatchDays)), id)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, tid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []Transaction{}
	for _, tid := range ids {
		t, err := s.GetTransaction(tid)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	abs := func(n int64) int64 {
		if n < 0 {
			return -n
		}
		return n
	}
	dayGap := func(date string) int64 {
		t, err := time.Parse(dateLayout, date)
		if err != nil {
			return 1 << 30
		}
		return abs(int64(t.Sub(d).Hours() / 24))
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := abs(out[i].Amount-p.Amount), abs(out[j].Amount-p.Amount)
		if ai != aj {
			return ai < aj
		}
		return dayGap(out[i].Date) < dayGap(out[j].Date)
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out, nil
}

// SettlePlannedExpense 는 예정 지출을 실제 거래와 이어 끝낸다(냈어요). 바구니에서 낼 돈이었으면 그 거래를 바구니
// 꺼냄으로도 남긴다(moved: 생활비 통장으로 옮겨 온 돈인지). 그 거래가 이미 같은 바구니에서 꺼낸 것으로 있으면 두지 않는다.
func (s *Store) SettlePlannedExpense(id, txID int64, moved bool) (PlannedExpense, error) {
	p, err := s.getPlanned(id)
	if err != nil {
		return p, err
	}
	if p.Status != PlannedOpen {
		return p, fmt.Errorf("이미 끝낸 예정 지출입니다")
	}
	t, err := s.GetTransaction(txID)
	if err != nil {
		return p, err
	}
	if t.Direction != "expense" {
		return p, fmt.Errorf("지출 거래만 연결할 수 있습니다")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(pq(`UPDATE planned_expenses SET status='done', transaction_id=? WHERE id=?`), txID, id); err != nil {
		if strings.Contains(err.Error(), "idx_planned_tx") {
			return p, fmt.Errorf("이 거래는 이미 다른 예정 지출에 연결돼 있습니다")
		}
		return p, err
	}
	if p.BasketID != nil {
		var other sql.NullInt64
		err := tx.QueryRow(pq(`SELECT basket_id FROM basket_entries WHERE transaction_id=?`), txID).Scan(&other)
		switch {
		case err == sql.ErrNoRows:
			if _, err := tx.Exec(pq(`
INSERT INTO basket_entries(basket_id, date, kind, amount, transaction_id, moved, memo, created_at) VALUES(?,?,?,?,?,?,?,?)`),
				*p.BasketID, t.Date, BasketSpend, -t.Amount, txID, boolToInt(moved), p.Name, nowFunc().Format("2006-01-02 15:04:05")); err != nil {
				return p, err
			}
		case err != nil:
			return p, err
		case other.Int64 != *p.BasketID:
			return p, fmt.Errorf("이 거래는 이미 다른 바구니에서 꺼낸 것으로 기록돼 있습니다")
		}
	}
	if err := tx.Commit(); err != nil {
		return p, err
	}
	return s.getPlanned(id)
}

// plannedEvents 는 생활비 통장에서 낼 예정 지출의 출금 일정. 날짜가 지났는데 아직 연결하지 않은 건 오늘로 당겨 남긴다.
func plannedEvents(list []PlannedExpense, today, horizon time.Time) []CashEvent {
	var out []CashEvent
	todayStr, horizonStr := fmtDay(today), fmtDay(horizon)
	for _, p := range list {
		if p.Status != PlannedOpen || p.BasketID != nil {
			continue
		}
		date, label := p.Date, p.Name
		if date < todayStr {
			date, label = todayStr, p.Name+" (예정일 지남)"
		}
		if date > horizonStr {
			continue
		}
		out = append(out, CashEvent{Date: date, Kind: "planned", Label: label, Amount: -p.Amount, Estimated: true, RefID: p.ID})
	}
	return out
}

// basketPlanned 는 바구니 id → 그 바구니에서 낼 아직 끝나지 않은 예정 지출.
func basketPlanned(list []PlannedExpense) map[int64][]PlannedExpense {
	out := map[int64][]PlannedExpense{}
	for _, p := range list {
		if p.Status == PlannedOpen && p.BasketID != nil {
			out[*p.BasketID] = append(out[*p.BasketID], p)
		}
	}
	return out
}
