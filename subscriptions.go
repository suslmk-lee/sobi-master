package main

import (
	"fmt"
	"strings"
	"time"

	"sobi/internal/classifier"
	"sobi/internal/store"
)

// ---- 정기결제 관리 ----
//
// 등록해 둔 정기결제를 실제 거래와 대조해 "이번 달에 빠졌는지 / 얼마가 빠졌는지"를 본다.
// 거래를 자동으로 만들지는 않는다 — 카드 내역을 가져오면 진짜 거래가 들어오기 때문에
// 자동 생성하면 같은 결제가 두 번 잡힌다.

// SubStatus 는 정기결제 한 건의 특정 월 현황.
type SubStatus struct {
	Sub store.Subscription `json:"sub"`
	// Amount 는 그 달에 적용되는 요금(예약 변경 반영).
	Amount int64 `json:"amount"`
	// Due 는 그 달에 빠질 예정인지(꺼짐·기간 밖·연결제 비결제월이면 false).
	Due bool `json:"due"`
	// Seen 은 대조되는 실제 거래를 찾았는지. 찾았으면 금액·날짜를 채운다.
	Seen       bool   `json:"seen"`
	SeenAmount int64  `json:"seenAmount"`
	SeenDate   string `json:"seenDate"`
	// Diff 는 실제 − 예상 (빠졌을 때만 의미 있음).
	Diff int64 `json:"diff"`
	// Ending 은 이번 달이 마지막 결제 달인지.
	Ending bool `json:"ending"`
	// ChangingYM/ChangingAmount 는 앞으로 예정된 요금 변경(없으면 빈 값).
	ChangingYM     string `json:"changingYm"`
	ChangingAmount int64  `json:"changingAmount"`
}

// SubMonth 는 정기결제 화면 한 달치 전체.
type SubMonth struct {
	Month string      `json:"month"`
	Items []SubStatus `json:"items"`
	// Planned 는 그 달에 빠질 예정 금액 합, Seen 은 실제로 빠진 금액 합.
	Planned   int64 `json:"planned"`
	SeenTotal int64 `json:"seenTotal"`
	DueCount  int   `json:"dueCount"`
	SeenCount int   `json:"seenCount"`
	// MonthlyEquivalent 는 연 결제까지 12로 나눠 더한 "월 환산" 총액.
	MonthlyEquivalent int64 `json:"monthlyEquivalent"`
	// YearlyProjection 은 월 환산 × 12 (연간 예상 지출).
	YearlyProjection int64 `json:"yearlyProjection"`
	// Trend 는 최근 6개월 실제 정기결제 지출, TrendMonths 는 그 라벨.
	Trend       []int64  `json:"trend"`
	TrendMonths []string `json:"trendMonths"`
}

// ymOf 는 (year, month) 를 "YYYY-MM" 로.
func ymOf(year, month int) string { return fmt.Sprintf("%04d-%02d", year, month) }

// subMatcher 는 정기결제를 실제 거래와 대조한다. 가맹점명을 정규화해 부분 일치로 본다
// (금액은 보지 않는다 — 요금이 바뀌어도 같은 결제로 봐야 하기 때문).
type subMatcher struct {
	norm []string            // norm[i] 는 txs[i].Merchant 의 정규화 결과
	txs  []store.Transaction // 대조 대상 거래(지출만)
}

func newSubMatcher(txs []store.Transaction) *subMatcher {
	m := &subMatcher{txs: txs, norm: make([]string, len(txs))}
	for i, t := range txs {
		m.norm[i] = classifier.Normalize(t.Merchant)
	}
	return m
}

// find 는 target 과 맞는 거래 중 가장 이른 것을 돌려준다. 없으면 nil.
func (m *subMatcher) find(target string) *store.Transaction {
	nt := classifier.Normalize(target)
	if nt == "" {
		return nil
	}
	var best *store.Transaction
	for i := range m.txs {
		nm := m.norm[i]
		if nm == "" {
			continue
		}
		if !strings.Contains(nm, nt) && !strings.Contains(nt, nm) {
			continue
		}
		if best == nil || m.txs[i].Date < best.Date {
			best = &m.txs[i]
		}
	}
	return best
}

// ListSubscriptions 는 등록된 정기결제 전체.
func (a *App) ListSubscriptions() ([]store.Subscription, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListSubscriptions()
}

// SaveSubscription 은 정기결제를 추가(ID=0)하거나 수정한다.
func (a *App) SaveSubscription(sub store.Subscription) (store.Subscription, error) {
	if err := a.ensure(); err != nil {
		return sub, err
	}
	saved, err := a.st.SaveSubscription(sub)
	logIf("SaveSubscription", err)
	return saved, err
}

func (a *App) DeleteSubscription(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DeleteSubscription(id)
	logIf("DeleteSubscription", err)
	return err
}

// subTrendMonths 는 추이 차트에 쓸 개월 수.
const subTrendMonths = 6

// GetSubscriptionMonth 는 해당 월의 정기결제 현황 전체를 한 번에 모아 돌려준다.
func (a *App) GetSubscriptionMonth(year, month int) (SubMonth, error) {
	if err := a.ensure(); err != nil {
		return SubMonth{}, err
	}
	ym := ymOf(year, month)
	subs, err := a.st.ListSubscriptions()
	if err != nil {
		return SubMonth{}, err
	}

	// 그 달 지출 거래를 한 번만 읽어 대조에 재사용한다.
	txs, err := a.st.ListTransactions(store.TxFilter{Month: ym, Direction: "expense"})
	if err != nil {
		return SubMonth{}, err
	}
	matcher := newSubMatcher(txs)

	out := SubMonth{Month: ym, Items: []SubStatus{}}
	for _, sub := range subs {
		st := SubStatus{
			Sub:    sub,
			Amount: sub.AmountAt(ym),
			Due:    sub.DueIn(ym),
			Ending: sub.EndsIn(ym),
		}
		// 아직 오지 않은 요금 변경만 "예정"으로 알린다.
		if sub.NextAmountYM != "" && sub.NextAmountYM > ym {
			st.ChangingYM = sub.NextAmountYM
			st.ChangingAmount = sub.NextAmount
		}
		if st.Due {
			out.DueCount++
			out.Planned += st.Amount
			out.MonthlyEquivalent += sub.MonthlyEquivalent(ym)
			if t := matcher.find(sub.MatchTarget()); t != nil {
				st.Seen = true
				st.SeenAmount = t.Amount
				st.SeenDate = t.Date
				st.Diff = t.Amount - st.Amount
				out.SeenCount++
				out.SeenTotal += t.Amount
			}
		}
		out.Items = append(out.Items, st)
	}
	out.YearlyProjection = out.MonthlyEquivalent * 12

	// 최근 subTrendMonths 개월의 "실제로 빠진" 정기결제 지출 추이.
	trend, months, err := a.subTrend(subs, year, month, subTrendMonths)
	if err != nil {
		return out, err
	}
	out.Trend, out.TrendMonths = trend, months
	return out, nil
}

// subTrend 는 (year, month) 까지 n개월 동안 정기결제로 대조된 실제 지출 합계.
func (a *App) subTrend(subs []store.Subscription, year, month, n int) ([]int64, []string, error) {
	base := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	trend := make([]int64, 0, n)
	months := make([]string, 0, n)
	for i := n - 1; i >= 0; i-- {
		d := base.AddDate(0, -i, 0)
		ym := ymOf(d.Year(), int(d.Month()))
		txs, err := a.st.ListTransactions(store.TxFilter{Month: ym, Direction: "expense"})
		if err != nil {
			return nil, nil, err
		}
		matcher := newSubMatcher(txs)
		var sum int64
		for _, sub := range subs {
			if !sub.DueIn(ym) {
				continue
			}
			if t := matcher.find(sub.MatchTarget()); t != nil {
				sum += t.Amount
			}
		}
		trend = append(trend, sum)
		months = append(months, ym)
	}
	return trend, months, nil
}
