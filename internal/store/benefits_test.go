package store

import "testing"

func benefitCard(t *testing.T, st *Store, name string) int64 {
	t.Helper()
	pm, err := st.SavePaymentMethod(PaymentMethod{Name: name, Type: "card"})
	if err != nil {
		t.Fatal(err)
	}
	return pm.ID
}

func areas(list []CardBenefit) []string {
	out := make([]string, len(list))
	for i, b := range list {
		out[i] = b.Area
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 혜택은 카드별로 등록 순서를 지켜 보여야 하고, 다른 카드 목록에 섞이면 안 된다.
func TestCardBenefitsOrderAndScope(t *testing.T) {
	st := openTestStore(t)
	a := benefitCard(t, st, "카드A")
	b := benefitCard(t, st, "카드B")

	for _, area := range []string{"차량유지관리", "대중교통", "그 외"} {
		if _, err := st.SaveCardBenefit(CardBenefit{PaymentMethodID: a, Area: area, Kind: "point", RateBp: 150}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.SaveCardBenefit(CardBenefit{PaymentMethodID: b, Area: "커피", Kind: "discount", RateBp: 1000}); err != nil {
		t.Fatal(err)
	}

	got, err := st.ListCardBenefits(a)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"차량유지관리", "대중교통", "그 외"}; !sameStrings(areas(got), want) {
		t.Fatalf("등록 순서가 유지되지 않음: %v", areas(got))
	}

	all, err := st.AllCardBenefits()
	if err != nil {
		t.Fatal(err)
	}
	if len(all[a]) != 3 || len(all[b]) != 1 {
		t.Fatalf("카드별로 나뉘지 않음: A=%d B=%d", len(all[a]), len(all[b]))
	}
}

// 위/아래 이동은 같은 카드 안에서만 자리를 바꾸고, 끝에서 더 밀어도 조용히 제자리에 있어야 한다.
func TestMoveCardBenefit(t *testing.T) {
	st := openTestStore(t)
	pm := benefitCard(t, st, "카드")

	var ids []int64
	for _, area := range []string{"주유", "마트", "통신"} {
		saved, err := st.SaveCardBenefit(CardBenefit{PaymentMethodID: pm, Area: area, Kind: "point"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, saved.ID)
	}

	check := func(step string, want ...string) {
		t.Helper()
		got, err := st.ListCardBenefits(pm)
		if err != nil {
			t.Fatal(err)
		}
		if !sameStrings(areas(got), want) {
			t.Fatalf("%s 후 순서 %v, 기대 %v", step, areas(got), want)
		}
	}

	if err := st.MoveCardBenefit(ids[2], -1); err != nil {
		t.Fatal(err)
	}
	check("통신 위로", "주유", "통신", "마트")

	if err := st.MoveCardBenefit(ids[0], 1); err != nil {
		t.Fatal(err)
	}
	check("주유 아래로", "통신", "주유", "마트")

	// 맨 위에서 더 위로 — 오류 없이 그대로
	if err := st.MoveCardBenefit(ids[2], -1); err != nil {
		t.Fatal(err)
	}
	check("맨 위에서 위로", "통신", "주유", "마트")
}

// 서비스 혜택은 요율이 없다. 실수로 값을 넣어도 저장 단계에서 0으로 정리돼야
// 화면에 "0% 적립" 같은 잘못된 숫자가 뜨지 않는다.
func TestSaveCardBenefitNormalizes(t *testing.T) {
	st := openTestStore(t)
	pm := benefitCard(t, st, "카드")

	svc, err := st.SaveCardBenefit(CardBenefit{
		PaymentMethodID: pm, Area: "  공항 라운지  ", Kind: "service", RateBp: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.RateBp != 0 {
		t.Fatalf("서비스 혜택에 요율이 남음: %d", svc.RateBp)
	}
	if svc.Area != "공항 라운지" {
		t.Fatalf("영역 공백이 정리되지 않음: %q", svc.Area)
	}

	bad, err := st.SaveCardBenefit(CardBenefit{PaymentMethodID: pm, Area: "알 수 없음", Kind: "cashback"})
	if err != nil {
		t.Fatal(err)
	}
	if bad.Kind != "point" {
		t.Fatalf("알 수 없는 종류가 기본값으로 바뀌지 않음: %q", bad.Kind)
	}
}

// 카드를 지우면 그 카드의 혜택 자료도 함께 사라져야 한다(고아 행 방지).
func TestCardBenefitsCascade(t *testing.T) {
	st := openTestStore(t)
	pm := benefitCard(t, st, "지울 카드")
	if _, err := st.SaveCardBenefit(CardBenefit{PaymentMethodID: pm, Area: "주유", Kind: "point"}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePaymentMethod(pm); err != nil {
		t.Fatal(err)
	}
	left, err := st.ListCardBenefits(pm)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("카드 삭제 후 혜택 %d건이 남음", len(left))
	}
}
