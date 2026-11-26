//go:build windows

package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// detachProcess 는 자식 프로세스를 앱과 떨어진 새 프로세스 그룹·콘솔 없이 띄워, 앱이 끝나도 같이 끝나지 않게 한다.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
}
