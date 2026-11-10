package store

import (
	"database/sql"
	"os"
)

// sqliteBackupSchema 는 현재 Postgres 스키마를 SQLite 호환 DDL 로 옮긴 것.
// 백업 파일 자체가 열람·복원 가능한 온전한 DB 가 되도록 모든 컬럼을 포함한다.
const sqliteBackupSchema = `
CREATE TABLE members (
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE categories (
	id         INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	kind       TEXT NOT NULL DEFAULT 'expense',
	parent_id  INTEGER,
	sort_order INTEGER NOT NULL DEFAULT 0,
	fixed_cost INTEGER NOT NULL DEFAULT 0,
	default_member_id INTEGER
);
CREATE TABLE payment_methods (
	id              INTEGER PRIMARY KEY,
	name            TEXT NOT NULL,
	type            TEXT NOT NULL DEFAULT 'card',
	issuer          TEXT NOT NULL DEFAULT '',
	billing_day     INTEGER NOT NULL DEFAULT 0,
	cycle_start_day INTEGER NOT NULL DEFAULT 1,
	statement_start_day INTEGER NOT NULL DEFAULT 0,
	perf_target     INTEGER NOT NULL DEFAULT 0,
	color           TEXT NOT NULL DEFAULT '',
	cashback_bp         INTEGER NOT NULL DEFAULT 0,
	cashback_bonus_bp   INTEGER NOT NULL DEFAULT 0,
	cashback_bonus_days INTEGER NOT NULL DEFAULT 0,
	annual_fee          INTEGER NOT NULL DEFAULT 0,
	annual_fee_global   INTEGER NOT NULL DEFAULT 0,
	benefit_url         TEXT NOT NULL DEFAULT '',
	benefit_note        TEXT NOT NULL DEFAULT '',
	closed_at           TEXT NOT NULL DEFAULT ''
);
CREATE TABLE card_benefits (
	id                INTEGER PRIMARY KEY,
	payment_method_id INTEGER NOT NULL,
	area              TEXT NOT NULL,
	kind              TEXT NOT NULL DEFAULT 'point',
	rate_bp           INTEGER NOT NULL DEFAULT 0,
	is_max            INTEGER NOT NULL DEFAULT 0,
	monthly_cap       INTEGER NOT NULL DEFAULT 0,
	note              TEXT NOT NULL DEFAULT '',
	needs_check       INTEGER NOT NULL DEFAULT 0,
	sort_order        INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE transactions (
	id                INTEGER PRIMARY KEY,
	date              TEXT NOT NULL,
	amount            INTEGER NOT NULL,
	direction         TEXT NOT NULL DEFAULT 'expense',
	merchant          TEXT NOT NULL DEFAULT '',
	memo              TEXT NOT NULL DEFAULT '',
	member_id         INTEGER,
	category_id       INTEGER,
	payment_method_id INTEGER,
	source            TEXT NOT NULL DEFAULT 'manual',
	auto_classified   INTEGER NOT NULL DEFAULT 0,
	exclude_perf      INTEGER NOT NULL DEFAULT 0,
	paid_at           TEXT NOT NULL DEFAULT '',
	created_at        TEXT
);
CREATE TABLE rules (
	id          INTEGER PRIMARY KEY,
	merchant    TEXT NOT NULL,
	amount_min  INTEGER NOT NULL,
	amount_max  INTEGER NOT NULL,
	member_id   INTEGER,
	category_id INTEGER,
	label       TEXT NOT NULL DEFAULT ''
);
CREATE TABLE budgets (
	id          INTEGER PRIMARY KEY,
	category_id INTEGER NOT NULL,
	ym          TEXT NOT NULL DEFAULT '*',
	amount      INTEGER NOT NULL
);
CREATE TABLE subscriptions (
	id                INTEGER PRIMARY KEY,
	name              TEXT NOT NULL,
	merchant          TEXT NOT NULL DEFAULT '',
	amount            INTEGER NOT NULL DEFAULT 0,
	cycle             TEXT NOT NULL DEFAULT 'monthly',
	billing_day       INTEGER NOT NULL DEFAULT 1,
	billing_month     INTEGER NOT NULL DEFAULT 0,
	start_ym          TEXT NOT NULL DEFAULT '',
	end_ym            TEXT NOT NULL DEFAULT '',
	next_amount       INTEGER NOT NULL DEFAULT 0,
	next_amount_ym    TEXT NOT NULL DEFAULT '',
	category_id       INTEGER,
	member_id         INTEGER,
	payment_method_id INTEGER,
	memo              TEXT NOT NULL DEFAULT '',
	active            INTEGER NOT NULL DEFAULT 1,
	auto_post         INTEGER NOT NULL DEFAULT 0,
	auto_since        TEXT NOT NULL DEFAULT '',
	day_rule          TEXT NOT NULL DEFAULT 'day',
	week_nth          INTEGER NOT NULL DEFAULT 1,
	weekday           INTEGER NOT NULL DEFAULT 5,
	holiday_rule      TEXT NOT NULL DEFAULT 'next',
	kind              TEXT NOT NULL DEFAULT '',
	paid_through      TEXT NOT NULL DEFAULT '',
	paid_through_at   TEXT NOT NULL DEFAULT '',
	paid_through_tx   INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE subscription_posts (
	subscription_id INTEGER NOT NULL,
	ym              TEXT NOT NULL,
	transaction_id  INTEGER,
	matched         INTEGER NOT NULL DEFAULT 0,
	posted_at       TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (subscription_id, ym)
);
CREATE TABLE custom_holidays (
	date       TEXT PRIMARY KEY,
	name       TEXT NOT NULL DEFAULT '',
	is_holiday INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE app_settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE cash_balances (
	id         INTEGER PRIMARY KEY,
	as_of      TEXT NOT NULL,
	amount     INTEGER NOT NULL,
	memo       TEXT NOT NULL DEFAULT '',
	created_at TEXT
);
CREATE TABLE prepaid_passes (
	id                INTEGER PRIMARY KEY,
	name              TEXT NOT NULL,
	merchant          TEXT NOT NULL DEFAULT '',
	discount_mode     TEXT NOT NULL DEFAULT 'none',
	bonus_bp          INTEGER NOT NULL DEFAULT 0,
	discount_bp       INTEGER NOT NULL DEFAULT 0,
	opening_balance   INTEGER NOT NULL DEFAULT 0,
	opening_date      TEXT NOT NULL DEFAULT '',
	expires_at        TEXT NOT NULL DEFAULT '',
	category_id       INTEGER,
	payment_method_id INTEGER,
	member_id         INTEGER,
	memo              TEXT NOT NULL DEFAULT '',
	active            INTEGER NOT NULL DEFAULT 1,
	created_at        TEXT
);
CREATE TABLE prepaid_charges (
	transaction_id INTEGER PRIMARY KEY,
	pass_id        INTEGER NOT NULL,
	credit         INTEGER NOT NULL
);
CREATE TABLE dues_links (
	transaction_id  INTEGER PRIMARY KEY,
	subscription_id INTEGER NOT NULL,
	state           TEXT NOT NULL,
	created_at      TEXT NOT NULL DEFAULT ''
);
CREATE TABLE prepaid_uses (
	id         INTEGER PRIMARY KEY,
	pass_id    INTEGER NOT NULL,
	date       TEXT NOT NULL,
	list_price INTEGER NOT NULL DEFAULT 0,
	deducted   INTEGER NOT NULL DEFAULT 0,
	member_id  INTEGER,
	memo       TEXT NOT NULL DEFAULT '',
	created_at TEXT
);
`

// BackupToSQLite 는 현재 DB(Supabase) 전체를 새 SQLite 파일로 스냅샷 백업한다.
// 임시 파일에 먼저 쓰고 성공하면 원자적으로 교체하므로, 도중에 실패해도 기존 백업은 온전하다.
// 테이블별 행 수를 돌려준다.
func (s *Store) BackupToSQLite(path string) (map[string]int, error) {
	tmp := path + ".tmp"
	_ = os.Remove(tmp)

	lite, err := sql.Open("sqlite", tmp)
	if err != nil {
		return nil, err
	}
	closed := false
	defer func() {
		if !closed {
			lite.Close()
		}
		_ = os.Remove(tmp) // 성공 시엔 이미 rename 되어 사라졌고, 실패 시엔 잔여물 정리
	}()

	if _, err := lite.Exec(sqliteBackupSchema); err != nil {
		return nil, err
	}
	tx, err := lite.Begin()
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	// copy 는 Postgres(sel)에서 읽어 SQLite(ins)로 넣는다. scan 은 한 행을 인자 슬라이스로 변환.
	copy := func(table, sel, ins string, scan func(*sql.Rows) ([]interface{}, error)) error {
		rows, err := s.db.Query(sel)
		if err != nil {
			return err
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			args, err := scan(rows)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ins, args...); err != nil {
				return err
			}
			n++
		}
		counts[table] = n
		return rows.Err()
	}

	err = firstErr(
		copy("members",
			`SELECT id, name FROM members`,
			`INSERT INTO members(id, name) VALUES(?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id int64
				var name string
				e := r.Scan(&id, &name)
				return []interface{}{id, name}, e
			}),
		copy("categories",
			`SELECT id, name, kind, parent_id, sort_order, fixed_cost, default_member_id FROM categories`,
			`INSERT INTO categories(id, name, kind, parent_id, sort_order, fixed_cost, default_member_id) VALUES(?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id int64
				var name, kind string
				var parent, defMember sql.NullInt64
				var order, fixed int
				e := r.Scan(&id, &name, &kind, &parent, &order, &fixed, &defMember)
				return []interface{}{id, name, kind, nullable(parent), order, fixed, nullable(defMember)}, e
			}),
		copy("payment_methods",
			`SELECT id, name, type, issuer, billing_day, cycle_start_day, statement_start_day, perf_target, color, cashback_bp, cashback_bonus_bp, cashback_bonus_days, annual_fee, annual_fee_global, benefit_url, benefit_note, closed_at FROM payment_methods`,
			`INSERT INTO payment_methods(id, name, type, issuer, billing_day, cycle_start_day, statement_start_day, perf_target, color, cashback_bp, cashback_bonus_bp, cashback_bonus_days, annual_fee, annual_fee_global, benefit_url, benefit_note, closed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, target, fee, feeG int64
				var day, cycle, stmt, cbBp, cbBonusBp, cbDays int
				var name, typ, issuer, color, bURL, bNote, closedAt string
				e := r.Scan(&id, &name, &typ, &issuer, &day, &cycle, &stmt, &target, &color, &cbBp, &cbBonusBp, &cbDays,
					&fee, &feeG, &bURL, &bNote, &closedAt)
				return []interface{}{id, name, typ, issuer, day, cycle, stmt, target, color, cbBp, cbBonusBp, cbDays,
					fee, feeG, bURL, bNote, closedAt}, e
			}),
		copy("card_benefits",
			`SELECT id, payment_method_id, area, kind, rate_bp, is_max, monthly_cap, note, needs_check, sort_order FROM card_benefits`,
			`INSERT INTO card_benefits(id, payment_method_id, area, kind, rate_bp, is_max, monthly_cap, note, needs_check, sort_order) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, pmID, cap int64
				var rateBp, isMax, needsCheck, order int
				var area, kind, note string
				e := r.Scan(&id, &pmID, &area, &kind, &rateBp, &isMax, &cap, &note, &needsCheck, &order)
				return []interface{}{id, pmID, area, kind, rateBp, isMax, cap, note, needsCheck, order}, e
			}),
		copy("rules",
			`SELECT id, merchant, amount_min, amount_max, member_id, category_id, label FROM rules`,
			`INSERT INTO rules(id, merchant, amount_min, amount_max, member_id, category_id, label) VALUES(?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, min, max int64
				var mid, cid sql.NullInt64
				var merchant, label string
				e := r.Scan(&id, &merchant, &min, &max, &mid, &cid, &label)
				return []interface{}{id, merchant, min, max, nullable(mid), nullable(cid), label}, e
			}),
		copy("budgets",
			`SELECT id, category_id, ym, amount FROM budgets`,
			`INSERT INTO budgets(id, category_id, ym, amount) VALUES(?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, cid, amount int64
				var ym string
				e := r.Scan(&id, &cid, &ym, &amount)
				return []interface{}{id, cid, ym, amount}, e
			}),
		copy("subscriptions",
			`SELECT id, name, merchant, amount, cycle, billing_day, billing_month,
			        start_ym, end_ym, next_amount, next_amount_ym,
			        category_id, member_id, payment_method_id, memo, active, auto_post, auto_since,
			        day_rule, week_nth, weekday, holiday_rule,
			        kind, paid_through, paid_through_at, paid_through_tx FROM subscriptions`,
			`INSERT INTO subscriptions(id, name, merchant, amount, cycle, billing_day, billing_month,
			        start_ym, end_ym, next_amount, next_amount_ym,
			        category_id, member_id, payment_method_id, memo, active, auto_post, auto_since,
			        day_rule, week_nth, weekday, holiday_rule,
			        kind, paid_through, paid_through_at, paid_through_tx)
			 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, amount, next, paidTx int64
				var day, bmonth, active, autoPost, weekNth, weekday int
				var name, merchant, cycle, startYM, endYM, nextYM, memo, autoSince, dayRule, holidayRule string
				var kind, paidThrough, paidAt string
				var cid, mid, pid sql.NullInt64
				e := r.Scan(&id, &name, &merchant, &amount, &cycle, &day, &bmonth,
					&startYM, &endYM, &next, &nextYM, &cid, &mid, &pid, &memo, &active, &autoPost, &autoSince,
					&dayRule, &weekNth, &weekday, &holidayRule, &kind, &paidThrough, &paidAt, &paidTx)
				return []interface{}{id, name, merchant, amount, cycle, day, bmonth,
					startYM, endYM, next, nextYM,
					nullable(cid), nullable(mid), nullable(pid), memo, active, autoPost, autoSince,
					dayRule, weekNth, weekday, holidayRule, kind, paidThrough, paidAt, paidTx}, e
			}),
		// 자동 등록 기록이 빠지면 복원 후 같은 달 거래가 다시 만들어진다
		copy("subscription_posts",
			`SELECT subscription_id, ym, transaction_id, matched, posted_at FROM subscription_posts`,
			`INSERT INTO subscription_posts(subscription_id, ym, transaction_id, matched, posted_at) VALUES(?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var sid int64
				var tid sql.NullInt64
				var matched int
				var ym, at string
				e := r.Scan(&sid, &ym, &tid, &matched, &at)
				return []interface{}{sid, ym, nullable(tid), matched, at}, e
			}),
		copy("custom_holidays",
			`SELECT date, name, is_holiday FROM custom_holidays`,
			`INSERT INTO custom_holidays(date, name, is_holiday) VALUES(?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var date, name string
				var h int
				e := r.Scan(&date, &name, &h)
				return []interface{}{date, name, h}, e
			}),
		copy("app_settings",
			`SELECT key, value FROM app_settings`,
			`INSERT INTO app_settings(key, value) VALUES(?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var k, v string
				e := r.Scan(&k, &v)
				return []interface{}{k, v}, e
			}),
		copy("cash_balances",
			`SELECT id, as_of, amount, memo, created_at::text FROM cash_balances`,
			`INSERT INTO cash_balances(id, as_of, amount, memo, created_at) VALUES(?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, amount int64
				var asOf, memo string
				var createdAt sql.NullString
				e := r.Scan(&id, &asOf, &amount, &memo, &createdAt)
				return []interface{}{id, asOf, amount, memo, nullable2(createdAt)}, e
			}),
		copy("prepaid_passes",
			`SELECT id, name, merchant, discount_mode, bonus_bp, discount_bp, opening_balance, opening_date, expires_at, category_id, payment_method_id, member_id, memo, active, created_at::text FROM prepaid_passes`,
			`INSERT INTO prepaid_passes(id, name, merchant, discount_mode, bonus_bp, discount_bp, opening_balance, opening_date, expires_at, category_id, payment_method_id, member_id, memo, active, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, opening int64
				var bonus, disc, active int
				var name, merchant, mode, openDate, expires, memo string
				var cid, pid, mid sql.NullInt64
				var createdAt sql.NullString
				e := r.Scan(&id, &name, &merchant, &mode, &bonus, &disc, &opening, &openDate, &expires, &cid, &pid, &mid, &memo, &active, &createdAt)
				return []interface{}{id, name, merchant, mode, bonus, disc, opening, openDate, expires,
					nullable(cid), nullable(pid), nullable(mid), memo, active, nullable2(createdAt)}, e
			}),
		copy("prepaid_uses",
			`SELECT id, pass_id, date, list_price, deducted, member_id, memo, created_at::text FROM prepaid_uses`,
			`INSERT INTO prepaid_uses(id, pass_id, date, list_price, deducted, member_id, memo, created_at) VALUES(?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, pass, list, ded int64
				var date, memo string
				var mid sql.NullInt64
				var createdAt sql.NullString
				e := r.Scan(&id, &pass, &date, &list, &ded, &mid, &memo, &createdAt)
				return []interface{}{id, pass, date, list, ded, nullable(mid), memo, nullable2(createdAt)}, e
			}),
		copy("transactions",
			`SELECT id, date, amount, direction, merchant, memo, member_id, category_id, payment_method_id, source, auto_classified, exclude_perf, paid_at, created_at::text FROM transactions`,
			`INSERT INTO transactions(id, date, amount, direction, merchant, memo, member_id, category_id, payment_method_id, source, auto_classified, exclude_perf, paid_at, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, amount int64
				var auto, excl int
				var mid, cid, pid sql.NullInt64
				var date, direction, merchant, memo, source, paidAt string
				var createdAt sql.NullString
				e := r.Scan(&id, &date, &amount, &direction, &merchant, &memo, &mid, &cid, &pid, &source, &auto, &excl, &paidAt, &createdAt)
				return []interface{}{id, date, amount, direction, merchant, memo, nullable(mid), nullable(cid), nullable(pid), source, auto, excl, paidAt, nullable2(createdAt)}, e
			}),
		// 회비 연결이 빠지면 납부 기록·"회비 아님"이 사라져 장부가 달라진다
		copy("dues_links",
			`SELECT transaction_id, subscription_id, state, created_at FROM dues_links`,
			`INSERT INTO dues_links(transaction_id, subscription_id, state, created_at) VALUES(?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var tid, sid int64
				var state, at string
				e := r.Scan(&tid, &sid, &state, &at)
				return []interface{}{tid, sid, state, at}, e
			}),
		// 충전 연결은 거래·선불권 뒤에 복사한다
		copy("prepaid_charges",
			`SELECT transaction_id, pass_id, credit FROM prepaid_charges`,
			`INSERT INTO prepaid_charges(transaction_id, pass_id, credit) VALUES(?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var tid, pass, credit int64
				e := r.Scan(&tid, &pass, &credit)
				return []interface{}{tid, pass, credit}, e
			}),
	)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := lite.Close(); err != nil {
		return nil, err
	}
	closed = true
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return counts, nil
}

// firstErr 는 여러 단계 중 첫 번째 오류를 돌려준다(순차 실행).
func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func nullable(v sql.NullInt64) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func nullable2(v sql.NullString) interface{} {
	if !v.Valid {
		return nil
	}
	return v.String
}
