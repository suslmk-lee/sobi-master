package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrepaidMath(t *testing.T) {
	if got := prepaidCredit("bonus", 1000, 300000); got != 330000 {
		t.Errorf("bonus credit = %d", got)
	}
	if got := prepaidCredit("bonus", 333, 1000); got != 1033 { // 33.3 → 33
		t.Errorf("bonus rounding = %d", got)
	}
	if got := prepaidCredit("discount", 1000, 300000); got != 300000 {
		t.Errorf("discount credit = %d", got)
	}
	if got := prepaidDeduct("discount", 1000, 50000); got != 45000 {
		t.Errorf("discount deduct = %d", got)
	}
	if got := prepaidDeduct("discount", 333, 1005); got != 972 { // 971.67 → 972
		t.Errorf("deduct rounding = %d", got)
	}
	if got := prepaidDeduct("bonus", 1000, 50000); got != 50000 {
		t.Errorf("bonus deduct = %d", got)
	}
	if normalizePrepaidMode("weird") != "none" || normalizePrepaidMode("bonus") != "bonus" {
		t.Error("mode normalize")
	}
}

func TestPrepaidUsage(t *testing.T) {
	if l, a, d := prepaidUsage(nil); l != "" || a != 0 || d != 0 {
		t.Error("empty")
	}
	if l, a, d := prepaidUsage([]PrepaidUse{{Date: "2026-05-01", Deducted: 40000}}); l != "2026-05-01" || a != 40000 || d != 0 {
		t.Errorf("single: %s %d %d", l, a, d)
	}
	// 순서 무관, 날짜 간격 30일씩
	uses := []PrepaidUse{
		{ID: 3, Date: "2026-06-30", Deducted: 60000},
		{ID: 1, Date: "2026-05-01", Deducted: 20000},
		{ID: 2, Date: "2026-05-31", Deducted: 40000},
	}
	l, a, d := prepaidUsage(uses)
	if l != "2026-06-30" || a != 40000 || d != 30 {
		t.Errorf("usage: %s %d %d", l, a, d)
	}
	// 평균 차감액은 최근 5회만: 오래된 1000000 은 제외
	var many []PrepaidUse
	many = append(many, PrepaidUse{ID: 1, Date: "2026-01-01", Deducted: 1000000})
	for i := 0; i < 5; i++ {
		many = append(many, PrepaidUse{ID: int64(i + 2), Date: time.Date(2026, 2, 1+i, 0, 0, 0, 0, time.UTC).Format(dateLayout), Deducted: 10000})
	}
	if _, a, _ := prepaidUsage(many); a != 10000 {
		t.Errorf("recent 5 avg = %d", a)
	}
}

func TestBuildPrepaidStatus(t *testing.T) {
	p := PrepaidPass{Name: "헤어", OpeningBalance: 100000, OpeningDate: "2026-03-01", ExpiresAt: "2026-12-31", Active: true}
	charges := []prepaidChargeRow{
		{Date: "2026-02-01", Paid: 999999, Credit: 999999}, // 기준일 이전 — 무시
		{Date: "2026-03-10", Paid: 300000, Credit: 330000},
	}
	uses := []PrepaidUse{
		{ID: 1, Date: "2026-02-15", ListPrice: 50000, Deducted: 50000}, // 무시
		{ID: 2, Date: "2026-04-01", ListPrice: 50000, Deducted: 45000},
		{ID: 3, Date: "2026-05-01", ListPrice: 50000, Deducted: 45000},
	}
	st := buildPrepaidStatus(p, charges, uses, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if st.Balance != 100000+330000-90000 {
		t.Errorf("balance = %d", st.Balance)
	}
	if st.TotalPaid != 300000 || st.TotalCredit != 330000 || st.TotalUsed != 90000 || st.ChargeCount != 1 || st.UseCount != 2 {
		t.Errorf("totals: %+v", st)
	}
	if st.Saved != 30000+10000 {
		t.Errorf("saved = %d", st.Saved)
	}
	if st.LastUse != "2026-05-01" || st.AvgUseAmount != 45000 || st.AvgUseDays != 30 {
		t.Errorf("usage: %+v", st)
	}
	if st.UsesLeft != 7 { // 340000/45000
		t.Errorf("usesLeft = %d", st.UsesLeft)
	}
	if st.RunOutDate != "2026-11-27" { // 5/1 + 7*30일
		t.Errorf("runOut = %q", st.RunOutDate)
	}
	if st.Low || st.Expired {
		t.Errorf("flags: %+v", st)
	}
	// 잔액 부족 + 만료
	st = buildPrepaidStatus(PrepaidPass{ExpiresAt: "2026-05-31"}, nil, []PrepaidUse{{ID: 1, Date: "2026-05-01", Deducted: 45000}},
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if !st.Low || !st.Expired || st.UsesLeft != 0 || st.Balance != -45000 {
		t.Errorf("low/expired: %+v", st)
	}
}

func prepaidTestStore(t *testing.T) *Store {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE prepaid_passes RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestPrepaidChargeAndStatus(t *testing.T) {
	st := prepaidTestStore(t)
	cats, _ := st.ListCategories()
	food := findCatID(cats, "식비")
	card, err := st.SavePaymentMethod(PaymentMethod{Name: "선불카드", Type: "card"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.SavePrepaidPass(PrepaidPass{Name: "OO헤어", Merchant: "OO헤어샵", DiscountMode: "bonus", BonusBp: 1000,
		OpeningBalance: 50000, OpeningDate: "2026-03-01", CategoryID: &food, PaymentMethodID: &card.ID, Active: true})
	if err != nil || p.ID == 0 {
		t.Fatalf("save pass: %v", err)
	}
	if _, err := st.SavePrepaidPass(PrepaidPass{Name: " "}); err == nil {
		t.Error("빈 이름 허용됨")
	}
	if _, err := st.SavePrepaidPass(PrepaidPass{Name: "x", BonusBp: 10001}); err == nil {
		t.Error("범위 밖 bp 허용됨")
	}
	if _, err := st.SavePrepaidPass(PrepaidPass{Name: "x", OpeningDate: "2026/03/01"}); err == nil {
		t.Error("잘못된 날짜 허용됨")
	}
	if q, err := st.SavePrepaidPass(PrepaidPass{Name: "x", DiscountMode: "zzz"}); err != nil || q.DiscountMode != "none" {
		t.Errorf("mode fallback: %v %q", err, q.DiscountMode)
	}

	c, err := st.AddPrepaidCharge(p.ID, "2026-03-10", 300000, 0, 0, "충전")
	if err != nil {
		t.Fatal(err)
	}
	if c.Paid != 300000 || c.Credit != 330000 || c.Merchant != "OO헤어샵" || c.PaymentMethodName != "선불카드" {
		t.Errorf("charge: %+v", c)
	}
	tx, err := st.GetTransaction(c.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Direction != "expense" || tx.Amount != 300000 || tx.CategoryID == nil || *tx.CategoryID != food || tx.PrepaidName != "OO헤어" || tx.Source != "manual" {
		t.Errorf("tx: %+v", tx)
	}
	if _, err := st.AddPrepaidCharge(p.ID, "2026-03-10", 0, 0, 0, ""); err == nil {
		t.Error("0원 충전 허용됨")
	}
	if _, err := st.AddPrepaidCharge(p.ID, "bad", 1000, 0, 0, ""); err == nil {
		t.Error("잘못된 날짜 허용됨")
	}
	// 기준일 이전 충전은 잔액에서 제외
	if _, err := st.AddPrepaidCharge(p.ID, "2026-01-10", 100000, 0, 0, ""); err != nil {
		t.Fatal(err)
	}

	u, err := st.SavePrepaidUse(PrepaidUse{PassID: p.ID, Date: "2026-04-01", ListPrice: 50000, Deducted: 50000, Memo: "커트"})
	if err != nil || u.ID == 0 {
		t.Fatal(err)
	}
	if _, err := st.SavePrepaidUse(PrepaidUse{PassID: p.ID, Date: "2026-04-02", ListPrice: -1}); err == nil {
		t.Error("음수 허용됨")
	}
	if _, err := st.SavePrepaidUse(PrepaidUse{PassID: 9999, Date: "2026-04-02", ListPrice: 1}); err == nil {
		t.Error("없는 선불권 허용됨")
	}
	sts, err := st.ListPrepaidStatuses(date("2026-05-01"))
	if err != nil {
		t.Fatal(err)
	}
	var got *PrepaidStatus
	for i := range sts {
		if sts[i].Pass.ID == p.ID {
			got = &sts[i]
		}
	}
	if got == nil {
		t.Fatal("목록에 없음")
	}
	if got.Balance != 50000+330000-50000 || got.TotalPaid != 300000 || got.ChargeCount != 1 || got.Saved != 30000 {
		t.Errorf("status: %+v", *got)
	}
	// 순서: 사용 중 먼저, 이름순 — "x"(active=false) 는 뒤
	if !sts[0].Pass.Active {
		t.Errorf("정렬: %+v", sts[0].Pass)
	}

	// discount 방식은 정가에서 차감액 자동 계산
	d, _ := st.SavePrepaidPass(PrepaidPass{Name: "할인권", DiscountMode: "discount", DiscountBp: 1000, Active: true})
	du, err := st.SavePrepaidUse(PrepaidUse{PassID: d.ID, Date: "2026-04-01", ListPrice: 50000})
	if err != nil || du.Deducted != 45000 {
		t.Errorf("discount deduct: %v %+v", err, du)
	}

	h, err := st.GetPrepaidHistory(p.ID)
	if err != nil || len(h.Charges) != 2 || len(h.Uses) != 1 || h.Charges[0].Date != "2026-03-10" {
		t.Errorf("history: %v %+v", err, h)
	}
	if err := st.DeletePrepaidUse(u.ID); err != nil {
		t.Fatal(err)
	}
	if h, _ = st.GetPrepaidHistory(p.ID); len(h.Uses) != 0 || h.Uses == nil {
		t.Errorf("use delete / nil slice: %+v", h)
	}
}

func TestPrepaidLinking(t *testing.T) {
	st := prepaidTestStore(t)
	a, _ := st.SavePrepaidPass(PrepaidPass{Name: "A", Merchant: "미용실", DiscountMode: "bonus", BonusBp: 500, Active: true})
	b, _ := st.SavePrepaidPass(PrepaidPass{Name: "B", Merchant: "다른곳", Active: true})
	exp, _ := st.AddTransaction(Transaction{Date: "2026-06-01", Amount: 200000, Direction: "expense", Merchant: "미용실 강남점"})
	inc, _ := st.AddTransaction(Transaction{Date: "2026-06-01", Amount: 200000, Direction: "income", Merchant: "미용실"})
	other, _ := st.AddTransaction(Transaction{Date: "2026-06-02", Amount: 1000, Direction: "expense", Merchant: "편의점"})

	match := func(target, m string) bool {
		return strings.Contains(strings.ToLower(m), strings.ToLower(target))
	}
	sug, err := st.SuggestPrepaidCharges(a.ID, match)
	if err != nil || len(sug) != 1 || sug[0].ID != exp {
		t.Fatalf("suggest: %v %+v", err, sug)
	}
	c, err := st.LinkPrepaidCharge(a.ID, exp)
	if err != nil || c.Credit != 210000 {
		t.Fatalf("link: %v %+v", err, c)
	}
	if sug, _ = st.SuggestPrepaidCharges(a.ID, match); len(sug) != 0 {
		t.Errorf("연결된 거래가 후보에 남음: %+v", sug)
	}
	if _, err := st.LinkPrepaidCharge(b.ID, exp); err == nil || !strings.Contains(err.Error(), "다른 선불권") {
		t.Errorf("이미 연결된 거래: %v", err)
	}
	if _, err := st.LinkPrepaidCharge(a.ID, inc); err == nil || !strings.Contains(err.Error(), "지출") {
		t.Errorf("수입 거래: %v", err)
	}
	if _, err := st.LinkPrepaidCharge(a.ID, 99999); err == nil {
		t.Error("없는 거래 허용")
	}
	if _, err := st.LinkPrepaidCharge(9999, other); err == nil {
		t.Error("없는 선불권 허용")
	}
	if err := st.UnlinkPrepaidCharge(exp); err != nil {
		t.Fatal(err)
	}
	if tx, _ := st.GetTransaction(exp); tx.PrepaidName != "" {
		t.Errorf("연결 해제 후 PrepaidName=%q", tx.PrepaidName)
	}
	if _, err := st.LinkPrepaidCharge(b.ID, exp); err != nil {
		t.Errorf("해제 후 재연결: %v", err)
	}
}

func prepaidCount(t *testing.T, st *Store, q string, args ...interface{}) int {
	var n int
	if err := st.queryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPrepaidDeleteCascades(t *testing.T) {
	st := prepaidTestStore(t)
	p, _ := st.SavePrepaidPass(PrepaidPass{Name: "A", Merchant: "샵", Active: true})
	c1, _ := st.AddPrepaidCharge(p.ID, "2026-06-01", 10000, 0, 0, "")
	c2, _ := st.AddPrepaidCharge(p.ID, "2026-06-02", 20000, 0, 0, "")
	st.SavePrepaidUse(PrepaidUse{PassID: p.ID, Date: "2026-06-03", ListPrice: 5000})

	// 거래를 지우면 연결도 함께 사라진다
	if err := st.DeleteTransaction(c1.TransactionID); err != nil {
		t.Fatal(err)
	}
	if n := prepaidCount(t, st, `SELECT COUNT(*) FROM prepaid_charges WHERE pass_id=?`, p.ID); n != 1 {
		t.Errorf("거래 삭제 후 연결 %d건", n)
	}
	// 선불권을 지우면 거래는 남고 연결·사용 기록만 사라진다
	if err := st.DeletePrepaidPass(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetTransaction(c2.TransactionID); err != nil {
		t.Errorf("선불권 삭제로 거래가 사라짐: %v", err)
	}
	tx, _ := st.GetTransaction(c2.TransactionID)
	if tx.PrepaidName != "" {
		t.Errorf("PrepaidName=%q", tx.PrepaidName)
	}
	if prepaidCount(t, st, `SELECT COUNT(*) FROM prepaid_charges`) != 0 || prepaidCount(t, st, `SELECT COUNT(*) FROM prepaid_uses`) != 0 {
		t.Error("연결/사용 기록이 남음")
	}
}

func TestPrepaidRestoreLink(t *testing.T) {
	st := prepaidTestStore(t)
	p, _ := st.SavePrepaidPass(PrepaidPass{Name: "A", Merchant: "샵", DiscountMode: "bonus", BonusBp: 1000, Active: true})
	c, _ := st.AddPrepaidCharge(p.ID, "2026-06-01", 10000, 0, 0, "")
	links, err := st.PrepaidLinksFor([]int64{c.TransactionID, 424242})
	if err != nil || len(links) != 1 || links[c.TransactionID].Credit != 11000 {
		t.Fatalf("links: %v %+v", err, links)
	}
	newID, _ := st.AddTransaction(Transaction{Date: "2026-06-01", Amount: 10000, Direction: "expense", Merchant: "샵"})
	if err := st.RestorePrepaidLink(newID, links[c.TransactionID]); err != nil {
		t.Fatal(err)
	}
	if tx, _ := st.GetTransaction(newID); tx.PrepaidName != "A" {
		t.Errorf("복원 실패: %q", tx.PrepaidName)
	}
	// 선불권이 사라졌으면 조용히 건너뜀
	id2, _ := st.AddTransaction(Transaction{Date: "2026-06-01", Amount: 1, Direction: "expense", Merchant: "샵"})
	if err := st.RestorePrepaidLink(id2, PrepaidLink{PassID: 99999, Credit: 1}); err != nil {
		t.Errorf("없는 선불권 복원 오류: %v", err)
	}
}

func TestBackupIncludesPrepaid(t *testing.T) {
	st := prepaidTestStore(t)
	p, _ := st.SavePrepaidPass(PrepaidPass{Name: "백업권", Merchant: "샵", DiscountMode: "bonus", BonusBp: 1000, Active: true})
	if _, err := st.AddPrepaidCharge(p.ID, "2026-06-01", 10000, 0, 0, ""); err != nil {
		t.Fatal(err)
	}
	st.SavePrepaidUse(PrepaidUse{PassID: p.ID, Date: "2026-06-02", ListPrice: 3000})
	path := filepath.Join(t.TempDir(), "b.db")
	counts, err := st.BackupToSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if counts["prepaid_passes"] != 1 || counts["prepaid_charges"] != 1 || counts["prepaid_uses"] != 1 {
		t.Errorf("counts: %+v", counts)
	}
	lite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lite.Close()
	var credit int64
	if err := lite.QueryRow(`SELECT credit FROM prepaid_charges`).Scan(&credit); err != nil || credit != 11000 {
		t.Errorf("백업 credit=%d err=%v", credit, err)
	}
}
