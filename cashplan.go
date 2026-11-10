package main

import (
	"time"

	"sobi/internal/holiday"
	"sobi/internal/store"
)

// ---- 자금 계획 ----
// 월급(입금)과 카드대금(출금) 사이에 통장에 돈이 충분한지 보여 준다.
// 계산 본체는 store.BuildCashPlan — 여기서는 "오늘"과 정기결제 대조 훅만 끼워 넣는다.

// GetCashPlan 은 자금 계획 화면 데이터(다음 결제일 카드값, 잔액 흐름, 안전 지출 가능액)를 돌려준다.
func (a *App) GetCashPlan() (store.CashPlan, error) {
	if err := a.ensure(); err != nil {
		return store.CashPlan{}, err
	}
	plan, err := a.st.BuildCashPlan(time.Now(), a.newSubChargeMatcher())
	logIf("GetCashPlan", err)
	return plan, err
}

// newSubChargeMatcher 는 "정기결제의 그 달 결제가 이미 (손으로) 입력된 거래로 있는지" 판정기를 만든다.
// 정기결제 화면(GetSubscriptionMonth)과 같은 짝짓기(assignCharges)를 쓴다 — 가맹점 이름만 보면
// 이름이 같거나 섞인 두 대출 중 한 건이 빠지는 순간 다른 건까지 '빠짐'으로 보여 일정에서 사라지므로,
// 금액이 가까운 것끼리 한 번씩만 묶는다. 같은 달은 한 번만 계산해 둔다.
func (a *App) newSubChargeMatcher() store.SubMatchFunc {
	var (
		loaded  bool
		loadErr error
		subs    []store.Subscription
		cal     *holiday.Calendar
		posts   map[int64]map[string]store.SubPost
		linked  = map[int64]bool{}
		byMonth = map[string]map[int64]bool{}
	)
	load := func() {
		if loaded {
			return
		}
		loaded = true
		if subs, loadErr = a.st.ListSubscriptions(); loadErr != nil {
			return
		}
		if cal, loadErr = a.st.Calendar(); loadErr != nil {
			return
		}
		if posts, loadErr = a.st.SubscriptionPosts(); loadErr != nil {
			return
		}
		for _, byYM := range posts {
			for _, p := range byYM {
				if p.TransactionID != nil {
					linked[*p.TransactionID] = true
				}
			}
		}
	}
	return func(sub store.Subscription, ym, _, _ string) bool {
		if got, ok := byMonth[ym]; ok {
			return got[sub.ID]
		}
		load()
		if loadErr != nil {
			logIf("자금 계획 정기결제 대조", loadErr)
			return false
		}
		var wants []chargeWant
		for _, s := range subs {
			if !s.DueIn(ym) {
				continue
			}
			if p, ok := posts[s.ID][ym]; ok && p.TransactionID != nil {
				continue
			}
			wants = append(wants, newChargeWant(s, ym, cal))
		}
		got := map[int64]bool{}
		if len(wants) > 0 {
			txs, err := a.st.ListTransactions(wantsTxFilter(ym, wants))
			if err != nil {
				logIf("자금 계획 정기결제 대조", err)
				return false
			}
			for id, t := range assignCharges(wants, txs, linked) {
				if t != nil {
					got[id] = true
				}
			}
		}
		byMonth[ym] = got
		return got[sub.ID]
	}
}

// ---- 정기 출금 후보 ----

// ListRecurringCandidates 는 기록에서 찾은 정기 출금 후보 중 아직 정기결제로 등록되지 않은 것.
// 가맹점이 맞고 금액이 비슷한 정기결제가 있으면(이미 등록) 뺀다.
func (a *App) ListRecurringCandidates() ([]store.RecurringCandidate, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	cands, err := a.st.RecurringCandidates(time.Now())
	if err != nil {
		logIf("ListRecurringCandidates", err)
		return nil, err
	}
	subs, err := a.st.ListSubscriptions()
	if err != nil {
		return nil, err
	}
	out := []store.RecurringCandidate{}
	for _, c := range cands {
		if !candidateRegistered(c, subs) {
			out = append(out, c)
		}
	}
	return out, nil
}

// candidateRegistered 는 후보가 이미 등록된 정기결제와 같은지 본다:
// 기록에 쓰인 가맹점 이름 중 하나가 정기결제와 맞고, 금액이 후보 범위 ±10% 안.
func candidateRegistered(c store.RecurringCandidate, subs []store.Subscription) bool {
	for _, s := range subs {
		if float64(s.Amount) < float64(c.AmountMin)*0.9 || float64(s.Amount) > float64(c.AmountMax)*1.1 {
			continue
		}
		for _, m := range c.Merchants {
			if merchantMatch(s.MatchTarget(), m) {
				return true
			}
		}
	}
	return false
}

// DismissRecurringCandidate 는 후보를 '무시'해 다시 제안하지 않게 한다.
func (a *App) DismissRecurringCandidate(key string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DismissRecurring(key)
	logIf("DismissRecurringCandidate", err)
	return err
}

// SaveCashSettings 는 월급일·예상 월급·최소 잔액 설정을 저장한다.
func (a *App) SaveCashSettings(s store.CashSettings) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.SaveCashSettings(s)
	logIf("SaveCashSettings", err)
	return err
}

// AddCashBalance 는 통장 잔액을 기록한다(기준일 asOf, YYYY-MM-DD). 계획은 가장 최근 기록을 쓴다.
func (a *App) AddCashBalance(asOf string, amount int64, memo string) (store.CashBalance, error) {
	if err := a.ensure(); err != nil {
		return store.CashBalance{}, err
	}
	b, err := a.st.AddCashBalance(asOf, amount, memo)
	logIf("AddCashBalance", err)
	return b, err
}

// ListCashBalances 는 입력한 잔액 기록을 최근 것부터 돌려준다.
func (a *App) ListCashBalances() ([]store.CashBalance, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListCashBalances()
}

// DeleteCashBalance 는 잔액 기록 하나를 지운다.
func (a *App) DeleteCashBalance(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DeleteCashBalance(id)
	logIf("DeleteCashBalance", err)
	return err
}
