package main

import (
	"log"
	"time"

	"sobi/internal/store"
)

// ---- 교통카드(K-패스) ----
//
// 계산·저장은 store(transit.go). 여기서는 출금일이 지난 달을 어림값으로 기록하고(시간마다·화면을 열 때),
// 실제 금액을 받아 고치고, 거래내역에 교통카드 표시를 붙인다.

// runTransit 은 출금일이 지났는데 기록이 없는 달을 어림값('추정')으로 기록한다.
func (a *App) runTransit(now time.Time) error {
	n, err := a.st.RunTransit(now)
	if n > 0 {
		log.Printf("교통카드 자동 기록 %d건", n)
	}
	return err
}

// ListTransit 은 교통카드(K-패스) 화면: 월 기록 · 다음 출금 어림값. 지난 출금일의 기록부터 채운다.
func (a *App) ListTransit() ([]store.TransitStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	now := time.Now()
	logIf("교통카드 자동 기록", a.runTransit(now))
	out, err := a.st.TransitBook(now)
	logIf("ListTransit", err)
	return out, err
}

// SaveTransitMonth 는 ym 달 실제 요금·환급을 넣는다(추정 기록을 고치거나, 지난 달을 새로 기록).
func (a *App) SaveTransitMonth(subID int64, ym string, fare, refund int64) (store.TransitMonth, error) {
	if err := a.ensure(); err != nil {
		return store.TransitMonth{}, err
	}
	out, err := a.st.SaveTransitMonth(subID, ym, fare, refund)
	logIf("SaveTransitMonth", err)
	return out, err
}

// annotateTransit 은 교통카드 월 기록 거래에 교통카드 이름을 붙인다(거래내역에서 정기결제 버튼 대신 표시).
func (a *App) annotateTransit(txs []store.Transaction) {
	if len(txs) == 0 {
		return
	}
	names, err := a.st.TransitTxNames()
	if err != nil {
		logIf("거래 목록(교통카드 표시)", err)
		return
	}
	for i := range txs {
		if n, ok := names[txs[i].ID]; ok {
			txs[i].TransitName = n
		}
	}
}
