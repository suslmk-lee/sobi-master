package main

import (
	"testing"

	"sobi/internal/store"
)

func TestCandidateRegistered(t *testing.T) {
	subs := []store.Subscription{
		{Name: "카카오구독", Merchant: "카카오구독", Amount: 990},
		{Name: "하나은행 대출 (13일)", Merchant: "하나은행대출", Amount: 382194},
	}
	cases := []struct {
		name string
		c    store.RecurringCandidate
		want bool
	}{
		// 기록 이름 '카카오' ↔ 정기결제 '카카오구독' (포함 관계) + 같은 금액
		{"이름 일부만 같아도 등록된 것", store.RecurringCandidate{Merchants: []string{"카카오"}, AmountMin: 990, AmountMax: 990}, true},
		// 이름이 섞인 대출 A: 기록 이름 중 하나가 맞고 금액도 맞다
		{"섞인 이름 중 하나", store.RecurringCandidate{Merchants: []string{"주택금융공사", "하나은행대출"}, AmountMin: 382194, AmountMax: 382194}, true},
		// 대출 B: 이름(하나은행대출)이 기록에 섞여 있어도 금액이 두 배라 다른 출금
		{"이름이 맞아도 금액이 다르면 아님", store.RecurringCandidate{Merchants: []string{"주택금융공사", "하나은행대출"}, AmountMin: 783180, AmountMax: 793717}, false},
		{"등록 안 된 것", store.RecurringCandidate{Merchants: []string{"삼성생명"}, AmountMin: 162162, AmountMax: 162162}, false},
	}
	for _, c := range cases {
		if got := candidateRegistered(c.c, subs); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
