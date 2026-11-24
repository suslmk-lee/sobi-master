package store

import (
	"fmt"
	"sync"
)

// parallel 은 서로 상관없는 조회들을 동시에 돌리고 모두 끝날 때까지 기다린다.
// DB(Supabase)가 멀리 있어 조회 하나마다 왕복 시간이 붙으므로, 차례로 부르면 그 합만큼 걸리고
// 동시에 부르면 가장 느린 것 하나만큼 걸린다. (app.parallel 과 같은 규칙)
//
// 오류는 순서상 앞선 것(인자 순서)을 돌려준다 — 차례로 부르던 때와 같은 오류가 나오도록.
// 각 함수는 서로 다른 변수에만 써야 한다.
func parallel(fns ...func() error) error {
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for i, fn := range fns {
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					errs[i] = fmt.Errorf("내부 오류(panic): %v", p)
				}
			}()
			errs[i] = fn()
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
