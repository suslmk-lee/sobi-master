package store

import (
	"fmt"
	"time"
)

// MonthPoint 는 월별 추이 그래프의 한 점.
type MonthPoint struct {
	Month    string `json:"month"` // YYYY-MM
	Income   int64  `json:"income"`
	Expense  int64  `json:"expense"`
	Transfer int64  `json:"transfer"`
}

// DayPoint 는 일별 지출 그래프의 한 점.
type DayPoint struct {
	Day    int   `json:"day"`
	Amount int64 `json:"amount"`
}

// MonthlyTrend 는 (year, month)를 마지막으로 하는 최근 n개월의 수입/지출/이체 합계.
// 거래가 없는 달도 0으로 채워 돌려준다.
func (s *Store) MonthlyTrend(year, month, n int) ([]MonthPoint, error) {
	if n < 1 {
		n = 6
	}
	pts := make([]MonthPoint, n)
	idx := map[string]int{}
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	for i := n - 1; i >= 0; i-- {
		key := t.Format("2006-01")
		pts[i] = MonthPoint{Month: key}
		idx[key] = i
		t = t.AddDate(0, -1, 0)
	}
	first := pts[0].Month
	last := pts[n-1].Month

	rows, err := s.query(`
SELECT substr(date,1,7), direction, SUM(amount)
FROM transactions
WHERE substr(date,1,7) >= ? AND substr(date,1,7) <= ?
GROUP BY 1, 2`, first, last)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m, dir string
		var amt int64
		if err := rows.Scan(&m, &dir, &amt); err != nil {
			return nil, err
		}
		i, ok := idx[m]
		if !ok {
			continue
		}
		switch dir {
		case "income":
			pts[i].Income = amt
		case "expense":
			pts[i].Expense = amt
		case "transfer":
			pts[i].Transfer = amt
		}
	}
	return pts, rows.Err()
}

// DailyExpenses 는 해당 월의 1일~말일 일별 지출 합계(없는 날은 0).
func (s *Store) DailyExpenses(year, month int) ([]DayPoint, error) {
	lastDay := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	pts := make([]DayPoint, lastDay)
	for i := range pts {
		pts[i].Day = i + 1
	}
	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"
	rows, err := s.query(`
SELECT CAST(substr(date,9,2) AS INTEGER), SUM(amount)
FROM transactions
WHERE date LIKE ? AND direction='expense'
GROUP BY 1`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d int
		var amt int64
		if err := rows.Scan(&d, &amt); err != nil {
			return nil, err
		}
		if d >= 1 && d <= lastDay {
			pts[d-1].Amount = amt
		}
	}
	return pts, rows.Err()
}

// TopMerchants 는 해당 월 지출 상위 가맹점.
func (s *Store) TopMerchants(year, month, limit int) ([]NamedAmount, error) {
	if limit < 1 {
		limit = 10
	}
	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"
	rows, err := s.query(`
SELECT CASE WHEN merchant='' THEN '(내용 없음)' ELSE merchant END, SUM(amount)
FROM transactions
WHERE date LIKE ? AND direction='expense'
GROUP BY 1 ORDER BY SUM(amount) DESC LIMIT ?`, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NamedAmount{}
	for rows.Next() {
		var na NamedAmount
		na.Kind = "expense"
		if err := rows.Scan(&na.Name, &na.Amount); err != nil {
			return nil, err
		}
		out = append(out, na)
	}
	return out, rows.Err()
}
