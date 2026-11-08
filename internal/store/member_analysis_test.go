package store

import (
	"testing"
	"time"
)

func TestMemberRange(t *testing.T) {
	// 진행 중인 달: 오늘까지로 자르고, 전월도 같은 날까지
	r, end, same := memberRange(2026, 11, "month", date("2026-11-08"))
	if fmtDay(end) != "2026-11-08" || !same || fmtDay(r.PrevFrom) != "2026-10-01" || fmtDay(r.PrevTo) != "2026-10-08" {
		t.Errorf("진행 중인 달: end %s same %v prev %s~%s", fmtDay(end), same, fmtDay(r.PrevFrom), fmtDay(r.PrevTo))
	}
	if r.PrevLabel != "전월 같은 날까지" {
		t.Errorf("PrevLabel %q", r.PrevLabel)
	}
	// 지난달을 보면 그대로
	r, end, same = memberRange(2026, 10, "month", date("2026-11-08"))
	if fmtDay(end) != "2026-10-31" || same || fmtDay(r.PrevTo) != "2026-09-30" || r.PrevLabel != "전월" {
		t.Errorf("지난달: end %s same %v prevTo %s %q", fmtDay(end), same, fmtDay(r.PrevTo), r.PrevLabel)
	}
	// 3월 31일까지 본 달의 전월(2월)은 말일을 넘지 않는다
	r, _, _ = memberRange(2026, 3, "month", date("2026-03-30"))
	if fmtDay(r.PrevTo) != "2026-02-28" {
		t.Errorf("2월 자르기: %s", fmtDay(r.PrevTo))
	}
	// 최근 30일은 같은 날 자르기 없음
	r, end, same = memberRange(2026, 11, "recent30", date("2026-11-08"))
	if same || fmtDay(r.From) != "2026-10-10" || fmtDay(end) != "2026-11-08" {
		t.Errorf("최근 30일: %s~%s same %v", fmtDay(r.From), fmtDay(end), same)
	}
}

func TestCompareWindows(t *testing.T) {
	today := date("2026-11-08")
	str := func(ws [][2]time.Time) []string {
		out := []string{}
		for _, w := range ws {
			out = append(out, fmtDay(w[0])+"~"+fmtDay(w[1]))
		}
		return out
	}
	eq := func(name string, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %v want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s[%d]: %s want %s", name, i, got[i], want[i])
			}
		}
	}

	// 지난 10월(끝난 달): 9·8·7월 전체
	r, end, _ := memberRange(2026, 10, "month", today)
	eq("10월", str(compareWindows(r, "month", end, time.Time{})),
		[]string{"2026-09-01~2026-09-30", "2026-08-01~2026-08-31", "2026-07-01~2026-07-31"})
	// 진행 중인 11월(8일까지): 직전 세 달의 1~8일
	r, end, _ = memberRange(2026, 11, "month", today)
	eq("11월 8일까지", str(compareWindows(r, "month", end, time.Time{})),
		[]string{"2026-10-01~2026-10-08", "2026-09-01~2026-09-08", "2026-08-01~2026-08-08"})
	// 기록이 8/15 부터면 8월 1~8일 묶음은 기록 전이라 뺀다
	eq("기록 시작 전 제외", str(compareWindows(r, "month", end, date("2026-08-15"))),
		[]string{"2026-10-01~2026-10-08", "2026-09-01~2026-09-08"})
	// 최근 30일: 그 앞 30일 세 묶음
	r, end, _ = memberRange(2026, 11, "recent30", today)
	eq("최근 30일", str(compareWindows(r, "recent30", end, time.Time{})),
		[]string{"2026-09-10~2026-10-09", "2026-08-11~2026-09-09", "2026-07-12~2026-08-10"})
	// 31일까지 본 3월의 비교 달 2월은 말일에서 끊긴다
	r, end, _ = memberRange(2026, 3, "month", date("2026-04-02"))
	eq("3월", str(compareWindows(r, "month", end, time.Time{}))[:1], []string{"2026-02-01~2026-02-28"})
}

func TestTrendSplit(t *testing.T) {
	ms := []MemberMonth{
		{Month: "2026-06", Living: 610}, {Month: "2026-07", Living: 424}, {Month: "2026-08", Living: 618},
		{Month: "2026-09", Living: 569}, {Month: "2026-10", Living: 575}, {Month: "2026-11", Living: 53, Partial: true},
	}
	recent, prior := trendSplit(ms, "2026-06")
	if monthSpanLabel(ms, recent) != "8~10월" || monthSpanLabel(ms, prior) != "6~7월" {
		t.Errorf("구간: %v %v", recent, prior)
	}
	if avgLiving(ms, recent) != (618+569+575)/3 || avgLiving(ms, prior) != (610+424)/2 {
		t.Errorf("평균: %d %d", avgLiving(ms, recent), avgLiving(ms, prior))
	}
	// 기록 시작(8월) 전 달은 쓰지 않는다 → 끝난 달 3개뿐이라 이전 구간 없음
	recent, prior = trendSplit(ms, "2026-08")
	if monthSpanLabel(ms, recent) != "8~10월" || len(prior) != 0 {
		t.Errorf("기록 시작 반영: %v %v", recent, prior)
	}
}

// 구성원 분석의 숫자는 서로 맞물려야 한다: 종류별 합 = 총액, 월별 막대 = 기간 합,
// 결제수단 막대·카테고리 증감·반복 소비 가게 목록은 거래 목록으로 되짚은 합계와 같아야 한다.
// (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestMemberAnalysis(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.db.Exec(`TRUNCATE subscriptions, subscription_posts RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	members, _ := st.ListMembers()
	var dad, mom int64
	for _, m := range members {
		switch m.Name {
		case "아빠":
			dad = m.ID
		case "엄마":
			mom = m.ID
		}
	}
	cats, _ := st.ListCategories()
	food := findCatID(cats, "식비")
	tele := findCatID(cats, "통신비") // 기본 고정비
	medical := findCatID(cats, "의료비")
	lunch, err := st.AddCategory("회사점심", "expense", food)
	if err != nil {
		t.Fatal(err)
	}
	card, err := st.SavePaymentMethod(PaymentMethod{Name: "A카드", Type: "card"})
	if err != nil {
		t.Fatal(err)
	}
	add := func(d, merchant string, amt, cat int64, who int64, pm *int64) {
		t.Helper()
		if _, err := st.AddTransaction(Transaction{Date: d, Amount: amt, Direction: "expense", Merchant: merchant,
			CategoryID: &cat, MemberID: &who, PaymentMethodID: pm}); err != nil {
			t.Fatal(err)
		}
	}
	// 9월: 점심 2번, 통신비
	add("2026-09-07", "감자탕", 10000, lunch.ID, dad, &card.ID) // 월
	add("2026-09-08", "구내식당", 8000, lunch.ID, dad, &card.ID) // 화
	add("2026-09-20", "통신사", 50000, tele, dad, nil)
	// 10월: 점심 3번(새 가게 1), 통신비, 큰 병원비, 엄마 지출(섞이면 안 됨)
	add("2026-10-05", "구내식당", 8000, lunch.ID, dad, &card.ID) // 월
	add("2026-10-06", "구내식당", 8000, lunch.ID, dad, &card.ID) // 화
	add("2026-10-07", "마라탕", 12000, lunch.ID, dad, nil)      // 수 (새 가게)
	add("2026-10-20", "통신사", 50000, tele, dad, nil)
	add("2026-10-15", "병원", 386550, medical, dad, &card.ID)
	add("2026-10-10", "카페", 5000, food, mom, &card.ID)

	today := date("2026-11-08")
	a, err := st.MemberAnalysis(2026, 10, dad, "month", today)
	if err != nil {
		t.Fatal(err)
	}
	if a.Member != "아빠" || a.From != "2026-10-01" || a.To != "2026-10-31" {
		t.Fatalf("기본: %+v", a)
	}
	if a.Total != 464550 || a.Living != 28000 || a.TxCount != 5 || a.LivingCount != 3 || a.Prev != 68000 {
		t.Errorf("요약: total %d living %d n %d/%d prev %d", a.Total, a.Living, a.TxCount, a.LivingCount, a.Prev)
	}
	if a.FamilyTotal != 469550 {
		t.Errorf("가족 합계 %d", a.FamilyTotal)
	}
	oct := a.Months[len(a.Months)-1]
	if oct.Month != "2026-10" || oct.Living != 28000 || oct.Fixed != 50000 || oct.Big != 386550 || oct.Partial {
		t.Errorf("10월 막대: %+v", oct)
	}
	if len(a.BigItems) != 1 || a.BigItems[0].Merchant != "병원" {
		t.Errorf("큰 지출: %+v", a.BigItems)
	}
	if a.HabitCategoryID != lunch.ID {
		t.Errorf("반복 소비 기본 카테고리 %d, want %d", a.HabitCategoryID, lunch.ID)
	}

	// 종류별 거래 목록 합 = 막대
	sum := func(txs []Transaction) (n int64) {
		for _, x := range txs {
			n += x.Amount
		}
		return n
	}
	for kind, want := range map[string]int64{"living": oct.Living, "fixed": oct.Fixed, "big": oct.Big} {
		txs, err := st.MemberTx(dad, "2026-10-01", "2026-10-31", MemberByKind, kind, 0)
		if err != nil {
			t.Fatal(err)
		}
		if sum(txs) != want {
			t.Errorf("종류 %s: 목록 %d, 막대 %d", kind, sum(txs), want)
		}
	}
	// 결제수단·카테고리 증감도 목록과 맞아야 한다
	for _, p := range a.ByPayment {
		txs, _ := st.MemberTx(dad, a.From, a.To, MemberByPayment, p.Name, 0)
		if sum(txs) != p.Amount || len(txs) != p.Count {
			t.Errorf("결제수단 %s: 목록 %d/%d건, 막대 %d/%d건", p.Name, sum(txs), len(txs), p.Amount, p.Count)
		}
	}
	for _, c := range a.CatChanges {
		txs, _ := st.MemberTx(dad, a.From, a.To, MemberByCategory, c.Name, 0)
		if sum(txs) != c.Current {
			t.Errorf("카테고리 %s: 목록 %d, 막대 %d", c.Name, sum(txs), c.Current)
		}
	}

	// 고정비 표시를 끄면 통신비가 생활 소비로 넘어온다
	if err := st.SetCategoryFixed(tele, false); err != nil {
		t.Fatal(err)
	}
	if a2, _ := st.MemberAnalysis(2026, 10, dad, "month", today); a2.Living != 78000 {
		t.Errorf("고정비 해제 후 생활 소비 %d, want 78000", a2.Living)
	}

	// 반복 소비
	h, err := st.MemberHabit(2026, 10, dad, lunch.ID, "month", today)
	if err != nil {
		t.Fatal(err)
	}
	if h.Category != "식비 > 회사점심" {
		t.Errorf("카테고리 이름 %q", h.Category)
	}
	sep, oc := h.Months[len(h.Months)-2], h.Months[len(h.Months)-1]
	if sep.Count != 2 || sep.Total != 18000 || oc.Count != 3 || oc.Total != 28000 {
		t.Errorf("월별 횟수: %+v %+v", sep, oc)
	}
	if h.Weekday[1] != 2 || h.Weekday[2] != 2 || h.Weekday[3] != 1 || h.Weekday[5] != 0 {
		t.Errorf("요일: %v", h.Weekday)
	}
	if len(h.Places) != 2 || h.Places[0].Name != "구내식당" || h.Places[0].Count != 2 || h.Places[0].New {
		t.Fatalf("가게: %+v", h.Places)
	}
	if !h.Places[1].New || h.Places[1].Name != "마라탕" {
		t.Errorf("새 가게 표시: %+v", h.Places[1])
	}
	txs, _ := st.MemberTx(dad, a.From, a.To, MemberByMerchant, "구내식당", lunch.ID)
	if len(txs) != 2 {
		t.Errorf("가게 내역 %d건", len(txs))
	}

	// 월 × 항목 표
	mt, err := st.MemberMonthly(2026, 10, 2, dad, MemberByCategory)
	if err != nil {
		t.Fatal(err)
	}
	if s := findSeries(mt.Series, "식비"); s == nil || s.Values[0] != 18000 || s.Values[1] != 28000 {
		t.Errorf("월 × 카테고리 식비: %+v", s)
	}
}
