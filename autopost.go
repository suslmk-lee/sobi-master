package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"sobi/internal/holiday"
	"sobi/internal/store"
)

// ---- 정기결제 자동 등록 ----
//
// 자동 등록을 켠 정기결제는 결제일에 거래가 저절로 생긴다. 결제일이 주말·공휴일이면
// 실제 카드·계좌 결제처럼 다음 업무일로 밀린다.
//
// 지켜야 할 것:
//   - 같은 달은 정확히 한 번. (정기결제, 월) 기록을 DB 가 기본키로 막는다.
//   - 이미 손으로 입력한 결제가 있으면 새로 만들지 않고 그 거래에 연결만 한다.
//   - 자동 등록을 켠 달(AutoSince) 이전으로는 소급하지 않는다.
//   - 결제일이 아직 안 온 달은 만들지 않는다(미래 거래를 미리 만들지 않음).
//
// 데스크톱 앱이라 서버 스케줄러가 없다. 시작 직후와 실행 중 한 시간마다 "밀린 달"을
// 따라잡는 방식이라, 며칠 앱을 안 켰어도 다음에 켜면 빠진 달이 채워진다.

// autoPostInterval 은 앱이 켜져 있는 동안 자동 등록을 다시 확인하는 주기.
// 자정을 넘겨 켜 둔 경우에도 그날 결제일 거래가 생기게 한다.
const autoPostInterval = time.Hour

// autoPostMaxMonths 는 한 정기결제에서 한 번에 따라잡는 최대 개월 수(폭주 방지).
const autoPostMaxMonths = 60

// autoPostMu 는 주기 실행과 수동 실행이 겹치지 않게 한다.
// (겹쳐도 DB 제약 때문에 중복은 안 생기지만, 결과 집계가 엇갈리지 않게 직렬화한다)
var autoPostMu sync.Mutex

// AutoPostItem 은 자동 등록 한 건의 결과.
type AutoPostItem struct {
	SubscriptionID int64  `json:"subscriptionId"`
	Name           string `json:"name"`
	YM             string `json:"ym"`
	PlannedDate    string `json:"plannedDate"` // 원래 결제일
	Date           string `json:"date"`        // 실제 등록 날짜(업무일로 민 뒤)
	Reason         string `json:"reason"`      // 밀린 까닭("토요일", "추석" …). 안 밀렸으면 ""
	Amount         int64  `json:"amount"`
	Matched        bool   `json:"matched"` // 새로 만들지 않고 이미 있던 거래에 연결함
}

// AutoPostResult 는 자동 등록 한 번 돌린 결과.
type AutoPostResult struct {
	Created  int            `json:"created"`
	Matched  int            `json:"matched"`
	Items    []AutoPostItem `json:"items"`
	Warnings []string       `json:"warnings"`
}

// chargeDate 는 정기결제의 ym 달 결제일. planned 는 원래 날짜(말일보다 큰 결제일은 말일로
// 당김), actual 은 주말·공휴일이면 다음 업무일로 민 날짜. 밀다 보면 다음 달로 넘어갈 수 있다.
func chargeDate(sub store.Subscription, ym string, cal *holiday.Calendar) (planned, actual time.Time, err error) {
	first, err := time.Parse("2006-01", ym)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	last := first.AddDate(0, 1, -1).Day()
	day := sub.BillingDay
	if day < 1 {
		day = 1
	}
	if day > last {
		day = last
	}
	planned = time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)
	return planned, cal.NextBusinessDay(planned), nil
}

// nextYM 은 "YYYY-MM" 의 다음 달.
func nextYM(ym string) string {
	t, err := time.Parse("2006-01", ym)
	if err != nil {
		return "9999-12"
	}
	return t.AddDate(0, 1, 0).Format("2006-01")
}

// PostDueSubscriptions 는 결제일이 지났는데 아직 등록되지 않은 정기결제 거래를 지금 만든다.
// 정기결제 화면의 "지금 반영" 버튼과 백그라운드 주기가 함께 쓴다.
func (a *App) PostDueSubscriptions() (AutoPostResult, error) {
	if err := a.ensure(); err != nil {
		return AutoPostResult{}, err
	}
	return a.postDue(time.Now())
}

// postDue 는 today 기준으로 밀린 정기결제를 따라잡는다(테스트에서 날짜를 바꿔 끼운다).
func (a *App) postDue(today time.Time) (AutoPostResult, error) {
	autoPostMu.Lock()
	defer autoPostMu.Unlock()

	out := AutoPostResult{Items: []AutoPostItem{}, Warnings: []string{}}
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	curYM := today.Format("2006-01")

	subs, err := a.st.ListSubscriptions()
	if err != nil {
		return out, err
	}
	var auto []store.Subscription
	for _, s := range subs {
		if s.AutoPost && s.Active && s.AutoSince != "" {
			auto = append(auto, s)
		}
	}
	if len(auto) == 0 {
		return out, nil
	}

	cal, err := a.st.Calendar()
	if err != nil {
		return out, err
	}
	posts, err := a.st.SubscriptionPosts()
	if err != nil {
		return out, err
	}
	// 이미 어떤 정기결제 달에 연결된 거래는 다시 연결하지 않는다
	// (손으로 입력한 거래 하나가 두 달·두 정기결제를 동시에 채우면 안 된다).
	linked := map[int64]bool{}
	for _, byYM := range posts {
		for _, p := range byYM {
			if p.TransactionID != nil {
				linked[*p.TransactionID] = true
			}
		}
	}
	warnedYear := map[int]bool{}

	for _, sub := range auto {
		ym := sub.AutoSince
		for i := 0; ym <= curYM && i < autoPostMaxMonths; i, ym = i+1, nextYM(ym) {
			if !sub.DueIn(ym) {
				continue
			}
			if _, done := posts[sub.ID][ym]; done {
				continue
			}
			planned, actual, err := chargeDate(sub, ym, cal)
			if err != nil {
				return out, err
			}
			if actual.After(today) {
				continue // 결제일이 아직 안 옴
			}
			if y := actual.Year(); !holiday.Covered(y) && !warnedYear[y] {
				warnedYear[y] = true
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"%d년 공휴일 표가 없어 음력 공휴일·대체공휴일은 반영하지 못했습니다. 설정 → 공휴일에서 추가하세요.", y))
			}

			item := AutoPostItem{
				SubscriptionID: sub.ID, Name: sub.Name, YM: ym,
				PlannedDate: planned.Format("2006-01-02"),
				Date:        actual.Format("2006-01-02"),
				Amount:      sub.AmountAt(ym),
			}
			if !actual.Equal(planned) {
				item.Reason = cal.Reason(planned)
			}

			// 이미 손으로 입력해 둔 그 달 결제가 있으면 연결만 한다.
			match, err := a.findManualCharge(sub, ym, actual, linked)
			if err != nil {
				return out, err
			}
			t := store.Transaction{
				Date: item.Date, Amount: item.Amount, Direction: "expense",
				Merchant: sub.MatchTarget(), Memo: "정기결제 자동 등록 · " + sub.Name,
				CategoryID: sub.CategoryID, MemberID: sub.MemberID, PaymentMethodID: sub.PaymentMethodID,
			}
			var matchID int64
			if match != nil {
				matchID = match.ID
			}
			posted, txID, err := a.st.PostSubscription(sub.ID, ym, t, matchID)
			if err != nil {
				return out, err
			}
			if !posted {
				continue // 다른 PC 가 먼저 처리함
			}
			linked[txID] = true
			if match != nil {
				item.Matched = true
				item.Date = match.Date
				item.Amount = match.Amount
				out.Matched++
			} else {
				out.Created++
			}
			out.Items = append(out.Items, item)
		}
	}
	if out.Created > 0 || out.Matched > 0 {
		log.Printf("정기결제 자동 등록: 새로 %d건, 기존 거래 연결 %d건", out.Created, out.Matched)
	}
	return out, nil
}

// findManualCharge 는 ym 달 결제로 볼 수 있는, 이미 입력된 거래를 찾는다.
// 기간은 그 달 1일부터 말일까지 — 결제일이 다음 업무일로 밀려 다음 달로 넘어갔으면 그 날까지.
// 대조 규칙은 정기결제 화면의 "빠짐/예정" 판정과 같다(가맹점 정규화 후 부분 일치, 금액 무관).
func (a *App) findManualCharge(sub store.Subscription, ym string, actual time.Time, linked map[int64]bool) (*store.Transaction, error) {
	first, _ := time.Parse("2006-01", ym)
	to := first.AddDate(0, 1, -1)
	if actual.After(to) {
		to = actual
	}
	txs, err := a.st.ListTransactions(store.TxFilter{
		From: first.Format("2006-01-02"), To: to.Format("2006-01-02"), Direction: "expense",
	})
	if err != nil {
		return nil, err
	}
	free := txs[:0]
	for _, t := range txs {
		if !linked[t.ID] {
			free = append(free, t)
		}
	}
	return newSubMatcher(free).find(sub.MatchTarget()), nil
}

// startAutoPostLoop 은 시작 직후 한 번, 이후 autoPostInterval 마다 자동 등록을 돌린다.
func (a *App) startAutoPostLoop() {
	a.autoPostDone = make(chan struct{})
	go func() {
		select {
		case <-time.After(3 * time.Second): // 시작 화면을 막지 않게 잠깐 뒤에
		case <-a.autoPostDone:
			return
		}
		run := func(when string) {
			if a.ensure() != nil {
				return // 연결 실패는 화면에 따로 뜬다
			}
			if _, err := a.postDue(time.Now()); err != nil {
				logIf("정기결제 자동 등록("+when+")", err)
			}
		}
		run("시작")
		t := time.NewTicker(autoPostInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				run("주기")
			case <-a.autoPostDone:
				return
			}
		}
	}()
}
