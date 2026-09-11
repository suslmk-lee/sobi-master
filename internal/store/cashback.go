package store

import (
	"fmt"
	"time"
)

// 캐시백 계산.
//
// 카드마다 결제액의 일정 비율(CashbackBp)을 기본으로 쌓아 주고, 카드에 따라서는
// 결제일로부터 CashbackBonusDays 안에 카드대금을 갚으면 CashbackBonusBp 만큼
// 한 번 더 얹어 준다. 그 "추가분"은 기한을 넘기면 영영 못 받으므로,
// 아직 받을 수 있는 건을 남은 일수와 함께 따로 모아 보여주는 게 이 화면의 핵심이다.

// 캐시백 항목의 추가분 상태.
const (
	CashbackEarned  = "earned"  // 기한 안에 갚아서 추가분을 받음
	CashbackPending = "pending" // 아직 안 갚았지만 기한이 남아 있음 (지금 갚으면 받는다)
	CashbackMissed  = "missed"  // 기한을 넘김 (또는 기한 지나 갚음)
)

// CashbackItem 은 결제 한 건의 캐시백 내역.
type CashbackItem struct {
	TxID     int64  `json:"txId"`
	Card     string `json:"card"`
	Date     string `json:"date"`
	Merchant string `json:"merchant"`
	Amount   int64  `json:"amount"`
	PaidAt   string `json:"paidAt"`   // 카드대금 납부일 (빈 값이면 미납부)
	Deadline string `json:"deadline"` // 추가분을 받으려면 이 날까지 갚아야 한다
	// DaysLeft: 오늘 기준 남은 일수 (pending 일 때만 의미 있음. 0이면 오늘이 마감)
	DaysLeft int `json:"daysLeft"`
	// PaidInDays: 결제일로부터 며칠 만에 갚았는지 (납부한 건만)
	PaidInDays int    `json:"paidInDays"`
	Base       int64  `json:"base"`   // 기본 캐시백
	Bonus      int64  `json:"bonus"`  // 실제로 받은 추가 캐시백 (미획득이면 0)
	Possible   int64  `json:"possible"` // 받을 수 있었던/있는 추가 캐시백 (pending·missed 안내용)
	Status     string `json:"status"`
}

// CardCashback 은 카드 한 장의 해당 월 캐시백 집계.
type CardCashback struct {
	Card         PaymentMethod  `json:"card"`
	TotalAmount  int64          `json:"totalAmount"`  // 대상 결제 합계
	TotalBase    int64          `json:"totalBase"`    // 기본 캐시백 합계
	TotalBonus   int64          `json:"totalBonus"`   // 받은 추가 캐시백 합계
	MissedBonus  int64          `json:"missedBonus"`  // 기한을 넘겨 못 받은 추가분
	PendingBonus int64          `json:"pendingBonus"` // 아직 받을 수 있는 추가분
	Items        []CashbackItem `json:"items"`
}

// CashbackReport 는 캐시백 화면 한 장에 필요한 자료.
type CashbackReport struct {
	// Pending: 오늘 기준 아직 추가분을 받을 수 있는 결제(월과 무관하게 전부).
	// 기한이 임박한 순서로 정렬한다 — 이 목록이 이 화면의 존재 이유다.
	Pending []CashbackItem `json:"pending"`
	// Cards: 선택한 월의 카드별 집계.
	Cards []CardCashback `json:"cards"`
	// HasCashbackCard: 캐시백이 설정된 카드가 하나라도 있는지(안내 문구용).
	HasCashbackCard bool `json:"hasCashbackCard"`
}

// cashbackOf 는 만분율(bp)을 적용한 캐시백 금액. 원 단위 미만은 버린다(카드사 관행).
func cashbackOf(amount int64, bp int) int64 {
	if bp <= 0 || amount <= 0 {
		return 0
	}
	return amount * int64(bp) / 10000
}

// daysBetween 은 두 "YYYY-MM-DD" 사이의 일수(b - a). 파싱 실패 시 0, false.
func daysBetween(a, b string) (int, bool) {
	ta, err1 := time.Parse("2006-01-02", a)
	tb, err2 := time.Parse("2006-01-02", b)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return int(tb.Sub(ta).Hours() / 24), true
}

// Cashback 은 캐시백이 설정된 카드들의 (year, month) 집계와, 오늘 기준 아직
// 추가분을 받을 수 있는 결제 목록을 함께 돌려준다.
func (s *Store) Cashback(year, month int, today time.Time) (CashbackReport, error) {
	rep := CashbackReport{Pending: []CashbackItem{}, Cards: []CardCashback{}}

	pms, err := s.ListPaymentMethods()
	if err != nil {
		return rep, err
	}
	cards := []PaymentMethod{}
	for _, pm := range pms {
		if pm.CashbackBp > 0 || pm.CashbackBonusBp > 0 {
			cards = append(cards, pm)
		}
	}
	if len(cards) == 0 {
		return rep, nil
	}
	rep.HasCashbackCard = true

	todayStr := today.Format("2006-01-02")
	prefix := fmt.Sprintf("%04d-%02d", year, month) + "%"

	for _, pm := range cards {
		cc := CardCashback{Card: pm, Items: []CashbackItem{}}

		// 선택한 월의 결제 내역
		rows, err := s.query(`
SELECT id, date, CASE WHEN merchant='' THEN '(내용 없음)' ELSE merchant END, amount, paid_at
FROM transactions
WHERE payment_method_id=? AND direction='expense' AND date LIKE ?
ORDER BY date DESC, id DESC`, pm.ID, prefix)
		if err != nil {
			return rep, err
		}
		for rows.Next() {
			var it CashbackItem
			if err := rows.Scan(&it.TxID, &it.Date, &it.Merchant, &it.Amount, &it.PaidAt); err != nil {
				rows.Close()
				return rep, err
			}
			it.Card = pm.Name
			fillCashback(&it, pm, todayStr)

			cc.TotalAmount += it.Amount
			cc.TotalBase += it.Base
			cc.TotalBonus += it.Bonus
			switch it.Status {
			case CashbackMissed:
				cc.MissedBonus += it.Possible
			case CashbackPending:
				cc.PendingBonus += it.Possible
			}
			cc.Items = append(cc.Items, it)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return rep, err
		}
		rows.Close()
		rep.Cards = append(rep.Cards, cc)

		// 아직 추가분을 받을 수 있는 건은 월과 무관하게 모은다.
		// 기한 안에 있으려면 결제일이 (오늘 - 기한일) 이후여야 한다.
		if pm.CashbackBonusBp <= 0 || pm.CashbackBonusDays <= 0 {
			continue
		}
		from := today.AddDate(0, 0, -pm.CashbackBonusDays).Format("2006-01-02")
		prows, err := s.query(`
SELECT id, date, CASE WHEN merchant='' THEN '(내용 없음)' ELSE merchant END, amount, paid_at
FROM transactions
WHERE payment_method_id=? AND direction='expense' AND paid_at='' AND date >= ? AND date <= ?
ORDER BY date`, pm.ID, from, todayStr)
		if err != nil {
			return rep, err
		}
		for prows.Next() {
			var it CashbackItem
			if err := prows.Scan(&it.TxID, &it.Date, &it.Merchant, &it.Amount, &it.PaidAt); err != nil {
				prows.Close()
				return rep, err
			}
			it.Card = pm.Name
			fillCashback(&it, pm, todayStr)
			if it.Status == CashbackPending {
				rep.Pending = append(rep.Pending, it)
			}
		}
		if err := prows.Err(); err != nil {
			prows.Close()
			return rep, err
		}
		prows.Close()
	}

	// 기한이 급한 순 → 같으면 금액 큰 순
	for i := 1; i < len(rep.Pending); i++ {
		for j := i; j > 0; j-- {
			a, b := rep.Pending[j-1], rep.Pending[j]
			if a.DaysLeft < b.DaysLeft || (a.DaysLeft == b.DaysLeft && a.Amount >= b.Amount) {
				break
			}
			rep.Pending[j-1], rep.Pending[j] = b, a
		}
	}
	return rep, nil
}

// fillCashback 은 결제 한 건의 캐시백 금액과 추가분 상태를 채운다.
func fillCashback(it *CashbackItem, pm PaymentMethod, todayStr string) {
	it.Base = cashbackOf(it.Amount, pm.CashbackBp)
	it.Possible = cashbackOf(it.Amount, pm.CashbackBonusBp)

	// 추가 캐시백을 안 주는 카드면 상태 판정 자체가 없다.
	if pm.CashbackBonusBp <= 0 || pm.CashbackBonusDays <= 0 {
		it.Possible = 0
		it.Status = CashbackEarned // 더 받을 게 없으므로 "완료"로 둔다
		return
	}

	if t, err := time.Parse("2006-01-02", it.Date); err == nil {
		it.Deadline = t.AddDate(0, 0, pm.CashbackBonusDays).Format("2006-01-02")
	}

	if it.PaidAt != "" {
		days, ok := daysBetween(it.Date, it.PaidAt)
		it.PaidInDays = days
		if ok && days >= 0 && days <= pm.CashbackBonusDays {
			it.Bonus = it.Possible
			it.Status = CashbackEarned
		} else {
			it.Status = CashbackMissed // 기한 지나서 갚음
		}
		return
	}

	// 미납부 — 기한이 남았으면 아직 받을 수 있다
	if left, ok := daysBetween(todayStr, it.Deadline); ok && left >= 0 {
		it.DaysLeft = left
		it.Status = CashbackPending
	} else {
		it.Status = CashbackMissed
	}
}
