package store

import "testing"

// 카테고리 기본 귀속자: 주에 정하면 부가 이어받고(부 자기 값이 우선), 비우면 사라지며,
// 귀속자를 지우면 기본값도 함께 비워진다. (TEST_DATABASE_URL 미설정 시 자동 스킵)
func TestCategoryDefaultMember(t *testing.T) {
	st := openTestStore(t)

	parents, err := st.AddMember("부모님")
	if err != nil {
		t.Fatal(err)
	}
	members, _ := st.ListMembers()
	var dad int64
	for _, m := range members {
		if m.Name == "아빠" {
			dad = m.ID
		}
	}
	main, err := st.AddCategory("부모요양비", "expense", 0)
	if err != nil {
		t.Fatal(err)
	}
	med, err := st.AddCategory("의료비", "expense", main.ID)
	if err != nil {
		t.Fatal(err)
	}
	gift, err := st.AddCategory("용돈", "expense", main.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := st.SetCategoryDefaultMember(main.ID, parents.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.SetCategoryDefaultMember(gift.ID, dad); err != nil { // 부 자기 값
		t.Fatal(err)
	}
	find := func(id int64) Category {
		t.Helper()
		cats, err := st.ListCategories()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cats {
			if c.ID == id {
				return c
			}
		}
		t.Fatalf("카테고리 %d 없음", id)
		return Category{}
	}
	if c := find(main.ID); c.DefaultMemberID == nil || *c.DefaultMemberID != parents.ID {
		t.Errorf("주 기본 귀속자: %+v", c.DefaultMemberID)
	}
	if c := find(med.ID); c.DefaultMemberID != nil || c.ParentDefaultMemberID == nil || *c.ParentDefaultMemberID != parents.ID {
		t.Errorf("부는 주의 값을 이어받아야 함: own %v parent %v", c.DefaultMemberID, c.ParentDefaultMemberID)
	}
	if c := find(gift.ID); c.DefaultMemberID == nil || *c.DefaultMemberID != dad {
		t.Errorf("부 자기 값: %+v", c.DefaultMemberID)
	}

	// 비우기
	if err := st.SetCategoryDefaultMember(gift.ID, 0); err != nil {
		t.Fatal(err)
	}
	if c := find(gift.ID); c.DefaultMemberID != nil {
		t.Errorf("비운 뒤에도 남음: %v", *c.DefaultMemberID)
	}
	// 없는 귀속자·없는 카테고리
	if err := st.SetCategoryDefaultMember(main.ID, 999999); err == nil {
		t.Error("없는 귀속자가 통과함")
	}
	if err := st.SetCategoryDefaultMember(999999, parents.ID); err != ErrNotFound {
		t.Errorf("없는 카테고리: %v", err)
	}
	// 귀속자를 지우면 기본값도 비워진다
	if err := st.DeleteMember(parents.ID); err != nil {
		t.Fatal(err)
	}
	if c := find(main.ID); c.DefaultMemberID != nil {
		t.Errorf("귀속자 삭제 후에도 기본값이 남음: %v", *c.DefaultMemberID)
	}
}
