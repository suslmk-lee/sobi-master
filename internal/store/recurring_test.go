package store

import (
	"testing"
	"time"

	"sobi/internal/holiday"
)

func TestDetectRecurring(t *testing.T) {
	cal := holiday.New(nil)
	today := date("2026-11-10")
	id := func(v int64) *int64 { return &v }
	cash := recTx{PMID: id(2), PMName: "현금", PMType: "cash", CatID: id(50), CatName: "대출상환", MainCatID: 50, MemberID: id(4), Member: "공동"}
	tx := func(base recTx, d string, amt int64, merchant string) recTx {
		base.Date, base.Amount, base.Merchant = date(d), amt, merchant
		return base
	}
	var txs []recTx
	// 대출 A: 매월 13일(휴일이면 다음 영업일) 382,194원 — 가맹점 이름이 달마다 섞여 있다
	for _, x := range []struct{ d, m string }{
		{"2026-06-15", "하나은행대출"}, {"2026-07-13", "주택금융공사"}, {"2026-08-13", "주택금융공사"},
		{"2026-09-14", "하나은행대출"}, {"2026-10-13", "하나은행대출"},
	} {
		txs = append(txs, tx(cash, x.d, 382194, x.m))
	}
	// 대출 B: 매월 30일(휴일이면 다음 영업일), 금액이 조금씩 다름 — 5월분이 6/1 로 밀림
	for _, x := range []struct {
		d string
		a int64
		m string
	}{
		{"2026-06-01", 785125, "주택금융공사"}, {"2026-06-30", 793717, "주택금융공사"}, {"2026-07-30", 783180, "주택금융공사"},
		{"2026-08-31", 791707, "주택금융공사"}, {"2026-09-30", 790702, "주택금융공사"}, {"2026-10-30", 786262, "하나은행대출"},
	} {
		txs = append(txs, tx(cash, x.d, x.a, x.m))
	}
	// 카드 정기결제: 매월 8일 38,280원 (Jade카드, 통신비)
	card := recTx{PMID: id(9), PMName: "Jade카드", PMType: "card", CatID: id(60), CatName: "통신비", MainCatID: 60}
	for _, d := range []string{"2026-06-08", "2026-07-08", "2026-08-10", "2026-09-08", "2026-10-08"} {
		txs = append(txs, tx(card, d, 38280, "LG U+"))
	}
	// 잡음 1: 매주 월·수 점심 8,000원 (같은 카드·같은 금액이지만 한 달에 여러 번)
	lunch := recTx{PMID: id(9), PMName: "Jade카드", PMType: "card", CatID: id(70), CatName: "식비 > 점심", MainCatID: 10}
	for d := date("2026-06-01"); d.Before(today); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Monday || d.Weekday() == time.Wednesday {
			txs = append(txs, tx(lunch, fmtDay(d), 8000, "구내식당"))
		}
	}
	// 잡음 2: 격월 5일 50,000원 — 매월이 아니다
	other := recTx{PMType: "", CatID: id(80), CatName: "회비", MainCatID: 80}
	for _, d := range []string{"2026-06-05", "2026-08-05", "2026-10-05"} {
		txs = append(txs, tx(other, d, 50000, "동호회"))
	}
	// 잡음 3: 6월에 끝난 매월 20일 100,000원 — 지금은 안 빠진다
	ended := recTx{PMType: "", CatID: id(81), CatName: "교육비", MainCatID: 81}
	for _, d := range []string{"2026-04-20", "2026-05-20", "2026-06-22"} {
		txs = append(txs, tx(ended, d, 100000, "학원"))
	}
	// 잡음 4: 금액이 제각각인 장보기
	for i, d := range []string{"2026-07-03", "2026-08-11", "2026-09-19", "2026-10-02"} {
		txs = append(txs, tx(cash, d, int64(53000+i*21000), "마트"))
	}

	got := detectRecurring(txs, cal, today)
	if len(got) != 3 {
		for _, c := range got {
			t.Logf("후보: %+v", c)
		}
		t.Fatalf("후보 %d개, want 3 (대출 2 + 카드 1)", len(got))
	}
	b, a, c := got[0], got[1], got[2]

	// 계좌 출금이 먼저, 금액 큰 순
	if b.Day != 30 || b.HolidayRule != HolidayNext || b.Fixed || b.Amount != 786262 || b.NextDate != "2026-11-30" {
		t.Errorf("대출 B: %+v", b)
	}
	// 가맹점은 가장 많이 쓴 이름 (주택금융공사 5번, 하나은행대출 1번) — 대출 A 와 겹치지 않는다
	if b.AmountMin != 783180 || b.AmountMax != 793717 || b.Months != 6 || !b.Account || b.Merchant != "주택금융공사" {
		t.Errorf("대출 B 세부: %+v", b)
	}
	if a.Day != 13 || a.HolidayRule != HolidayNext || !a.Fixed || a.Amount != 382194 || a.NextDate != "2026-11-13" || a.Months != 5 {
		t.Errorf("대출 A: %+v", a)
	}
	if a.Merchant != "하나은행대출" || len(a.Merchants) != 2 || a.Dates[0] != "2026-10-13" || a.Category != "대출상환" || a.Member != "공동" || *a.PaymentMethodID != 2 {
		t.Errorf("대출 A 세부: %+v", a)
	}
	// 카드: 11/8 이 일요일이라 11/9 로 밀리는데 오늘(11/10) 이전이므로 다음은 12/8
	if c.Account || c.Day != 8 || c.NextDate != "2026-12-08" || c.PaymentLabel != "Jade카드" || c.Merchant != "LG U+" {
		t.Errorf("카드 정기결제: %+v", c)
	}
	if a.Key == b.Key || a.Key == "" {
		t.Errorf("서명: %q %q", a.Key, b.Key)
	}
}

func TestRoundSig2(t *testing.T) {
	for v, want := range map[int64]int64{382194: 380000, 786262: 790000, 14300: 14000, 8: 8, 0: 0} {
		if got := roundSig2(v); got != want {
			t.Errorf("roundSig2(%d) = %d, want %d", v, got, want)
		}
	}
}

// 무시한 후보는 다시 나오지 않는다. (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestDismissRecurring(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`DELETE FROM app_settings WHERE key = 'recurring.dismissed'`); err != nil {
		t.Fatal(err)
	}
	cats, _ := st.ListCategories()
	loan := findCatID(cats, "대출상환")
	for _, d := range []string{"2026-08-13", "2026-09-14", "2026-10-13"} {
		if _, err := st.AddTransaction(Transaction{Date: d, Amount: 382194, Direction: "expense",
			Merchant: "하나은행대출", CategoryID: &loan}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.RecurringCandidates(date("2026-11-10"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Day != 13 {
		t.Fatalf("후보: %+v", got)
	}
	if err := st.DismissRecurring(got[0].Key); err != nil {
		t.Fatal(err)
	}
	if again, _ := st.RecurringCandidates(date("2026-11-10")); len(again) != 0 {
		t.Errorf("무시한 뒤에도 나옴: %+v", again)
	}
}
