//go:build !windows

package main

import "fmt"

// verifyStagedExecutable 은 Windows 에서만 쓴다 (exe 의 버전 리소스 읽기).
func verifyStagedExecutable(staged, current, wantVersion string) error {
	return fmt.Errorf("exe 버전 확인은 Windows 에서만 됩니다")
}
