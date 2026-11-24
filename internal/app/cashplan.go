package app

import (
	"sync"
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
	now := time.Now()
	// 정기결제 대조는 계획이 쓸 달(지난달~석 달 뒤)을 미리, 계획의 다른 조회와 동시에 계산해 둔다.
	m := a.newSubChargeMatcher()
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		var months []string
		for i := -1; i <= 3; i++ {
			months = append(months, start.AddDate(0, i, 0).Format("2006-01"))
		}
		m.prefetch(months)
	}()
	match := func(sub store.Subscription, ym, from, to string) bool {
		<-ready
		return m.match(sub, ym)
	}
	// 회비는 장부로 본다: 밀린 돈과 다음 납부일 몫을 출금으로 넣는다. 못 읽어도 계획은 보여 준다.
	loadDues := func() []store.DuesStatus {
		dues, err := a.duesBook(now)
		logIf("GetCashPlan(회비)", err)
		return dues
	}
	plan, err := a.st.BuildCashPlanWith(now, match, loadDues)
	<-ready // 대조 계산이 연결을 쓰는 중에 돌아가지 않게
	logIf("GetCashPlan", err)
	return plan, err
}

// subChargeMatcher 는 "정기결제의 그 달 결제가 이미 (손으로) 입력된 거래로 있는지" 판정기.
// 정기결제 화면(GetSubscriptionMonth)과 같은 짝짓기(assignCharges)를 쓴다 — 가맹점 이름만 보면
// 이름이 같거나 섞인 두 대출 중 한 건이 빠지는 순간 다른 건까지 '빠짐'으로 보여 일정에서 사라지므로,
// 금액이 가까운 것끼리 한 번씩만 묶는다. 같은 달은 한 번만 계산해 둔다.
type subChargeMatcher struct {
	a       *App
	mu      sync.Mutex
	loaded  bool
	loadErr error
	subs    []store.Subscription
	cal     *holiday.Calendar
	posts   map[int64]map[string]store.SubPost
	linked  map[int64]bool
	byMonth map[string]map[int64]bool
}

func (a *App) newSubChargeMatcher() *subChargeMatcher {
	return &subChargeMatcher{a: a, linked: map[int64]bool{}, byMonth: map[string]map[int64]bool{}}
}

// load 는 정기결제·달력·처리 기록을 동시에 읽는다(한 번만). mu 를 잡고 부른다.
func (m *subChargeMatcher) load() error {
	if m.loaded {
		return m.loadErr
	}
	m.loaded = true
	st := m.a.st
	m.loadErr = parallel(
		func() (err error) { m.subs, err = st.ListSubscriptions(); return },
		func() (err error) { m.cal, err = st.Calendar(); return },
		func() (err error) { m.posts, err = st.SubscriptionPosts(); return },
	)
	if m.loadErr != nil {
		return m.loadErr
	}
	for _, byYM := range m.posts {
		for _, p := range byYM {
			if p.TransactionID != nil {
				m.linked[*p.TransactionID] = true
			}
		}
	}
	return nil
}

// wants 는 ym 달에 아직 처리 기록이 없는 정기결제의 결제 예정. mu 를 잡고 부른다.
func (m *subChargeMatcher) wants(ym string) []chargeWant {
	var out []chargeWant
	for _, s := range m.subs {
		if !s.DueIn(ym) {
			continue
		}
		if p, ok := m.posts[s.ID][ym]; ok && p.TransactionID != nil {
			continue
		}
		out = append(out, newChargeWant(s, ym, m.cal))
	}
	return out
}

// monthResult 는 ym 달의 짝짓기 결과(정기결제 id → 거래가 있음). 거래 조회는 잠금 밖에서 한다.
func (m *subChargeMatcher) monthResult(ym string, wants []chargeWant, linked map[int64]bool) (map[int64]bool, error) {
	got := map[int64]bool{}
	if len(wants) == 0 {
		return got, nil
	}
	txs, err := m.a.st.ListTransactions(wantsTxFilter(ym, wants))
	if err != nil {
		return nil, err
	}
	for id, t := range assignCharges(wants, txs, linked) {
		if t != nil {
			got[id] = true
		}
	}
	return got, nil
}

// prefetch 는 여러 달의 짝짓기를 동시에 계산해 둔다. 실패한 달은 비워 두어 match 가 다시 계산하게 한다.
func (m *subChargeMatcher) prefetch(months []string) {
	m.mu.Lock()
	if err := m.load(); err != nil {
		m.mu.Unlock()
		return
	}
	type job struct {
		ym    string
		wants []chargeWant
	}
	var jobs []job
	for _, ym := range months {
		if _, ok := m.byMonth[ym]; !ok {
			jobs = append(jobs, job{ym, m.wants(ym)})
		}
	}
	linked := m.linked // 읽기만 한다
	m.mu.Unlock()

	results := make([]map[int64]bool, len(jobs))
	fns := make([]func() error, len(jobs))
	for i, j := range jobs {
		fns[i] = func() (err error) {
			results[i], err = m.monthResult(j.ym, j.wants, linked)
			if err != nil {
				logIf("자금 계획 정기결제 대조", err)
			}
			return nil
		}
	}
	parallel(fns...)
	m.mu.Lock()
	for i, j := range jobs {
		if results[i] != nil {
			m.byMonth[j.ym] = results[i]
		}
	}
	m.mu.Unlock()
}

// match 는 sub 의 ym 달 결제가 이미 거래로 있는지. 미리 계산한 달이 아니면 그 자리에서 계산한다.
func (m *subChargeMatcher) match(sub store.Subscription, ym string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if got, ok := m.byMonth[ym]; ok {
		return got[sub.ID]
	}
	if err := m.load(); err != nil {
		logIf("자금 계획 정기결제 대조", err)
		return false
	}
	got, err := m.monthResult(ym, m.wants(ym), m.linked)
	if err != nil {
		logIf("자금 계획 정기결제 대조", err)
		return false
	}
	m.byMonth[ym] = got
	return got[sub.ID]
}

// ---- 정기 출금 후보 ----

// ListRecurringCandidates 는 기록에서 찾은 정기 출금 후보 중 아직 정기결제로 등록되지 않은 것.
// 가맹점이 맞고 금액이 비슷한 정기결제가 있으면(이미 등록) 뺀다.
func (a *App) ListRecurringCandidates() ([]store.RecurringCandidate, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	var (
		cands []store.RecurringCandidate
		subs  []store.Subscription
	)
	err := parallel(
		func() (err error) { cands, err = a.st.RecurringCandidates(time.Now()); return },
		func() (err error) { subs, err = a.st.ListSubscriptions(); return },
	)
	if err != nil {
		logIf("ListRecurringCandidates", err)
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
