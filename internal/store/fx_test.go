package store

import (
	"math"
	"testing"
)

func fxCur(code string) FxCurrency {
	c, _ := FxCurrencyOf(code)
	return c
}

func TestFxAmounts(t *testing.T) {
	cases := []struct {
		cur  string
		krw  int64
		rate float64
		want int64
	}{
		{"USD", 10000, 1335.11, 749},    // 7.49달러(내림)
		{"JPY", 10000, 8.3925, 1191},    // 1,191엔
		{"VND", 10000, 0.0521, 191938},  // 191,938동
		{"USD", 13351, 1335.1, 1000},    // 딱 10달러가 부동소수 오차로 9.99가 되지 않게
		{"EUR", 0, 1488, 0},
	}
	for _, c := range cases {
		if got := fxFromKRW(fxCur(c.cur), c.krw, c.rate); got != c.want {
			t.Errorf("%s %d원 @%v = %d, want %d", c.cur, c.krw, c.rate, got, c.want)
		}
	}
	if got := fxKRW(fxCur("USD"), 1050, 1335.11); got != 14019 { // 10.50달러 → 14,018.66원
		t.Errorf("fxKRW = %d", got)
	}
	if got := fxKRW(fxCur("JPY"), 10000, 8.3925); got != 83925 {
		t.Errorf("fxKRW JPY = %d", got)
	}
	if len(FxCurrencies) != 17 {
		t.Errorf("통화 %d개", len(FxCurrencies))
	}
}

func rateBook(cur string, pts ...fxRatePoint) fxRateBook { return fxRateBook{cur: pts} }

func TestFxRateBookOn(t *testing.T) {
	b := rateBook("USD", fxRatePoint{"2026-11-10", 1300}, fxRatePoint{"2026-11-12", 1320})
	for _, c := range []struct {
		date string
		want float64
	}{
		{"2026-11-10", 1300}, {"2026-11-11", 1300}, {"2026-11-12", 1320}, {"2026-11-20", 1320},
		{"2026-11-01", 1300}, // 첫 값보다 이른 날은 첫 값
	} {
		if got, _, ok := b.on("USD", c.date); !ok || got != c.want {
			t.Errorf("%s: %v %v", c.date, got, ok)
		}
	}
	if _, _, ok := b.on("JPY", "2026-11-10"); ok {
		t.Error("없는 통화")
	}
}

func TestPlanFxTradesDaily(t *testing.T) {
	rates := rateBook("USD", fxRatePoint{"2026-11-13", 1300}, fxRatePoint{"2026-11-15", 1250})
	p := FxPlan{ID: 7, Kind: "daily", Currency: "USD", Unit: "krw", Amount: 13000}
	trades, last, done := planFxTrades(p, "2026-11-13", "2026-11-16", rates)
	if done || last != "2026-11-16" || len(trades) != 4 {
		t.Fatalf("last=%s done=%v n=%d", last, done, len(trades))
	}
	// 14일은 13일 환율, 15·16일은 15일 환율
	if trades[1].Rate != 1300 || trades[2].Rate != 1250 || trades[3].Rate != 1250 {
		t.Errorf("rates %v %v %v", trades[1].Rate, trades[2].Rate, trades[3].Rate)
	}
	if trades[0].Amount != 1000 || trades[0].KRW != 13000 || !trades[0].Estimated || trades[0].Source != FxDaily || *trades[0].PlanID != 7 {
		t.Errorf("trade %+v", trades[0])
	}
	if trades[2].Amount != 1040 { // 13,000 / 1,250 = 10.40달러
		t.Errorf("amount %d", trades[2].Amount)
	}

	// 평일만: 11/14(토)·15(일) 건너뜀
	p.WeekdaysOnly = true
	trades, last, _ = planFxTrades(p, "2026-11-13", "2026-11-16", rates)
	if len(trades) != 2 || trades[0].Date != "2026-11-13" || trades[1].Date != "2026-11-16" || last != "2026-11-16" {
		t.Errorf("평일만: %+v last=%s", trades, last)
	}

	// 외화 금액: 매일 5달러
	p.WeekdaysOnly, p.Unit, p.Amount = false, "fx", 500
	trades, _, _ = planFxTrades(p, "2026-11-13", "2026-11-13", rates)
	if len(trades) != 1 || trades[0].Amount != 500 || trades[0].KRW != 6500 {
		t.Errorf("외화 금액: %+v", trades)
	}

	// 환율을 하나도 모르면 아무것도 하지 않는다(다음에 다시)
	trades, last, _ = planFxTrades(p, "2026-11-13", "2026-11-16", fxRateBook{})
	if len(trades) != 0 || last != "" {
		t.Errorf("환율 없음: %+v last=%s", trades, last)
	}
}

func TestPlanFxTradesTarget(t *testing.T) {
	rates := rateBook("JPY",
		fxRatePoint{"2026-11-13", 8.5}, fxRatePoint{"2026-11-14", 8.4}, fxRatePoint{"2026-11-15", 8.35})
	p := FxPlan{ID: 3, Kind: "target", Currency: "JPY", Unit: "krw", Amount: 84000, TargetRate: 8.4}
	trades, last, done := planFxTrades(p, "2026-11-13", "2026-11-15", rates)
	if !done || last != "2026-11-14" || len(trades) != 1 || trades[0].Date != "2026-11-14" || trades[0].Amount != 10000 || trades[0].Source != FxTarget {
		t.Fatalf("한 번만: %+v last=%s done=%v", trades, last, done)
	}
	p.Repeating = true
	trades, last, done = planFxTrades(p, "2026-11-13", "2026-11-15", rates)
	if done || last != "2026-11-15" || len(trades) != 2 {
		t.Fatalf("반복: %+v last=%s done=%v", trades, last, done)
	}
}

func TestBuildFxPositions(t *testing.T) {
	trades := []FxTrade{
		{ID: 1, Date: "2026-11-01", Currency: "USD", Side: "buy", Amount: 10000, KRW: 130000},
		{ID: 2, Date: "2026-11-02", Currency: "USD", Side: "buy", Amount: 10000, KRW: 140000},
		{ID: 3, Date: "2026-11-03", Currency: "USD", Side: "sell", Amount: 5000, KRW: 75000},
		{ID: 4, Date: "2026-11-03", Currency: "JPY", Side: "buy", Amount: 10000, KRW: 84000, Source: FxOpening},
	}
	rates := fxRateBook{"USD": {{"2026-11-12", 1450}}}
	pos := buildFxPositions(trades, rates)
	u := pos["USD"]
	// 평균 1,350원 → 50달러 판 원가 67,500원, 실현 +7,500원. 남은 150달러 원가 202,500원, 지금 217,500원
	if u.Amount != 15000 || u.Cost != 202500 || u.Realized != 7500 || u.Value != 217500 || u.Unrealized != 15000 {
		t.Fatalf("USD %+v", u)
	}
	if math.Abs(u.AvgRate-1350) > 1e-9 || u.Bought != 270000 || u.Sold != 75000 || u.Trades != 3 {
		t.Fatalf("USD %+v", u)
	}
	j := pos["JPY"]
	if j.Amount != 10000 || j.Rate != 0 || j.Value != 0 || math.Abs(j.AvgRate-8.4) > 1e-9 {
		t.Fatalf("JPY(환율 모름) %+v", j)
	}
}

func TestFxCheckHoldings(t *testing.T) {
	ok := []FxTrade{
		{Date: "2026-11-02", Currency: "USD", Side: "sell", Amount: 100},
		{Date: "2026-11-02", Currency: "USD", Side: "buy", Amount: 100}, // 같은 날 사고 팔기
	}
	if err := fxCheckHoldings(ok, "USD"); err != nil {
		t.Fatal(err)
	}
	bad := []FxTrade{
		{Date: "2026-11-01", Currency: "USD", Side: "sell", Amount: 100},
		{Date: "2026-11-02", Currency: "USD", Side: "buy", Amount: 100},
	}
	if err := fxCheckHoldings(bad, "USD"); err == nil {
		t.Fatal("사기 전에 판 것을 막지 못함")
	}
}

func TestFxCashEvents(t *testing.T) {
	today, horizon := date("2026-11-12"), date("2026-12-02") // 20일 뒤까지
	rates := fxRateBook{"USD": {{"2026-11-12", 1300}}}
	plans := []FxPlan{
		{ID: 1, Kind: "daily", Currency: "USD", Unit: "krw", Amount: 10000, Active: true, StartDate: "2026-11-01", LastRun: "2026-11-12"},
		{ID: 2, Kind: "target", Currency: "USD", Unit: "krw", Amount: 99999, TargetRate: 1, Active: true}, // 목표 매수는 넣지 않음
		{ID: 3, Kind: "daily", Currency: "USD", Unit: "fx", Amount: 100, Active: false},                  // 꺼짐
	}
	got := fxCashEvents(plans, rates, today, horizon)
	want := []struct {
		date string
		amt  int64
		lbl  string
	}{
		{"2026-11-13", -60000, "외화 매일 모으기 6일치"}, // 오늘 몫은 이미 기록됨
		{"2026-11-19", -70000, "외화 매일 모으기 7일치"},
		{"2026-11-26", -70000, "외화 매일 모으기 7일치"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if got[i].Date != w.date || got[i].Amount != w.amt || got[i].Label != w.lbl || got[i].Kind != "fx" {
			t.Errorf("[%d] %+v", i, got[i])
		}
	}

	// 외화 금액 계획은 지금 환율로 환산하고 '예상' 표시
	plans = []FxPlan{{ID: 4, Kind: "daily", Currency: "USD", Unit: "fx", Amount: 100, Active: true,
		StartDate: "2026-11-12", EndDate: "2026-11-13", WeekdaysOnly: true}}
	got = fxCashEvents(plans, rates, today, horizon)
	if len(got) != 1 || got[0].Amount != -2600 || !got[0].Estimated || got[0].Label != "외화 매일 모으기 2일치" {
		t.Fatalf("외화 금액: %+v", got)
	}
}
