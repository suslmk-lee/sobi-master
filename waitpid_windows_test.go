//go:build windows

package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestWaitForProcessExit(t *testing.T) {
	// 이미 없는 프로세스는 바로 돌아온다
	start := time.Now()
	waitForProcessExit(0x7ffffff0, time.Minute)
	if time.Since(start) > 5*time.Second {
		t.Error("없는 프로세스를 기다림")
	}

	cmd := exec.Command("ping", "-n", "3", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go cmd.Wait()
	start = time.Now()
	waitForProcessExit(cmd.Process.Pid, time.Minute)
	if d := time.Since(start); d < 1*time.Second || d > 30*time.Second {
		t.Errorf("프로세스가 끝날 때까지 기다려야 함: %v", d)
	}

	// 한도: 끝나지 않는 프로세스는 한도에서 포기
	long := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := long.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { long.Process.Kill(); long.Wait() }()
	start = time.Now()
	waitForProcessExit(long.Process.Pid, 700*time.Millisecond)
	if d := time.Since(start); d < 500*time.Millisecond || d > 5*time.Second {
		t.Errorf("한도에서 돌아와야 함: %v", d)
	}
}
