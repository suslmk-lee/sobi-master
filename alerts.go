package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"sobi/internal/store"
)

// ---- 알림 ----
//
// 헤더의 종 버튼(알림창)에 모아 보여 줄 경고·안내. 상태를 보고 만드는 알림이라(예산을 넘었다, 회비가 밀렸다)
// 따로 저장하지 않고 부를 때마다 새로 만든다. Key 는 "같은 알림"을 알아보는 값 — 화면은 이미 본 Key 를 기억해
// 새 알림만 배지로 센다. 상황이 나빠지면(예산 임박 → 초과, 밀린 달이 늘어남) Key 가 바뀌어 다시 새 알림이 된다.

// Alert 은 알림 한 건.
type Alert struct {
	Key    string `json:"key"`
	Kind   string `json:"kind"`  // budget | card | unclassified | dues | transit | basket — 누르면 갈 화면을 정한다
	Level  string `json:"level"` // danger | warn | info
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// alertInputs 는 알림을 만드는 데 쓰는 지금 상태.
type alertInputs struct {
	Month        string // "YYYY-MM" — 달마다 새로 알리는 알림의 Key 에 넣는다
	Budgets      []store.BudgetStatus
	Paces        []store.CardPace
	Unclassified int
	Dues         []store.DuesStatus
	Transit      []store.TransitStatus
	Baskets      []store.BasketStatus
	Candidates   []store.BasketCandidate
}

var alertLevelOrder = map[string]int{"danger": 0, "warn": 1, "info": 2}

// ymLabel 은 "2026-09" → "9월". 해가 다른 범위를 쓸 때는 withYear 로 "2025년 12월".
func ymLabel(ym string, withYear bool) string {
	t, err := time.Parse("2006-01", ym)
	if err != nil {
		return ym
	}
	if withYear {
		return fmt.Sprintf("%d년 %d월", t.Year(), int(t.Month()))
	}
	return fmt.Sprintf("%d월", int(t.Month()))
}

// overdueRange 는 밀린 달 범위를 "11월분", "9~11월분", "2025년 12월~2026년 2월분" 으로.
func overdueRange(from, to string) string {
	if from == "" || from == to {
		return ymLabel(to, false) + "분"
	}
	if len(from) >= 4 && len(to) >= 4 && from[:4] != to[:4] {
		return ymLabel(from, true) + "~" + ymLabel(to, true) + "분"
	}
	return strings.TrimSuffix(ymLabel(from, false), "월") + "~" + ymLabel(to, false) + "분"
}

// buildAlerts 는 지금 상태로 알림 목록을 만든다. 위험 → 주의 → 안내 순.
func buildAlerts(in alertInputs) []Alert {
	out := []Alert{}
	for _, b := range in.Budgets {
		switch {
		case b.Over:
			out = append(out, Alert{Key: fmt.Sprintf("budget:%s:%d:over", in.Month, b.CategoryID), Kind: "budget", Level: "danger",
				Title:  b.Category + " 예산 초과",
				Detail: fmt.Sprintf("%s / %s (%d%%)", won(b.Spent), won(b.Amount), b.Pct)})
		case b.Pct >= 80:
			out = append(out, Alert{Key: fmt.Sprintf("budget:%s:%d:near", in.Month, b.CategoryID), Kind: "budget", Level: "warn",
				Title:  b.Category + " 예산 임박",
				Detail: fmt.Sprintf("%s / %s (%d%%)", won(b.Spent), won(b.Amount), b.Pct)})
		}
	}
	for _, p := range in.Paces {
		if !p.Achieved && p.Days > 0 && p.Elapsed*100/p.Days >= 60 {
			out = append(out, Alert{Key: fmt.Sprintf("card:%s:%d", p.PeriodStart, p.Card.ID), Kind: "card", Level: "warn",
				Title:  p.Card.Name + " 실적 미달 주의",
				Detail: fmt.Sprintf("%d/%d일차 · %s 더 써야 달성", p.Elapsed, p.Days, won(p.Target-p.Spent))})
		}
	}
	for _, d := range in.Dues {
		if !d.Sub.Active {
			continue
		}
		if d.OverdueMonths > 0 {
			out = append(out, Alert{Key: fmt.Sprintf("dues:%d:%s", d.Sub.ID, d.OverdueTo), Kind: "dues", Level: "warn",
				Title:  fmt.Sprintf("%s 회비 미납 %d개월", d.Sub.Name, d.OverdueMonths),
				Detail: fmt.Sprintf("%s · %s", won(d.Overdue), overdueRange(d.OverdueFrom, d.OverdueTo))})
		}
		var review int
		var last int64
		for _, p := range d.Payments {
			if p.State == store.DuesReview {
				review++
				if p.TransactionID > last {
					last = p.TransactionID
				}
			}
		}
		if review > 0 {
			out = append(out, Alert{Key: fmt.Sprintf("duesreview:%d:%d", d.Sub.ID, last), Kind: "dues", Level: "info",
				Title:  fmt.Sprintf("%s 회비 확인 필요 %d건", d.Sub.Name, review),
				Detail: "회비와 금액이 달라요 — 회비로 넣을지 골라 주세요"})
		}
	}
	for _, st := range in.Transit {
		for _, m := range st.Months {
			if !m.Estimated {
				continue
			}
			out = append(out, Alert{Key: fmt.Sprintf("transit:%d:%s", st.Sub.ID, m.YM), Kind: "transit", Level: "info",
				Title:  fmt.Sprintf("%s %s 금액 확인", st.Sub.Name, ymLabel(m.YM, false)),
				Detail: fmt.Sprintf("추정 요금 %s · 환급 %s — 실제 금액을 넣어 주세요", won(m.Fare), won(m.Refund))})
		}
	}
	for _, st := range in.Baskets {
		b := st.Basket
		if !b.Active {
			continue
		}
		if st.Short > 0 {
			detail := "더 넣기로 채우거나 매달 넣는 돈을 올려 주세요"
			if st.Recommended > b.Monthly {
				detail = fmt.Sprintf("매달 %s → %s 로 올리면 마감을 맞춥니다", won(b.Monthly), won(st.Recommended))
			}
			out = append(out, Alert{Key: fmt.Sprintf("basket:%d:short:%s", b.ID, st.ShortYM), Kind: "basket", Level: "warn",
				Title:  fmt.Sprintf("%s %s까지 %s 모자람", st.ShortName, ymLabel(st.ShortYM, false), won(st.Short)),
				Detail: b.Name + " · " + detail})
		}
		if st.CapReached && b.AutoFill && b.Monthly > 0 {
			out = append(out, Alert{Key: fmt.Sprintf("basket:%d:cap", b.ID), Kind: "basket", Level: "info",
				Title:  b.Name + " 목표 도달",
				Detail: fmt.Sprintf("%s 를 다 채웠습니다 — 자동이체를 멈춰도 됩니다", won(b.Cap))})
		}
	}
	if n := len(in.Candidates); n > 0 {
		var last int64
		for _, c := range in.Candidates {
			if c.TransactionID > last {
				last = c.TransactionID
			}
		}
		first := in.Candidates[0]
		out = append(out, Alert{Key: fmt.Sprintf("basket:cand:%d", last), Kind: "basket", Level: "info",
			Title:  fmt.Sprintf("바구니에 연결할 지출 %d건", n),
			Detail: fmt.Sprintf("%s %s 등 — 바구니에서 냈으면 연결하세요", first.Merchant, won(first.Amount))})
	}
	if in.Unclassified > 0 {
		out = append(out, Alert{Key: "unclassified:" + in.Month, Kind: "unclassified", Level: "info",
			Title:  fmt.Sprintf("미분류 거래 %d건", in.Unclassified),
			Detail: "규칙으로 한꺼번에 분류하거나 거래내역에서 직접 분류하세요"})
	}
	sort.SliceStable(out, func(i, j int) bool { return alertLevelOrder[out[i].Level] < alertLevelOrder[out[j].Level] })
	return out
}

// GetAlerts 는 이번 달 기준 알림: 예산 초과·임박, 카드 실적 미달 주의, 회비 미납·확인 필요,
// 교통카드 금액 확인(추정으로 남은 달), 미분류 거래.
// 한 가지를 못 읽어도 나머지는 보여 준다(오류는 기록만).
func (a *App) GetAlerts() ([]Alert, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	now := time.Now()
	y, m := now.Year(), int(now.Month())
	in := alertInputs{Month: now.Format("2006-01")}
	var err error
	if in.Budgets, err = a.st.BudgetStatuses(y, m); err != nil {
		logIf("알림(예산)", err)
	}
	if in.Paces, err = a.st.CardPaces(now); err != nil {
		logIf("알림(카드 실적)", err)
	}
	if sum, err := a.st.MonthlySummary(y, m); err != nil {
		logIf("알림(미분류)", err)
	} else {
		in.Unclassified = sum.UnclassifiedCount
	}
	if in.Dues, err = a.duesBook(now); err != nil {
		logIf("알림(회비)", err)
	}
	if in.Transit, err = a.st.TransitBook(now); err != nil {
		logIf("알림(교통카드)", err)
	}
	if in.Baskets, err = a.st.BasketBook(now); err != nil {
		logIf("알림(바구니)", err)
	}
	if in.Candidates, err = a.st.BasketCandidates(now); err != nil {
		logIf("알림(바구니 후보)", err)
	}
	return buildAlerts(in), nil
}
