package store

import (
	"testing"

	"sobi/internal/holiday"
)

func TestBuildBasketStatusScheduled(t *testing.T) {
	cal := holiday.New(nil)
	b := Basket{ID: 1, Name: "정해진 연간 지출", Kind: BasketScheduled, Monthly: 100000, FillDay: 10, Active: true,
		Items: []BasketItem{
			{ID: 11, Name: "자동차세", Month: 1, Day: 0, Amount: 450000},
			{ID: 12, Name: "자동차보험", Month: 3, Day: 15, Amount: 900000},
			{ID: 13, Name: "재산세", Month: 7, Day: 0, Amount: 300000},
			{ID: 14, Name: "재산세", Month: 9, Day: 0, Amount: 300000},
		}}
	open := []BasketEntry{{ID: 1, Kind: BasketOpen, Amount: 200000, Date: "2026-11-01"}}
	st := BuildBasketStatus(b, open, date("2026-11-14"), cal)

	if st.Balance != 200000 || st.AnnualTotal != 1950000 {
		t.Fatalf("잔액·연간 합: %d %d", st.Balance, st.AnnualTotal)
	}
	if len(st.Dues) != 4 || st.Dues[0].Name != "자동차세" || st.Dues[0].Date != "2027-01-31" || st.Dues[1].Date != "2027-03-15" {
		t.Fatalf("마감: %+v", st.Dues)
	}
	// 3월 보험까지 135만 원 − 잔액 20만 원을 채움 4번(12·1·2·3월)으로 → 287,500 → 천 원 올림
	if st.Recommended != 288000 {
		t.Errorf("권장 월 적립액: %d", st.Recommended)
	}
	// 지금 월 10만 원이면 1월 자동차세(채움 2번)에서 5만 원 모자람
	if st.ShortName != "자동차세" || st.ShortYM != "2027-01" || st.Short != 50000 {
		t.Errorf("처음 모자라는 마감: %s %s %d", st.ShortName, st.ShortYM, st.Short)
	}
	if st.Dues[0].Projected != 400000 || st.Dues[1].Short != 750000 {
		t.Errorf("예상 잔액·누적 부족: %+v", st.Dues[:2])
	}
	if st.NextFillDate != "2026-12-10" || st.NextFillAmount != 100000 {
		t.Errorf("다음 채움: %s %d", st.NextFillDate, st.NextFillAmount)
	}

	// 1월 자동차세를 미리 냈으면 그 마감은 빠지고, 낸 돈만큼 잔액이 준다
	item := int64(11)
	paid := append(open, BasketEntry{ID: 2, Kind: BasketSpend, Amount: -450000, Date: "2026-11-14", ItemID: &item, DueYM: "2027-01"},
		BasketEntry{ID: 3, Kind: BasketExtra, Amount: 450000, Date: "2026-11-14"})
	st = BuildBasketStatus(b, paid, date("2026-11-14"), cal)
	if !st.Dues[0].Paid || st.Balance != 200000 || st.ShortName != "자동차보험" {
		t.Errorf("낸 마감: paid=%v balance=%d short=%s", st.Dues[0].Paid, st.Balance, st.ShortName)
	}

	// 이번 달 자동 채움을 이미 기록했으면 다음 채움은 다음 달
	auto := append(open, BasketEntry{ID: 4, Kind: BasketFill, Amount: 100000, Auto: true, YM: "2026-11", Date: "2026-11-10"})
	if st = BuildBasketStatus(b, auto, date("2026-11-05"), cal); st.NextFillDate != "2026-12-10" {
		t.Errorf("자동 채움 뒤 다음 채움: %s", st.NextFillDate)
	}
}

func TestBuildBasketStatusReserve(t *testing.T) {
	cal := holiday.New(nil)
	b := Basket{ID: 2, Name: "생활 예비비", Kind: BasketReserve, Cap: 3000000, Monthly: 300000, FillDay: 10, Active: true}
	st := BuildBasketStatus(b, []BasketEntry{{Kind: BasketOpen, Amount: 1000000}}, date("2026-11-14"), cal)
	if st.CapReached || st.MonthsToCap != 7 {
		t.Errorf("상한까지: reached=%v months=%d", st.CapReached, st.MonthsToCap)
	}
	st = BuildBasketStatus(b, []BasketEntry{{Kind: BasketOpen, Amount: 3000000}}, date("2026-11-14"), cal)
	if !st.CapReached || st.MonthsToCap != 0 {
		t.Errorf("상한 도달: %v %d", st.CapReached, st.MonthsToCap)
	}
	if len(st.Dues) != 0 || st.Recommended != 0 {
		t.Errorf("예비비에 마감 계산: %+v", st)
	}
}

func TestBasketEvents(t *testing.T) {
	cal := holiday.New(nil)
	b := Basket{ID: 3, Name: "부모님 병원비", Kind: BasketReserve, Monthly: 200000, FillDay: 10, Active: true, AutoFill: true}
	book := []BasketStatus{BuildBasketStatus(b, nil, date("2026-11-14"), cal)}
	ev := basketEvents(book, cal, date("2026-11-14"), date("2027-01-31"))
	// 12/10(목), 1/10 은 일요일 → 1/11(월)
	if len(ev) != 2 || ev[0].Date != "2026-12-10" || ev[1].Date != "2027-01-11" || ev[0].Amount != -200000 || ev[0].Kind != "basket" || ev[0].Estimated {
		t.Fatalf("채움 일정: %+v", ev)
	}
	b.Active = false
	if ev = basketEvents([]BasketStatus{BuildBasketStatus(b, nil, date("2026-11-14"), cal)}, cal, date("2026-11-14"), date("2027-01-31")); len(ev) != 0 {
		t.Fatalf("꺼 둔 바구니: %+v", ev)
	}
}
