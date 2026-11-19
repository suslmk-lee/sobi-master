package app

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelRunsConcurrently(t *testing.T) {
	var n atomic.Int32
	start := time.Now()
	err := parallel(
		func() error { time.Sleep(100 * time.Millisecond); n.Add(1); return nil },
		func() error { time.Sleep(100 * time.Millisecond); n.Add(1); return nil },
		func() error { time.Sleep(100 * time.Millisecond); n.Add(1); return nil },
	)
	if err != nil || n.Load() != 3 {
		t.Fatalf("err=%v n=%d", err, n.Load())
	}
	if d := time.Since(start); d > 250*time.Millisecond {
		t.Fatalf("동시에 돌지 않았다: %v", d)
	}
}

func TestParallelFirstErrorInOrder(t *testing.T) {
	e1, e2 := errors.New("첫째"), errors.New("둘째")
	err := parallel(
		func() error { return nil },
		func() error { time.Sleep(30 * time.Millisecond); return e1 }, // 늦게 끝나도
		func() error { return e2 },                                    // 순서상 앞선 오류가 나온다
	)
	if err != e1 {
		t.Fatalf("err=%v, want %v", err, e1)
	}
}

func TestParallelPanicBecomesError(t *testing.T) {
	err := parallel(func() error { panic("터짐") }, func() error { return nil })
	if err == nil {
		t.Fatal("panic 이 오류로 바뀌지 않았다")
	}
}

func TestParallelEmpty(t *testing.T) {
	if err := parallel(); err != nil {
		t.Fatal(err)
	}
}
