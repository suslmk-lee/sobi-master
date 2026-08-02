package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestBackupToSQLite 는 Supabase(Postgres) → 로컬 SQLite 백업이 모든 행을 온전히
// 옮기는지 검증한다. (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestBackupToSQLite(t *testing.T) {
	st := openTestStore(t)

	// 표본 데이터: 카드 + 예산 + 거래 몇 건
	card, err := st.SavePaymentMethod(PaymentMethod{Name: "백업카드", Type: "card", Color: "#abcdef"})
	if err != nil {
		t.Fatal(err)
	}
	cats, _ := st.ListCategories()
	mems, _ := st.ListMembers()
	food := findCatID(cats, "식비")
	if err := st.SetBudget(food, "*", 400000); err != nil {
		t.Fatal(err)
	}
	for _, amt := range []int64{11000, 22000, 33000} {
		if _, err := st.AddTransaction(Transaction{
			Date: "2026-06-03", Amount: amt, Direction: "expense", Merchant: "마트",
			PaymentMethodID: &card.ID, CategoryID: &food, MemberID: &mems[0].ID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(t.TempDir(), "backup.db")
	counts, err := st.BackupToSQLite(path)
	if err != nil {
		t.Fatalf("BackupToSQLite: %v", err)
	}
	if counts["transactions"] != 3 || counts["budgets"] != 1 {
		t.Errorf("백업 행 수 오류: %+v", counts)
	}

	// 백업 파일을 열어 실제 데이터가 들어갔는지 확인
	lite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lite.Close()

	var n int
	var sum int64
	if err := lite.QueryRow(`SELECT COUNT(*), COALESCE(SUM(amount),0) FROM transactions`).Scan(&n, &sum); err != nil {
		t.Fatalf("백업 조회: %v", err)
	}
	if n != 3 || sum != 66000 {
		t.Errorf("백업 거래 = %d건/%d원, want 3/66000", n, sum)
	}
	// 새 컬럼(color)까지 보존됐는지
	var color string
	if err := lite.QueryRow(`SELECT color FROM payment_methods WHERE id=?`, card.ID).Scan(&color); err != nil {
		t.Fatalf("백업 color 조회: %v", err)
	}
	if color != "#abcdef" {
		t.Errorf("백업 color=%q, want #abcdef", color)
	}
	// 예산도 보존
	var bcat, bamt int64
	if err := lite.QueryRow(`SELECT category_id, amount FROM budgets`).Scan(&bcat, &bamt); err != nil {
		t.Fatalf("백업 예산 조회: %v", err)
	}
	if bcat != food || bamt != 400000 {
		t.Errorf("백업 예산 오류: cat=%d amt=%d", bcat, bamt)
	}
}
