//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 버전 리소스가 없는 exe(이 테스트 바이너리)는 오류여야 한다.
func TestReadExeVersionInfoWithoutResource(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if info, err := readExeVersionInfo(exe); err == nil {
		t.Errorf("리소스가 없는 exe 인데 성공: %+v", info)
	}
	if _, err := readExeVersionInfo(filepath.Join(t.TempDir(), "none.exe")); err == nil {
		t.Error("없는 파일인데 성공")
	}
	if err := verifyStagedExecutable(exe, exe, "1.0.0"); err == nil {
		t.Error("리소스가 없는 exe 가 확인을 통과")
	}
}

// 시스템 exe(notepad)에는 버전 리소스가 있다 — 문자열 정보를 읽을 수 있어야 한다.
func TestReadExeVersionInfoSystemExe(t *testing.T) {
	p := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	if _, err := os.Stat(p); err != nil {
		t.Skip("notepad.exe 없음")
	}
	info, err := readExeVersionInfo(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.ProductVersion == "" || info.ProductName == "" {
		t.Errorf("notepad 버전 정보: %+v", info)
	}
}

// 실제 wails 로 빌드한 옛/새 exe 와 새 버전 zip 으로 설치 흐름을 끝까지 돌려 본다 (함수 단위).
// 환경변수가 없으면 건너뛴다:
//
//	SOBI_E2E_OLD = 옛 버전 exe, SOBI_E2E_ZIP = 새 버전 zip, SOBI_E2E_VERSION = 새 버전(예: 1.99.0)
//
// 새 앱이 DB 를 열지 않도록, --wait-pid 로 넘기는 PID 는 끝나지 않는 더미 프로세스 — 새 앱은 기다리는 단계에서
// 확인만 하고 곧 종료시킨다.
func TestE2EInstallUpdate(t *testing.T) {
	oldExe, zipPath, ver := os.Getenv("SOBI_E2E_OLD"), os.Getenv("SOBI_E2E_ZIP"), os.Getenv("SOBI_E2E_VERSION")
	if oldExe == "" || zipPath == "" || ver == "" {
		t.Skip("SOBI_E2E_OLD / SOBI_E2E_ZIP / SOBI_E2E_VERSION 이 없음")
	}
	dir, err := os.MkdirTemp(filepath.Dir(t.TempDir()), "sobi-e2e-")
	if err != nil {
		t.Fatal(err)
	}
	// 작업 폴더를 임시 폴더 밖으로 쓰려면 SOBI_E2E_DIR 지정
	if d := os.Getenv("SOBI_E2E_DIR"); d != "" {
		os.RemoveAll(dir)
		dir = filepath.Join(d, fmt.Sprintf("run-%d", time.Now().UnixNano()))
		os.MkdirAll(dir, 0o755)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	target := filepath.Join(dir, "sobi-master.exe")
	if err := copyFile(oldExe, target); err != nil {
		t.Fatal(err)
	}
	before, err := readExeVersionInfo(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("설치 전: %+v", before)
	if compareVersions(before.ProductVersion, ver) >= 0 {
		t.Fatalf("옛 exe 가 이미 새 버전: %s", before.ProductVersion)
	}

	// 설치 자리 확인은 임시 폴더를 거절하므로 이 시험은 SOBI_E2E_DIR 가 임시 폴더 밖일 때만 확인한다
	if _, err := windowsInstallTarget(target); err != nil {
		t.Logf("windowsInstallTarget: %v (임시 폴더 안이라 자동 설치 불가 — 아래는 함수를 직접 이어 붙여 시험)", err)
	}

	// 끝나지 않는 더미 프로세스 (옛 앱 역할)
	ctx, cancel := context.WithCancel(context.Background())
	dummy := exec.CommandContext(ctx, "ping", "-n", "120", "127.0.0.1")
	if err := dummy.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); dummy.Wait() }()

	stage, staged, err := stageWindows(zipPath, target, func(s string) error { return verifyStagedExecutable(s, target, ver) })
	if err != nil {
		t.Fatalf("stage/verify: %v", err)
	}
	t.Logf("stage ok: %s", staged)

	// 버전이 다르면 확인에서 걸러야 한다
	if _, _, err := stageWindows(zipPath, target, func(s string) error { return verifyStagedExecutable(s, target, "9.9.9") }); err == nil {
		t.Error("다른 버전을 받아들임")
	} else {
		t.Logf("버전 불일치 거절: %v", err)
	}

	var child *exec.Cmd
	old, err := swapExecutable(target, staged, func() error {
		child = exec.Command(target, "--wait-pid", fmt.Sprint(dummy.Process.Pid))
		child.Dir = dir
		detachProcess(child)
		return child.Start()
	})
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	os.RemoveAll(stage)
	defer func() { child.Process.Kill(); child.Wait() }()

	after, err := readExeVersionInfo(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("설치 후: %+v, old=%s", after, filepath.Base(old))
	if compareVersions(after.ProductVersion, ver) != 0 {
		t.Errorf("제자리 exe 버전 %s, 기대 %s", after.ProductVersion, ver)
	}
	if o, err := readExeVersionInfo(old); err != nil || compareVersions(o.ProductVersion, before.ProductVersion) != 0 {
		t.Errorf(".old 가 옛 버전이 아님: %+v %v", o, err)
	}

	// 시작한 프로세스가 제자리 새 exe 인지, 더미를 기다리며 살아 있는지 (DB 는 열지 않음)
	time.Sleep(1500 * time.Millisecond)
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf("(Get-Process -Id %d -ErrorAction Stop).Path", child.Process.Pid)).Output()
	if err != nil {
		t.Fatalf("새 프로세스가 살아 있지 않음: %v", err)
	}
	if got := strings.TrimSpace(string(out)); !strings.EqualFold(got, target) {
		t.Errorf("새 프로세스 경로 %q, 기대 %q", got, target)
	}
	t.Logf("새 프로세스 PID %d 가 %s 에서 --wait-pid %d 를 기다리는 중", child.Process.Pid, target, dummy.Process.Pid)

	// 새 앱이 켜진 뒤의 정리: .old 지우기
	child.Process.Kill()
	child.Wait()
	removeOldExecutables(target)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf(".old 가 안 지워짐: %v", err)
	}
}
