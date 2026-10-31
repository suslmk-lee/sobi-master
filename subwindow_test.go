package main

import (
	"testing"

	"sobi/internal/holiday"
	"sobi/internal/store"
)

// 이전 영업일로 당겨진 결제가 전달로 넘어와도 자기 달(ym)로 잡히고, 전달 결제와 겹치지 않는다.
func TestChargeWindowPrevShift(t *testing.T) {
	cal := holiday.New(nil)
	// 매월 1일, 휴일이면 이전 영업일. 2026-11-01 은 일요일 → 10-30(금).
	sub := store.Subscription{ID: 1, Active: true, Cycle: "monthly", BillingDay: 1, HolidayRule: "prev"}

	from, to, err := chargeWindow(sub, "2026-11", cal)
	if err != nil {
		t.Fatal(err)
	}
	if got := from.Format("2006-01-02"); got != "2026-10-30" {
		t.Errorf("11월 창 시작 %s, want 2026-10-30", got)
	}
	if got := to.Format("2006-01-02"); got != "2026-11-30" {
		t.Errorf("11월 창 끝 %s, want 2026-11-30", got)
	}

	// 10월 창은 11월 결제(10-30)가 들어오기 전날까지만 — 같은 거래가 10월 결제로 잡히지 않게.
	// (10-01 은 목요일이라 10월 결제는 10-01 에 있다)
	from, to, err = chargeWindow(sub, "2026-10", cal)
	if err != nil {
		t.Fatal(err)
	}
	if from.Format("2006-01-02") != "2026-10-01" || to.Format("2006-01-02") != "2026-10-29" {
		t.Errorf("10월 창 %s~%s, want 2026-10-01~2026-10-29", from.Format("2006-01-02"), to.Format("2006-01-02"))
	}

	// 같은 거래 하나(10-30)는 11월 결제에만 짝지어진다.
	txs := []store.Transaction{{ID: 7, Date: "2026-10-30", Amount: 5000, Direction: "expense", Merchant: "구독"}}
	sub.Name = "구독"
	w10, w11 := newChargeWant(sub, "2026-10", cal), newChargeWant(sub, "2026-11", cal)
	w10.Sub.ID, w11.Sub.ID = 1, 2
	got := assignCharges([]chargeWant{w10, w11}, txs, nil)
	if got[1] != nil || got[2] == nil {
		t.Errorf("10-30 거래는 11월 결제에만 잡혀야 한다: %v", got)
	}

	f := wantsTxFilter("2026-11", []chargeWant{w11})
	if f.From != "2026-10-30" || f.To != "2026-11-30" {
		t.Errorf("조회 범위 %s~%s", f.From, f.To)
	}
}

// 다음 영업일로 밀려 다음 달로 넘어가는 경우는 창 끝이 늘어난다(기존 동작).
func TestChargeWindowNextShift(t *testing.T) {
	cal := holiday.New(nil)
	sub := store.Subscription{ID: 1, Active: true, Cycle: "monthly", BillingDay: 31} // 2026-10-31 토 → 11-02
	from, to, err := chargeWindow(sub, "2026-10", cal)
	if err != nil {
		t.Fatal(err)
	}
	if from.Format("2006-01-02") != "2026-10-01" || to.Format("2006-01-02") != "2026-11-02" {
		t.Errorf("창 %s~%s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
}
