package store

import (
	"database/sql"
	"strings"

	"sobi/internal/holiday"
)

// 정기결제 자동 등록의 저장 계층.
//
// 어느 달을 언제 등록할지(업무일 계산, 이미 입력된 거래 찾기)는 앱 계층이 정하고,
// 여기서는 "한 달치를 정확히 한 번만 기록"하는 원자적 처리만 맡는다.

// SourceSubscription 은 정기결제 자동 등록으로 만들어진 거래의 source 값.
const SourceSubscription = "subscription"

// SubPost 는 정기결제 한 달치 처리 기록.
type SubPost struct {
	SubscriptionID int64  `json:"subscriptionId"`
	YM             string `json:"ym"`
	TransactionID  *int64 `json:"transactionId"` // 거래가 지워지면 nil
	Matched        bool   `json:"matched"`       // 새로 만들지 않고 기존 거래에 연결함
	PostedAt       string `json:"postedAt"`
}

// SubscriptionPosts 는 처리 기록 전체를 정기결제 id → 월 → 기록 으로 묶어 돌려준다.
func (s *Store) SubscriptionPosts() (map[int64]map[string]SubPost, error) {
	rows, err := s.query(`SELECT subscription_id, ym, transaction_id, matched, posted_at FROM subscription_posts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]SubPost{}
	for rows.Next() {
		var p SubPost
		var tid sql.NullInt64
		var matched int
		if err := rows.Scan(&p.SubscriptionID, &p.YM, &tid, &matched, &p.PostedAt); err != nil {
			return nil, err
		}
		p.TransactionID = scanNullableID(tid)
		p.Matched = matched == 1
		if out[p.SubscriptionID] == nil {
			out[p.SubscriptionID] = map[string]SubPost{}
		}
		out[p.SubscriptionID][p.YM] = p
	}
	return out, rows.Err()
}

// PostSubscription 은 정기결제 subID 의 ym 달치를 기록한다.
//
// (정기결제, 월) 자리를 먼저 차지하고 나서 거래를 만들기 때문에, 같은 달은 절대 두 번
// 등록되지 않는다. 두 PC 가 같은 DB 를 쓰다가 동시에 돌아도 한쪽만 성공한다.
// 차지와 거래 생성이 한 트랜잭션이라, 중간에 실패하면 자리도 같이 풀려 다음에 다시 시도된다.
//
// matchedTxID > 0 이면 거래를 새로 만들지 않고 그 거래를 이 달 결제로 연결만 한다
// (사용자가 이미 손으로 입력해 둔 경우). 아니면 t 로 거래를 만든다.
// 이미 기록돼 있으면 posted=false 로 아무것도 하지 않는다.
func (s *Store) PostSubscription(subID int64, ym string, t Transaction, matchedTxID int64) (posted bool, txID int64, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback() // 커밋한 뒤에는 아무 일도 하지 않는다

	res, err := tx.Exec(pq(`
INSERT INTO subscription_posts(subscription_id, ym, posted_at) VALUES(?,?,?)
ON CONFLICT (subscription_id, ym) DO NOTHING`),
		subID, ym, nowFunc().Format("2006-01-02 15:04:05"))
	if err != nil {
		return false, 0, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return false, 0, err
	} else if n == 0 {
		return false, 0, nil // 이미 처리된 달
	}

	matched := matchedTxID > 0
	txID = matchedTxID
	if !matched {
		t.Source = SourceSubscription
		if err := tx.QueryRow(pq(insertTxSQL), insertTxArgs(t)...).Scan(&txID); err != nil {
			return false, 0, err
		}
	}
	if _, err := tx.Exec(pq(`
UPDATE subscription_posts SET transaction_id=?, matched=? WHERE subscription_id=? AND ym=?`),
		txID, boolToInt(matched), subID, ym); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, 0, err
	}
	return true, txID, nil
}

// ---- 공휴일 정정 ----

// ListCustomHolidays 는 사용자가 더하거나 뺀 공휴일 목록.
func (s *Store) ListCustomHolidays() ([]holiday.Override, error) {
	rows, err := s.query(`SELECT date, name, is_holiday FROM custom_holidays ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []holiday.Override{}
	for rows.Next() {
		var o holiday.Override
		var h int
		if err := rows.Scan(&o.Date, &o.Name, &h); err != nil {
			return nil, err
		}
		o.IsHoliday = h == 1
		out = append(out, o)
	}
	return out, rows.Err()
}

// SaveCustomHoliday 는 그 날짜의 정정을 넣거나 바꾼다(날짜당 하나).
func (s *Store) SaveCustomHoliday(o holiday.Override) error {
	_, err := s.exec(`
INSERT INTO custom_holidays(date, name, is_holiday) VALUES(?,?,?)
ON CONFLICT (date) DO UPDATE SET name = EXCLUDED.name, is_holiday = EXCLUDED.is_holiday`,
		o.Date, strings.TrimSpace(o.Name), boolToInt(o.IsHoliday))
	return err
}

// DeleteCustomHoliday 는 정정을 지워 그 날짜를 내장 표대로 되돌린다.
func (s *Store) DeleteCustomHoliday(date string) error {
	_, err := s.exec(`DELETE FROM custom_holidays WHERE date=?`, date)
	return err
}

// Calendar 는 내장 공휴일 표에 사용자 정정을 얹은 달력.
func (s *Store) Calendar() (*holiday.Calendar, error) {
	over, err := s.ListCustomHolidays()
	if err != nil {
		return nil, err
	}
	return holiday.New(over), nil
}
