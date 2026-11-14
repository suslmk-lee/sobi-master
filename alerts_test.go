package main

import (
	"strings"
	"testing"

	"sobi/internal/store"
)

func TestBuildAlerts(t *testing.T) {
	in := alertInputs{
		Month: "2026-11",
		Budgets: []store.BudgetStatus{
			{CategoryID: 1, Category: "식비", Amount: 500000, Spent: 520000, Pct: 104, Over: true},
			{CategoryID: 2, Category: "교통", Amount: 100000, Spent: 85000, Pct: 85},
			{CategoryID: 3, Category: "쇼핑", Amount: 300000, Spent: 10000, Pct: 3}, // 알릴 것 없음
		},
		Paces: []store.CardPace{
			{Card: store.PaymentMethod{ID: 7, Name: "Mobility카드"}, PeriodStart: "2026-11-01", Target: 300000, Spent: 6741, Days: 31, Elapsed: 24},
			{Card: store.PaymentMethod{ID: 8, Name: "이른카드"}, PeriodStart: "2026-11-01", Target: 300000, Days: 31, Elapsed: 10}, // 아직 초반
			{Card: store.PaymentMethod{ID: 9, Name: "다쓴카드"}, PeriodStart: "2026-11-01", Target: 300000, Spent: 310000, Days: 31, Elapsed: 24, Achieved: true},
		},
		Unclassified: 12,
		Dues: []store.DuesStatus{
			{Sub: store.Subscription{ID: 4, Name: "동아리", Active: true}, OverdueMonths: 3, Overdue: 90000, OverdueFrom: "2026-09", OverdueTo: "2026-11",
				Payments: []store.DuesPayment{{TransactionID: 50, State: store.DuesReview}, {TransactionID: 61, State: store.DuesReview}, {TransactionID: 70, State: store.DuesCounted}}},
			{Sub: store.Subscription{ID: 5, Name: "아파트", Active: true}},                                                    // 다 냄
			{Sub: store.Subscription{ID: 6, Name: "그만둔 모임", Active: false}, OverdueMonths: 2, OverdueTo: "2026-11"}, // 꺼 둔 것
		},
	}
	got := buildAlerts(in)
	var keys, titles []string
	for _, a := range got {
		keys = append(keys, a.Key)
		titles = append(titles, a.Title)
	}
	want := []string{
		"식비 예산 초과",          // 위험이 먼저
		"교통 예산 임박",          // 그다음 주의
		"Mobility카드 실적 미달 주의",
		"동아리 회비 미납 3개월",
		"동아리 회비 확인 필요 2건", // 안내는 마지막
		"미분류 거래 12건",
	}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Fatalf("알림\n got %v\nwant %v", titles, want)
	}
	wantKeys := []string{"budget:2026-11:1:over", "budget:2026-11:2:near", "card:2026-11-01:7", "dues:4:2026-11", "duesreview:4:61", "unclassified:2026-11"}
	if strings.Join(keys, "|") != strings.Join(wantKeys, "|") {
		t.Errorf("키\n got %v\nwant %v", keys, wantKeys)
	}
	if got[3].Detail != "90,000원 · 9~11월분" {
		t.Errorf("회비 미납 설명: %q", got[3].Detail)
	}
	if got[2].Detail != "24/31일차 · 293,259원 더 써야 달성" {
		t.Errorf("카드 설명: %q", got[2].Detail)
	}
	for _, a := range got {
		if a.Kind == "" || a.Level == "" {
			t.Errorf("종류·수준 빠짐: %+v", a)
		}
	}

	// 교통카드: 추정으로 남은 달만 '금액 확인' 안내
	tr := buildAlerts(alertInputs{Month: "2026-11", Transit: []store.TransitStatus{{
		Sub: store.Subscription{ID: 3, Name: "K-패스"},
		Months: []store.TransitMonth{
			{YM: "2026-11", Fare: 60000, Refund: 12000, Estimated: true},
			{YM: "2026-10", Fare: 58000, Refund: 11600},
		},
	}}})
	if len(tr) != 1 || tr[0].Key != "transit:3:2026-11" || tr[0].Kind != "transit" || tr[0].Title != "K-패스 11월 금액 확인" ||
		tr[0].Detail != "추정 요금 60,000원 · 환급 12,000원 — 실제 금액을 넣어 주세요" {
		t.Errorf("교통카드 알림: %+v", tr)
	}

	if n := len(buildAlerts(alertInputs{Month: "2026-11"})); n != 0 {
		t.Errorf("알릴 것이 없는데 %d건", n)
	}
}

func TestOverdueRange(t *testing.T) {
	cases := map[[2]string]string{
		{"2026-11", "2026-11"}: "11월분",
		{"", "2026-11"}:        "11월분",
		{"2026-09", "2026-11"}: "9~11월분",
		{"2025-12", "2026-02"}: "2025년 12월~2026년 2월분",
	}
	for in, want := range cases {
		if got := overdueRange(in[0], in[1]); got != want {
			t.Errorf("%v → %q, want %q", in, got, want)
		}
	}
}
