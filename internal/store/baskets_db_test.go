package store

import (
	"path/filepath"
	"testing"
	"time"
)

// 목돈 바구니: 저장 · 처음 잔액 → 자동 채움(한 달 한 번) → 더 넣기 · 꺼내 쓰기(거래 연결) → 건너뜀
// → 자금 계획 잔액(생활비 통장 흐름, 바구니 통장 결제 제외) → 항목 고치기 → 백업.
func TestBasketsDB(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE baskets, custom_holidays RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	old := nowFunc
	t.Cleanup(func() { nowFunc = old })
	nowFunc = func() time.Time { return time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC) }

	acct, err := st.AddPaymentMethod("생활 예비비 통장", "bank")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveBasket(Basket{Name: " ", Kind: BasketReserve}, 0); err == nil {
		t.Fatal("이름 없이 저장됨")
	}
	b, err := st.SaveBasket(Basket{Name: "생활 예비비", Kind: BasketReserve, Cap: 3000000, Monthly: 100000, AutoFill: true,
		FillDay: 10, Active: true, PaymentMethodID: &acct.ID, Items: []BasketItem{{Name: "무시", Month: 1, Amount: 1}}}, 500000)
	if err != nil {
		t.Fatal(err)
	}
	// 11/10 이 아직이라 이번 달부터 자동 채움, 예비비에는 항목이 없다
	if b.FillSince != "2026-11" || len(b.Items) != 0 || b.PaymentMethodName != "생활 예비비 통장" {
		t.Fatalf("저장: %+v", b)
	}

	// 채우는 날 전에는 아무것도, 지나면 한 번만
	if n, err := st.RunBaskets(date("2026-11-09")); err != nil || n != 0 {
		t.Fatalf("채우는 날 전: %d %v", n, err)
	}
	if n, err := st.RunBaskets(date("2026-11-14")); err != nil || n != 1 {
		t.Fatalf("자동 채움: %d %v", n, err)
	}
	if n, _ := st.RunBaskets(date("2026-11-14")); n != 0 {
		t.Fatal("두 번 채움")
	}

	// 더 넣기, 꺼내 쓰기(축의금 거래에 연결, 생활비 통장으로 옮겨 옴)
	if _, err := st.AddBasketEntry(BasketEntry{BasketID: b.ID, Kind: BasketExtra, Amount: 200000, Date: "2026-11-12", Memo: "상여금"}); err != nil {
		t.Fatal(err)
	}
	gift, err := st.AddTransaction(Transaction{Date: "2026-11-13", Amount: 300000, Direction: "expense", Merchant: "축의금", Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddBasketEntry(BasketEntry{BasketID: b.ID, Kind: BasketSpend, Amount: 300001, TransactionID: &gift}); err == nil {
		t.Fatal("거래보다 많이 꺼냄")
	}
	spend, err := st.AddBasketEntry(BasketEntry{BasketID: b.ID, Kind: BasketSpend, Amount: 300000, Date: "2026-11-13", TransactionID: &gift, Moved: true})
	if err != nil {
		t.Fatal(err)
	}
	if spend.Amount != -300000 {
		t.Fatalf("꺼냄은 − 로 저장: %d", spend.Amount)
	}
	if _, err := st.AddBasketEntry(BasketEntry{BasketID: b.ID, Kind: BasketSpend, Amount: 1000, TransactionID: &gift}); err == nil {
		t.Fatal("같은 거래를 두 번 연결")
	}
	book, err := st.BasketBook(date("2026-11-14"))
	if err != nil || len(book) != 1 {
		t.Fatalf("바구니 목록: %v %d", err, len(book))
	}
	if book[0].Balance != 500000 { // 50 + 10 + 20 − 30 (만 원)
		t.Fatalf("잔액: %d", book[0].Balance)
	}
	if names, _ := st.BasketTxNames(); names[gift] != "생활 예비비" {
		t.Fatalf("거래 표시: %v", names)
	}

	// 자금 계획: 생활비 통장에서 축의금 30만 원이 나갔고, 바구니로 20만 원 보냈고(자동 채움 10만 원은 건너뜀으로 바꿈),
	// 바구니에서 30만 원을 옮겨 왔다. 바구니 통장으로 바로 낸 5만 원은 생활비 통장과 상관없다.
	var autoID int64
	for _, e := range book[0].Entries {
		if e.Auto {
			autoID = e.ID
		}
	}
	if err := st.DeleteBasketEntry(autoID); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.RunBaskets(date("2026-11-14")); n != 0 {
		t.Fatal("건너뛴 달을 다시 채움")
	}
	if _, err := st.AddTransaction(Transaction{Date: "2026-11-13", Amount: 50000, Direction: "expense", Merchant: "정비소",
		PaymentMethodID: &acct.ID, Source: "manual"}); err != nil {
		t.Fatal(err)
	}
	flow, err := st.basketMainFlow("2026-11-01", "2026-11-14")
	if err != nil || flow != -100000 { // 20만 원 보냄 − 30만 원 받음
		t.Fatalf("바구니 흐름: %d %v", flow, err)
	}
	adj, err := st.balanceAdjust("2026-11-01", date("2026-11-14"), nil)
	if err != nil || adj != -300000 { // 축의금만 — 바구니 통장 결제는 빠짐
		t.Fatalf("잔액 보정: %d %v", adj, err)
	}

	// 정해진 지출 바구니: 항목을 고치면 있는 항목은 id 를 지키고, 빠진 건 지운다
	sch, err := st.SaveBasket(Basket{Name: "정해진 연간 지출", Kind: BasketScheduled, Monthly: 150000, FillDay: 25, Active: true, Cap: 99,
		Items: []BasketItem{{Name: "자동차세", Month: 1, Amount: 450000}, {Name: "재산세", Month: 7, Amount: 300000}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if sch.Cap != 0 || len(sch.Items) != 2 || sch.FillSince != "" {
		t.Fatalf("정해진 지출 저장: %+v", sch)
	}
	if _, err := st.SaveBasket(Basket{ID: sch.ID, Name: "x", Kind: BasketScheduled, Items: []BasketItem{{Name: "보험", Month: 13, Amount: 1}}}, 0); err == nil {
		t.Fatal("13월이 저장됨")
	}
	keepID := sch.Items[0].ID
	sch.Items = []BasketItem{{ID: keepID, Name: "자동차세(연납)", Month: 1, Day: 31, Amount: 430000}, {Name: "자동차보험", Month: 3, Day: 15, Amount: 900000}}
	if sch, err = st.SaveBasket(sch, 0); err != nil {
		t.Fatal(err)
	}
	if len(sch.Items) != 2 || sch.Items[0].ID != keepID || sch.Items[0].Name != "자동차세(연납)" || sch.Items[1].Name != "자동차보험" {
		t.Fatalf("항목 고치기: %+v", sch.Items)
	}
	// 정해진 지출을 내면 항목 · 달을 남긴다
	tax, _ := st.AddTransaction(Transaction{Date: "2026-11-14", Amount: 430000, Direction: "expense", Merchant: "자동차세", Source: "manual"})
	e, err := st.AddBasketEntry(BasketEntry{BasketID: sch.ID, Kind: BasketSpend, Amount: 430000, TransactionID: &tax, ItemID: &keepID, DueYM: "2027-01"})
	if err != nil || e.DueYM != "2027-01" {
		t.Fatalf("정해진 지출 꺼냄: %+v %v", e, err)
	}
	if _, err := st.AddBasketEntry(BasketEntry{BasketID: b.ID, Kind: BasketAdjust}); err == nil {
		t.Fatal("0원 잔액 맞춤")
	}

	// 다시 켤 때 그 사이 달을 한꺼번에 채우지 않는다
	b.Active = false
	if b, err = st.SaveBasket(b, 0); err != nil || b.FillSince != "" {
		t.Fatalf("끄기: %+v %v", b, err)
	}
	nowFunc = func() time.Time { return time.Date(2027, 3, 20, 9, 0, 0, 0, time.UTC) }
	b.Active = true
	if b, err = st.SaveBasket(b, 0); err != nil || b.FillSince != "2027-04" {
		t.Fatalf("다시 켜기: %s %v", b.FillSince, err)
	}

	// 백업
	counts, err := st.BackupToSQLite(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	if counts["baskets"] != 2 || counts["basket_items"] != 2 || counts["basket_entries"] != 5 {
		t.Fatalf("백업 counts: %+v", counts)
	}
}
