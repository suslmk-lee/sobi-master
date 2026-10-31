package main

import (
	"time"

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
	plan, err := a.st.BuildCashPlan(time.Now(), a.subChargeEntered)
	logIf("GetCashPlan", err)
	return plan, err
}

// subChargeEntered 는 정기결제의 그 달 결제가 이미 (손으로) 입력된 거래로 있는지 본다.
// 정기결제 화면의 "빠짐" 판정과 같은 가맹점 대조(merchantMatch)를 쓴다.
func (a *App) subChargeEntered(sub store.Subscription, ym, from, to string) bool {
	txs, err := a.st.ListTransactions(store.TxFilter{From: from, To: to, Direction: "expense"})
	if err != nil {
		logIf("자금 계획 정기결제 대조", err)
		return false
	}
	target := sub.MatchTarget()
	for _, t := range txs {
		if merchantMatch(target, t.Merchant) {
			return true
		}
	}
	return false
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
