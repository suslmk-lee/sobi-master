package main

import (
	"testing"

	"sobi/internal/store"
)

// 같은 가맹점 정기결제가 여럿(LG U+ 회선 셋)일 때 금액으로 구분해 짝지어야 한다.
// 가맹점만 보면 세 정기결제가 같은 거래 하나를 서로 가져간다.
func TestAssignChargesByAmount(t *testing.T) {
	sub := func(id int64, amt int64) chargeWant {
		return chargeWant{Sub: store.Subscription{ID: id, Name: "LG U+", Merchant: "LG U+"}, Amount: amt}
	}
	tx := func(id int64, amt int64) store.Transaction {
		return store.Transaction{ID: id, Date: "2026-11-09", Amount: amt, Direction: "expense", Merchant: "LG U+"}
	}
	wants := []chargeWant{sub(2, 38280), sub(4, 46800), sub(3, 44800)}

	t.Run("셋 다 있으면 각자 자기 금액과", func(t *testing.T) {
		got := assignCharges(wants, []store.Transaction{tx(10, 46800), tx(11, 38280), tx(12, 44800)}, nil)
		want := map[int64]int64{2: 11, 4: 10, 3: 12}
		for sid, tid := range want {
			if got[sid] == nil || got[sid].ID != tid {
				t.Errorf("정기결제 %d → 거래 %v, want %d", sid, got[sid], tid)
			}
		}
	})

	t.Run("하나만 있으면 금액이 맞는 정기결제만 가져간다", func(t *testing.T) {
		got := assignCharges(wants, []store.Transaction{tx(10, 46800)}, nil)
		if got[4] == nil || got[4].ID != 10 {
			t.Fatalf("46,800 정기결제가 짝을 못 찾음: %+v", got)
		}
		if got[2] != nil || got[3] != nil {
			t.Errorf("다른 회선이 엉뚱한 요금을 가져감: 2=%v 3=%v", got[2], got[3])
		}
	})

	t.Run("요금이 조금 바뀌어도 가까운 것끼리", func(t *testing.T) {
		got := assignCharges(wants, []store.Transaction{tx(10, 47300), tx(11, 38280), tx(12, 45100)}, nil)
		if got[4].ID != 10 || got[2].ID != 11 || got[3].ID != 12 {
			t.Errorf("요금 변동 후 짝이 뒤섞임: 2=%d 4=%d 3=%d", got[2].ID, got[4].ID, got[3].ID)
		}
	})

	t.Run("정기결제가 하나면 금액이 달라도 찾는다(요금 인상)", func(t *testing.T) {
		got := assignCharges([]chargeWant{sub(9, 17000)}, []store.Transaction{tx(20, 20900)}, nil)
		if got[9] == nil {
			t.Fatal("요금이 오른 달에 못 찾음")
		}
	})

	t.Run("이미 다른 달에 묶인 거래는 쓰지 않는다", func(t *testing.T) {
		got := assignCharges([]chargeWant{sub(4, 46800)}, []store.Transaction{tx(10, 46800)}, map[int64]bool{10: true})
		if got[4] != nil {
			t.Errorf("묶인 거래를 다시 씀: %+v", got[4])
		}
	})
}

func TestAmountClose(t *testing.T) {
	cases := []struct {
		a, b int64
		want bool
	}{
		{46800, 46800, true},
		{50000, 50500, true},  // 1% — 같은 결제를 두 번 등록
		{50000, 55000, false}, // 9% — 다른 결제로 본다
		{38280, 46800, false},
		{44800, 46800, false}, // 4% — 요금이 비슷한 다른 회선(은미·민규 모바일)
		{0, 100, false},
	}
	for _, c := range cases {
		if got := amountClose(c.a, c.b); got != c.want {
			t.Errorf("amountClose(%d, %d) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
