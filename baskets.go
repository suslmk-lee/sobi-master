package main

import (
	"log"
	"time"

	"sobi/internal/store"
)

// ---- 목돈 바구니 ----
//
// 계산·저장은 store(baskets.go). 여기서는 자동이체 채움을 기록하고(시간마다·화면을 열 때), 화면 요청을 받고,
// 거래내역에 바구니 표시를 붙인다.

// runBaskets 는 자동이체를 켠 바구니의 지난 채우는 날을 기록한다.
func (a *App) runBaskets(now time.Time) error {
	n, err := a.st.RunBaskets(now)
	if n > 0 {
		log.Printf("바구니 자동 채움 %d건", n)
	}
	return err
}

// ListBaskets 는 바구니 화면: 잔액 · 장부 · 마감 · 권장 월 적립액. 지난 자동 채움부터 기록한다.
func (a *App) ListBaskets() ([]store.BasketStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	now := time.Now()
	logIf("바구니 자동 채움", a.runBaskets(now))
	out, err := a.st.BasketBook(now)
	logIf("ListBaskets", err)
	return out, err
}

// PreviewBasket 은 저장 전에 화면 자료를 계산한다(만들거나 고칠 때 권장 월 적립액을 바로 보여 주려고).
// opening 은 새로 만들 때 지금 통장에 들어 있는 돈. 고칠 때는 지금까지의 장부를 그대로 쓴다.
func (a *App) PreviewBasket(b store.Basket, opening int64) (store.BasketStatus, error) {
	if err := a.ensure(); err != nil {
		return store.BasketStatus{}, err
	}
	cal, err := a.st.Calendar()
	if err != nil {
		return store.BasketStatus{}, err
	}
	var entries []store.BasketEntry
	if b.ID > 0 {
		if entries, err = a.st.BasketEntries(b.ID); err != nil {
			return store.BasketStatus{}, err
		}
	} else if opening > 0 {
		entries = []store.BasketEntry{{Kind: store.BasketOpen, Amount: opening, Date: time.Now().Format("2006-01-02")}}
	}
	st := store.BuildBasketStatus(b, entries, time.Now(), cal)
	if b.Kind == store.BasketReserve && len(b.CategoryIDs) > 0 {
		st.SuggestMonthly, st.SuggestCount, err = a.st.SuggestBasketMonthly(b.CategoryIDs, time.Now())
	}
	return st, err
}

// ListBasketCandidates 는 바구니에 연결할 만한 최근 큰 지출(이은 카테고리 · 정해진 지출 이름).
func (a *App) ListBasketCandidates() ([]store.BasketCandidate, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	out, err := a.st.BasketCandidates(time.Now())
	logIf("ListBasketCandidates", err)
	return out, err
}

// DismissBasketCandidate 는 후보를 '바구니 지출 아님'으로 지운다.
func (a *App) DismissBasketCandidate(txID int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DismissBasketCandidate(txID)
	logIf("DismissBasketCandidate", err)
	return err
}

// SaveBasket 은 바구니를 추가하거나 고친다(opening 은 새로 만들 때만 쓴다).
func (a *App) SaveBasket(b store.Basket, opening int64) (store.Basket, error) {
	if err := a.ensure(); err != nil {
		return b, err
	}
	out, err := a.st.SaveBasket(b, opening)
	logIf("SaveBasket", err)
	if err == nil {
		logIf("바구니 자동 채움(저장 후)", a.runBaskets(time.Now()))
	}
	return out, err
}

func (a *App) DeleteBasket(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DeleteBasket(id)
	logIf("DeleteBasket", err)
	return err
}

// AddBasketEntry 는 더 넣기 · 꺼내 쓰기(지출 거래 연결) · 잔액 맞춤을 기록한다.
func (a *App) AddBasketEntry(e store.BasketEntry) (store.BasketEntry, error) {
	if err := a.ensure(); err != nil {
		return e, err
	}
	out, err := a.st.AddBasketEntry(e)
	logIf("AddBasketEntry", err)
	return out, err
}

func (a *App) DeleteBasketEntry(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DeleteBasketEntry(id)
	logIf("DeleteBasketEntry", err)
	return err
}

// annotateBaskets 는 바구니에서 낸 지출 거래에 바구니 이름을 붙인다.
func (a *App) annotateBaskets(txs []store.Transaction) {
	if len(txs) == 0 {
		return
	}
	names, err := a.st.BasketTxNames()
	if err != nil {
		logIf("거래 목록(바구니 표시)", err)
		return
	}
	for i := range txs {
		if n, ok := names[txs[i].ID]; ok {
			txs[i].BasketName = n
		}
	}
}
