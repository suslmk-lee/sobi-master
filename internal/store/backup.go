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
	id        INTEGER PRIMARY KEY,
	name      TEXT NOT NULL,
	kind      TEXT NOT NULL DEFAULT 'expense',
	parent_id INTEGER
);
CREATE TABLE payment_methods (
	id              INTEGER PRIMARY KEY,
	name            TEXT NOT NULL,
	type            TEXT NOT NULL DEFAULT 'card',
	issuer          TEXT NOT NULL DEFAULT '',
	billing_day     INTEGER NOT NULL DEFAULT 0,
	cycle_start_day INTEGER NOT NULL DEFAULT 1,
	perf_target     INTEGER NOT NULL DEFAULT 0,
	color           TEXT NOT NULL DEFAULT ''
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
			`SELECT id, name, kind, parent_id FROM categories`,
			`INSERT INTO categories(id, name, kind, parent_id) VALUES(?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id int64
				var name, kind string
				var parent sql.NullInt64
				e := r.Scan(&id, &name, &kind, &parent)
				return []interface{}{id, name, kind, nullable(parent)}, e
			}),
		copy("payment_methods",
			`SELECT id, name, type, issuer, billing_day, cycle_start_day, perf_target, color FROM payment_methods`,
			`INSERT INTO payment_methods(id, name, type, issuer, billing_day, cycle_start_day, perf_target, color) VALUES(?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, target int64
				var day, cycle int
				var name, typ, issuer, color string
				e := r.Scan(&id, &name, &typ, &issuer, &day, &cycle, &target, &color)
				return []interface{}{id, name, typ, issuer, day, cycle, target, color}, e
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
		copy("transactions",
			`SELECT id, date, amount, direction, merchant, memo, member_id, category_id, payment_method_id, source, auto_classified, created_at::text FROM transactions`,
			`INSERT INTO transactions(id, date, amount, direction, merchant, memo, member_id, category_id, payment_method_id, source, auto_classified, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			func(r *sql.Rows) ([]interface{}, error) {
				var id, amount int64
				var auto int
				var mid, cid, pid sql.NullInt64
				var date, direction, merchant, memo, source string
				var createdAt sql.NullString
				e := r.Scan(&id, &date, &amount, &direction, &merchant, &memo, &mid, &cid, &pid, &source, &auto, &createdAt)
				return []interface{}{id, date, amount, direction, merchant, memo, nullable(mid), nullable(cid), nullable(pid), source, auto, nullable2(createdAt)}, e
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
