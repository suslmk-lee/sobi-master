package store

// 기록에서 정기 출금 후보 찾기.
//
// 대출 상환·관리비처럼 매달 정해진 날 빠지는 돈이 정기결제로 등록돼 있지 않으면 자금 계획이
// 앞으로 빠질 돈을 모른다. 최근 기록에서 '매달 같은 날(휴일이면 앞뒤 영업일) 비슷한 금액'으로
// 빠지는 출금을 찾아 정기결제 등록을 제안한다.
//
// 가맹점 이름은 섞여 적히는 일이 많아(같은 대출이 달마다 다른 이름) 이름이 아니라
// 결제 경로(계좌/카드) · 주 카테고리 · 금액(±5%)으로 묶고, 날짜 규칙으로 걸러 낸다.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"sobi/internal/holiday"
)

const (
	recurringLookbackDays = 210  // 최근 7개월
	recurringAmountTol    = 0.05 // 같은 묶음으로 보는 금액 차이
	recurringMinMonths    = 3    // 이만큼의 달에 나와야 후보
	recurringMinFit       = 0.8  // 출금의 이 비율 이상이 결제일 규칙에 맞아야 한다
	recurringMinCoverage  = 0.7  // 처음~마지막 달 중 실제로 나온 달 비율 (격월 등을 거른다)
	recurringRecentDays   = 45   // 마지막 출금이 이보다 오래되면 끝난 것으로 본다
	keyRecurringDismissed = "recurring.dismissed"
)

// RecurringCandidate 는 정기결제로 등록할 만한 반복 출금 하나.
type RecurringCandidate struct {
	Key         string   `json:"key"`       // '무시'할 때 쓰는 서명
	Merchant    string   `json:"merchant"`  // 가장 많이 쓴 가맹점 이름 (등록할 때 가맹점, 같으면 최근 것)
	Merchants   []string `json:"merchants"` // 기록에 쓰인 가맹점 이름들 (섞여 있으면 여러 개)
	Amount      int64    `json:"amount"`    // 등록할 금액 (가장 최근 금액)
	AmountMin   int64    `json:"amountMin"`
	AmountMax   int64    `json:"amountMax"`
	Fixed       bool     `json:"fixed"`       // 매번 같은 금액 → 자동 등록을 권한다
	Day         int      `json:"day"`         // 결제일 (매월)
	HolidayRule string   `json:"holidayRule"` // next | prev | none
	Dates       []string `json:"dates"`       // 최근 출금일 (최신 먼저)
	Months      int      `json:"months"`      // 나온 달 수
	NextDate    string   `json:"nextDate"`    // 다음 예상 출금일 (오늘 포함 이후)

	Account         bool   `json:"account"` // 카드가 아닌 계좌·현금 출금 (자금 계획 일정에 바로 영향)
	PaymentMethodID *int64 `json:"paymentMethodId"`
	PaymentLabel    string `json:"paymentLabel"`
	CategoryID      *int64 `json:"categoryId"`
	Category        string `json:"category"`
	MemberID        *int64 `json:"memberId"`
	Member          string `json:"member"`
}

// recTx 는 후보 찾기에 쓰는 거래 한 건.
type recTx struct {
	Date      time.Time
	Amount    int64
	Merchant  string
	PMID      *int64
	PMName    string
	PMType    string
	CatID     *int64
	CatName   string
	MainCatID int64 // 0 이면 미분류
	MemberID  *int64
	Member    string
}

// RecurringCandidates 는 최근 기록에서 정기 출금 후보를 찾는다('무시'한 것은 뺀다).
// 이미 등록된 정기결제와 겹치는지는 가맹점 대조 규칙을 가진 앱 계층이 거른다.
func (s *Store) RecurringCandidates(today time.Time) ([]RecurringCandidate, error) {
	today = dayOf(today)
	cal, err := s.Calendar()
	if err != nil {
		return nil, err
	}
	rows, err := s.query(`
SELECT t.date, t.amount, t.merchant, t.payment_method_id, COALESCE(p.name, ''), COALESCE(p.type, ''),
       t.category_id, COALESCE(cp.name || ' > ' || c.name, c.name, ''), COALESCE(c.parent_id, c.id, 0),
       t.member_id, COALESCE(m.name, '')
FROM transactions t
LEFT JOIN payment_methods p ON p.id = t.payment_method_id
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
LEFT JOIN members m ON m.id = t.member_id
WHERE t.direction = 'expense' AND t.date >= ? AND t.date <= ?
  AND NOT EXISTS (SELECT 1 FROM subscription_posts sp WHERE sp.transaction_id = t.id)
  AND `+transitTxFilter+`
ORDER BY t.date`, fmtDay(today.AddDate(0, 0, -recurringLookbackDays)), fmtDay(today))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []recTx
	for rows.Next() {
		var x recTx
		var d string
		var pm, cat, mem sql.NullInt64
		if err := rows.Scan(&d, &x.Amount, &x.Merchant, &pm, &x.PMName, &x.PMType,
			&cat, &x.CatName, &x.MainCatID, &mem, &x.Member); err != nil {
			return nil, err
		}
		t, err := time.Parse(dateLayout, d)
		if err != nil || x.Amount <= 0 {
			continue
		}
		x.Date = t
		x.PMID, x.CatID, x.MemberID = scanNullableID(pm), scanNullableID(cat), scanNullableID(mem)
		txs = append(txs, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	dismissed, err := s.recurringDismissed()
	if err != nil {
		return nil, err
	}
	out := []RecurringCandidate{}
	for _, c := range detectRecurring(txs, cal, today) {
		if !dismissed[c.Key] {
			out = append(out, c)
		}
	}
	return out, nil
}

// DismissRecurring 은 후보를 '무시'로 기록해 다시 제안하지 않게 한다.
func (s *Store) DismissRecurring(key string) error {
	if key == "" {
		return fmt.Errorf("빈 후보")
	}
	set, err := s.recurringDismissed()
	if err != nil {
		return err
	}
	set[key] = true
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b, _ := json.Marshal(keys)
	_, err = s.exec(`
INSERT INTO app_settings(key, value) VALUES(?, ?)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, keyRecurringDismissed, string(b))
	return err
}

func (s *Store) recurringDismissed() (map[string]bool, error) {
	var v string
	err := s.queryRow(`SELECT value FROM app_settings WHERE key = ?`, keyRecurringDismissed).Scan(&v)
	if err == sql.ErrNoRows {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var keys []string
	_ = json.Unmarshal([]byte(v), &keys)
	out := map[string]bool{}
	for _, k := range keys {
		out[k] = true
	}
	return out, nil
}

// detectRecurring 은 거래들에서 정기 출금 후보를 찾는다(순수 함수 — 테스트용으로 따로 둔다).
func detectRecurring(txs []recTx, cal *holiday.Calendar, today time.Time) []RecurringCandidate {
	// 1) 결제 경로(계좌·현금은 한 묶음, 카드는 카드별) + 주 카테고리로 나눈다
	groups := map[string][]recTx{}
	for _, x := range txs {
		groups[recGroupKey(x)] = append(groups[recGroupKey(x)], x)
	}
	var out []RecurringCandidate
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, gk := range keys {
		// 2) 금액 순으로 늘어놓고 ±5% 안을 한 묶음으로
		g := groups[gk]
		sort.SliceStable(g, func(i, j int) bool { return g[i].Amount < g[j].Amount })
		for i := 0; i < len(g); {
			j := i + 1
			for j < len(g) && float64(g[j].Amount) <= float64(g[i].Amount)*(1+recurringAmountTol) {
				j++
			}
			if c, ok := recurringFromCluster(gk, g[i:j], cal, today); ok {
				out = append(out, c)
			}
			i = j
		}
	}
	// 계좌 출금 먼저(자금 계획에 바로 영향), 그다음 금액 큰 순
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Account != out[j].Account {
			return out[i].Account
		}
		return out[i].Amount > out[j].Amount
	})
	return out
}

func recGroupKey(x recTx) string {
	path := "acct"
	if x.PMType == "card" && x.PMID != nil {
		path = fmt.Sprintf("card%d", *x.PMID)
	}
	return fmt.Sprintf("%s|%d", path, x.MainCatID)
}

// recurringFromCluster 는 비슷한 금액의 거래 묶음이 '매월 N일' 규칙으로 설명되는지 보고 후보를 만든다.
func recurringFromCluster(groupKey string, c []recTx, cal *holiday.Calendar, today time.Time) (RecurringCandidate, bool) {
	if len(c) < recurringMinMonths {
		return RecurringCandidate{}, false
	}
	day, rule, months, fit := fitMonthlySchedule(c, cal)
	if day == 0 || len(months) < recurringMinMonths || float64(fit) < recurringMinFit*float64(len(c)) {
		return RecurringCandidate{}, false
	}
	// 처음~마지막 달 중 실제로 나온 달의 비율 — 격월·분기 결제를 매월로 착각하지 않게
	yms := make([]string, 0, len(months))
	for ym := range months {
		yms = append(yms, ym)
	}
	sort.Strings(yms)
	span := monthsBetween(yms[0], yms[len(yms)-1]) + 1
	if float64(len(yms)) < recurringMinCoverage*float64(span) {
		return RecurringCandidate{}, false
	}
	// 최근 것부터
	sort.SliceStable(c, func(i, j int) bool { return c[i].Date.After(c[j].Date) })
	latest := c[0]
	if today.Sub(latest.Date) > recurringRecentDays*24*time.Hour {
		return RecurringCandidate{}, false
	}
	cand := RecurringCandidate{
		Merchant: latest.Merchant, Amount: latest.Amount, AmountMin: latest.Amount, AmountMax: latest.Amount,
		Day: day, HolidayRule: rule, Months: len(months),
		Account: latest.PMType != "card", PaymentMethodID: latest.PMID, PaymentLabel: latest.PMName,
		CategoryID: latest.CatID, Category: latest.CatName, MemberID: latest.MemberID, Member: latest.Member,
		Merchants: []string{}, Dates: []string{},
	}
	if cand.PaymentLabel == "" {
		cand.PaymentLabel = "계좌"
	}
	seen := map[string]bool{}
	count := map[string]int{}
	amounts := make([]int64, 0, len(c))
	for _, x := range c {
		count[x.Merchant]++
		if x.Amount < cand.AmountMin {
			cand.AmountMin = x.Amount
		}
		if x.Amount > cand.AmountMax {
			cand.AmountMax = x.Amount
		}
		if m := x.Merchant; m != "" && !seen[m] {
			seen[m] = true
			cand.Merchants = append(cand.Merchants, m)
		}
		if len(cand.Dates) < 6 {
			cand.Dates = append(cand.Dates, fmtDay(x.Date))
		}
		amounts = append(amounts, x.Amount)
	}
	// 등록할 가맹점은 가장 많이 쓴 이름 — 같은 대출이 달마다 다른 이름으로 적혀 있어도
	// 두 정기결제가 같은 이름을 갖지 않게(최근 이름만 쓰면 둘 다 같은 이름이 될 수 있다)
	for _, m := range cand.Merchants { // 최근 것부터 — 동점이면 최근 이름
		if count[m] > count[cand.Merchant] {
			cand.Merchant = m
		}
	}
	cand.Fixed = cand.AmountMin == cand.AmountMax
	cand.Key = fmt.Sprintf("%s|%d|%d", groupKey, day, roundSig2(median64(amounts)))
	// 다음 예상 출금일: 이번 달·다음 달 결제일 중 오늘 이후 첫날
	sub := Subscription{Cycle: "monthly", BillingDay: day, DayRule: "day", HolidayRule: rule}
	for m := monthStart(today); m.Before(monthStart(today).AddDate(0, 2, 0)); m = m.AddDate(0, 1, 0) {
		if _, actual, err := sub.ChargeDate(m.Format("2006-01"), cal); err == nil && !actual.Before(today) {
			cand.NextDate = fmtDay(actual)
			break
		}
	}
	return cand, true
}

// fitMonthlySchedule 은 출금일들을 가장 많이 설명하는 '매월 day 일 + 휴일 처리(rule)'를 찾는다.
// 정기결제와 같은 계산(Subscription.ChargeDate)으로 각 달의 실제 출금일을 내어 맞춰 본다.
// months 는 설명된 결제 달(YYYY-MM), fit 은 설명된 거래 수.
func fitMonthlySchedule(c []recTx, cal *holiday.Calendar) (day int, rule string, months map[string]bool, fit int) {
	days := map[int]bool{}
	for _, x := range c {
		d := x.Date.Day()
		for k := -5; k <= 5; k++ { // 연휴로 며칠씩 밀리거나 당겨진 경우
			if v := d + k; v >= 1 && v <= 31 {
				days[v] = true
			}
		}
		if d <= 6 { // 말일 결제가 다음 달 초로 밀린 경우
			for v := 27; v <= 31; v++ {
				days[v] = true
			}
		}
	}
	cands := make([]int, 0, len(days))
	for d := range days {
		cands = append(cands, d)
	}
	sort.Ints(cands)
	bestScore := -1
	for _, r := range []string{HolidayNext, HolidayNone, HolidayPrev} { // 동점이면 앞의 규칙(다음 영업일)
		for _, d := range cands {
			sub := Subscription{Cycle: "monthly", BillingDay: d, DayRule: "day", HolidayRule: r}
			ms := map[string]bool{}
			n := 0
			for _, x := range c {
				for _, off := range []int{0, -1, 1} {
					ym := monthStart(x.Date).AddDate(0, off, 0).Format("2006-01")
					if ms[ym] {
						continue // 한 달에 한 번만 센다
					}
					if _, actual, err := sub.ChargeDate(ym, cal); err == nil && actual.Equal(dayOf(x.Date)) {
						ms[ym] = true
						n++
						break
					}
				}
			}
			// 설명한 달 수가 많을수록, 같으면 설명한 거래 수가 많을수록
			if score := len(ms)*1000 + n; score > bestScore {
				bestScore, day, rule, months, fit = score, d, r, ms, n
			}
		}
	}
	if bestScore <= 0 {
		return 0, "", nil, 0
	}
	return day, rule, months, fit
}

func monthsBetween(a, b string) int {
	ta, err1 := time.Parse("2006-01", a)
	tb, err2 := time.Parse("2006-01", b)
	if err1 != nil || err2 != nil {
		return 0
	}
	return (tb.Year()-ta.Year())*12 + int(tb.Month()) - int(ta.Month())
}

// roundSig2 는 유효숫자 두 자리로 반올림한다(382,194 → 380,000). 금액이 조금 바뀌어도 '무시' 서명이 유지되게.
func roundSig2(v int64) int64 {
	if v <= 0 {
		return 0
	}
	p := math.Pow(10, math.Floor(math.Log10(float64(v)))-1)
	return int64(math.Round(float64(v)/p) * p)
}
