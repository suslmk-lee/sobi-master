package store

import "testing"

// TxKey 는 import 중복 판정 기준(날짜+금액+가맹점+방향)을 문자열 하나로 묶는다.
// HasTransaction 의 WHERE 조건과 같은 필드를, 같은 방식(가맹점 trim)으로 봐야 한다.
func TestTxKey(t *testing.T) {
	base := TxKey("2026-08-29", 15000, "쿠팡", "expense")

	same := []struct {
		name                string
		date                string
		amount              int64
		merchant, direction string
	}{
		{"가맹점 앞뒤 공백은 무시(저장 시 trim 되므로)", "2026-08-29", 15000, "  쿠팡  ", "expense"},
		{"완전히 같은 값", "2026-08-29", 15000, "쿠팡", "expense"},
	}
	for _, c := range same {
		if got := TxKey(c.date, c.amount, c.merchant, c.direction); got != base {
			t.Errorf("%s: 키가 달라졌다 (%q != %q)", c.name, got, base)
		}
	}

	diff := []struct {
		name                string
		date                string
		amount              int64
		merchant, direction string
	}{
		{"날짜가 다름", "2026-08-30", 15000, "쿠팡", "expense"},
		{"금액이 다름", "2026-08-29", 15001, "쿠팡", "expense"},
		{"가맹점이 다름", "2026-08-29", 15000, "쿠팡이츠", "expense"},
		{"방향이 다름", "2026-08-29", 15000, "쿠팡", "income"},
	}
	for _, c := range diff {
		if got := TxKey(c.date, c.amount, c.merchant, c.direction); got == base {
			t.Errorf("%s: 키가 같아져 중복으로 오인된다 (%q)", c.name, got)
		}
	}
}

// 구분자 없이 이어 붙이면 서로 다른 거래가 같은 키가 될 수 있다.
// (예: 가맹점 "A" + 방향 "BC" 와 가맹점 "AB" + 방향 "C")
func TestTxKeyNoFieldCollision(t *testing.T) {
	a := TxKey("2026-08-29", 100, "쿠팡", "expense")
	b := TxKey("2026-08-29", 100, "쿠팡expense", "")
	if a == b {
		t.Errorf("필드 경계가 뭉개져 서로 다른 거래가 같은 키가 됐다: %q", a)
	}

	c := TxKey("2026-08-29", 1, "00쿠팡", "expense")
	d := TxKey("2026-08-29", 100, "쿠팡", "expense")
	if c == d {
		t.Errorf("금액/가맹점 경계가 뭉개졌다: %q", c)
	}
}

// idPlaceholders 는 IN (...) 절에 쓸 "?,?,?" 와 인자를 만든다.
func TestIDPlaceholders(t *testing.T) {
	ph, args := idPlaceholders([]int64{7, 8, 9})
	if ph != "?,?,?" {
		t.Errorf("플레이스홀더가 %q", ph)
	}
	if len(args) != 3 || args[0] != int64(7) || args[2] != int64(9) {
		t.Errorf("인자가 %v", args)
	}

	// pq() 를 거치면 Postgres 형식이 돼야 한다.
	if got := pq("DELETE FROM transactions WHERE id IN (" + ph + ")"); got != "DELETE FROM transactions WHERE id IN ($1,$2,$3)" {
		t.Errorf("pq 변환 결과가 %q", got)
	}

	ph1, args1 := idPlaceholders([]int64{42})
	if ph1 != "?" || len(args1) != 1 {
		t.Errorf("단건: ph=%q args=%v", ph1, args1)
	}
}
