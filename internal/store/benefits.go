package store

import "strings"

// 카드 혜택 자료 CRUD.
//
// 이 표는 카드사 안내를 옮겨 적는 "자료"다. 앱이 계산에 쓰지는 않고 보여주기만 하므로,
// 카드사 표기를 그대로 남길 수 있도록 최대치(IsMax)·미확인(NeedsCheck) 표시를 함께 둔다.

const benefitSelect = `
SELECT id, payment_method_id, area, kind, rate_bp, is_max, monthly_cap, note, needs_check, sort_order
FROM card_benefits `

func (s *Store) scanBenefits(query string, args ...interface{}) ([]CardBenefit, error) {
	rows, err := s.query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CardBenefit{}
	for rows.Next() {
		var b CardBenefit
		var isMax, needsCheck int
		if err := rows.Scan(&b.ID, &b.PaymentMethodID, &b.Area, &b.Kind, &b.RateBp,
			&isMax, &b.MonthlyCap, &b.Note, &needsCheck, &b.SortOrder); err != nil {
			return nil, err
		}
		b.IsMax = isMax == 1
		b.NeedsCheck = needsCheck == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListCardBenefits 는 카드 한 장의 혜택 목록(표시 순서대로).
func (s *Store) ListCardBenefits(pmID int64) ([]CardBenefit, error) {
	return s.scanBenefits(benefitSelect+
		`WHERE payment_method_id=? ORDER BY sort_order, id`, pmID)
}

// AllCardBenefits 는 전체 카드의 혜택을 카드별로 묶어 돌려준다(목록 화면용).
func (s *Store) AllCardBenefits() (map[int64][]CardBenefit, error) {
	all, err := s.scanBenefits(benefitSelect + `ORDER BY payment_method_id, sort_order, id`)
	if err != nil {
		return nil, err
	}
	out := map[int64][]CardBenefit{}
	for _, b := range all {
		out[b.PaymentMethodID] = append(out[b.PaymentMethodID], b)
	}
	return out, nil
}

// SaveCardBenefit 은 혜택 한 줄을 추가(ID=0)하거나 수정한다.
func (s *Store) SaveCardBenefit(b CardBenefit) (CardBenefit, error) {
	b.Area = strings.TrimSpace(b.Area)
	b.Note = strings.TrimSpace(b.Note)
	if b.Kind != "point" && b.Kind != "discount" && b.Kind != "service" {
		b.Kind = "point"
	}
	if b.Kind == "service" {
		b.RateBp = 0 // 서비스 혜택은 요율이 없다
	}
	if b.ID > 0 {
		_, err := s.exec(`
UPDATE card_benefits SET area=?, kind=?, rate_bp=?, is_max=?, monthly_cap=?, note=?, needs_check=?
WHERE id=?`,
			b.Area, b.Kind, b.RateBp, boolToInt(b.IsMax), b.MonthlyCap, b.Note, boolToInt(b.NeedsCheck), b.ID)
		return b, err
	}
	// 새 항목은 그 카드 목록의 맨 뒤에 붙인다
	var next int
	if err := s.queryRow(
		`SELECT COALESCE(MAX(sort_order) + 1, 0) FROM card_benefits WHERE payment_method_id=?`,
		b.PaymentMethodID).Scan(&next); err != nil {
		return b, err
	}
	b.SortOrder = next
	err := s.queryRow(`
INSERT INTO card_benefits(payment_method_id, area, kind, rate_bp, is_max, monthly_cap, note, needs_check, sort_order)
VALUES(?,?,?,?,?,?,?,?,?) RETURNING id`,
		b.PaymentMethodID, b.Area, b.Kind, b.RateBp, boolToInt(b.IsMax),
		b.MonthlyCap, b.Note, boolToInt(b.NeedsCheck), b.SortOrder).Scan(&b.ID)
	return b, err
}

func (s *Store) DeleteCardBenefit(id int64) error {
	_, err := s.exec(`DELETE FROM card_benefits WHERE id=?`, id)
	return err
}

// MoveCardBenefit 은 같은 카드 안에서 혜택 순서를 delta 칸 옮긴다(-1 위, +1 아래).
func (s *Store) MoveCardBenefit(id int64, delta int) error {
	if delta == 0 {
		return nil
	}
	var pmID int64
	if err := s.queryRow(`SELECT payment_method_id FROM card_benefits WHERE id=?`, id).Scan(&pmID); err != nil {
		return err
	}
	list, err := s.ListCardBenefits(pmID)
	if err != nil {
		return err
	}
	for i, b := range list {
		if b.ID != id {
			continue
		}
		to := i + delta
		if to < 0 || to >= len(list) {
			return nil // 이미 맨 끝
		}
		list[i], list[to] = list[to], list[i]
		for order, item := range list {
			if _, err := s.exec(`UPDATE card_benefits SET sort_order=? WHERE id=?`, order, item.ID); err != nil {
				return err
			}
		}
		return nil
	}
	return ErrNotFound
}
