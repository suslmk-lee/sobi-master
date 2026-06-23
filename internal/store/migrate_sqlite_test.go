package store

import (
	"os"
	"testing"
)

// 실제 로컬 SQLite 파일(복사본)을 Postgres 로 옮기는 전체 흐름 검증.
// MIGRATE_SQLITE_PATH 가 설정된 경우에만 실행된다.
func TestMigrateFromSQLite(t *testing.T) {
	src := os.Getenv("MIGRATE_SQLITE_PATH")
	if src == "" {
		t.Skip("MIGRATE_SQLITE_PATH 미설정 — 마이그레이션 테스트 건너뜀")
	}
	st := openTestStore(t)
	// openTestStore 가 시드를 넣으므로 비우고 시작 (실제 Open() 의 빈 DB 조건 재현)
	if _, err := st.db.Exec(`TRUNCATE members, categories, payment_methods, rules, transactions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	if err := st.migrateFromSQLite(src); err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for _, table := range []string{"members", "categories", "payment_methods", "rules", "transactions"} {
		var n int
		if err := st.queryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		counts[table] = n
		t.Logf("%s: %d행", table, n)
	}
	if counts["transactions"] == 0 {
		t.Error("거래가 옮겨지지 않음")
	}

	// 시퀀스가 갱신되어 새 INSERT 가 기존 ID와 충돌하지 않아야 한다
	m, err := st.AddMember("시퀀스검증용")
	if err != nil {
		t.Fatalf("마이그레이션 후 INSERT 실패(시퀀스 문제): %v", err)
	}
	if err := st.DeleteMember(m.ID); err != nil {
		t.Fatal(err)
	}

	// 조인 조회가 정상 동작하는지 (FK 보존 확인)
	txs, err := st.ListTransactions(TxFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != counts["transactions"] {
		t.Errorf("조회된 거래 %d건, want %d건", len(txs), counts["transactions"])
	}
	classified := 0
	for _, tx := range txs {
		if tx.MemberName != "" || tx.CategoryName != "" {
			classified++
		}
	}
	t.Logf("분류 연결 유지된 거래: %d/%d", classified, len(txs))
}
