package store

import "testing"

func TestPlannedEvents(t *testing.T) {
	basket := int64(2)
	list := []PlannedExpense{
		{ID: 1, Date: "2026-12-05", Name: "김○○ 결혼식 축의금", Amount: 100000, Status: PlannedOpen},
		{ID: 2, Date: "2026-11-15", Name: "이○○ 돌잔치", Amount: 50000, Status: PlannedOpen},               // 지남 → 오늘로
		{ID: 3, Date: "2026-12-12", Name: "박○○ 결혼식 축의금", Amount: 100000, Status: PlannedOpen, BasketID: &basket}, // 바구니에서
		{ID: 4, Date: "2026-11-20", Name: "끝낸 것", Amount: 70000, Status: PlannedDone},
		{ID: 5, Date: "2027-03-01", Name: "먼 것", Amount: 300000, Status: PlannedOpen}, // 내다보는 기간 밖
	}
	ev := plannedEvents(list, date("2026-11-19"), date("2027-01-10"))
	if len(ev) != 2 {
		t.Fatalf("일정: %+v", ev)
	}
	if ev[0].Date != "2026-12-05" || ev[0].Amount != -100000 || ev[0].Kind != "planned" || ev[0].RefID != 1 {
		t.Errorf("예정 지출: %+v", ev[0])
	}
	if ev[1].Date != "2026-11-19" || ev[1].Label != "이○○ 돌잔치 (예정일 지남)" {
		t.Errorf("지난 예정: %+v", ev[1])
	}
	if got := basketPlanned(list); len(got[2]) != 1 || got[2][0].ID != 3 {
		t.Errorf("바구니 예정: %+v", got)
	}
}
