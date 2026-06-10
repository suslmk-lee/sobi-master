package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // 이전 버전 로컬 DB 읽기 전용
)

// migrateFromSQLite 는 이전 버전이 쓰던 로컬 SQLite 파일의 데이터를
// ID 를 보존한 채 Supabase(Postgres)로 옮긴다. 서버가 비어 있을 때 한 번만 실행된다.
func (s *Store) migrateFromSQLite(path string) error {
	lite, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer lite.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	copyTable := func(query string, insert string, scan func(*sql.Rows) ([]interface{}, error)) error {
		rows, err := lite.Query(query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			args, err := scan(rows)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(pq(insert), args...); err != nil {
				return err
			}
		}
		return rows.Err()
	}

	// FK 참조 순서대로: 구성원 → 카테고리 → 결제수단 → 규칙 → 거래
	if err := copyTable(
		`SELECT id, name FROM members`,
		`INSERT INTO members(id, name) VALUES(?,?)`,
		func(r *sql.Rows) ([]interface{}, error) {
			var id int64
			var name string
			err := r.Scan(&id, &name)
			return []interface{}{id, name}, err
		}); err != nil {
		return fmt.Errorf("members: %w", err)
	}

	if err := copyTable(
		`SELECT id, name, kind FROM categories`,
		`INSERT INTO categories(id, name, kind) VALUES(?,?,?)`,
		func(r *sql.Rows) ([]interface{}, error) {
			var id int64
			var name, kind string
			err := r.Scan(&id, &name, &kind)
			return []interface{}{id, name, kind}, err
		}); err != nil {
		return fmt.Errorf("categories: %w", err)
	}

	if err := copyTable(
		`SELECT id, name, type, issuer, billing_day, cycle_start_day, perf_target FROM payment_methods`,
		`INSERT INTO payment_methods(id, name, type, issuer, billing_day, cycle_start_day, perf_target) VALUES(?,?,?,?,?,?,?)`,
		func(r *sql.Rows) ([]interface{}, error) {
			var id, target int64
			var day, cycle int
			var name, typ, issuer string
			err := r.Scan(&id, &name, &typ, &issuer, &day, &cycle, &target)
			return []interface{}{id, name, typ, issuer, day, cycle, target}, err
		}); err != nil {
		return fmt.Errorf("payment_methods: %w", err)
	}

	if err := copyTable(
		`SELECT id, merchant, amount_min, amount_max, member_id, category_id, label FROM rules`,
		`INSERT INTO rules(id, merchant, amount_min, amount_max, member_id, category_id, label) VALUES(?,?,?,?,?,?,?)`,
		func(r *sql.Rows) ([]interface{}, error) {
			var id, min, max int64
			var mid, cid sql.NullInt64
			var merchant, label string
			err := r.Scan(&id, &merchant, &min, &max, &mid, &cid, &label)
			return []interface{}{id, merchant, min, max, mid, cid, label}, err
		}); err != nil {
		return fmt.Errorf("rules: %w", err)
	}

	if err := copyTable(
		`SELECT id, date, amount, direction, merchant, memo, member_id, category_id, payment_method_id, source, auto_classified FROM transactions`,
		`INSERT INTO transactions(id, date, amount, direction, merchant, memo, member_id, category_id, payment_method_id, source, auto_classified) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		func(r *sql.Rows) ([]interface{}, error) {
			var id, amount int64
			var auto int
			var mid, cid, pid sql.NullInt64
			var date, direction, merchant, memo, source string
			err := r.Scan(&id, &date, &amount, &direction, &merchant, &memo, &mid, &cid, &pid, &source, &auto)
			return []interface{}{id, date, amount, direction, merchant, memo, mid, cid, pid, source, auto}, err
		}); err != nil {
		return fmt.Errorf("transactions: %w", err)
	}

	// 명시적 ID 로 넣었으므로 각 시퀀스를 현재 최대값으로 맞춘다
	for _, table := range []string{"members", "categories", "payment_methods", "rules", "transactions"} {
		q := fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('%s','id'), GREATEST(COALESCE(MAX(id),0), 1)) FROM %s`,
			table, table)
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("시퀀스 갱신(%s): %w", table, err)
		}
	}

	return tx.Commit()
}
