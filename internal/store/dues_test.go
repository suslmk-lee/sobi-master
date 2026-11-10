package store

import (
	"testing"

	"sobi/internal/holiday"
)

func duesSub(fee int64, day int, through string) Subscription {
	return Subscription{
		ID: 1, Name: "모임", Amount: fee, Cycle: "monthly", BillingDay: day, DayRule: "day",
		HolidayRule: HolidayNone, Active: true, Kind: KindDues, PaidThrough: through,
	}
}

func TestBuildDuesStatus(t *testing.T) {
	cal := holiday.New(nil)
	type want struct {
		through          string
		credit           int64
		overdueN         int
		overdue          int64
		from, to         string
		nextYM, nextDate string
		nextDue          int64
		ahead            int
		ended            bool
	}
	cases := []struct {
		name  string
		sub   Subscription
		paid  int64
		today string
		w     want
	}{
		{
			name: "아파트: 지난달까지 냄, 이번 달 납부일 전",
			sub:  duesSub(20000, 25, "2026-10"), today: "2026-11-11",
			w: want{through: "2026-10", nextYM: "2026-11", nextDate: "2026-11-25", nextDue: 20000},
		},
		{
			name: "납부일 당일은 아직 미납이 아니다",
			sub:  duesSub(20000, 11, "2026-10"), today: "2026-11-11",
			w: want{through: "2026-10", nextYM: "2026-11", nextDate: "2026-11-11", nextDue: 20000},
		},
		{
			name: "납부일이 지나면 미납",
			sub:  duesSub(20000, 10, "2026-10"), today: "2026-11-11",
			w: want{through: "2026-10", overdueN: 1, overdue: 20000, from: "2026-11", to: "2026-11",
				nextYM: "2026-12", nextDate: "2026-12-10", nextDue: 20000},
		},
		{
			name: "동아리: 6월분까지 내고 다섯 달 밀림",
			sub:  duesSub(30000, 10, "2026-06"), today: "2026-11-11",
			w: want{through: "2026-06", overdueN: 5, overdue: 150000, from: "2026-07", to: "2026-11",
				nextYM: "2026-12", nextDate: "2026-12-10", nextDue: 30000},
		},
		{
			name: "몰아서 9만 원 → 9월분까지",
			sub:  duesSub(30000, 10, "2026-06"), paid: 90000, today: "2026-11-11",
			w: want{through: "2026-09", overdueN: 2, overdue: 60000, from: "2026-10", to: "2026-11",
				nextYM: "2026-12", nextDate: "2026-12-10", nextDue: 30000},
		},
		{
			name: "한 달치에 못 미치는 돈은 첫 미납 달에서 뺀다",
			sub:  duesSub(30000, 10, "2026-06"), paid: 40000, today: "2026-11-11",
			w: want{through: "2026-07", credit: 10000, overdueN: 4, overdue: 110000, from: "2026-08", to: "2026-11",
				nextYM: "2026-12", nextDate: "2026-12-10", nextDue: 30000},
		},
		{
			name: "미리 낸 달",
			sub:  duesSub(20000, 25, "2026-10"), paid: 60000, today: "2026-11-11",
			w: want{through: "2027-01", ahead: 2, nextYM: "2027-02", nextDate: "2027-02-25", nextDue: 20000},
		},
		{
			name: "남은 돈이 다음 낼 돈을 줄인다",
			sub:  duesSub(20000, 25, "2026-10"), paid: 30000, today: "2026-11-11",
			w: want{through: "2026-11", credit: 10000, nextYM: "2026-12", nextDate: "2026-12-25", nextDue: 10000},
		},
		{
			name: "종료 월까지 다 냄",
			sub: func() Subscription {
				s := duesSub(20000, 25, "2026-10")
				s.EndYM = "2026-12"
				return s
			}(),
			paid: 40000, today: "2026-11-11",
			w: want{through: "2026-12", ahead: 1, ended: true},
		},
		{
			name: "꺼 둔 회비는 미납을 세지 않는다",
			sub: func() Subscription {
				s := duesSub(30000, 10, "2026-06")
				s.Active = false
				return s
			}(),
			today: "2026-11-11",
			w:     want{through: "2026-06"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BuildDuesStatus(c.sub, c.paid, date(c.today), cal)
			g := want{got.PaidThrough, got.Credit, got.OverdueMonths, got.Overdue, got.OverdueFrom, got.OverdueTo,
				got.NextYM, got.NextDate, got.NextDue, got.AheadMonths, got.Ended}
			if g != c.w {
				t.Errorf("\n got %+v\nwant %+v", g, c.w)
			}
		})
	}
}

func TestBuildDuesStatusFeeChange(t *testing.T) {
	cal := holiday.New(nil)
	s := duesSub(20000, 25, "2026-11")
	s.NextAmount, s.NextAmountYM = 25000, "2027-01"
	// 12월(2만) + 1월(2.5만) = 4.5만
	got := BuildDuesStatus(s, 45000, date("2026-11-11"), cal)
	if got.PaidThrough != "2027-01" || got.Credit != 0 || got.NextDue != 25000 || got.NextYM != "2027-02" {
		t.Fatalf("got through=%s credit=%d next=%s %d", got.PaidThrough, got.Credit, got.NextYM, got.NextDue)
	}
	// 이번 달 회비는 아직 옛 요금
	if got.Fee != 20000 {
		t.Fatalf("fee = %d", got.Fee)
	}
}

func TestBuildDuesStatusHoliday(t *testing.T) {
	cal := holiday.New(nil)
	s := duesSub(20000, 14, "2026-10")
	s.HolidayRule = HolidayNext // 2026-11-14 은 토요일 → 16일(월)
	got := BuildDuesStatus(s, 0, date("2026-11-15"), cal)
	if got.OverdueMonths != 0 || got.NextDate != "2026-11-16" {
		t.Fatalf("overdue=%d next=%s", got.OverdueMonths, got.NextDate)
	}
}

func TestDuesEvents(t *testing.T) {
	cal := holiday.New(nil)
	cardID, bankID := int64(7), int64(8)
	pms := []PaymentMethod{{ID: cardID, Type: "card"}, {ID: bankID, Type: "bank"}}
	today, horizon := date("2026-11-11"), date("2026-12-31")

	club := duesSub(30000, 10, "2026-06")
	club.ID, club.Name, club.PaymentMethodID = 2, "동아리", &bankID
	apt := duesSub(20000, 25, "2026-10")
	apt.ID, apt.Name = 3, "아파트"
	byCard := duesSub(10000, 5, "2026-10")
	byCard.ID, byCard.PaymentMethodID = 4, &cardID

	var book []DuesStatus
	for _, s := range []Subscription{club, apt, byCard} {
		book = append(book, BuildDuesStatus(s, 0, today, cal))
	}
	got := duesEvents(book, pms, cal, today, horizon)
	type ev struct {
		date, label string
		amount      int64
	}
	var g []ev
	for _, e := range got {
		if e.Kind != "dues" {
			t.Errorf("kind = %q", e.Kind)
		}
		g = append(g, ev{e.Date, e.Label, e.Amount})
	}
	want := []ev{
		// 밀린 다섯 달(15만)을 다음 납부일 12/10 몫(3만)에 얹는다
		{"2026-12-10", "동아리 회비 (밀린 5개월 포함)", -180000},
		{"2026-11-25", "아파트 회비", -20000},
		{"2026-12-25", "아파트 회비", -20000},
	}
	if len(g) != len(want) {
		t.Fatalf("got %+v", g)
	}
	for i := range want {
		if g[i] != want[i] {
			t.Errorf("[%d] got %+v want %+v", i, g[i], want[i])
		}
	}

	// 다음 납부일이 계획 기간 밖이면 밀린 돈은 오늘 출금으로
	got = duesEvents(book[:1], pms, cal, today, date("2026-11-30"))
	if len(got) != 1 || got[0].Date != "2026-11-11" || got[0].Amount != -150000 || got[0].Label != "동아리 밀린 회비" {
		t.Fatalf("got %+v", got)
	}
}

func TestAddYM(t *testing.T) {
	if addYM("2026-12", 1) != "2027-01" || addYM("2026-01", -1) != "2025-12" || addYM("bad", 1) != "" {
		t.Fatal("addYM")
	}
}
