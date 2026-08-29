package store

import (
	"fmt"
	"math/rand"
	"testing"
)

func eq(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 드래그로 옮겼을 때 기대하는 자리에 들어가는지.
// 아래로 끌면 대상 뒤에, 위로 끌면 대상 앞에 놓이는 흔한 동작이어야 한다.
func TestMoveInSlice(t *testing.T) {
	base := []int64{10, 20, 30, 40}
	cases := []struct {
		name     string
		from, to int
		want     []int64
	}{
		{"맨 앞을 맨 뒤로", 0, 3, []int64{20, 30, 40, 10}},
		{"맨 뒤를 맨 앞으로", 3, 0, []int64{40, 10, 20, 30}},
		{"한 칸 아래로", 0, 1, []int64{20, 10, 30, 40}},
		{"한 칸 위로", 1, 0, []int64{20, 10, 30, 40}},
		{"가운데를 아래로", 1, 2, []int64{10, 30, 20, 40}},
		{"가운데를 위로", 2, 1, []int64{10, 30, 20, 40}},
		{"제자리", 2, 2, []int64{10, 20, 30, 40}},
		{"두 칸 아래로", 0, 2, []int64{20, 30, 10, 40}},
		{"두 칸 위로", 3, 1, []int64{10, 40, 20, 30}},
	}
	for _, c := range cases {
		src := append([]int64(nil), base...)
		got := moveInSlice(src, c.from, c.to)
		if !eq(got, c.want) {
			t.Errorf("%s: moveInSlice(%v, %d, %d) = %v, 기대 %v", c.name, base, c.from, c.to, got, c.want)
		}
		// 원본을 덮어쓰면 호출 측에서 조용히 깨진다
		if !eq(src, base) {
			t.Errorf("%s: 원본이 바뀌었다 %v → %v", c.name, base, src)
		}
	}
}

// 어떤 (from, to) 조합에서도 원소가 사라지거나 늘어나면 안 된다.
func TestMoveInSlicePreservesElements(t *testing.T) {
	rng := rand.New(rand.NewSource(20260830))
	for n := 1; n <= 12; n++ {
		ids := make([]int64, n)
		for i := range ids {
			ids[i] = int64((i + 1) * 10)
		}
		for from := 0; from < n; from++ {
			for to := 0; to < n; to++ {
				got := moveInSlice(ids, from, to)
				if len(got) != n {
					t.Fatalf("n=%d from=%d to=%d: 길이가 %d", n, from, to, len(got))
				}
				seen := map[int64]int{}
				for _, v := range got {
					seen[v]++
				}
				for _, v := range ids {
					if seen[v] != 1 {
						t.Fatalf("n=%d from=%d to=%d: %d 이 %d번 나옴 (%v)", n, from, to, v, seen[v], got)
					}
				}
				// 옮긴 원소가 실제로 to 자리에 있어야 한다
				if got[to] != ids[from] {
					t.Fatalf("n=%d from=%d to=%d: got[%d]=%d, 기대 %d (%v)",
						n, from, to, to, got[to], ids[from], got)
				}
			}
		}
		_ = rng
	}
}

// 옮기지 않은 나머지 원소들의 상대 순서는 그대로여야 한다.
func TestMoveInSliceKeepsRelativeOrder(t *testing.T) {
	ids := []int64{1, 2, 3, 4, 5, 6}
	for from := 0; from < len(ids); from++ {
		for to := 0; to < len(ids); to++ {
			got := moveInSlice(ids, from, to)
			rest := []int64{}
			for _, v := range got {
				if v != ids[from] {
					rest = append(rest, v)
				}
			}
			want := []int64{}
			for i, v := range ids {
				if i != from {
					want = append(want, v)
				}
			}
			if !eq(rest, want) {
				t.Errorf("from=%d to=%d: 나머지 순서가 %v, 기대 %v", from, to, rest, want)
			}
		}
	}
	_ = fmt.Sprint()
}
