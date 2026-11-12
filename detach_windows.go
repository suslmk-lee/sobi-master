//go:build windows

package main

import "os/exec"

// detachProcess 는 Windows 에서는 하는 일이 없다 (자동 설치는 macOS 에서만 한다).
func detachProcess(cmd *exec.Cmd) {}
