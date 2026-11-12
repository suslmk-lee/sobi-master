package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFxDB(t *testing.T) {
	st := openTestStore(t)
	old := nowFunc
	t.Cleanup(func() { nowFunc = old })
	nowFunc = func() time.Time { return time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC) }

	if err := st.SaveFxRates("2026-11-11", map[string]float64{"USD": 1330, "JPY": 8.5, "XXX": 1}, "test"); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveFxRates("2026-11-12", map[string]float64{"USD": 1300, "JPY": 8.4}, "test"); err != nil {
		t.Fatal(err)
	}

	// 거래 검증
	if _, err := st.SaveFxTrade(FxTrade{Date: "2026-11-12", Currency: "XXX", Side: "buy", Amount: 1, Rate: 1}); err == nil {
		t.Fatal("모르는 통화가 저장됨")
	}
	if _, err := st.SaveFxTrade(FxTrade{Date: "2026-11-12", Currency: "USD", Side: "sell", Amount: 100, Rate: 1300}); err == nil {
		t.Fatal("없는 외화를 팖")
	}
	open, err := st.SaveFxTrade(FxTrade{Date: "2026-11-01", Currency: "USD", Side: "buy", Amount: 10000, Rate: 1320, Source: FxOpening})
	if err != nil {
		t.Fatal(err)
	}
	if open.KRW != 132000 {
		t.Fatalf("원화 자동 계산: %+v", open)
	}
	// 원화만 주면 환율을 거꾸로 계산
	sold, err := st.SaveFxTrade(FxTrade{Date: "2026-11-12", Currency: "USD", Side: "sell", Amount: 5000, KRW: 66000})
	if err != nil {
		t.Fatal(err)
	}
	if sold.Rate != 1320 || sold.Source != FxManual {
		t.Fatalf("환율 계산: %+v", sold)
	}
	// 처음 잔고를 지우면 판 것이 보유량을 넘으므로 막는다
	if err := st.DeleteFxTrade(open.ID); err == nil {
		t.Fatal("팔기 앞의 매수가 지워짐")
	}

	// 계획: 시작일은 오늘보다 이를 수 없다
	daily, err := st.SaveFxPlan(FxPlan{Kind: "daily", Currency: "USD", Unit: "krw", Amount: 13000, Active: true, StartDate: "2026-11-01"})
	if err != nil {
		t.Fatal(err)
	}
	if daily.StartDate != "2026-11-12" {
		t.Fatalf("시작일: %s", daily.StartDate)
	}
	target, err := st.SaveFxPlan(FxPlan{Kind: "target", Currency: "JPY", Unit: "krw", Amount: 84000, TargetRate: 8.45, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveFxPlan(FxPlan{Kind: "target", Currency: "JPY", Amount: 1, Active: true}); err == nil {
		t.Fatal("목표 환율 없이 저장됨")
	}

	n, err := st.RunFxPlans(nowFunc())
	if err != nil || n != 2 {
		t.Fatalf("자동 기록 %d %v", n, err)
	}
	if n, _ := st.RunFxPlans(nowFunc()); n != 0 {
		t.Fatalf("같은 날 또 기록: %d", n)
	}
	tp, _ := st.getFxPlan(target.ID)
	if tp.DoneAt != "2026-11-12" || tp.LastRun != "2026-11-12" {
		t.Fatalf("목표 매수 끝: %+v", tp)
	}

	ov, err := st.GetFxOverview()
	if err != nil {
		t.Fatal(err)
	}
	if ov.RateDate != "2026-11-12" || len(ov.Rates) != 17 || len(ov.Trades) != 4 {
		t.Fatalf("overview: date=%s rates=%d trades=%d", ov.RateDate, len(ov.Rates), len(ov.Trades))
	}
	var usd FxRateRow
	for _, r := range ov.Rates {
		if r.Currency == "USD" {
			usd = r
		}
	}
	if usd.Rate != 1300 || usd.Prev != 1330 {
		t.Fatalf("USD 환율 %+v", usd)
	}
	var dst FxPlanStatus
	for _, p := range ov.Plans {
		if p.Plan.ID == daily.ID {
			dst = p
		}
	}
	if dst.Count != 1 || dst.Spent != 13000 || dst.Bought != 1000 || dst.DailyKRW != 13000 {
		t.Fatalf("계획 현황 %+v", dst)
	}

	// 자동 기록을 실제 체결 환율로 고치면 추정 표시가 꺼지고 출처는 그대로
	var auto FxTrade
	for _, x := range ov.Trades {
		if x.Source == FxDaily {
			auto = x
		}
	}
	auto.Rate, auto.KRW = 1299.5, 0
	fixed, err := st.SaveFxTrade(auto)
	if err != nil {
		t.Fatal(err)
	}
	if fixed.Estimated || fixed.Source != FxDaily || fixed.PlanID == nil || fixed.KRW != 12995 {
		t.Fatalf("고친 자동 기록 %+v", fixed)
	}

	// 자금 계획: 10/31 이후 외화 매수(처음 잔고 제외)는 잔액에서 빠지고, 판 돈은 들어온다
	if _, err := st.AddCashBalance("2026-10-31", 1000000, ""); err != nil {
		t.Fatal(err)
	}
	plan, err := st.BuildCashPlan(nowFunc(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 매수 12,995(매일) + 84,000(목표) − 판 66,000 = 30,995
	if plan.FxAdjust != -30995 {
		t.Fatalf("FxAdjust %d", plan.FxAdjust)
	}
	var fxEv int
	for _, e := range plan.Events {
		if e.Kind == "fx" {
			fxEv++
			if e.Date <= "2026-11-12" {
				t.Errorf("오늘 몫이 또 들어감: %+v", e)
			}
		}
	}
	if fxEv == 0 {
		t.Fatal("자금 계획에 매일 모으기 출금이 없음")
	}

	// 백업
	counts, err := st.BackupToSQLite(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	if counts["fx_rates"] != 4 || counts["fx_plans"] != 2 || counts["fx_trades"] != 4 {
		t.Fatalf("백업 %+v", counts)
	}

	// 계획을 지워도 기록은 남는다
	if err := st.DeleteFxPlan(daily.ID); err != nil {
		t.Fatal(err)
	}
	all, _ := st.ListFxTrades()
	if len(all) != 4 {
		t.Fatalf("계획 삭제 후 거래 %d건", len(all))
	}
}
