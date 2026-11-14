package main

import (
	"sort"
	"time"

	"sobi/internal/store"
)

// ---- 회비 ----
//
// 장부 계산은 store.BuildDuesStatus — 여기서는 어느 거래가 어느 회비의 납부인지 고른다
// (받는 사람 이름 대조 merchantMatch 가 이 계층에 있다).

// duesMaxMonthsPerPayment 는 한 번에 낸 돈을 회비로 볼 때 허용하는 최대 개월 수(3년치).
const duesMaxMonthsPerPayment = 36

// duesAmountFits 는 금액이 회비 몇 달치(1~36배)로 딱 떨어지는지 본다. 현재 요금, 예약된 변경 요금,
// 거래한 달의 요금 중 하나라도 맞으면 된다(인상 전후 금액이 섞여도 알아보게).
func duesAmountFits(sub store.Subscription, t store.Transaction) bool {
	fees := []int64{sub.Amount, sub.NextAmount}
	if len(t.Date) >= 7 {
		fees = append(fees, sub.AmountAt(t.Date[:7]))
	}
	for _, f := range fees {
		if f > 0 && t.Amount%f == 0 && t.Amount/f >= 1 && t.Amount/f <= duesMaxMonthsPerPayment {
			return true
		}
	}
	return false
}

func duesPayment(t store.Transaction, state string, manual bool) store.DuesPayment {
	return store.DuesPayment{
		TransactionID: t.ID, Date: t.Date, Amount: t.Amount, Merchant: t.Merchant, Memo: t.Memo,
		PaymentMethodName: t.PaymentMethodName, State: state, Manual: manual,
	}
}

// assignDues 는 거래들을 회비 장부에 나눠 담는다(회비 id → 그 회비에 보일 거래).
//
//   - 회비를 "어디까지 냈다"고 밝힌 뒤(기준)에 생긴 거래만 본다. 그때 이미 있던 거래는 그 말에 들어 있다.
//   - 사용자가 정한 연결이 있으면 그대로: 넣음 → 장부에 더함, 회비 아님 → 뺀 거래로 보여 줌.
//   - 없으면 받는 사람(가맹점)이 맞는 지출을 찾는다. 금액이 회비 몇 달치로 떨어지면 장부에 더하고,
//     아니면 '확인 필요'로만 보여 준다 — 같은 사람에게 보낸 회식비를 회비로 잘못 세지 않게.
//   - 정기결제·선불권에 이미 묶인 거래는 건드리지 않는다.
//   - 받는 사람이 겹치는 회비가 여럿이면 금액이 맞는 쪽, 그다음 먼저 만든 쪽.
func assignDues(dues []store.Subscription, txs []store.Transaction, links map[int64]store.DuesLink) map[int64][]store.DuesPayment {
	dues = append([]store.Subscription(nil), dues...)
	sort.Slice(dues, func(i, j int) bool { return dues[i].ID < dues[j].ID })
	byID := map[int64]store.Subscription{}
	for _, d := range dues {
		byID[d.ID] = d
	}
	out := map[int64][]store.DuesPayment{}
	for _, t := range txs {
		if t.Direction != "expense" {
			continue
		}
		if l, ok := links[t.ID]; ok {
			sub, ok := byID[l.SubscriptionID]
			if !ok || t.ID <= sub.PaidThroughTx {
				continue
			}
			state := store.DuesCounted
			if l.State == store.DuesLinkExclude {
				state = store.DuesExcluded
			}
			out[sub.ID] = append(out[sub.ID], duesPayment(t, state, true))
			continue
		}
		if t.SubscriptionName != "" || t.PrepaidName != "" || t.Source == store.SourceSubscription {
			continue
		}
		var review int64
		counted := false
		for _, d := range dues {
			if t.ID <= d.PaidThroughTx || t.Date < d.PaidThroughAt || !merchantMatch(d.MatchTarget(), t.Merchant) {
				continue
			}
			if duesAmountFits(d, t) {
				out[d.ID] = append(out[d.ID], duesPayment(t, store.DuesCounted, false))
				counted = true
				break
			}
			if review == 0 {
				review = d.ID
			}
		}
		if !counted && review != 0 {
			out[review] = append(out[review], duesPayment(t, store.DuesReview, false))
		}
	}
	return out
}

// duesSubs 는 정기결제 중 회비만.
func duesSubs(subs []store.Subscription) []store.Subscription {
	var out []store.Subscription
	for _, s := range subs {
		if s.Kind == store.KindDues {
			out = append(out, s)
		}
	}
	return out
}

// regularSubs 는 회비·교통카드를 뺀 정기결제.
func regularSubs(subs []store.Subscription) []store.Subscription {
	out := make([]store.Subscription, 0, len(subs))
	for _, s := range subs {
		if s.Kind != store.KindDues && s.Kind != store.KindTransit {
			out = append(out, s)
		}
	}
	return out
}

// duesBook 은 회비 전체의 장부(정렬: 켜진 것 먼저, 이름순 — ListSubscriptions 순서).
func (a *App) duesBook(today time.Time) ([]store.DuesStatus, error) {
	out := []store.DuesStatus{}
	subs, err := a.st.ListSubscriptions()
	if err != nil {
		return out, err
	}
	dues := duesSubs(subs)
	if len(dues) == 0 {
		return out, nil
	}
	cal, err := a.st.Calendar()
	if err != nil {
		return out, err
	}
	links, err := a.st.DuesLinks()
	if err != nil {
		return out, err
	}
	// 기준일 이후 지출 + 그보다 앞 날짜로 직접 넣은 거래
	from := ""
	for _, d := range dues {
		if from == "" || d.PaidThroughAt < from {
			from = d.PaidThroughAt
		}
	}
	txs, err := a.st.ListTransactions(store.TxFilter{From: from, Direction: "expense"})
	if err != nil {
		return out, err
	}
	seen := map[int64]bool{}
	for _, t := range txs {
		seen[t.ID] = true
	}
	var extra []int64
	for id := range links {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	if len(extra) > 0 {
		more, err := a.st.GetTransactions(extra)
		if err != nil {
			return out, err
		}
		txs = append(txs, more...)
	}

	assigned := assignDues(dues, txs, links)
	for _, d := range dues {
		pays := assigned[d.ID]
		var paid int64
		for _, p := range pays {
			if p.State == store.DuesCounted {
				paid += p.Amount
			}
		}
		st := store.BuildDuesStatus(d, paid, today, cal)
		sort.Slice(pays, func(i, j int) bool {
			if pays[i].Date != pays[j].Date {
				return pays[i].Date > pays[j].Date
			}
			return pays[i].TransactionID > pays[j].TransactionID
		})
		if pays != nil {
			st.Payments = pays
		}
		out = append(out, st)
	}
	return out, nil
}

// annotateDues 는 거래 목록에서 회비 장부에 들어간 거래에 회비 이름(DuesName)을 붙인다.
func (a *App) annotateDues(txs []store.Transaction) {
	if len(txs) == 0 {
		return
	}
	subs, err := a.st.ListSubscriptions()
	if err != nil {
		logIf("거래 목록(회비 표시)", err)
		return
	}
	dues := duesSubs(subs)
	if len(dues) == 0 {
		return
	}
	links, err := a.st.DuesLinks()
	if err != nil {
		logIf("거래 목록(회비 표시)", err)
		return
	}
	names := map[int64]string{}
	for _, d := range dues {
		names[d.ID] = d.Name
	}
	byTx := map[int64]string{}
	for sid, pays := range assignDues(dues, txs, links) {
		for _, p := range pays {
			if p.State == store.DuesCounted {
				byTx[p.TransactionID] = names[sid]
			}
		}
	}
	for i := range txs {
		if n, ok := byTx[txs[i].ID]; ok {
			txs[i].DuesName = n
		}
	}
}

// ListDues 는 회비 전체의 장부: 어디까지 냈는지, 미납·다음 납부, 기준 이후 납부 거래.
func (a *App) ListDues() ([]store.DuesStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	out, err := a.duesBook(time.Now())
	logIf("ListDues", err)
	return out, err
}

// AddDuesPayment 는 회비 납부를 거래로 기록하고 장부에 넣는다.
func (a *App) AddDuesPayment(subID int64, date string, amount int64, memo string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	_, err := a.st.AddDuesPayment(subID, date, amount, memo)
	logIf("AddDuesPayment", err)
	return err
}

// SetDuesLink 는 거래를 회비 납부로 넣거나(include) 회비 아님으로 뺀다(exclude).
func (a *App) SetDuesLink(subID, transactionID int64, state string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.SetDuesLink(subID, transactionID, state)
	logIf("SetDuesLink", err)
	return err
}

// ClearDuesLink 는 직접 정한 연결을 지워 자동 판정으로 되돌린다.
func (a *App) ClearDuesLink(transactionID int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.ClearDuesLink(transactionID)
	logIf("ClearDuesLink", err)
	return err
}
