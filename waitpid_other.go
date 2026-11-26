//go:build !windows

package main

import (
	"os"
	"syscall"
	"time"
)

// waitForProcessExit 는 pid 프로세스가 끝날 때까지 최대 max 만큼 기다린다 (0.1초마다 확인).
func waitForProcessExit(pid int, max time.Duration) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if p.Signal(syscall.Signal(0)) != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
