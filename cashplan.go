package main

import (
	"fmt"

	"sobi/internal/store"
)

// ---- 자금 계획 ----
// 월급(입금)과 카드대금(출금) 사이에 통장에 돈이 충분한지 보여 준다.

// GetCashPlan 은 자금 계획 화면 데이터(다음 결제일 카드값, 잔액 흐름, 안전 지출 가능액)를 돌려준다.
func (a *App) GetCashPlan() (store.CashPlan, error) {
	if err := a.ensure(); err != nil {
		return store.CashPlan{}, err
	}
	return store.CashPlan{}, fmt.Errorf("미구현")
}

// SaveCashSettings 는 월급일·예상 월급·최소 잔액 설정을 저장한다.
func (a *App) SaveCashSettings(s store.CashSettings) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return fmt.Errorf("미구현")
}

// AddCashBalance 는 통장 잔액을 기록한다(기준일 asOf, YYYY-MM-DD). 계획은 가장 최근 기록을 쓴다.
func (a *App) AddCashBalance(asOf string, amount int64, memo string) (store.CashBalance, error) {
	if err := a.ensure(); err != nil {
		return store.CashBalance{}, err
	}
	return store.CashBalance{}, fmt.Errorf("미구현")
}

// ListCashBalances 는 입력한 잔액 기록을 최근 것부터 돌려준다.
func (a *App) ListCashBalances() ([]store.CashBalance, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("미구현")
}

// DeleteCashBalance 는 잔액 기록 하나를 지운다.
func (a *App) DeleteCashBalance(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return fmt.Errorf("미구현")
}
