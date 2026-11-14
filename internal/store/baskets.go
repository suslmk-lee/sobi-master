package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"sobi/internal/holiday"
)

// ---- 목돈 바구니 ----
//
// 갑자기 나가는 지출(경조사·자동차 수리·부모님 병원비)과 해마다 정해진 큰 지출(재산세·자동차세·자동차보험)을
// 위해 미리 조금씩 모아 두는 바구니. 바구니마다 자기 통장(세이프박스 등)이 있고, 잔액은 바구니 장부로 따라간다:
//
//	open   — 만들 때 이미 들어 있던 돈
//	fill   — 매달 채움(자동이체면 채우는 날 앱이 한 달 한 번 기록)
//	extra  — 여윳돈 더 넣기
//	spend  — 꺼내 씀(보통 지출 거래 하나에 연결). Moved 면 생활비 통장으로 옮겨 온 돈
//	adjust — 잔액 맞춤(이자 등, 부호 있음)
//
// 바구니 통장은 생활비 통장(자금 계획의 현재 잔액)과 따로다. 자금 계획은 이 장부로 생활비 통장에서 오간 돈만
// 반영한다: 채움·더 넣기는 빠지고, 옮겨 온 돈은 들어온다. 바구니 통장으로 바로 결제한 거래는 잔액 계산에서 뺀다.
// 이체를 거래로 따로 만들지 않아 통계가 어지러워지지 않는다.
//
// 종류
//
//	scheduled — 정해진 연간 지출: 항목(이름·달·일·금액)이 해마다 돌아온다. 마감마다 다 차도록 권장 월 적립액을 계산한다.
//	reserve   — 예비비: 상한까지 채워 두고, 쓰면 다시 채운다.

const (
	BasketScheduled = "scheduled"
	BasketReserve   = "reserve"
)

// 장부 항목 종류.
const (
	BasketOpen   = "open"
	BasketFill   = "fill"
	BasketExtra  = "extra"
	BasketSpend  = "spend"
	BasketAdjust = "adjust"
)

// basketHorizonMonths 는 정해진 지출을 내다보는 달 수(이번 달 포함).
const basketHorizonMonths = 12

const basketSchema = `
CREATE TABLE IF NOT EXISTS baskets (
	id                BIGSERIAL PRIMARY KEY,
	name              TEXT NOT NULL,
	kind              TEXT NOT NULL CHECK (kind IN ('scheduled','reserve')),
	payment_method_id BIGINT REFERENCES payment_methods(id) ON DELETE SET NULL,
	cap               BIGINT NOT NULL DEFAULT 0,
	monthly           BIGINT NOT NULL DEFAULT 0,
	auto_fill         INTEGER NOT NULL DEFAULT 0,
	fill_day          INTEGER NOT NULL DEFAULT 10,
	fill_since        TEXT NOT NULL DEFAULT '',
	sort_order        INTEGER NOT NULL DEFAULT 0,
	active            INTEGER NOT NULL DEFAULT 1,
	memo              TEXT NOT NULL DEFAULT '',
	created_at        TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS basket_items (
	id        BIGSERIAL PRIMARY KEY,
	basket_id BIGINT NOT NULL REFERENCES baskets(id) ON DELETE CASCADE,
	name      TEXT NOT NULL,
	month     INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12),
	day       INTEGER NOT NULL DEFAULT 0,
	amount    BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_basket_items_basket ON basket_items(basket_id);
CREATE TABLE IF NOT EXISTS basket_entries (
	id             BIGSERIAL PRIMARY KEY,
	basket_id      BIGINT NOT NULL REFERENCES baskets(id) ON DELETE CASCADE,
	date           TEXT NOT NULL,
	kind           TEXT NOT NULL CHECK (kind IN ('open','fill','extra','spend','adjust')),
	amount         BIGINT NOT NULL,
	auto           INTEGER NOT NULL DEFAULT 0,
	ym             TEXT NOT NULL DEFAULT '',
	transaction_id BIGINT REFERENCES transactions(id) ON DELETE SET NULL,
	item_id        BIGINT REFERENCES basket_items(id) ON DELETE SET NULL,
	due_ym         TEXT NOT NULL DEFAULT '',
	moved          INTEGER NOT NULL DEFAULT 0,
	memo           TEXT NOT NULL DEFAULT '',
	created_at     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_basket_entries_basket ON basket_entries(basket_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_basket_autofill ON basket_entries(basket_id, ym) WHERE auto = 1;
CREATE UNIQUE INDEX IF NOT EXISTS idx_basket_entry_tx ON basket_entries(transaction_id) WHERE transaction_id IS NOT NULL;
`

// basketTxFilter 는 바구니 통장으로 결제한 거래를 빼는 조건(자금 계획 잔액용). t 는 transactions 별칭.
const basketTxFilter = `NOT EXISTS (SELECT 1 FROM baskets bk WHERE bk.payment_method_id = t.payment_method_id)`

// Basket 은 바구니 하나.
type Basket struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"` // scheduled | reserve
	PaymentMethodID *int64 `json:"paymentMethodId"`
	Cap             int64  `json:"cap"`       // reserve: 상한(0 = 없음)
	Monthly         int64  `json:"monthly"`   // 매달 넣는 돈(자동이체 금액)
	AutoFill        bool   `json:"autoFill"`  // 매달 자동이체 — 채우는 날 앱이 채움을 기록
	FillDay         int    `json:"fillDay"`   // 채우는 날(1~31, 말일보다 크면 말일, 쉬는 날이면 다음 영업일)
	FillSince       string `json:"fillSince"` // 자동 기록을 시작하는 달("YYYY-MM"). 서버가 정한다
	SortOrder       int    `json:"sortOrder"`
	Active          bool   `json:"active"`
	Memo            string `json:"memo"`
	Items           []BasketItem `json:"items"` // scheduled 의 항목
	// 조회 편의용
	PaymentMethodName string `json:"paymentMethodName"`
}

// BasketItem 은 정해진 연간 지출 한 항목(해마다 Month 월 Day 일까지 Amount 원).
type BasketItem struct {
	ID       int64  `json:"id"`
	BasketID int64  `json:"basketId"`
	Name     string `json:"name"`
	Month    int    `json:"month"`
	Day      int    `json:"day"` // 0 이면 말일
	Amount   int64  `json:"amount"`
}

// BasketEntry 는 바구니 장부 한 줄.
type BasketEntry struct {
	ID            int64  `json:"id"`
	BasketID      int64  `json:"basketId"`
	Date          string `json:"date"`
	Kind          string `json:"kind"`
	Amount        int64  `json:"amount"` // 바구니 잔액 변화(채움 +, 꺼냄 −)
	Auto          bool   `json:"auto"`   // 자동이체로 앱이 기록한 채움
	YM            string `json:"ym"`
	TransactionID *int64 `json:"transactionId"`
	ItemID        *int64 `json:"itemId"`
	DueYM         string `json:"dueYm"` // 정해진 지출을 낸 꺼냄: 어느 달 몫
	Moved         bool   `json:"moved"` // 꺼낸 돈을 생활비 통장으로 옮겨 옴
	Memo          string `json:"memo"`
	// 조회 편의용
	TxMerchant string `json:"txMerchant"`
	TxAmount   int64  `json:"txAmount"`
	ItemName   string `json:"itemName"`
}

// BasketDue 는 정해진 지출의 한 번(어느 항목의 어느 달).
type BasketDue struct {
	ItemID    int64  `json:"itemId"`
	Name      string `json:"name"`
	YM        string `json:"ym"`
	Date      string `json:"date"` // 마감일
	Amount    int64  `json:"amount"`
	Paid      bool   `json:"paid"`      // 이 바구니에서 꺼내 낸 기록이 있음
	Projected int64  `json:"projected"` // 지금 월 적립액으로 이 마감에 남아 있을 돈(이 항목을 내기 전)
	Short     int64  `json:"short"`     // 그때까지 모자라는 돈(누적)
}

// BasketStatus 는 바구니 화면 자료.
type BasketStatus struct {
	Basket  Basket        `json:"basket"`
	Balance int64         `json:"balance"`
	Entries []BasketEntry `json:"entries"` // 최근 먼저
	// 다음 채움
	NextFillDate   string `json:"nextFillDate"`
	NextFillAmount int64  `json:"nextFillAmount"`
	// 예비비
	CapReached  bool `json:"capReached"`
	MonthsToCap int  `json:"monthsToCap"` // 지금 월 적립액으로 상한까지 남은 달 수(0 = 이미 닿음·모름)
	// 정해진 지출
	Dues        []BasketDue `json:"dues"`        // 앞으로 12개월(이번 달 포함) 마감
	AnnualTotal int64       `json:"annualTotal"` // 항목 금액 합(1년)
	Recommended int64       `json:"recommended"` // 앞으로의 마감을 모두 맞추는 가장 작은 월 적립액(천 원 올림)
	ShortName   string      `json:"shortName"`   // 지금 월 적립액으로 처음 모자라는 마감
	ShortYM     string      `json:"shortYm"`
	Short       int64       `json:"short"`

	autoDone map[string]bool // 자동 채움을 기록한 달(장부 전체 기준, 화면에는 안 보냄)
}

func validBasketKind(k string) bool { return k == BasketScheduled || k == BasketReserve }

// basketDay 는 y년 m월 day 일(말일보다 크거나 0 이하면 말일 — 항목의 0 은 '말일까지').
func basketDay(y int, m time.Month, day int) time.Time {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day <= 0 || day > last {
		day = last
	}
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// basketFillDate 는 ym 달 채우는 날: FillDay(말일 보정), 쉬는 날이면 다음 영업일(자동이체가 그날 나간다).
func basketFillDate(b Basket, ym string, cal *holiday.Calendar) (time.Time, error) {
	t, err := time.Parse("2006-01", ym)
	if err != nil {
		return time.Time{}, err
	}
	return cal.NextBusinessDay(basketDay(t.Year(), t.Month(), b.FillDay)), nil
}

func ceilDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// BuildBasketStatus 는 바구니 화면 자료를 계산한다(순수 함수).
// entries 는 이 바구니의 장부 전체.
func BuildBasketStatus(b Basket, entries []BasketEntry, today time.Time, cal *holiday.Calendar) BasketStatus {
	today = dayOf(today)
	todayStr := fmtDay(today)
	st := BasketStatus{Basket: b, Entries: []BasketEntry{}, Dues: []BasketDue{}, autoDone: map[string]bool{}}
	if st.Basket.Items == nil {
		st.Basket.Items = []BasketItem{}
	}
	autoDone := st.autoDone
	paid := map[string]bool{} // "항목id/달"
	for _, e := range entries {
		st.Balance += e.Amount
		if e.Auto {
			autoDone[e.YM] = true
		}
		if e.Kind == BasketSpend && e.ItemID != nil && e.DueYM != "" {
			paid[fmt.Sprintf("%d/%s", *e.ItemID, e.DueYM)] = true
		}
	}
	sorted := append([]BasketEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Date != sorted[j].Date {
			return sorted[i].Date > sorted[j].Date
		}
		return sorted[i].ID > sorted[j].ID
	})
	if len(sorted) > 30 {
		sorted = sorted[:30]
	}
	st.Entries = append(st.Entries, sorted...)

	// 앞으로의 채우는 날: 이번 달부터, 이미 자동 기록한 달과 지난 날은 빼고
	curYM := today.Format("2006-01")
	var fills []string
	for ym, n := curYM, 0; n <= basketHorizonMonths; ym, n = addYM(ym, 1), n+1 {
		if autoDone[ym] {
			continue
		}
		d, err := basketFillDate(b, ym, cal)
		if err != nil || fmtDay(d) < todayStr {
			continue
		}
		fills = append(fills, fmtDay(d))
	}
	if b.Active && b.Monthly > 0 && len(fills) > 0 {
		st.NextFillDate, st.NextFillAmount = fills[0], b.Monthly
	}
	fillsBy := func(date string) int64 {
		var n int64
		for _, f := range fills {
			if f <= date {
				n++
			}
		}
		return n
	}

	switch b.Kind {
	case BasketReserve:
		if b.Cap > 0 {
			st.CapReached = st.Balance >= b.Cap
			if !st.CapReached && b.Monthly > 0 {
				st.MonthsToCap = int(ceilDiv(b.Cap-st.Balance, b.Monthly))
			}
		}
	case BasketScheduled:
		for _, it := range b.Items {
			st.AnnualTotal += it.Amount
		}
		// 앞으로 12개월 마감(이번 달 포함). 마감일이 지난 건 이미 냈다고 본다.
		for ym, n := curYM, 0; n < basketHorizonMonths; ym, n = addYM(ym, 1), n+1 {
			t, _ := time.Parse("2006-01", ym)
			for _, it := range b.Items {
				if int(t.Month()) != it.Month {
					continue
				}
				due := fmtDay(basketDay(t.Year(), t.Month(), it.Day))
				if due < todayStr {
					continue
				}
				st.Dues = append(st.Dues, BasketDue{ItemID: it.ID, Name: it.Name, YM: ym, Date: due, Amount: it.Amount,
					Paid: paid[fmt.Sprintf("%d/%s", it.ID, ym)]})
			}
		}
		sort.SliceStable(st.Dues, func(i, j int) bool { return st.Dues[i].Date < st.Dues[j].Date })
		// 아직 안 낸 마감을 차례로: 그때까지 낼 돈(누적) ≤ 지금 잔액 + 그때까지 채움
		var cum, need int64
		for i := range st.Dues {
			d := &st.Dues[i]
			if d.Paid {
				continue
			}
			n := fillsBy(d.Date)
			d.Projected = st.Balance + n*b.Monthly - cum
			cum += d.Amount
			if gap := cum - st.Balance - n*b.Monthly; gap > 0 {
				d.Short = gap
				if st.ShortName == "" {
					st.ShortName, st.ShortYM, st.Short = d.Name, d.YM, gap
				}
			}
			if n > 0 {
				if r := ceilDiv(cum-st.Balance, n); r > need {
					need = r
				}
			}
		}
		st.Recommended = ceilDiv(need, 1000) * 1000
	}
	return st
}

// ---- 저장 ----

const basketCols = `b.id, b.name, b.kind, b.payment_method_id, b.cap, b.monthly, b.auto_fill, b.fill_day, b.fill_since,
	b.sort_order, b.active, b.memo, COALESCE(p.name, '')`

// ListBaskets 는 바구니 전체(정렬 순서, 항목 포함).
func (s *Store) ListBaskets() ([]Basket, error) {
	rows, err := s.query(`SELECT ` + basketCols + ` FROM baskets b LEFT JOIN payment_methods p ON p.id = b.payment_method_id
ORDER BY b.active DESC, b.sort_order, b.id`)
	if err != nil {
		return nil, err
	}
	out := []Basket{}
	byID := map[int64]int{}
	for rows.Next() {
		var b Basket
		var pm sql.NullInt64
		var auto, active int
		if err := rows.Scan(&b.ID, &b.Name, &b.Kind, &pm, &b.Cap, &b.Monthly, &auto, &b.FillDay, &b.FillSince,
			&b.SortOrder, &active, &b.Memo, &b.PaymentMethodName); err != nil {
			rows.Close()
			return nil, err
		}
		b.PaymentMethodID = scanNullableID(pm)
		b.AutoFill, b.Active = auto == 1, active == 1
		b.Items = []BasketItem{}
		byID[b.ID] = len(out)
		out = append(out, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	irows, err := s.query(`SELECT id, basket_id, name, month, day, amount FROM basket_items ORDER BY month, day, id`)
	if err != nil {
		return nil, err
	}
	defer irows.Close()
	for irows.Next() {
		var it BasketItem
		if err := irows.Scan(&it.ID, &it.BasketID, &it.Name, &it.Month, &it.Day, &it.Amount); err != nil {
			return nil, err
		}
		if i, ok := byID[it.BasketID]; ok {
			out[i].Items = append(out[i].Items, it)
		}
	}
	return out, irows.Err()
}

func (s *Store) getBasket(id int64) (Basket, error) {
	all, err := s.ListBaskets()
	if err != nil {
		return Basket{}, err
	}
	for _, b := range all {
		if b.ID == id {
			return b, nil
		}
	}
	return Basket{}, ErrNotFound
}

// nextFillSince 는 자동 채움을 지금 켤 때 시작 달: 이번 달 채우는 날이 아직이면 이번 달, 지났으면(이미 이체됐을 돈은
// 지금 잔액에 들어 있다고 보고) 다음 달.
func nextFillSince(b Basket, today time.Time, cal *holiday.Calendar) string {
	cur := today.Format("2006-01")
	if d, err := basketFillDate(b, cur, cal); err == nil && fmtDay(d) > fmtDay(today) {
		return cur
	}
	return addYM(cur, 1)
}

// SaveBasket 은 바구니를 추가하거나 고친다. 새로 만들 때 opening 은 지금 통장에 들어 있는 돈(장부의 시작).
// 항목은 넘긴 목록으로 맞춘다(있는 id 는 고치고, 없으면 넣고, 빠진 건 지운다 — 꺼냄 기록의 항목 연결은 비워진다).
func (s *Store) SaveBasket(b Basket, opening int64) (Basket, error) {
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		return b, fmt.Errorf("바구니 이름을 입력하세요")
	}
	if !validBasketKind(b.Kind) {
		return b, fmt.Errorf("바구니 종류를 고르세요")
	}
	if b.Monthly < 0 || b.Cap < 0 || opening < 0 {
		return b, fmt.Errorf("금액은 0원 이상이어야 합니다")
	}
	if b.FillDay < 1 || b.FillDay > 31 {
		b.FillDay = 10
	}
	if b.Kind == BasketScheduled {
		b.Cap = 0
		for i := range b.Items {
			it := &b.Items[i]
			it.Name = strings.TrimSpace(it.Name)
			if it.Name == "" {
				return b, fmt.Errorf("항목 이름을 입력하세요")
			}
			if it.Month < 1 || it.Month > 12 {
				return b, fmt.Errorf("%s: 달을 고르세요", it.Name)
			}
			if it.Day < 0 || it.Day > 31 {
				it.Day = 0
			}
			if it.Amount <= 0 {
				return b, fmt.Errorf("%s: 금액을 입력하세요", it.Name)
			}
		}
	} else {
		b.Items = nil
	}
	cal, err := s.Calendar()
	if err != nil {
		return b, err
	}
	now := nowFunc()

	// 자동 채움 시작 달: 새로 켜거나, 꺼 두었다 다시 켤 때만 다시 잡는다(그 사이 달을 한꺼번에 채우지 않게)
	if b.AutoFill && b.Active {
		since := ""
		if b.ID > 0 {
			var wasAuto, wasActive int
			err := s.queryRow(`SELECT auto_fill, active, fill_since FROM baskets WHERE id=?`, b.ID).Scan(&wasAuto, &wasActive, &since)
			if err != nil && err != sql.ErrNoRows {
				return b, err
			}
			if wasAuto == 0 || wasActive == 0 {
				since = ""
			}
		}
		if since == "" {
			since = nextFillSince(b, now, cal)
		}
		b.FillSince = since
	} else {
		b.FillSince = ""
	}

	tx, err := s.db.Begin()
	if err != nil {
		return b, err
	}
	defer tx.Rollback()
	args := []interface{}{b.Name, b.Kind, nullableID(b.PaymentMethodID), b.Cap, b.Monthly, boolToInt(b.AutoFill), b.FillDay,
		b.FillSince, b.SortOrder, boolToInt(b.Active), b.Memo}
	if b.ID == 0 {
		if err := tx.QueryRow(pq(`
INSERT INTO baskets(name, kind, payment_method_id, cap, monthly, auto_fill, fill_day, fill_since, sort_order, active, memo, created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`), append(args, now.Format("2006-01-02 15:04:05"))...).Scan(&b.ID); err != nil {
			return b, err
		}
		if opening > 0 {
			if _, err := tx.Exec(pq(`INSERT INTO basket_entries(basket_id, date, kind, amount, memo, created_at) VALUES(?,?,?,?,?,?)`),
				b.ID, now.Format(dateLayout), BasketOpen, opening, "처음 잔액", now.Format("2006-01-02 15:04:05")); err != nil {
				return b, err
			}
		}
	} else {
		res, err := tx.Exec(pq(`
UPDATE baskets SET name=?, kind=?, payment_method_id=?, cap=?, monthly=?, auto_fill=?, fill_day=?, fill_since=?, sort_order=?, active=?, memo=?
WHERE id=?`), append(args, b.ID)...)
		if err != nil {
			return b, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return b, ErrNotFound
		}
	}
	// 항목 맞추기
	keep := []int64{}
	for i := range b.Items {
		it := &b.Items[i]
		it.BasketID = b.ID
		if it.ID > 0 {
			res, err := tx.Exec(pq(`UPDATE basket_items SET name=?, month=?, day=?, amount=? WHERE id=? AND basket_id=?`),
				it.Name, it.Month, it.Day, it.Amount, it.ID, b.ID)
			if err != nil {
				return b, err
			}
			if n, _ := res.RowsAffected(); n == 1 {
				keep = append(keep, it.ID)
				continue
			}
		}
		if err := tx.QueryRow(pq(`INSERT INTO basket_items(basket_id, name, month, day, amount) VALUES(?,?,?,?,?) RETURNING id`),
			b.ID, it.Name, it.Month, it.Day, it.Amount).Scan(&it.ID); err != nil {
			return b, err
		}
		keep = append(keep, it.ID)
	}
	q := `DELETE FROM basket_items WHERE basket_id=?`
	dargs := []interface{}{b.ID}
	if len(keep) > 0 {
		q += ` AND id NOT IN (` + strings.TrimSuffix(strings.Repeat("?,", len(keep)), ",") + `)`
		for _, id := range keep {
			dargs = append(dargs, id)
		}
	}
	if _, err := tx.Exec(pq(q), dargs...); err != nil {
		return b, err
	}
	if err := tx.Commit(); err != nil {
		return b, err
	}
	return s.getBasket(b.ID)
}

// DeleteBasket 은 바구니와 장부를 지운다. 연결했던 지출 거래는 그대로 남는다.
func (s *Store) DeleteBasket(id int64) error {
	_, err := s.exec(`DELETE FROM baskets WHERE id=?`, id)
	return err
}

const basketEntryCols = `e.id, e.basket_id, e.date, e.kind, e.amount, e.auto, e.ym, e.transaction_id, e.item_id, e.due_ym, e.moved, e.memo,
	COALESCE(t.merchant, ''), COALESCE(t.amount, 0), COALESCE(i.name, '')`

// BasketEntries 는 바구니 장부 전체(basketID 가 0 이면 모든 바구니).
func (s *Store) BasketEntries(basketID int64) ([]BasketEntry, error) {
	q := `SELECT ` + basketEntryCols + ` FROM basket_entries e
LEFT JOIN transactions t ON t.id = e.transaction_id
LEFT JOIN basket_items i ON i.id = e.item_id`
	var args []interface{}
	if basketID > 0 {
		q += ` WHERE e.basket_id=?`
		args = append(args, basketID)
	}
	rows, err := s.query(q+` ORDER BY e.date, e.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BasketEntry{}
	for rows.Next() {
		var e BasketEntry
		var auto, moved int
		var tid, iid sql.NullInt64
		if err := rows.Scan(&e.ID, &e.BasketID, &e.Date, &e.Kind, &e.Amount, &auto, &e.YM, &tid, &iid, &e.DueYM, &moved, &e.Memo,
			&e.TxMerchant, &e.TxAmount, &e.ItemName); err != nil {
			return nil, err
		}
		e.Auto, e.Moved = auto == 1, moved == 1
		e.TransactionID, e.ItemID = scanNullableID(tid), scanNullableID(iid)
		out = append(out, e)
	}
	return out, rows.Err()
}

// AddBasketEntry 는 장부에 한 줄 넣는다: 더 넣기(extra, +), 꺼냄(spend, 금액은 양수로 받아 − 로 저장), 잔액 맞춤(adjust, ±).
// 꺼냄을 지출 거래에 연결하면 거래 하나는 한 번만 연결되고, 금액은 그 거래 금액을 넘을 수 없다.
func (s *Store) AddBasketEntry(e BasketEntry) (BasketEntry, error) {
	b, err := s.getBasket(e.BasketID)
	if err != nil {
		return e, err
	}
	if e.Date == "" {
		e.Date = nowFunc().Format(dateLayout)
	}
	if _, err := time.Parse(dateLayout, e.Date); err != nil {
		return e, fmt.Errorf("날짜가 올바르지 않습니다")
	}
	e.Auto, e.YM = false, ""
	switch e.Kind {
	case BasketExtra, BasketFill:
		if e.Amount <= 0 {
			return e, fmt.Errorf("넣을 금액을 입력하세요")
		}
		e.TransactionID, e.ItemID, e.DueYM, e.Moved = nil, nil, "", false
	case BasketAdjust:
		if e.Amount == 0 {
			return e, fmt.Errorf("맞출 금액을 입력하세요")
		}
		e.TransactionID, e.ItemID, e.DueYM, e.Moved = nil, nil, "", false
	case BasketSpend:
		if e.Amount <= 0 {
			return e, fmt.Errorf("꺼낸 금액을 입력하세요")
		}
		if e.TransactionID != nil {
			t, err := s.GetTransaction(*e.TransactionID)
			if err != nil {
				return e, err
			}
			if t.Direction != "expense" {
				return e, fmt.Errorf("지출 거래만 바구니에서 낼 수 있습니다")
			}
			if e.Amount > t.Amount {
				return e, fmt.Errorf("꺼낸 금액이 거래 금액(%s원)보다 클 수 없습니다", commaInt(t.Amount))
			}
		}
		if b.Kind != BasketScheduled || e.ItemID == nil {
			e.ItemID, e.DueYM = nil, ""
		} else {
			found := false
			for _, it := range b.Items {
				found = found || it.ID == *e.ItemID
			}
			if !found {
				return e, fmt.Errorf("항목을 다시 고르세요")
			}
			if !validYM(e.DueYM) {
				e.DueYM = e.Date[:7]
			}
		}
		e.Amount = -e.Amount
	default:
		return e, fmt.Errorf("기록 종류가 올바르지 않습니다")
	}
	err = s.queryRow(`
INSERT INTO basket_entries(basket_id, date, kind, amount, transaction_id, item_id, due_ym, moved, memo, created_at)
VALUES(?,?,?,?,?,?,?,?,?,?) RETURNING id`,
		e.BasketID, e.Date, e.Kind, e.Amount, nullableID(e.TransactionID), nullableID(e.ItemID), e.DueYM, boolToInt(e.Moved),
		strings.TrimSpace(e.Memo), nowFunc().Format("2006-01-02 15:04:05")).Scan(&e.ID)
	if err != nil && strings.Contains(err.Error(), "idx_basket_entry_tx") {
		return e, fmt.Errorf("이 거래는 이미 바구니에 연결돼 있습니다")
	}
	return e, err
}

// DeleteBasketEntry 는 장부 한 줄을 지운다. 자동 채움은 지우면 다음에 다시 기록되므로 0원(건너뜀)으로 남긴다.
func (s *Store) DeleteBasketEntry(id int64) error {
	var auto int
	if err := s.queryRow(`SELECT auto FROM basket_entries WHERE id=?`, id).Scan(&auto); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	if auto == 1 {
		_, err := s.exec(`UPDATE basket_entries SET amount=0, memo='이번 달 건너뜀' WHERE id=?`, id)
		return err
	}
	_, err := s.exec(`DELETE FROM basket_entries WHERE id=?`, id)
	return err
}

// RunBaskets 는 자동이체를 켠 바구니마다, 시작 달부터 이번 달까지 채우는 날이 지났는데 기록이 없는 달을 채움으로
// 기록한다(한 달 한 번 — 두 PC 가 동시에 돌아도 한쪽만). 기록한 수를 돌려준다.
func (s *Store) RunBaskets(today time.Time) (int, error) {
	baskets, err := s.ListBaskets()
	if err != nil {
		return 0, err
	}
	cal, err := s.Calendar()
	if err != nil {
		return 0, err
	}
	todayStr, curYM := fmtDay(dayOf(today)), today.Format("2006-01")
	n := 0
	for _, b := range baskets {
		if !b.Active || !b.AutoFill || b.Monthly <= 0 || b.FillSince == "" {
			continue
		}
		for ym, k := b.FillSince, 0; ym <= curYM && k < maxDuesMonths; ym, k = addYM(ym, 1), k+1 {
			d, err := basketFillDate(b, ym, cal)
			if err != nil || fmtDay(d) > todayStr {
				continue
			}
			res, err := s.exec(`
INSERT INTO basket_entries(basket_id, date, kind, amount, auto, ym, memo, created_at) VALUES(?,?,?,?,1,?,?,?)
ON CONFLICT (basket_id, ym) WHERE auto = 1 DO NOTHING`,
				b.ID, fmtDay(d), BasketFill, b.Monthly, ym, "매달 자동이체", nowFunc().Format("2006-01-02 15:04:05"))
			if err != nil {
				return n, err
			}
			if k, _ := res.RowsAffected(); k > 0 {
				n++
			}
		}
	}
	return n, nil
}

// BasketBook 은 바구니 전체의 화면 자료.
func (s *Store) BasketBook(today time.Time) ([]BasketStatus, error) {
	out := []BasketStatus{}
	baskets, err := s.ListBaskets()
	if err != nil || len(baskets) == 0 {
		return out, err
	}
	cal, err := s.Calendar()
	if err != nil {
		return out, err
	}
	entries, err := s.BasketEntries(0)
	if err != nil {
		return out, err
	}
	by := map[int64][]BasketEntry{}
	for _, e := range entries {
		by[e.BasketID] = append(by[e.BasketID], e)
	}
	for _, b := range baskets {
		out = append(out, BuildBasketStatus(b, by[b.ID], today, cal))
	}
	return out, nil
}

// BasketTxNames 는 바구니에서 낸 지출 거래 id → 바구니 이름 (거래내역 표시용).
func (s *Store) BasketTxNames() (map[int64]string, error) {
	rows, err := s.query(`SELECT e.transaction_id, b.name FROM basket_entries e JOIN baskets b ON b.id = e.basket_id
WHERE e.transaction_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// basketMainFlow 는 (asOf, today] 사이 바구니 장부로 생활비 통장에서 빠져나간 돈(채움·더 넣기 − 옮겨 온 꺼냄).
func (s *Store) basketMainFlow(asOf, today string) (int64, error) {
	var out int64
	err := s.queryRow(`
SELECT COALESCE(SUM(CASE WHEN kind IN ('fill','extra') THEN amount WHEN kind='spend' AND moved=1 THEN amount ELSE 0 END), 0)
FROM basket_entries WHERE date > ? AND date <= ?`, asOf, today).Scan(&out)
	return out, err
}

// basketEvents 는 앞으로 생활비 통장에서 바구니로 빠질 채움(매달 넣는 돈이 있는 켜진 바구니).
// 자동이체가 아니면 '예상'으로 표시한다(직접 넣을 돈).
func basketEvents(book []BasketStatus, cal *holiday.Calendar, today, horizon time.Time) []CashEvent {
	var out []CashEvent
	todayStr, horizonStr := fmtDay(today), fmtDay(horizon)
	for _, st := range book {
		b := st.Basket
		if !b.Active || b.Monthly <= 0 {
			continue
		}
		done := st.autoDone
		for ym, n := today.Format("2006-01"), 0; n < 14; ym, n = addYM(ym, 1), n+1 {
			if done[ym] {
				continue
			}
			d, err := basketFillDate(b, ym, cal)
			if err != nil {
				continue
			}
			ds := fmtDay(d)
			if ds < todayStr {
				continue
			}
			if ds > horizonStr {
				break
			}
			out = append(out, CashEvent{Date: ds, Kind: "basket", Label: b.Name + " 채우기", Amount: -b.Monthly,
				Estimated: !b.AutoFill, RefID: b.ID})
		}
	}
	return out
}
