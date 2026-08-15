package store

import (
	"log"
	"strings"
)

// splitSlashCategories 는 "식비/배달" 처럼 슬래시로 계층을 흉내낸 기존 카테고리를
// 주/부 계층으로 옮긴다. 단, 슬래시 앞부분이 이미 주 카테고리로 존재할 때만 분리한다.
//   - "식비/배달"  + 주 "식비" 존재  → 이름 "배달", parent = 식비
//   - "회비/경조사" + 주 "회비" 없음  → 그대로 둔다 ("또는" 의미의 이름 보호)
//
// 거래·규칙·예산은 category_id 를 그대로 참조하므로 연결이 끊기지 않는다.
// 이름에서 슬래시가 사라지므로 다시 실행해도 중복 분리되지 않는다(멱등).
func (s *Store) splitSlashCategories() error {
	// 분리 대상 후보: 슬래시가 있고 아직 주인 카테고리
	rows, err := s.query(`SELECT id, name, kind FROM categories WHERE parent_id IS NULL AND name LIKE '%/%'`)
	if err != nil {
		return err
	}
	type cand struct {
		id   int64
		name string
		kind string
	}
	cands := []cand{}
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.id, &c.name, &c.kind); err != nil {
			rows.Close()
			return err
		}
		cands = append(cands, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(cands) == 0 {
		return nil
	}

	for _, c := range cands {
		i := strings.Index(c.name, "/")
		head := strings.TrimSpace(c.name[:i])
		tail := strings.TrimSpace(c.name[i+1:])
		if head == "" || tail == "" {
			continue
		}
		// 슬래시 앞부분이 같은 종류의 주 카테고리로 존재해야 분리한다
		var parentID int64
		err := s.queryRow(`
SELECT id FROM categories
WHERE name=? AND parent_id IS NULL AND kind=? AND id <> ?`, head, c.kind, c.id).Scan(&parentID)
		if err != nil {
			continue // 없으면(ErrNoRows) 그대로 둔다
		}
		// 같은 주 아래 같은 이름의 부가 이미 있으면 건너뛴다(유니크 충돌 방지)
		var dup int
		if err := s.queryRow(`SELECT COUNT(*) FROM categories WHERE parent_id=? AND name=?`,
			parentID, tail).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			continue
		}
		if _, err := s.exec(`UPDATE categories SET name=?, parent_id=? WHERE id=?`, tail, parentID, c.id); err != nil {
			return err
		}
		log.Printf("카테고리 계층 정리: %q → %q > %q", c.name, head, tail)
	}
	return nil
}
