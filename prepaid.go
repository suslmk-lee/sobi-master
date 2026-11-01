package main

import (
	"fmt"

	"sobi/internal/store"
)

// ---- 선불권 (미리 충전하고 쓸 때마다 차감) ----
// 자세한 기준은 internal/store/prepaid.go 머리말 참고.

// ListPrepaidStatuses 는 선불권 목록과 잔액·절약액·소진 예상.
func (a *App) ListPrepaidStatuses() ([]store.PrepaidStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("미구현")
}

// SavePrepaidPass 는 선불권 등록/수정 (ID 0 이면 새로).
func (a *App) SavePrepaidPass(p store.PrepaidPass) (store.PrepaidPass, error) {
	if err := a.ensure(); err != nil {
		return p, err
	}
	return p, fmt.Errorf("미구현")
}

// DeletePrepaidPass 는 선불권을 지운다. 연결된 충전 거래는 지출로 그대로 남고 연결만 끊긴다.
func (a *App) DeletePrepaidPass(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return fmt.Errorf("미구현")
}

// AddPrepaidCharge 는 충전을 기록한다: 지출 거래를 새로 만들고(선불권의 기본 카테고리·결제수단, 가맹점 = Merchant)
// 선불권에 연결한다. paymentMethodID/memberID 가 0 이면 선불권 기본값. 적립액은 할인 방식으로 계산한다.
func (a *App) AddPrepaidCharge(passID int64, date string, amount int64, paymentMethodID, memberID int64, memo string) (store.PrepaidCharge, error) {
	if err := a.ensure(); err != nil {
		return store.PrepaidCharge{}, err
	}
	return store.PrepaidCharge{}, fmt.Errorf("미구현")
}

// LinkPrepaidCharge 는 이미 있는 지출 거래를 이 선불권의 충전으로 연결한다(적립액은 할인 방식으로 계산).
func (a *App) LinkPrepaidCharge(passID, transactionID int64) (store.PrepaidCharge, error) {
	if err := a.ensure(); err != nil {
		return store.PrepaidCharge{}, err
	}
	return store.PrepaidCharge{}, fmt.Errorf("미구현")
}

// UnlinkPrepaidCharge 는 충전 연결을 끊는다(거래는 지출로 그대로 남는다).
func (a *App) UnlinkPrepaidCharge(transactionID int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return fmt.Errorf("미구현")
}

// SuggestPrepaidCharges 는 충전으로 연결할 만한 지출 거래 후보(가맹점이 맞고 아직 연결 안 된 것, 최근순).
func (a *App) SuggestPrepaidCharges(passID int64) ([]store.Transaction, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("미구현")
}

// SavePrepaidUse 는 사용(차감) 기록 등록/수정 (ID 0 이면 새로).
func (a *App) SavePrepaidUse(u store.PrepaidUse) (store.PrepaidUse, error) {
	if err := a.ensure(); err != nil {
		return u, err
	}
	return u, fmt.Errorf("미구현")
}

// DeletePrepaidUse 는 사용 기록 하나를 지운다.
func (a *App) DeletePrepaidUse(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return fmt.Errorf("미구현")
}

// GetPrepaidHistory 는 선불권 한 장의 충전·사용 내역.
func (a *App) GetPrepaidHistory(passID int64) (store.PrepaidHistory, error) {
	if err := a.ensure(); err != nil {
		return store.PrepaidHistory{}, err
	}
	return store.PrepaidHistory{}, fmt.Errorf("미구현")
}
