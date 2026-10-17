package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"sobi/internal/classifier"
	"sobi/internal/holiday"
	"sobi/internal/store"
)

// ---- 정기결제 관리 ----
//
// 등록해 둔 정기결제를 실제 거래와 대조해 "이번 달에 빠졌는지 / 얼마가 빠졌는지"를 본다.
// 자동 등록을 켠 정기결제는 결제일에 거래를 만들어 준다(autopost.go). 같은 달 결제를
// 이미 손으로 입력했으면 새로 만들지 않고 연결만 해서 두 번 잡히지 않게 한다.

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
	// PlannedDate 는 원래 결제일, ChargeDate 는 주말·공휴일을 피해 민 실제 결제일.
	// ShiftReason 은 밀린 까닭("토요일", "추석" …) — 안 밀렸으면 "".
	PlannedDate string `json:"plannedDate"`
	ChargeDate  string `json:"chargeDate"`
	ShiftReason string `json:"shiftReason"`
	// PostState 는 자동 등록 상태: "" (아직) | "created" (자동으로 만듦) |
	// "matched" (이미 입력된 거래에 연결) | "deleted" (만들었으나 사용자가 지움 — 다시 만들지 않음).
	PostState string `json:"postState"`
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

// merchantMatch 는 정기결제 대조 가맹점과 거래 가맹점이 같은 곳인지 본다.
// 정규화한 뒤 한쪽이 다른 쪽을 포함하면 같다고 본다("LG U+" ↔ "LG U+ 자동결제").
func merchantMatch(target, merchant string) bool {
	nt, nm := classifier.Normalize(target), classifier.Normalize(merchant)
	if nt == "" || nm == "" {
		return false
	}
	return strings.Contains(nm, nt) || strings.Contains(nt, nm)
}

// chargeWant 는 "이 정기결제의 이 달 결제를 찾는다"는 요청 하나.
type chargeWant struct {
	Sub      store.Subscription
	Amount   int64  // 그 달 예상 요금
	From, To string // 허용 날짜 범위("YYYY-MM-DD", 양끝 포함). 빈 값이면 제한 없음
}

// assignCharges 는 정기결제(wants)와 실제 거래(txs)를 짝짓는다.
//
// 가맹점이 맞는 (정기결제, 거래) 쌍을 모두 모아 금액 차이가 작은 순으로 훑고, 정기결제도
// 거래도 한 번씩만 쓴다. 그래서
//   - 같은 가맹점 정기결제가 하나면 후보도 그것뿐이라 금액이 달라도 찾는다(요금이 오른 달).
//   - 같은 가맹점이 여럿이면(LG U+ 회선 셋) 금액이 가장 가까운 것끼리 묶여 뒤섞이지 않고,
//     거래가 모자라면 금액이 먼 정기결제는 짝이 없다(엉뚱한 회선의 요금을 가져가지 않는다).
// exclude 의 거래는 이미 다른 정기결제 달에 묶여 있어 후보에서 뺀다.
// 결과는 정기결제 id → 짝지은 거래.
func assignCharges(wants []chargeWant, txs []store.Transaction, exclude map[int64]bool) map[int64]*store.Transaction {
	type pair struct {
		w, t int
		diff int64
	}
	var pairs []pair
	for wi, w := range wants {
		target := w.Sub.MatchTarget()
		for ti, t := range txs {
			if exclude[t.ID] || t.Direction != "expense" {
				continue
			}
			if (w.From != "" && t.Date < w.From) || (w.To != "" && t.Date > w.To) {
				continue
			}
			if !merchantMatch(target, t.Merchant) {
				continue
			}
			d := t.Amount - w.Amount
			if d < 0 {
				d = -d
			}
			pairs = append(pairs, pair{wi, ti, d})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].diff != pairs[j].diff {
			return pairs[i].diff < pairs[j].diff
		}
		if txs[pairs[i].t].Date != txs[pairs[j].t].Date {
			return txs[pairs[i].t].Date < txs[pairs[j].t].Date
		}
		return wants[pairs[i].w].Sub.ID < wants[pairs[j].w].Sub.ID
	})
	out := map[int64]*store.Transaction{}
	usedTx := map[int]bool{}
	for _, p := range pairs {
		sid := wants[p.w].Sub.ID
		if _, done := out[sid]; done || usedTx[p.t] {
			continue
		}
		out[sid] = &txs[p.t]
		usedTx[p.t] = true
	}
	return out
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
	// 자동 등록을 켠 채 저장했으면 이미 결제일이 지난 이번 달 몫을 바로 반영한다
	// (다음 주기까지 기다리면 저장하고 나서 거래가 안 보여 헷갈린다).
	if err == nil && saved.AutoPost {
		if _, perr := a.postDue(time.Now()); perr != nil {
			logIf("SaveSubscription(자동 등록)", perr)
		}
	}
	return saved, err
}

// ---- 공휴일 ----

// HolidayYear 는 설정 화면의 공휴일 목록 한 해치.
type HolidayYear struct {
	Year    int                `json:"year"`
	Covered bool               `json:"covered"` // 음력·대체공휴일까지 내장 표에 있는지
	Entries []holiday.Entry    `json:"entries"`
	Custom  []holiday.Override `json:"custom"` // 그 해의 사용자 정정
}

// GetHolidays 는 그 해 공휴일(내장 표 + 사용자 정정)을 돌려준다.
func (a *App) GetHolidays(year int) (HolidayYear, error) {
	if err := a.ensure(); err != nil {
		return HolidayYear{}, err
	}
	over, err := a.st.ListCustomHolidays()
	if err != nil {
		return HolidayYear{}, err
	}
	out := HolidayYear{Year: year, Covered: holiday.Covered(year), Custom: []holiday.Override{}}
	out.Entries = holiday.New(over).Year(year)
	prefix := fmt.Sprintf("%04d-", year)
	for _, o := range over {
		if strings.HasPrefix(o.Date, prefix) {
			out.Custom = append(out.Custom, o)
		}
	}
	return out, nil
}

// SaveCustomHoliday 는 공휴일을 추가하거나(임시공휴일 등) 내장 공휴일을 업무일로 정정한다.
func (a *App) SaveCustomHoliday(o holiday.Override) error {
	if err := a.ensure(); err != nil {
		return err
	}
	if _, err := time.Parse("2006-01-02", o.Date); err != nil {
		return fmt.Errorf("날짜 형식이 올바르지 않습니다: %q", o.Date)
	}
	return a.st.SaveCustomHoliday(o)
}

// DeleteCustomHoliday 는 정정을 지워 그 날을 내장 표대로 되돌린다.
func (a *App) DeleteCustomHoliday(date string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.st.DeleteCustomHoliday(date)
}

// SubDraft 는 거래 한 건으로 만든 정기결제 초안과, 이미 같은 가맹점으로 걸려 있는 정기결제 목록.
type SubDraft struct {
	Draft store.Subscription `json:"draft"`
	// Duplicates: 같은 가맹점에 금액도 비슷해 같은 결제가 두 번 걸린 것으로 보이는 정기결제.
	Duplicates []store.Subscription `json:"duplicates"`
	// Siblings: 같은 가맹점이지만 금액이 달라 별개(다른 회선·상품)로 구분되는 정기결제.
	Siblings []store.Subscription `json:"siblings"`
}

// DraftSubscriptionFromTx 는 거래내역의 한 건을 정기결제로 등록할 수 있게 초안을 채운다.
// 그 거래가 있는 달은 이미 결제가 끝났으므로 다음 달부터 추적하도록 시작 월을 잡는다.
// 저장은 하지 않는다 — 사용자가 확인·수정한 뒤 SaveSubscription 으로 저장한다.
// 중복 판정은 실제 대조(assignCharges)와 같은 기준이다: 가맹점이 맞고 금액이 가까우면 중복,
// 금액이 다르면 같은 가맹점의 별개 결제(LG U+ 회선 여럿 등)로 본다.
func (a *App) DraftSubscriptionFromTx(id int64) (SubDraft, error) {
	if err := a.ensure(); err != nil {
		return SubDraft{}, err
	}
	t, err := a.st.GetTransaction(id)
	if err != nil {
		return SubDraft{}, err
	}
	if t.Direction != "expense" {
		return SubDraft{}, fmt.Errorf("지출 거래만 정기결제로 등록할 수 있습니다")
	}
	d, err := time.Parse("2006-01-02", t.Date)
	if err != nil {
		return SubDraft{}, fmt.Errorf("거래 날짜를 읽을 수 없습니다: %s", t.Date)
	}
	next := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)

	draft := store.Subscription{
		Name:            strings.TrimSpace(t.Merchant),
		Merchant:        strings.TrimSpace(t.Merchant),
		Amount:          t.Amount,
		Cycle:           "monthly",
		BillingDay:      d.Day(),
		BillingMonth:    int(d.Month()),
		StartYM:         next.Format("2006-01"),
		CategoryID:      t.CategoryID,
		MemberID:        t.MemberID,
		PaymentMethodID: t.PaymentMethodID,
		Memo:            fmt.Sprintf("%s 거래에서 등록", t.Date),
		Active:          true,
		// 거래에서 만드는 건 "다음 달부터 매달 자동으로 넣어 달라"는 뜻이라 기본으로 켠다
		AutoPost: true,
		// 확인 화면 표시용 이름 (저장에는 쓰이지 않음)
		CategoryName:      t.CategoryName,
		MemberName:        t.MemberName,
		PaymentMethodName: t.PaymentMethodName,
	}

	out := SubDraft{Draft: draft, Duplicates: []store.Subscription{}, Siblings: []store.Subscription{}}
	subs, err := a.st.ListSubscriptions()
	if err != nil {
		return out, err
	}
	// 같은 가맹점 정기결제를 금액으로 나눈다. 대조가 금액이 가까운 것끼리 짝지으므로
	// 금액이 비슷하면 같은 결제가 두 번 걸린 것(중복), 다르면 다른 회선·상품(별개)이다.
	for _, s := range subs {
		if !merchantMatch(s.MatchTarget(), t.Merchant) {
			continue
		}
		if amountClose(s.Amount, t.Amount) {
			out.Duplicates = append(out.Duplicates, s)
		} else {
			out.Siblings = append(out.Siblings, s)
		}
	}
	return out, nil
}

// amountClose 는 두 요금이 같은 결제로 볼 만큼 가까운지(±3%).
// 넓게 잡으면 요금이 비슷한 다른 회선(44,800 / 46,800)까지 "중복"으로 경고한다.
func amountClose(a, b int64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	d := a - b
	if d < 0 {
		d = -d
	}
	m := a
	if b > m {
		m = b
	}
	return d*100 <= 3*m
}

// LinkSubscriptionTx 는 거래내역의 거래를 정기결제의 그 달 결제로 연결한다.
// 거래에서 정기결제를 만든 직후 원본 거래를 묶어, 그 행에 정기결제 표시가 붙게 한다.
func (a *App) LinkSubscriptionTx(subID, txID int64) (bool, error) {
	if err := a.ensure(); err != nil {
		return false, err
	}
	ok, err := a.st.LinkSubscriptionTx(subID, txID)
	logIf("LinkSubscriptionTx", err)
	return ok, err
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
	cal, err := a.st.Calendar()
	if err != nil {
		return SubMonth{}, err
	}
	posts, err := a.st.SubscriptionPosts()
	if err != nil {
		return SubMonth{}, err
	}
	// 자동 등록된 달은 기록에 남은 그 거래로 "빠짐"을 판정한다. 결제일이 다음 업무일로
	// 밀려 다음 달로 넘어간 경우, 그 달 거래만 훑는 대조로는 못 찾기 때문이다.
	var postedIDs []int64
	for _, sub := range subs {
		if p, ok := posts[sub.ID][ym]; ok && p.TransactionID != nil {
			postedIDs = append(postedIDs, *p.TransactionID)
		}
	}
	postedTx := map[int64]store.Transaction{}
	if len(postedIDs) > 0 {
		list, err := a.st.GetTransactions(postedIDs)
		if err != nil {
			return SubMonth{}, err
		}
		for _, t := range list {
			postedTx[t.ID] = t
		}
	}

	// 기록이 없는 달은 실제 거래와 짝짓는다. 다른 정기결제 달에 이미 묶인 거래는 빼고,
	// 같은 가맹점 정기결제가 여럿이면 금액이 가까운 것끼리 묶는다(assignCharges).
	linkedTx := map[int64]bool{}
	for _, byYM := range posts {
		for _, p := range byYM {
			if p.TransactionID != nil {
				linkedTx[*p.TransactionID] = true
			}
		}
	}
	var wants []chargeWant
	for _, sub := range subs {
		if !sub.DueIn(ym) {
			continue
		}
		if p, ok := posts[sub.ID][ym]; ok && p.TransactionID != nil {
			continue
		}
		wants = append(wants, chargeWant{Sub: sub, Amount: sub.AmountAt(ym)})
	}
	assigned := assignCharges(wants, txs, linkedTx)

	out := SubMonth{Month: ym, Items: []SubStatus{}}
	for _, sub := range subs {
		st := SubStatus{
			Sub:    sub,
			Amount: sub.AmountAt(ym),
			Due:    sub.DueIn(ym),
			Ending: sub.EndsIn(ym),
		}
		if st.Due {
			if planned, actual, err := chargeDate(sub, ym, cal); err == nil {
				st.PlannedDate = planned.Format("2006-01-02")
				st.ChargeDate = actual.Format("2006-01-02")
				if !actual.Equal(planned) {
					st.ShiftReason = cal.Reason(planned)
				}
			}
		}
		if p, ok := posts[sub.ID][ym]; ok {
			switch {
			case p.TransactionID == nil:
				st.PostState = "deleted"
			case p.Matched:
				st.PostState = "matched"
			default:
				st.PostState = "created"
			}
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
			var t *store.Transaction
			if p, ok := posts[sub.ID][ym]; ok && p.TransactionID != nil {
				if pt, ok := postedTx[*p.TransactionID]; ok {
					t = &pt
				}
			}
			if t == nil {
				t = assigned[sub.ID]
			}
			if t != nil {
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
		var wants []chargeWant
		for _, sub := range subs {
			if sub.DueIn(ym) {
				wants = append(wants, chargeWant{Sub: sub, Amount: sub.AmountAt(ym)})
			}
		}
		var sum int64
		for _, t := range assignCharges(wants, txs, nil) {
			sum += t.Amount
		}
		trend = append(trend, sum)
		months = append(months, ym)
	}
	return trend, months, nil
}
