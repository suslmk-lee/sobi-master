package main

// 내려받은 새 버전을 앱이 직접 설치하고 다시 시작한다 (Windows).
//
//  1. 내려받은 zip(또는 exe)을 지금 exe 가 있는 폴더 안의 숨은 임시 폴더에 푼다 — 같은 드라이브여야
//     바꿔치기가 이름 바꾸기 두 번으로 끝난다. zip 안의 경로 탈출(zip-slip)·터무니없는 크기는 거른다.
//  2. 푼 exe 의 버전 정보(리소스)가 받으려던 버전이고 제품 이름이 지금 앱과 같은지 확인한다(selfupdate_windows.go).
//  3. Windows 는 실행 중인 exe 도 이름은 바꿀 수 있다: 지금 exe → <exe>.old, 새 exe → 원래 자리.
//     새 exe 를 `--wait-pid <지금 PID>` 로 띄우면 새 앱은 이 앱이 완전히 끝나길(종료 전 백업 포함) 기다렸다가 시작한다.
//     띄우지 못하면 이름을 되돌리고 오류를 돌려준다(앱은 종료하지 않는다).
//  4. 표시 파일을 남기고 앱을 종료한다. 새 앱은 켜질 때 .old 파일과 임시 폴더를 치운다.
//
// 이 파일은 운영체제에 상관없는 코드(테스트를 어디서나 돌릴 수 있게)이고,
// 버전 리소스 읽기만 selfupdate_windows.go / selfupdate_other.go 에 있다.

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// waitPIDLimit 는 --wait-pid 로 옛 앱이 끝나길 기다리는 한도 — 종료 전 백업이 있어 넉넉히 3분.
	waitPIDLimit = 3 * time.Minute
	// zip 을 풀 때의 한도 (압축 폭탄 방지)
	maxZipEntries = 2000
	maxZipBytes   = 1 << 29 // 512MB
)

// parseWaitPID 는 실행 인자에서 `--wait-pid N`(또는 `--wait-pid=N`)을 찾는다. 모르는·깨진 인자는 무시한다.
func parseWaitPID(args []string) (int, bool) {
	for i, a := range args {
		v := ""
		switch {
		case a == "--wait-pid" && i+1 < len(args):
			v = args[i+1]
		case strings.HasPrefix(a, "--wait-pid="):
			v = strings.TrimPrefix(a, "--wait-pid=")
		default:
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// windowsInstallTarget 은 exe 가 자동 설치로 바꿔 놓을 수 있는 자리인지 확인하고 그 경로를 돌려준다.
// .exe 여야 하고, 임시 폴더 안이 아니어야 하며, 같은 폴더에 임시 폴더를 만들 수 있어야 한다.
func windowsInstallTarget(exe string) (string, error) {
	if !strings.EqualFold(filepath.Ext(exe), ".exe") {
		return "", fmt.Errorf("실행 파일(.exe)에서 실행 중이 아닙니다")
	}
	dir := filepath.Dir(exe)
	if tmp := filepath.Clean(os.TempDir()); tmp != "" && (strings.EqualFold(dir, tmp) || hasPathPrefixFold(dir, tmp)) {
		return "", fmt.Errorf("임시 폴더에서 실행 중입니다. 앱을 다른 폴더에 두고 다시 열어 주세요")
	}
	probe, err := os.MkdirTemp(dir, stagePrefix)
	if err != nil {
		return "", fmt.Errorf("%s 폴더에 쓸 수 없습니다", dir)
	}
	os.RemoveAll(probe)
	return exe, nil
}

func hasPathPrefixFold(path, prefix string) bool {
	p := strings.ToLower(filepath.Clean(path)) + string(filepath.Separator)
	q := strings.ToLower(filepath.Clean(prefix)) + string(filepath.Separator)
	return strings.HasPrefix(p, q)
}

// installTargetWindows 는 지금 실행 중인 exe 경로. 자동 설치를 못 하는 곳이면 오류.
func installTargetWindows() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return windowsInstallTarget(exe)
}

// extractZip 는 zip 을 dest 안에 푼다. dest 밖으로 나가는 경로·심볼릭 링크·너무 큰 압축은 거절한다.
func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("설치 파일(zip)을 열지 못했습니다: %w", err)
	}
	defer r.Close()
	if len(r.File) > maxZipEntries {
		return fmt.Errorf("설치 파일에 파일이 너무 많습니다")
	}
	base := filepath.Clean(dest)
	var total int64
	for _, f := range r.File {
		name := f.Name
		if name == "" || strings.ContainsAny(name, ":\x00") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
			return fmt.Errorf("설치 파일에 올바르지 않은 경로가 있습니다: %q", name)
		}
		out := filepath.Join(base, filepath.FromSlash(name))
		if out != base && !strings.HasPrefix(out, base+string(filepath.Separator)) {
			return fmt.Errorf("설치 파일에 올바르지 않은 경로가 있습니다: %q", name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("설치 파일에 바로가기(링크)가 들어 있습니다: %q", name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > maxZipBytes || f.UncompressedSize64 > maxZipBytes {
			return fmt.Errorf("설치 파일이 너무 큽니다")
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(f, out); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, out string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	// 헤더에 적힌 크기보다 많이 나오면 거짓말한 압축 — 한도까지만 읽고 거절
	n, err := io.Copy(w, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err == nil && n != int64(f.UncompressedSize64) {
		err = fmt.Errorf("설치 파일의 크기가 맞지 않습니다: %s", f.Name)
	}
	return err
}

// findStagedExe 는 푼 폴더에서 설치할 exe 를 고른다: 지금 exe 와 같은 이름 → 하나뿐인 .exe.
func findStagedExe(dir, want string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var exes []string
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".exe") {
			continue
		}
		if strings.EqualFold(e.Name(), want) {
			return filepath.Join(dir, e.Name()), nil
		}
		exes = append(exes, e.Name())
	}
	if len(exes) != 1 {
		return "", fmt.Errorf("설치 파일에서 실행 파일(.exe)을 찾지 못했습니다")
	}
	return filepath.Join(dir, exes[0]), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// stageWindows 는 내려받은 zip/exe 를 target 옆 임시 폴더에 풀고 verify 로 확인한다.
// 돌려주는 값: 임시 폴더, 그 안의 새 exe. 실패하면 임시 폴더를 지운다.
func stageWindows(src, target string, verify func(staged string) error) (stage, staged string, err error) {
	stage, err = os.MkdirTemp(filepath.Dir(target), stagePrefix)
	if err != nil {
		return "", "", fmt.Errorf("%s 폴더에 쓸 수 없습니다: %w", filepath.Dir(target), err)
	}
	fail := func(err error) (string, string, error) {
		os.RemoveAll(stage)
		return "", "", err
	}
	switch strings.ToLower(filepath.Ext(src)) {
	case ".zip":
		if err := extractZip(src, stage); err != nil {
			return fail(err)
		}
		if staged, err = findStagedExe(stage, filepath.Base(target)); err != nil {
			return fail(err)
		}
	case ".exe":
		staged = filepath.Join(stage, filepath.Base(target))
		if err := copyFile(src, staged); err != nil {
			return fail(fmt.Errorf("새 앱을 복사하지 못했습니다: %w", err))
		}
	default:
		return fail(fmt.Errorf("자동 설치할 수 없는 파일입니다: %s", filepath.Base(src)))
	}
	if verify != nil {
		if err := verify(staged); err != nil {
			return fail(err)
		}
	}
	return stage, staged, nil
}

// swapExecutable 은 지금 exe(target)를 .old 로 비키고 새 exe(staged)를 제자리에 놓은 뒤 start 를 부른다.
// 어느 단계든 실패하면 모두 원래대로 되돌리고 오류를 돌려준다. 성공하면 비켜 둔 옛 파일 경로를 돌려준다.
func swapExecutable(target, staged string, start func() error) (old string, err error) {
	old = target + ".old"
	if _, err := os.Lstat(old); err == nil {
		if os.Remove(old) != nil { // 지난 .old 가 아직 잠겨 있으면 다른 이름을 쓴다
			old = fmt.Sprintf("%s.old-%d", target, time.Now().UnixNano())
		}
	}
	if err := os.Rename(target, old); err != nil {
		return "", fmt.Errorf("지금 앱을 비키지 못했습니다: %w", err)
	}
	if err := os.Rename(staged, target); err != nil {
		os.Rename(old, target)
		return "", fmt.Errorf("새 앱을 제자리에 놓지 못했습니다: %w", err)
	}
	if start != nil {
		if err := start(); err != nil {
			os.Rename(target, staged)
			if rerr := os.Rename(old, target); rerr != nil {
				return "", fmt.Errorf("%w (옛 앱을 되돌리지도 못했습니다: %v)", err, rerr)
			}
			return "", err
		}
	}
	return old, nil
}

// startNewExecutable 은 새 앱을 `--wait-pid <pid>` 로 앱과 떨어진 프로세스로 띄운다.
func startNewExecutable(exe string, pid int) error {
	cmd := exec.Command(exe, "--wait-pid", strconv.Itoa(pid))
	cmd.Dir = filepath.Dir(exe)
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("새 앱을 시작하지 못했습니다: %w", err)
	}
	return cmd.Process.Release()
}

// removeOldExecutables 는 exe 옆에 비켜 둔 옛 파일(<exe>.old, <exe>.old-…)을 지운다. 잠겨 있으면 건너뛴다.
func removeOldExecutables(exe string) {
	entries, _ := os.ReadDir(filepath.Dir(exe))
	prefix := strings.ToLower(filepath.Base(exe)) + ".old"
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), prefix) {
			os.Remove(filepath.Join(filepath.Dir(exe), e.Name()))
		}
	}
}

// installUpdateWindows 는 내려받은 zip/exe 로 새 버전을 설치하고 앱을 다시 시작한다.
func (a *App) installUpdateWindows(src, version string) error {
	target, err := installTarget()
	if err != nil {
		return err
	}
	current := target
	stage, staged, err := stageWindows(src, target, func(staged string) error {
		return verifyStagedExecutable(staged, current, version)
	})
	if err != nil {
		return err
	}
	if _, err := swapExecutable(target, staged, func() error { return startNewExecutable(target, os.Getpid()) }); err != nil {
		os.RemoveAll(stage)
		return err
	}
	os.RemoveAll(stage) // 새 exe 는 이미 제자리로 옮겼다 — 남은 임시 폴더만 치운다
	a.finishInstall(version)
	return nil
}
