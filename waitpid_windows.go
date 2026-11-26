//go:build windows

package main

import (
	"time"

	"golang.org/x/sys/windows"
)

// waitForProcessExit 는 pid 프로세스가 끝날 때까지 최대 max 만큼 기다린다.
// 이미 없는 프로세스면 바로 돌아온다.
func waitForProcessExit(pid int, max time.Duration) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	windows.WaitForSingleObject(h, uint32(max.Milliseconds()))
}
