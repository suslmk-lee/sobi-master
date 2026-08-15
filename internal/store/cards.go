package store

import "time"

// perfPeriod 는 기준일(today)이 속한 실적기간 [시작, 끝]을 돌려준다.
// startDay 가 실적 산정 시작일이며, 말일보다 크면 그 달의 말일로 줄여 잡는다.
// 예: startDay=1 → 매월 1일~말일, startDay=15 → 15일~다음 달 14일.
func perfPeriod(today time.Time, startDay int) (time.Time, time.Time) {
	if startDay < 1 {
		startDay = 1
	}
	clamp := func(y int, m time.Month, d int) time.Time {
		last := time.Date(y, m+1, 0, 0, 0, 0, 0, today.Location()).Day()
		if d > last {
			d = last
		}
		return time.Date(y, m, d, 0, 0, 0, 0, today.Location())
	}
	y, m, _ := today.Date()
	start := clamp(y, m, startDay)
	if today.Before(start) {
		// 전월로 이동할 때는 1일 기준으로 빼야 월 정규화(2/31→3/3) 오류가 없다
		prev := time.Date(y, m, 1, 0, 0, 0, 0, today.Location()).AddDate(0, -1, 0)
		start = clamp(prev.Year(), prev.Month(), startDay)
	}
	nextMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, today.Location()).AddDate(0, 1, 0)
	next := clamp(nextMonth.Year(), nextMonth.Month(), startDay)
	return start, next.AddDate(0, 0, -1)
}

// CardStatuses 는 card 타입 결제수단 각각의 현재 실적기간 사용액과 한도 달성 여부를 돌려준다.
func (s *Store) CardStatuses(today time.Time) ([]CardStatus, error) {
	pms, err := s.ListPaymentMethods()
	if err != nil {
		return nil, err
	}
	out := []CardStatus{}
	for _, pm := range pms {
		if pm.Type != "card" {
			continue
		}
		start, end := perfPeriod(today, pm.CycleStartDay)
		st := CardStatus{
			Card:        pm,
			PeriodStart: start.Format("2006-01-02"),
			PeriodEnd:   end.Format("2006-01-02"),
		}
		err := s.queryRow(`
SELECT COALESCE(SUM(amount), 0) FROM transactions
WHERE payment_method_id=? AND direction='expense' AND date >= ? AND date <= ?`,
			pm.ID, st.PeriodStart, st.PeriodEnd).Scan(&st.Spent)
		if err != nil {
			return nil, err
		}
		if pm.PerfTarget > 0 {
			st.Achieved = st.Spent >= pm.PerfTarget
			if !st.Achieved {
				st.Remaining = pm.PerfTarget - st.Spent
			}
		}
		out = append(out, st)
	}
	return out, nil
}

// CardBreakdown 은 특정 카드의 기간 내 지출을 가맹점별/카테고리별로 집계한다.
func (s *Store) CardBreakdown(pmID int64, start, end string) (CardBreakdown, error) {
	bd := CardBreakdown{ByMerchant: []NamedAmount{}, ByCategory: []NamedAmount{}}
	group := func(query string, dst *[]NamedAmount) error {
		rows, err := s.query(query, pmID, start, end)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var na NamedAmount
			na.Kind = "expense"
			if err := rows.Scan(&na.Name, &na.Amount); err != nil {
				return err
			}
			*dst = append(*dst, na)
		}
		return rows.Err()
	}
	if err := group(`
SELECT CASE WHEN t.merchant='' THEN '(내용 없음)' ELSE t.merchant END, SUM(t.amount)
FROM transactions t
WHERE t.payment_method_id=? AND t.direction='expense' AND t.date >= ? AND t.date <= ?
GROUP BY 1 ORDER BY SUM(t.amount) DESC`, &bd.ByMerchant); err != nil {
		return bd, err
	}
	// 카테고리는 주 기준으로 합산한다(부 지출은 상위 주에 포함)
	err := group(`
SELECT COALESCE(cp.name, c.name, '미분류'), SUM(t.amount)
FROM transactions t
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
WHERE t.payment_method_id=? AND t.direction='expense' AND t.date >= ? AND t.date <= ?
GROUP BY 1 ORDER BY SUM(t.amount) DESC`, &bd.ByCategory)
	return bd, err
}
