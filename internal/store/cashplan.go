package store

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"sobi/internal/holiday"
)

// 자금 계획: 월급과 카드대금 출금 사이에 통장 잔액이 충분한지 계산한다.
//
// 모든 계산은 today 를 인자로 받는다(테스트에서 날짜를 고정하기 위해). 날짜는 전부 UTC 자정으로 맞춘다.

const dateLayout = "2006-01-02"

// app_settings 키.
const (
	keyCashSalaryDay    = "cash.salary_day"
	keyCashSalaryAmount = "cash.salary_amount"
	keyCashReserve      = "cash.reserve"
)

// 자금 계획에서 쓰는 상수.
const (
	// billCloseGapDays: 이용기간이 끝난 뒤 결제일까지 최소로 두는 간격(일). 카드사는 대략 2주 전에 마감한다.
	billCloseGapDays = 10
	// spendAvgDays: 열린 이용기간의 남은 지출을 짐작할 때 보는 최근 일수.
	spendAvgDays = 90
	// minSpendAvgDays: 쓰기 시작한 지 얼마 안 된 카드라도 일평균을 낼 때 이보다 짧게 나누지 않는다.
	// 며칠치 기록에 큰 결제 한 건이 끼면 남은 기간 전체로 부풀려지기 때문이다.
	minSpendAvgDays = 30
	// minHorizonDays: 타임라인이 최소한 보여 줄 일수.
	minHorizonDays = 40
	// maxBalanceLookbackMonths: 오래된 잔액 기록에서 출발해도 청구서를 계산할 최대 과거 개월 수(쿼리 폭주 방지).
	maxBalanceLookbackMonths = 24
)

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func fmtDay(t time.Time) string { return t.Format(dateLayout) }

// monthStart 는 t 가 속한 달의 1일.
func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// clampDay 는 (y, m) 의 day 일. 말일보다 크면 말일로 줄인다.
func clampDay(y int, m time.Month, day int) time.Time {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day = last
	}
	if day < 1 {
		day = 1
	}
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// ---- 설정 ----

// GetCashSettings 는 자금 계획 설정을 읽는다. 없는 키는 기본값(월급일 10, 나머지 0).
func (s *Store) GetCashSettings() (CashSettings, error) {
	out := CashSettings{SalaryDay: 10}
	rows, err := s.query(`SELECT key, value FROM app_settings WHERE key IN (?,?,?)`,
		keyCashSalaryDay, keyCashSalaryAmount, keyCashReserve)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return out, err
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue // 깨진 값은 기본값을 쓴다
		}
		switch k {
		case keyCashSalaryDay:
			if n >= 1 && n <= 31 {
				out.SalaryDay = int(n)
			}
		case keyCashSalaryAmount:
			if n > 0 {
				out.SalaryAmount = n
			}
		case keyCashReserve:
			if n > 0 {
				out.Reserve = n
			}
		}
	}
	return out, rows.Err()
}

// SaveCashSettings 는 설정을 검증해 저장한다. 월급일은 1~31, 금액은 0 이상.
func (s *Store) SaveCashSettings(c CashSettings) error {
	if c.SalaryDay < 1 || c.SalaryDay > 31 {
		return fmt.Errorf("월급일은 1~31 사이여야 합니다")
	}
	if c.SalaryAmount < 0 || c.Reserve < 0 {
		return fmt.Errorf("금액은 0 이상이어야 합니다")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range map[string]int64{
		keyCashSalaryDay:    int64(c.SalaryDay),
		keyCashSalaryAmount: c.SalaryAmount,
		keyCashReserve:      c.Reserve,
	} {
		if _, err := tx.Exec(pq(`
INSERT INTO app_settings(key, value) VALUES(?,?)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`), k, strconv.FormatInt(v, 10)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- 잔액 기록 ----

// AddCashBalance 는 통장 잔액 기록을 추가한다. asOf 는 YYYY-MM-DD.
func (s *Store) AddCashBalance(asOf string, amount int64, memo string) (CashBalance, error) {
	d, err := time.Parse(dateLayout, asOf)
	if err != nil {
		return CashBalance{}, fmt.Errorf("기준일은 YYYY-MM-DD 형식이어야 합니다")
	}
	b := CashBalance{AsOf: fmtDay(d), Amount: amount, Memo: memo}
	err = s.queryRow(`INSERT INTO cash_balances(as_of, amount, memo) VALUES(?,?,?) RETURNING id`,
		b.AsOf, b.Amount, b.Memo).Scan(&b.ID)
	return b, err
}

// ListCashBalances 는 잔액 기록을 최근 것부터 돌려준다.
func (s *Store) ListCashBalances() ([]CashBalance, error) {
	rows, err := s.query(`SELECT id, as_of, amount, memo FROM cash_balances ORDER BY as_of DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CashBalance{}
	for rows.Next() {
		var b CashBalance
		if err := rows.Scan(&b.ID, &b.AsOf, &b.Amount, &b.Memo); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// DeleteCashBalance 는 잔액 기록 하나를 지운다.
func (s *Store) DeleteCashBalance(id int64) error {
	_, err := s.exec(`DELETE FROM cash_balances WHERE id=?`, id)
	return err
}

func (s *Store) latestCashBalance() (*CashBalance, error) {
	var b CashBalance
	err := s.queryRow(`SELECT id, as_of, amount, memo FROM cash_balances ORDER BY as_of DESC, id DESC LIMIT 1`).
		Scan(&b.ID, &b.AsOf, &b.Amount, &b.Memo)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ---- 순수 계산 도우미 (DB 없음) ----

// StatementStartDay 는 카드의 청구(이용)기간 시작일 S 를 정한다.
//  1. StatementStartDay > 0 이면 그 값
//  2. 아니면 CycleStartDay > 1 이면 그 값 (실적 시작일을 청구 시작일과 같게 쓰는 카드가 많다)
//  3. 아니면 결제일에서 유도한 기본값 S = ((billingDay - 14 + 31) % 31) + 1
//     — 결제일 약 2주 전에 이용기간이 끝난다고 보는 일반적인 카드사 규칙.
//     예: 결제일 1 → 19(전전월 19일~전월 18일), 결제일 25 → 12(전월 12일~당월 11일).
func StatementStartDay(pm PaymentMethod) int {
	if pm.StatementStartDay > 0 {
		return pm.StatementStartDay
	}
	if pm.CycleStartDay > 1 {
		return pm.CycleStartDay
	}
	return ((pm.BillingDay-14+31)%31 + 1)
}

// nominalDue 는 (y, m) 달의 약정 결제일. 결제일이 말일보다 크면 말일로 줄인다.
func nominalDue(y int, m time.Month, billingDay int) time.Time {
	return clampDay(y, m, billingDay)
}

// billPeriod 는 약정 결제일 nominal 에 빠지는 이용기간 [시작, 끝] 이다.
// 매월 S 일에 시작하는 이용 주기(perfPeriod 와 같은 방식) 중, 끝나는 날이 nominal−10일 이하인
// 가장 최근 주기. 예: 결제일 11/1, S=19 → 9/19~10/18.
func billPeriod(nominal time.Time, startDay int) (time.Time, time.Time) {
	limit := nominal.AddDate(0, 0, -billCloseGapDays)
	start, end := perfPeriod(limit, startDay)
	if end.After(limit) {
		start, end = perfPeriod(start.AddDate(0, 0, -1), startDay)
	}
	return start, end
}

// projectedBill 은 열린 이용기간의 예상 청구액: 지금까지 청구액 + 최근 일평균 지출 × 남은 일수.
func projectedBill(due, recentSum int64, avgDays, remainingDays int) int64 {
	if remainingDays <= 0 || avgDays <= 0 {
		return due
	}
	return due + recentSum*int64(remainingDays)/int64(avgDays)
}

// spendWindowDays 는 일평균 지출을 낼 때 나눌 일수.
// 기본은 최근 spendAvgDays 일. 카드를 처음 쓴 날(first)이 그보다 최근이면 첫 사용일부터 오늘까지로
// 줄인다 — 열흘 쓴 새 카드를 90일로 나누면 예상치가 1/9 로 작아진다. 다만 minSpendAvgDays 보다는
// 줄이지 않는다. first 가 영(사용 기록 없음)이면 기본값.
func spendWindowDays(today, first time.Time) int {
	if first.IsZero() {
		return spendAvgDays
	}
	days := int(today.Sub(first).Hours()/24) + 1 // 첫 사용일과 오늘을 모두 포함
	switch {
	case days > spendAvgDays:
		return spendAvgDays
	case days < minSpendAvgDays:
		return minSpendAvgDays
	}
	return days
}

// salaryDate 는 (y, m) 달의 월급 입금일: 월급일(말일 보정)이 휴일이면 직전 영업일.
func salaryDate(cal *holiday.Calendar, y int, m time.Month, day int) time.Time {
	return cal.PrevBusinessDay(clampDay(y, m, day))
}

// nextSalaryDate 는 today 이후(오늘 포함) 첫 입금일. 이번 달 입금일이 이미 지났으면 다음 달.
// received 가 true 이고 이번 달 입금일이 오늘이면 이미 받은 것이므로 다음 달.
func nextSalaryDate(cal *holiday.Calendar, today time.Time, day int, received bool) time.Time {
	t0 := salaryDate(cal, today.Year(), today.Month(), day)
	if t0.After(today) || (t0.Equal(today) && !received) {
		return t0
	}
	n := monthStart(today).AddDate(0, 1, 0)
	return salaryDate(cal, n.Year(), n.Month(), day)
}

// ---- 청구서 ----

// 청구서는 카드·달마다 따로 조회하지 않는다. 필요한 기간의 카드 지출을 (카드, 날짜)별 합계로 한 번에
// 읽어 두고(cardDaySums) 그 안에서 기간별로 더한다. 카드 4장 × 6달이면 조회 24번이 1번이 된다.
type billCtx struct {
	cal    *holiday.Calendar
	today  time.Time
	days   map[int64][]cardDay   // 카드 id → 날짜순 하루 합계
	first  map[int64]string      // 카드 id → 오늘까지 첫 지출일
	recent map[int64]recentSpend // 카드 id → 최근 지출
}

// cardDay 는 카드 하나의 하루 지출(교통카드 거래 제외).
type cardDay struct {
	Date    string
	Total   int64
	Prepaid int64 // 선결제한(paid_at 이 있는) 금액
	Count   int
}

// cardDaySums 는 from~to(양끝 포함) 카드 지출을 (카드, 날짜)별로 합친다.
func (s *Store) cardDaySums(from, to string) (map[int64][]cardDay, error) {
	rows, err := s.query(`
SELECT t.payment_method_id, t.date, COALESCE(SUM(t.amount),0),
       COALESCE(SUM(t.amount) FILTER (WHERE t.paid_at <> ''),0), COUNT(*)
FROM transactions t
JOIN payment_methods p ON p.id = t.payment_method_id AND p.type = 'card'
WHERE t.direction='expense' AND t.date >= ? AND t.date <= ? AND `+transitTxFilter+`
GROUP BY t.payment_method_id, t.date
ORDER BY t.payment_method_id, t.date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]cardDay{}
	for rows.Next() {
		var id int64
		var d cardDay
		if err := rows.Scan(&id, &d.Date, &d.Total, &d.Prepaid, &d.Count); err != nil {
			return nil, err
		}
		out[id] = append(out[id], d)
	}
	return out, rows.Err()
}

// cardFirstUse 는 카드별로 today 까지의 첫 지출일.
func (s *Store) cardFirstUse(today string) (map[int64]string, error) {
	rows, err := s.query(`
SELECT t.payment_method_id, MIN(t.date)
FROM transactions t
JOIN payment_methods p ON p.id = t.payment_method_id AND p.type = 'card'
WHERE t.direction='expense' AND t.date <= ? AND `+transitTxFilter+`
GROUP BY t.payment_method_id`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var d string
		if err := rows.Scan(&id, &d); err != nil {
			return nil, err
		}
		out[id] = d
	}
	return out, rows.Err()
}

// mergeCardDays 는 겹치지 않는 두 기간의 하루 합계를 합쳐 날짜순으로 둔다.
func mergeCardDays(a, b map[int64][]cardDay) map[int64][]cardDay {
	for id, ds := range b {
		a[id] = append(a[id], ds...)
		sort.Slice(a[id], func(i, j int) bool { return a[id][i].Date < a[id][j].Date })
	}
	return a
}

// sumCardDays 는 from~to(양끝 포함) 하루 합계를 더한다.
func sumCardDays(days []cardDay, from, to string) (total, prepaid int64, count int) {
	i := sort.Search(len(days), func(i int) bool { return days[i].Date >= from })
	for ; i < len(days) && days[i].Date <= to; i++ {
		total += days[i].Total
		prepaid += days[i].Prepaid
		count += days[i].Count
	}
	return
}

// recentSpend 는 예상 지출의 근거: 최근 Days 일(오늘 포함) 동안 청구로 넘어간(선결제하지 않은) 지출 합계.
// 선결제 캐시백 카드처럼 쓰자마자 갚는 카드는 앞으로의 사용분도 대부분 선결제될 것이므로,
// 남은 날 예상은 '선결제하지 않은 사용 속도'로 잡아야 실제 청구액에 가깝다.
type recentSpend struct {
	Sum  int64
	Days int
}

// recentSpend 는 미리 읽어 둔 하루 합계로 계산한다(최근 창은 오늘 앞 spendAvgDays 일 안이라 늘 들어 있다).
func (c *billCtx) recentSpend(cardID int64) recentSpend {
	if v, ok := c.recent[cardID]; ok {
		return v
	}
	var firstDay time.Time
	if f, ok := c.first[cardID]; ok {
		if d, err := time.Parse(dateLayout, f); err == nil {
			firstDay = d
		}
	}
	v := recentSpend{Days: spendWindowDays(c.today, firstDay)}
	from := c.today.AddDate(0, 0, -(v.Days - 1))
	total, prepaid, _ := sumCardDays(c.days[cardID], fmtDay(from), fmtDay(c.today))
	v.Sum = total - prepaid // 선결제하지 않은(청구로 넘어간) 지출
	c.recent[cardID] = v
	return v
}

// bill 은 카드 pm 의 (y, m) 달 약정 결제일 청구서를 계산한다.
func (c *billCtx) bill(pm PaymentMethod, y int, m time.Month) CardBill {
	start, end := cardBillPeriod(pm, y, m)
	nominal := nominalDue(y, m, pm.BillingDay)
	b := CardBill{
		CardID: pm.ID, CardName: pm.Name, Color: pm.Color,
		NominalDue:  fmtDay(nominal),
		DueDate:     fmtDay(c.cal.NextBusinessDay(nominal)),
		PeriodStart: fmtDay(start), PeriodEnd: fmtDay(end),
		Closed: c.today.After(end),
	}
	b.Total, b.Prepaid, b.TxCount = sumCardDays(c.days[pm.ID], b.PeriodStart, b.PeriodEnd)
	b.Due = b.Total - b.Prepaid
	b.Projected = b.Due
	// 해지된 카드는 앞으로 쓰지 않으므로 예상 지출을 더하지 않는다(해지 전 이용분만 청구).
	if !b.Closed && !pm.IsClosed(fmtDay(c.today)) {
		r := c.recentSpend(pm.ID)
		remaining := int(end.Sub(c.today).Hours() / 24) // 내일부터 이용기간 끝까지
		b.Projected = projectedBill(b.Due, r.Sum, r.Days, remaining)
		if b.Projected > b.Due {
			b.AvgDays, b.RemainDays = r.Days, remaining
		}
	}
	return b
}

// cardBillPeriod 는 카드 pm 의 (y, m) 달 결제일 청구서의 이용기간.
func cardBillPeriod(pm PaymentMethod, y int, m time.Month) (start, end time.Time) {
	return billPeriod(nominalDue(y, m, pm.BillingDay), StatementStartDay(pm))
}

// ---- 월급 ----

// salaryAverage 는 최근 3개월(완전한 달) 월평균 수입. 급여 카테고리(하위 포함) 우선, 없으면 전체 수입.
// 거래가 있는 달로만 나눈다(기록이 1~2개월뿐인 신규 사용자가 과소 추정되지 않게).
func (s *Store) salaryAverage(today time.Time) (avg int64, useCat bool, err error) {
	to := monthStart(today).AddDate(0, 0, -1)
	from := monthStart(today).AddDate(0, -3, 0)
	avgOf := func(query string) (int64, error) {
		rows, err := s.query(query, fmtDay(from), fmtDay(to))
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		var sum int64
		months := 0
		for rows.Next() {
			var ym string
			var amt int64
			if err := rows.Scan(&ym, &amt); err != nil {
				return 0, err
			}
			if amt > 0 {
				sum += amt
				months++
			}
		}
		if err := rows.Err(); err != nil {
			return 0, err
		}
		if months == 0 {
			return 0, nil
		}
		return sum / int64(months), nil
	}
	avg, err = avgOf(`
SELECT substr(t.date,1,7), SUM(t.amount)
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
WHERE t.direction='income' AND t.date >= ? AND t.date <= ? AND (c.name='급여' OR cp.name='급여')
GROUP BY 1`)
	if err != nil || avg > 0 {
		return avg, true, err
	}
	avg, err = avgOf(`
SELECT substr(date,1,7), SUM(amount) FROM transactions
WHERE direction='income' AND date >= ? AND date <= ? GROUP BY 1`)
	return avg, false, err
}

// salaryReceived 는 from(이번 달 입금일)~today 사이에 월급으로 볼 입금(≥ 월급의 50%)이 있었는지.
func (s *Store) salaryReceived(from, today time.Time, used int64, useCat bool) (bool, error) {
	if used <= 0 {
		return false, nil
	}
	q := `
SELECT COUNT(*) FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
WHERE t.direction='income' AND t.date >= ? AND t.date <= ? AND t.amount * 2 >= ?`
	if useCat {
		q += ` AND (c.name='급여' OR cp.name='급여')`
	}
	var n int
	err := s.queryRow(q, fmtDay(from), fmtDay(today), used).Scan(&n)
	return n > 0, err
}

// ---- 계획 ----

// SubMatchFunc 는 월급 계획에서 "이 정기결제의 이번 달 결제가 이미 거래로 입력됐는지"를 앱 계층이
// 알려 주는 훅이다(가맹점 대조 로직이 앱 계층·classifier 에 있어 store 가 직접 쓸 수 없다).
// from~to(YYYY-MM-DD, 양끝 포함) 사이에 이미 입력된 결제가 있으면 true. nil 이면 기록(subscription_posts)만 본다.
type SubMatchFunc func(sub Subscription, ym, from, to string) bool

// BuildCashPlan 은 today 기준 자금 계획을 만든다. duesBook 은 회비 장부(앱 계층이 납부 거래를 골라
// 계산한 것) — 다음 납부일마다 낼 돈(밀린 돈 포함)을 출금으로 넣는다. nil 이면 회비는 빠진다.
func (s *Store) BuildCashPlan(today time.Time, matched SubMatchFunc, duesBook []DuesStatus) (CashPlan, error) {
	return s.BuildCashPlanWith(today, matched, func() []DuesStatus { return duesBook })
}

// DuesLoader 는 회비 장부를 읽는 함수. 자금 계획의 다른 조회와 동시에 부른다(못 읽으면 nil — 회비만 빠진다).
type DuesLoader func() []DuesStatus

// BuildCashPlanWith 는 BuildCashPlan 과 같되, 회비 장부를 함수로 받아 다른 조회와 동시에 읽는다.
//
// DB 가 멀리 있어 조회 하나마다 왕복 시간이 붙으므로 두 단계로 한꺼번에 읽는다.
//   - 1단계: 서로 상관없는 것 전부(설정·달력·결제수단·잔액·월급 평균·카드 지출·정기결제·외화·바구니·예정 지출·회비)
//   - 2단계: 1단계 값이 있어야 하는 것(이번 달 월급을 받았는지, 잔액 기준일 뒤 통장 흐름, 더 이른 카드 지출)
func (s *Store) BuildCashPlanWith(today time.Time, matched SubMatchFunc, loadDues DuesLoader) (CashPlan, error) {
	today = dayOf(today)
	todayStr := fmtDay(today)
	plan := CashPlan{
		Today:          todayStr,
		NextDueBills:   []CardBill{},
		FollowingBills: []CardBill{},
		Events:         []CashEvent{},
	}

	var (
		settings   CashSettings
		cal        *holiday.Calendar
		pms        []PaymentMethod
		bal        *CashBalance
		auto       int64
		useCat     bool
		cardDays   map[int64][]cardDay
		firstUse   map[int64]string
		subs       []Subscription
		posts      map[int64]map[string]SubPost
		fxPlans    []FxPlan
		fxRates    fxRateBook
		basketBook []BasketStatus
		planned    []PlannedExpense
		duesBook   []DuesStatus
	)
	// 카드 지출은 최근 지출 창(오늘 앞 spendAvgDays 일)과 앞으로 결제할 청구서가 들어가게 넉넉히 읽는다.
	// 잔액 기준일이 오래돼 더 이른 청구서가 필요하면 2단계에서 모자란 앞부분만 더 읽는다.
	daysFrom := monthStart(today).AddDate(0, -3, 0)
	daysTo := monthStart(today).AddDate(0, 5, 0)
	err := parallel(
		func() (err error) { settings, err = s.GetCashSettings(); return },
		func() (err error) { cal, err = s.Calendar(); return },
		func() (err error) { pms, err = s.ListPaymentMethods(); return },
		func() (err error) { bal, err = s.latestCashBalance(); return },
		func() (err error) { auto, useCat, err = s.salaryAverage(today); return },
		func() (err error) { cardDays, err = s.cardDaySums(fmtDay(daysFrom), fmtDay(daysTo)); return },
		func() (err error) { firstUse, err = s.cardFirstUse(todayStr); return },
		func() (err error) { subs, err = s.ListSubscriptions(); return },
		func() (err error) { posts, err = s.SubscriptionPosts(); return },
		func() (err error) { fxPlans, err = s.ListFxPlans(); return },
		func() (err error) { fxRates, err = s.fxRateBook(); return },
		func() (err error) { basketBook, err = s.BasketBook(today); return },
		func() (err error) { planned, err = s.ListPlannedExpenses(today, 0); return },
		func() error {
			if loadDues != nil {
				duesBook = loadDues()
			}
			return nil
		},
	)
	if err != nil {
		return plan, err
	}
	plan.Settings = settings
	plan.Balance = bal

	// 월급
	plan.SalaryAuto = auto
	plan.SalaryUsed = auto
	switch {
	case settings.SalaryAmount > 0:
		plan.SalaryUsed = settings.SalaryAmount
		plan.SalaryBasis = "설정값"
	case auto > 0 && useCat:
		plan.SalaryBasis = "최근 3개월 급여 평균"
	case auto > 0:
		plan.SalaryBasis = "최근 3개월 수입 평균"
	default:
		plan.SalaryBasis = "설정값"
	}
	thisSalary := salaryDate(cal, today.Year(), today.Month(), settings.SalaryDay)

	// 청구서: 잔액 기준일 이전 달부터 오늘+3개월 뒤까지
	fromMonth := monthStart(today).AddDate(0, -1, 0)
	if bal != nil {
		if d, err := time.Parse(dateLayout, bal.AsOf); err == nil {
			if m := monthStart(d).AddDate(0, -1, 0); m.Before(fromMonth) {
				fromMonth = m
			}
		}
	}
	if floor := monthStart(today).AddDate(0, -maxBalanceLookbackMonths, 0); fromMonth.Before(floor) {
		fromMonth = floor
	}
	toMonth := monthStart(today).AddDate(0, 3, 0)
	var cards []PaymentMethod
	needFrom, needTo := daysFrom, daysTo // 청구서 이용기간 전체를 덮는 범위
	for _, pm := range pms {
		if pm.Type != "card" || pm.BillingDay <= 0 {
			continue
		}
		cards = append(cards, pm)
		if start, _ := cardBillPeriod(pm, fromMonth.Year(), fromMonth.Month()); start.Before(needFrom) {
			needFrom = start
		}
		if _, end := cardBillPeriod(pm, toMonth.Year(), toMonth.Month()); end.After(needTo) {
			needTo = end
		}
	}

	// 2단계
	var (
		received         bool
		flows            balanceFlows
		fxNet, basketNet int64
		earlier, later   map[int64][]cardDay
	)
	var phase2 []func() error
	if !thisSalary.After(today) {
		phase2 = append(phase2, func() (err error) {
			received, err = s.salaryReceived(thisSalary, today, plan.SalaryUsed, useCat)
			return
		})
	}
	if bal != nil {
		asOf := bal.AsOf
		phase2 = append(phase2,
			func() (err error) { flows, err = s.balanceFlows(asOf, today); return },
			func() (err error) { fxNet, err = s.fxNetSpent(asOf, todayStr); return },
			func() (err error) { basketNet, err = s.basketMainFlow(asOf, todayStr); return },
		)
	}
	if needFrom.Before(daysFrom) {
		phase2 = append(phase2, func() (err error) {
			earlier, err = s.cardDaySums(fmtDay(needFrom), fmtDay(daysFrom.AddDate(0, 0, -1)))
			return
		})
	}
	if needTo.After(daysTo) {
		phase2 = append(phase2, func() (err error) {
			later, err = s.cardDaySums(fmtDay(daysTo.AddDate(0, 0, 1)), fmtDay(needTo))
			return
		})
	}
	if err := parallel(phase2...); err != nil {
		return plan, err
	}
	cardDays = mergeCardDays(mergeCardDays(cardDays, earlier), later)
	nextSalary := nextSalaryDate(cal, today, settings.SalaryDay, received)
	plan.NextSalary = fmtDay(nextSalary)

	bc := &billCtx{cal: cal, today: today, days: cardDays, first: firstUse, recent: map[int64]recentSpend{}}
	var bills []CardBill
	for _, pm := range cards {
		for m := fromMonth; !m.After(toMonth); m = m.AddDate(0, 1, 0) {
			b := bc.bill(pm, m.Year(), m.Month())
			if b.Total == 0 && pm.IsClosed(todayStr) {
				continue // 해지된 카드의 빈 청구서는 보여 주지 않는다
			}
			bills = append(bills, b)
		}
	}

	// 잔액
	if bal != nil {
		adj := flows.adjust(bal.AsOf, today, bills)
		// 외화를 산 원화도 통장에서 빠졌다(판 원화는 들어왔다)
		adj -= fxNet
		plan.FxAdjust = -fxNet
		// 목돈 바구니로 채운 돈은 빠졌고, 바구니에서 옮겨 온 돈은 들어왔다
		adj -= basketNet
		plan.BasketAdjust = -basketNet
		plan.BalanceAdjust = adj
		plan.BalanceNow = bal.Amount + adj
	}

	// 다음/그다음 결제일
	dueSet := map[string]bool{}
	for _, b := range bills {
		if b.DueDate >= todayStr {
			dueSet[b.DueDate] = true
		}
	}
	dues := make([]string, 0, len(dueSet))
	for d := range dueSet {
		dues = append(dues, d)
	}
	sort.Strings(dues)
	if len(dues) > 0 {
		plan.NextDue = dues[0]
	}
	if len(dues) > 1 {
		plan.FollowingDue = dues[1]
	}
	for _, b := range bills {
		switch {
		case plan.NextDue != "" && b.DueDate == plan.NextDue:
			plan.NextDueBills = append(plan.NextDueBills, b)
			plan.NextDueTotal += b.Projected // 닫혔으면 Projected == Due
		case plan.FollowingDue != "" && b.DueDate == plan.FollowingDue:
			plan.FollowingBills = append(plan.FollowingBills, b)
			plan.FollowingTotal += b.Due
			plan.FollowingProjected += b.Projected
		}
	}

	// 타임라인
	horizon := today.AddDate(0, 0, minHorizonDays)
	if d := nextSalary.AddDate(0, 0, 1); d.After(horizon) {
		horizon = d
	}
	if plan.FollowingDue != "" {
		if d, err := time.Parse(dateLayout, plan.FollowingDue); err == nil && d.After(horizon) {
			horizon = d
		}
	}
	plan.Horizon = fmtDay(horizon)
	horizonStr := plan.Horizon

	var events []CashEvent
	for _, b := range bills {
		if b.DueDate < todayStr || b.DueDate > horizonStr || b.Projected == 0 {
			continue
		}
		events = append(events, CashEvent{
			Date: b.DueDate, Kind: "card", Label: b.CardName,
			Amount: -b.Projected, Estimated: !b.Closed, RefID: b.CardID,
		})
	}
	if plan.SalaryUsed > 0 {
		for m := monthStart(today); !m.After(horizon); m = m.AddDate(0, 1, 0) {
			d := salaryDate(cal, m.Year(), m.Month(), settings.SalaryDay)
			if d.Before(today) || d.After(horizon) || (d.Equal(today) && received) {
				continue
			}
			events = append(events, CashEvent{
				Date: fmtDay(d), Kind: "salary", Label: "월급",
				Amount: plan.SalaryUsed, Estimated: settings.SalaryAmount == 0,
			})
		}
	}
	events = append(events, subscriptionEvents(subs, posts, pms, cal, today, horizon, matched)...)
	events = append(events, duesEvents(duesBook, pms, cal, today, horizon)...)
	// 외화 매일 모으기로 앞으로 빠질 원화
	events = append(events, fxCashEvents(fxPlans, fxRates, today, horizon)...)
	// 목돈 바구니로 앞으로 빠질 채움
	events = append(events, basketEvents(basketBook, cal, today, horizon)...)
	// 생활비 통장에서 낼 예정 지출(축의금 등). 지났는데 아직 연결 안 한 건 오늘로 당겨 남긴다
	events = append(events, plannedEvents(planned, today, horizon)...)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Date != events[j].Date {
			return events[i].Date < events[j].Date
		}
		return events[i].Amount > 0 && events[j].Amount <= 0 // 같은 날은 입금이 먼저
	})

	running := plan.BalanceNow
	plan.LowestBalance, plan.LowestDate = running, todayStr
	plan.AfterNextDue = running
	safe := running // 다음 월급 전까지의 최저 잔액
	for i := range events {
		running += events[i].Amount
		events[i].Balance = running
		if running < plan.LowestBalance {
			plan.LowestBalance, plan.LowestDate = running, events[i].Date
		}
		if plan.NextDue != "" && events[i].Date <= plan.NextDue {
			plan.AfterNextDue = running
		}
		if events[i].Date < plan.NextSalary && running < safe {
			safe = running
		}
	}
	if events != nil {
		plan.Events = events
	}

	// 카드값 통장: 다음 월급날 옮길 금액(그 월급으로 낼 카드값)과 지금 보관 중이어야 할 금액
	afterNext := monthStart(nextSalary).AddDate(0, 1, 0)
	salaryAfter := salaryDate(cal, afterNext.Year(), afterNext.Month(), settings.SalaryDay)
	plan.SalaryTransfer = buildTransfer(bills, plan.NextSalary, fmtDay(salaryAfter))
	plan.Holding = buildTransfer(bills, todayStr, plan.NextSalary)

	plan.SafeToSpend = safe - settings.Reserve
	plan.DaysToSalary = int(nextSalary.Sub(today).Hours() / 24)
	spendable := plan.SafeToSpend
	if spendable < 0 {
		spendable = 0
	}
	days := plan.DaysToSalary
	if days < 1 {
		days = 1
	}
	plan.DailyAllowance = spendable / int64(days)
	return plan, nil
}

// transferBufferRate: 월급날 옮길 금액에서 '남은 기간 예상분'에 더하는 여유 비율.
// 확정된(이미 쓴) 금액은 그대로 두고, 추정한 부분만 넉넉히 잡는다.
const transferBufferRate = 0.1

// buildTransfer 는 출금일이 [from, until) 인 카드 청구서를 모아 카드값 통장에 둘 금액을 계산한다.
//   - Confirmed: 지금까지 이용한 금액(선결제 뺌) — 청구서 Due 의 합
//   - Estimated: 이용기간이 끝나지 않은 카드의 남은 날 예상분 — Projected − Due 의 합
//   - Suggested: Confirmed + Estimated × (1 + transferBufferRate) 를 만 원 단위로 올린 권장 이체액
//   - OpenUntil: 아직 안 끝난 이용기간 중 가장 늦은 마감일 — 그날이 지나면 확정액으로 차이를 맞춘다
func buildTransfer(bills []CardBill, from, until string) CardTransfer {
	t := CardTransfer{From: from, Until: until, Bills: []CardBill{}}
	for _, b := range bills {
		if b.DueDate < from || b.DueDate >= until || b.Projected <= 0 {
			continue
		}
		t.Bills = append(t.Bills, b)
		t.Confirmed += b.Due
		t.Estimated += b.Projected - b.Due
		if !b.Closed && b.PeriodEnd > t.OpenUntil {
			t.OpenUntil = b.PeriodEnd
		}
	}
	sort.SliceStable(t.Bills, func(i, j int) bool {
		if t.Bills[i].DueDate != t.Bills[j].DueDate {
			return t.Bills[i].DueDate < t.Bills[j].DueDate
		}
		return t.Bills[i].CardName < t.Bills[j].CardName
	})
	if len(t.Bills) > 0 {
		raw := t.Confirmed + t.Estimated + int64(math.Ceil(float64(t.Estimated)*transferBufferRate))
		t.Suggested = (raw + 9999) / 10000 * 10000
	}
	return t
}

// balanceAdjust 는 잔액 기준일(asOf) 이후 오늘까지 통장에서 실제로 늘고 준 돈.
//   - 수입은 더하고, 계좌·현금(또는 결제수단 없음)으로 나간 지출·이체는 뺀다.
//   - 이미 출금된 카드대금(실제 출금일이 asOf 초과, 오늘 미만)은 뺀다. 오늘 출금분은 이벤트로 본다.
//   - 카드 지출 중 이미 갚은(paid_at 이 asOf 초과, 오늘 이하) 것은 갚은 날 통장에서 빠졌으므로 뺀다.
func (s *Store) balanceAdjust(asOf string, today time.Time, bills []CardBill) (int64, error) {
	f, err := s.balanceFlows(asOf, today)
	if err != nil {
		return 0, err
	}
	return f.adjust(asOf, today, bills), nil
}

// balanceFlows 는 잔액 기준일(asOf) 다음 날부터 오늘까지 통장을 거친 돈(카드대금 출금은 빼고).
type balanceFlows struct {
	income  int64 // 수입
	outflow int64 // 계좌·현금(또는 결제수단 없음)으로 나간 지출·이체
	prepaid int64 // 카드 지출 중 그 사이에 미리 갚은 것
}

func (s *Store) balanceFlows(asOf string, today time.Time) (balanceFlows, error) {
	todayStr := fmtDay(today)
	var f balanceFlows
	err := parallel(
		func() error {
			return s.queryRow(`
SELECT COALESCE(SUM(t.amount) FILTER (WHERE t.direction='income'), 0),
       COALESCE(SUM(t.amount) FILTER (WHERE t.direction IN ('expense','transfer') AND (p.id IS NULL OR p.type IN ('cash','bank'))), 0)
FROM transactions t
LEFT JOIN payment_methods p ON p.id = t.payment_method_id
WHERE t.date > ? AND t.date <= ? AND `+transitTxFilter+` AND `+basketTxFilter, asOf, todayStr).Scan(&f.income, &f.outflow)
		},
		func() error {
			return s.queryRow(`
SELECT COALESCE(SUM(t.amount), 0)
FROM transactions t
JOIN payment_methods p ON p.id = t.payment_method_id
WHERE t.direction='expense' AND p.type='card' AND t.paid_at > ? AND t.paid_at <= ? AND `+transitTxFilter, asOf, todayStr).Scan(&f.prepaid)
		},
	)
	return f, err
}

// adjust 는 흐름에서 그 사이 이미 출금된 카드대금(실제 출금일이 asOf 초과, 오늘 미만)까지 뺀 잔액 변화.
func (f balanceFlows) adjust(asOf string, today time.Time, bills []CardBill) int64 {
	todayStr := fmtDay(today)
	var billed int64
	for _, b := range bills {
		if b.DueDate > asOf && b.DueDate < todayStr {
			billed += b.Due
		}
	}
	return f.income - f.outflow - billed - f.prepaid
}

// subscriptionEvents 는 카드가 아닌 결제수단(없음/현금/계좌)으로 나가는 정기결제의 예정 출금.
// 카드 정기결제는 카드 청구서에 들어가므로 따로 보여 주지 않는다.
func subscriptionEvents(subs []Subscription, posts map[int64]map[string]SubPost, pms []PaymentMethod,
	cal *holiday.Calendar, today, horizon time.Time, matched SubMatchFunc) []CashEvent {
	pmType := map[int64]string{}
	for _, p := range pms {
		pmType[p.ID] = p.Type
	}
	todayStr, horizonStr := fmtDay(today), fmtDay(horizon)
	var out []CashEvent
	for _, sub := range subs {
		if !sub.Active {
			continue
		}
		if sub.PaymentMethodID != nil && pmType[*sub.PaymentMethodID] == "card" {
			continue
		}
		// 지난달 결제일이 밀려 이번 달로 넘어온 경우까지 보려고 한 달 앞에서 시작한다.
		// 다음 달 결제일이 이전 영업일로 당겨져 이번 달로 넘어오는 경우까지 보려고 한 달 뒤까지 간다.
		for m := monthStart(today).AddDate(0, -1, 0); !m.After(horizon.AddDate(0, 1, 0)); m = m.AddDate(0, 1, 0) {
			ym := m.Format("2006-01")
			if !sub.DueIn(ym) {
				continue
			}
			if _, done := posts[sub.ID][ym]; done {
				continue
			}
			_, actual, err := sub.ChargeDate(ym, cal)
			if err != nil {
				continue
			}
			d := fmtDay(actual)
			if d < todayStr || d > horizonStr {
				continue
			}
			if matched != nil {
				from, to := fmtDay(m), fmtDay(m.AddDate(0, 1, -1))
				if d > to {
					to = d
				}
				if d < from {
					from = d // 이전 영업일로 당겨져 전달로 넘어온 결제
				}
				if matched(sub, ym, from, to) {
					continue
				}
			}
			out = append(out, CashEvent{
				Date: d, Kind: "subscription", Label: sub.Name,
				Amount: -sub.AmountAt(ym), RefID: sub.ID,
			})
		}
	}
	return out
}
