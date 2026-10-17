package main

import (
	"testing"
	"time"

	"sobi/internal/holiday"
	"sobi/internal/store"
)

// 정기결제 자동 등록 검증. 실제 DB 에 거래를 쓰는 기능이라 "같은 달은 정확히 한 번",
// "이미 입력한 결제와 겹치지 않음", "결제일은 다음 업무일" 을 모두 확인한다.
// (TEST_DATABASE_URL 미설정 시 자동 스킵)

func ymd(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

// autoSub 은 자동 등록이 켜진 정기결제를 만들고 시작 달을 since 로 고정한다
// (시작 달은 저장 시 "오늘" 기준으로 정해지므로, 테스트에서는 직접 박아 둔다).
func autoSub(t *testing.T, a *App, sub store.Subscription, since string) store.Subscription {
	t.Helper()
	sub.Active = true
	sub.AutoPost = true
	saved, err := a.st.SaveSubscription(sub)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.st.DB().Exec(`UPDATE subscriptions SET auto_since=$1 WHERE id=$2`, since, saved.ID); err != nil {
		t.Fatal(err)
	}
	saved.AutoSince = since
	return saved
}

func expensesOf(t *testing.T, a *App, merchant string) []store.Transaction {
	t.Helper()
	all, err := a.st.ListTransactions(store.TxFilter{From: "2000-01-01", To: "2100-12-31", Direction: "expense", Query: merchant})
	if err != nil {
		t.Fatal(err)
	}
	return all
}

func newAutoPostApp(t *testing.T) *App {
	a := newTestApp(t)
	if _, err := a.st.DB().Exec(`TRUNCATE subscriptions, subscription_posts, custom_holidays RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAutoPostBusinessDayAndOnce(t *testing.T) {
	a := newAutoPostApp(t)
	cats, _ := a.st.ListCategories()
	tele := categoryID(cats, "통신비")
	autoSub(t, a, store.Subscription{
		Name: "휴대폰", Merchant: "LG유플러스", Amount: 55000, Cycle: "monthly", BillingDay: 25, CategoryID: &tele,
	}, "2026-10")

	// 2026-10-25 는 일요일 → 실제 결제일은 10-26(월). 토요일엔 아직 아니다.
	r, err := a.postDue(ymd("2026-10-24"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 0 {
		t.Fatalf("결제일 전에 거래가 생김: %+v", r)
	}

	r, err = a.postDue(ymd("2026-10-26"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 1 || len(r.Items) != 1 {
		t.Fatalf("업무일에 1건이 생겨야 함: %+v", r)
	}
	it := r.Items[0]
	if it.Date != "2026-10-26" || it.PlannedDate != "2026-10-25" || it.Reason != "일요일" {
		t.Errorf("다음 업무일로 밀리지 않음: %+v", it)
	}

	txs := expensesOf(t, a, "LG유플러스")
	if len(txs) != 1 {
		t.Fatalf("거래 1건이어야 함: %d건", len(txs))
	}
	if tx := txs[0]; tx.Date != "2026-10-26" || tx.Amount != 55000 || tx.Source != store.SourceSubscription ||
		tx.CategoryID == nil || *tx.CategoryID != tele {
		t.Errorf("만들어진 거래가 정기결제와 다름: %+v", tx)
	}

	// 같은 날·다음 날 다시 돌려도 더 생기면 안 된다
	for _, d := range []string{"2026-10-26", "2026-10-30"} {
		r, err = a.postDue(ymd(d))
		if err != nil {
			t.Fatal(err)
		}
		if r.Created != 0 || r.Matched != 0 {
			t.Fatalf("%s 재실행에서 중복 등록: %+v", d, r)
		}
	}
	if n := len(expensesOf(t, a, "LG유플러스")); n != 1 {
		t.Fatalf("재실행 후 거래가 %d건 — 같은 달이 두 번 등록됨", n)
	}
}

func TestAutoPostHolidayChain(t *testing.T) {
	a := newAutoPostApp(t)
	// 10-03(토) 개천절 → 10-04(일) → 10-05(월) 대체공휴일 → 10-06(화)
	autoSub(t, a, store.Subscription{Name: "보험", Merchant: "삼성화재", Amount: 30000, Cycle: "monthly", BillingDay: 3}, "2026-10")
	r, err := a.postDue(ymd("2026-10-05"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 0 {
		t.Fatalf("대체공휴일(10-05)에 등록됨: %+v", r)
	}
	r, err = a.postDue(ymd("2026-10-06"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 1 || r.Items[0].Date != "2026-10-06" || r.Items[0].Reason != "개천절" {
		t.Fatalf("공휴일 연쇄를 건너 10-06 이어야 함: %+v", r)
	}
}

// 사용자가 그 달 결제를 이미 손으로 넣었으면 새로 만들지 않고 연결만 한다.
func TestAutoPostLinksManualEntry(t *testing.T) {
	a := newAutoPostApp(t)
	autoSub(t, a, store.Subscription{Name: "넷플릭스", Merchant: "넷플릭스", Amount: 17000, Cycle: "monthly", BillingDay: 15}, "2026-10")
	manual, err := a.st.AddTransaction(store.Transaction{
		Date: "2026-10-15", Amount: 17000, Direction: "expense", Merchant: "NETFLIX 넷플릭스", Source: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.postDue(ymd("2026-10-20"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 0 || r.Matched != 1 {
		t.Fatalf("이미 입력된 결제에 연결해야 함: %+v", r)
	}
	if n := len(expensesOf(t, a, "넷플릭스")); n != 1 {
		t.Fatalf("수동 입력이 있는데 거래가 %d건 — 중복 생성됨", n)
	}
	posts, _ := a.st.SubscriptionPosts()
	var p store.SubPost
	for _, byYM := range posts {
		p = byYM["2026-10"]
	}
	if p.TransactionID == nil || *p.TransactionID != manual || !p.Matched {
		t.Errorf("기록이 수동 거래를 가리키지 않음: %+v", p)
	}
}

// 사용자가 자동으로 생긴 거래를 지우면, 그 달은 다시 만들지 않는다.
func TestAutoPostDoesNotRecreateDeleted(t *testing.T) {
	a := newAutoPostApp(t)
	autoSub(t, a, store.Subscription{Name: "헬스장", Merchant: "피트니스", Amount: 80000, Cycle: "monthly", BillingDay: 1}, "2026-10")
	if _, err := a.postDue(ymd("2026-10-02")); err != nil {
		t.Fatal(err)
	}
	txs := expensesOf(t, a, "피트니스")
	if len(txs) != 1 {
		t.Fatalf("먼저 1건 생겨야 함: %d", len(txs))
	}
	if err := a.st.DeleteTransaction(txs[0].ID); err != nil {
		t.Fatal(err)
	}
	r, err := a.postDue(ymd("2026-10-10"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 0 || len(expensesOf(t, a, "피트니스")) != 0 {
		t.Fatalf("지운 달이 다시 만들어짐: %+v", r)
	}
	m, err := a.GetSubscriptionMonth(2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 1 || m.Items[0].PostState != "deleted" {
		t.Errorf("화면 상태가 '지움'이어야 함: %+v", m.Items)
	}
}

// 며칠·몇 달 앱을 안 켜도 다음에 켜면 빠진 달이 채워진다. 단 시작 달 이전은 소급하지 않는다.
func TestAutoPostCatchUpRespectsSince(t *testing.T) {
	a := newAutoPostApp(t)
	// 시작 월은 6월이지만 자동 등록은 8월에 켰다 → 6·7월은 만들지 않는다
	autoSub(t, a, store.Subscription{
		Name: "구독", Merchant: "유튜브프리미엄", Amount: 14900, Cycle: "monthly", BillingDay: 10, StartYM: "2026-06",
	}, "2026-08")
	r, err := a.postDue(ymd("2026-10-28"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 3 {
		t.Fatalf("8·9·10월 3건이어야 함: %+v", r)
	}
	got := map[string]string{}
	for _, it := range r.Items {
		got[it.YM] = it.Date
	}
	// 8/10(월) 그대로, 9/10(목) 그대로, 10/10(토) → 10/12(월)
	want := map[string]string{"2026-08": "2026-08-10", "2026-09": "2026-09-10", "2026-10": "2026-10-12"}
	for ym, d := range want {
		if got[ym] != d {
			t.Errorf("%s 결제일 %q, want %q", ym, got[ym], d)
		}
	}
	if _, ok := got["2026-07"]; ok {
		t.Error("자동 등록을 켜기 전 달(7월)이 소급 등록됨")
	}
}

// 31일 결제는 짧은 달엔 말일로, 말일이 주말이면 다음 달 첫 업무일로 밀린다.
// 다음 달로 넘어가도 그 거래는 원래 달 몫으로 한 번만 기록된다.
func TestAutoPostMonthEndAndSpillover(t *testing.T) {
	a := newAutoPostApp(t)
	autoSub(t, a, store.Subscription{Name: "관리비", Merchant: "아파트관리", Amount: 250000, Cycle: "monthly", BillingDay: 31}, "2026-01")

	// 2026-01-31 은 토요일 → 02-02(월)
	r, err := a.postDue(ymd("2026-02-01"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 0 {
		t.Fatalf("다음 업무일(2/2) 전에 등록됨: %+v", r)
	}
	r, err = a.postDue(ymd("2026-02-02"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 1 || r.Items[0].YM != "2026-01" || r.Items[0].Date != "2026-02-02" {
		t.Fatalf("1월 몫이 2/2 로 등록돼야 함: %+v", r)
	}
	// 2월 몫: 2/28(토) → 3/2(월) 은 삼일절 대체공휴일 → 3/3(화)
	r, err = a.postDue(ymd("2026-03-03"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 1 || r.Items[0].YM != "2026-02" || r.Items[0].PlannedDate != "2026-02-28" || r.Items[0].Date != "2026-03-03" {
		t.Fatalf("2월 몫(말일 28일)이 3/3 으로 등록돼야 함: %+v", r)
	}
	// 화면: 1월 몫이 2월 날짜로 들어갔어도 1월에 '빠짐'으로 보여야 한다
	m, err := a.GetSubscriptionMonth(2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 1 || !m.Items[0].Seen || m.Items[0].SeenDate != "2026-02-02" || m.Items[0].ChargeDate != "2026-02-02" {
		t.Errorf("다음 달로 넘어간 결제를 1월 화면에서 못 찾음: %+v", m.Items)
	}
}

func TestAutoPostYearlyAndSwitches(t *testing.T) {
	a := newAutoPostApp(t)
	autoSub(t, a, store.Subscription{Name: "연회비", Merchant: "카드연회비", Amount: 20000, Cycle: "yearly", BillingMonth: 11, BillingDay: 5}, "2026-09")
	// 꺼 둔 정기결제와 자동 등록을 안 켠 정기결제는 건드리지 않는다
	off := autoSub(t, a, store.Subscription{Name: "꺼짐", Merchant: "꺼진구독", Amount: 1000, Cycle: "monthly", BillingDay: 5}, "2026-09")
	off.Active = false
	if _, err := a.st.SaveSubscription(off); err != nil {
		t.Fatal(err)
	}
	if _, err := a.st.SaveSubscription(store.Subscription{Name: "추적만", Merchant: "추적구독", Amount: 1000, Cycle: "monthly", BillingDay: 5, Active: true}); err != nil {
		t.Fatal(err)
	}

	r, err := a.postDue(ymd("2026-11-10"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 1 || r.Items[0].Name != "연회비" || r.Items[0].YM != "2026-11" {
		t.Fatalf("연 결제는 결제 월(11월)에 한 번만, 꺼진·추적만은 0건이어야 함: %+v", r)
	}
}

// 사용자가 추가한 임시공휴일도 결제일을 민다.
func TestAutoPostCustomHoliday(t *testing.T) {
	a := newAutoPostApp(t)
	if err := a.st.SaveCustomHoliday(holiday.Override{Date: "2026-10-20", Name: "임시공휴일", IsHoliday: true}); err != nil {
		t.Fatal(err)
	}
	autoSub(t, a, store.Subscription{Name: "렌탈", Merchant: "정수기렌탈", Amount: 30000, Cycle: "monthly", BillingDay: 20}, "2026-10")
	r, err := a.postDue(ymd("2026-10-21"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 1 || r.Items[0].Date != "2026-10-21" || r.Items[0].Reason != "임시공휴일" {
		t.Fatalf("임시공휴일(10-20)을 건너 10-21 이어야 함: %+v", r)
	}
}

// 사용자 실제 상황: 같은 가맹점(LG U+) 회선 셋을 각자 정기결제로 걸어 두고, 그중 한 회선만
// 손으로 먼저 입력했다. 그 거래는 금액이 맞는 회선에 연결되고, 나머지 두 회선만 새로 만들어져야
// 한다 — 가맹점만 보면 엉뚱한 회선이 그 거래를 가져가고 남은 회선이 같은 요금을 또 만든다.
func TestAutoPostSameMerchantLines(t *testing.T) {
	a := newAutoPostApp(t)
	lines := []struct {
		name string
		amt  int64
	}{{"인터넷", 38280}, {"민규 모바일", 46800}, {"은미 모바일", 44800}}
	ids := map[string]int64{}
	for _, l := range lines {
		s := autoSub(t, a, store.Subscription{Name: l.name, Merchant: "LG U+", Amount: l.amt, Cycle: "monthly", BillingDay: 8}, "2026-11")
		ids[l.name] = s.ID
	}
	// 11/8 은 일요일 → 11/9(월). 민규 회선 요금만 손으로 먼저 입력
	manual, err := a.st.AddTransaction(store.Transaction{
		Date: "2026-11-09", Amount: 46800, Direction: "expense", Merchant: "LG U+", Source: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}

	r, err := a.postDue(ymd("2026-11-10"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Matched != 1 || r.Created != 2 {
		t.Fatalf("연결 1·생성 2 여야 함: %+v", r)
	}
	txs := expensesOf(t, a, "LG U+")
	if len(txs) != 3 {
		t.Fatalf("LG U+ 거래가 %d건 — 3건이어야 함(중복 없이)", len(txs))
	}
	amounts := map[int64]int{}
	for _, tx := range txs {
		amounts[tx.Amount]++
	}
	for _, l := range lines {
		if amounts[l.amt] != 1 {
			t.Errorf("%s(%d원) 거래가 %d건", l.name, l.amt, amounts[l.amt])
		}
	}
	posts, _ := a.st.SubscriptionPosts()
	if p := posts[ids["민규 모바일"]]["2026-11"]; p.TransactionID == nil || *p.TransactionID != manual || !p.Matched {
		t.Errorf("손으로 넣은 46,800원이 민규 회선에 연결되지 않음: %+v", p)
	}

	// 정기결제 화면: 세 회선이 각자 자기 금액으로 "빠짐"
	m, err := a.GetSubscriptionMonth(2026, 11)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range m.Items {
		if !it.Seen || it.SeenAmount != it.Amount {
			t.Errorf("%s: 빠짐=%v 금액 %d (예상 %d)", it.Sub.Name, it.Seen, it.SeenAmount, it.Amount)
		}
	}
	// 거래내역에 정기결제 이름이 붙는다
	for _, tx := range expensesOf(t, a, "LG U+") {
		if tx.SubscriptionName == "" {
			t.Errorf("거래 %d(%d원)에 정기결제 표시가 없음", tx.ID, tx.Amount)
		}
	}
}

// 거래에서 정기결제를 만들면 원본 거래가 그 달 결제로 연결된다(거래내역에 표시가 붙는다).
// 연결 기능이 생기기 전에 만든 정기결제는 메모의 날짜·가맹점·금액으로 찾아 소급 연결한다.
func TestLinkOriginTx(t *testing.T) {
	a := newAutoPostApp(t)
	tx1, _ := a.st.AddTransaction(store.Transaction{Date: "2026-10-08", Amount: 46800, Direction: "expense", Merchant: "LG U+"})
	tx2, _ := a.st.AddTransaction(store.Transaction{Date: "2026-10-08", Amount: 38280, Direction: "expense", Merchant: "LG U+"})

	s1, err := a.st.SaveSubscription(store.Subscription{Name: "모바일", Merchant: "LG U+", Amount: 46800, BillingDay: 8, StartYM: "2026-11", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := a.LinkSubscriptionTx(s1.ID, tx1)
	if err != nil || !ok {
		t.Fatalf("원본 연결 실패: ok=%v err=%v", ok, err)
	}
	got, _ := a.st.GetTransaction(tx1)
	if got.SubscriptionName != "모바일" {
		t.Errorf("원본 거래에 정기결제 표시가 없음: %q", got.SubscriptionName)
	}
	// 같은 거래를 다른 정기결제에 또 묶지 않는다
	s2, _ := a.st.SaveSubscription(store.Subscription{Name: "다른 것", Merchant: "LG U+", Amount: 46800, BillingDay: 8, Active: true})
	if ok, _ := a.LinkSubscriptionTx(s2.ID, tx1); ok {
		t.Error("이미 묶인 거래가 두 번째 정기결제에도 묶임")
	}

	// 소급 연결: 메모로 원본을 찾는다 — 금액이 맞는 거래만(같은 날 다른 회선과 섞이지 않게)
	s3, _ := a.st.SaveSubscription(store.Subscription{
		Name: "인터넷", Merchant: "LG U+", Amount: 38280, BillingDay: 8, StartYM: "2026-11",
		Memo: "2026-10-08 거래에서 등록", Active: true,
	})
	n, err := a.st.BackfillOriginLinks()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("소급 연결 %d건, want 1", n)
	}
	got2, _ := a.st.GetTransaction(tx2)
	if got2.SubscriptionName != "인터넷" {
		t.Errorf("38,280원 원본이 인터넷 정기결제에 연결되지 않음: %q", got2.SubscriptionName)
	}
	// 다시 돌려도 그대로
	if n, _ := a.st.BackfillOriginLinks(); n != 0 {
		t.Errorf("두 번째 소급 연결에서 %d건이 또 연결됨", n)
	}
	_ = s3
	// 원본 달(10월) 기록이 있어도 자동 등록은 시작 달(11월)부터만
	r, err := a.postDue(ymd("2026-10-20"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Created != 0 {
		t.Errorf("시작 전 달에 거래가 생김: %+v", r)
	}
}
