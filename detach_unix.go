//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detachProcess 는 자식 프로세스를 새 세션으로 띄워 앱이 끝나도 같이 끝나지 않게 한다.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
