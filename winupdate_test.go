package main

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestParseWaitPID(t *testing.T) {
	cases := []struct {
		args []string
		pid  int
		ok   bool
	}{
		{nil, 0, false},
		{[]string{"--wait-pid", "1234"}, 1234, true},
		{[]string{"-x", "--wait-pid=77", "y"}, 77, true},
		{[]string{"--wait-pid"}, 0, false},
		{[]string{"--wait-pid", "abc"}, 0, false},
		{[]string{"--wait-pid", "-5"}, 0, false},
		{[]string{"--wait-pid", "0"}, 0, false},
		{[]string{"--other", "9"}, 0, false},
		{[]string{"--wait-pid", "x", "--wait-pid", "42"}, 42, true},
	}
	for _, c := range cases {
		if pid, ok := parseWaitPID(c.args); pid != c.pid || ok != c.ok {
			t.Errorf("%v → %d,%v want %d,%v", c.args, pid, ok, c.pid, c.ok)
		}
	}
}

func TestExtractZipSlip(t *testing.T) {
	for _, bad := range []string{"../evil.exe", "a/../../evil.exe", "/abs.exe", `\abs.exe`, "C:/evil.exe", "x:evil"} {
		dir := t.TempDir()
		z := filepath.Join(dir, "a.zip")
		writeZip(t, z, map[string]string{bad: "x"})
		dest := filepath.Join(dir, "out")
		os.MkdirAll(dest, 0o755)
		if err := extractZip(z, dest); err == nil {
			t.Errorf("%q 를 풀었음", bad)
		}
		if _, err := os.Stat(filepath.Join(dir, "evil.exe")); err == nil {
			t.Errorf("%q 가 밖에 파일을 만듦", bad)
		}
	}
}

func TestExtractZipOK(t *testing.T) {
	dir := t.TempDir()
	z := filepath.Join(dir, "a.zip")
	writeZip(t, z, map[string]string{"sobi-master.exe": "NEW", "sub/readme.txt": "hi"})
	dest := filepath.Join(dir, "out")
	os.MkdirAll(dest, 0o755)
	if err := extractZip(z, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "sobi-master.exe")); string(b) != "NEW" {
		t.Errorf("exe 내용: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "sub", "readme.txt")); string(b) != "hi" {
		t.Errorf("하위 폴더 파일: %q", b)
	}
}

func TestFindStagedExe(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "Other.EXE"), []byte("x"), 0o755)
	if got, err := findStagedExe(dir, "sobi-master.exe"); err != nil || filepath.Base(got) != "Other.EXE" {
		t.Errorf("하나뿐인 exe: %q %v", got, err)
	}
	os.WriteFile(filepath.Join(dir, "SOBI-master.exe"), []byte("x"), 0o755)
	if got, err := findStagedExe(dir, "sobi-master.exe"); err != nil || filepath.Base(got) != "SOBI-master.exe" {
		t.Errorf("같은 이름(대소문자 무시): %q %v", got, err)
	}
	empty := t.TempDir()
	if _, err := findStagedExe(empty, "sobi-master.exe"); err == nil {
		t.Error("exe 가 없는데 성공")
	}
	os.WriteFile(filepath.Join(empty, "a.exe"), []byte("x"), 0o755)
	os.WriteFile(filepath.Join(empty, "b.exe"), []byte("x"), 0o755)
	if _, err := findStagedExe(empty, "sobi-master.exe"); err == nil {
		t.Error("exe 가 둘인데 아무거나 고름")
	}
}

func TestStageWindows(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sobi-master.exe")
	os.WriteFile(target, []byte("OLD"), 0o755)

	z := filepath.Join(t.TempDir(), "sobi-1.2.0-windows-amd64.zip")
	writeZip(t, z, map[string]string{"sobi-master.exe": "NEW"})
	var verified string
	stage, staged, err := stageWindows(z, target, func(s string) error { verified = s; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(stage) != dir || !strings.HasPrefix(filepath.Base(stage), stagePrefix) {
		t.Errorf("임시 폴더는 exe 옆의 숨은 폴더여야 함: %s", stage)
	}
	if verified != staged {
		t.Errorf("verify 에 새 exe 경로가 전달돼야 함: %q vs %q", verified, staged)
	}
	if b, _ := os.ReadFile(staged); string(b) != "NEW" {
		t.Errorf("새 exe: %q", b)
	}

	// 확인 실패 → 임시 폴더를 지운다
	_, _, err = stageWindows(z, target, func(string) error { return errors.New("버전이 다름") })
	if err == nil {
		t.Fatal("verify 실패인데 성공")
	}
	left, _ := filepath.Glob(filepath.Join(dir, stagePrefix+"*"))
	if len(left) != 1 { // 앞서 성공한 임시 폴더 하나만 남아 있어야 한다
		t.Errorf("실패한 임시 폴더가 남음: %v", left)
	}

	// zip-slip 은 거절하고 임시 폴더를 지운다
	bad := filepath.Join(t.TempDir(), "bad.zip")
	writeZip(t, bad, map[string]string{"../evil.exe": "x"})
	if _, _, err := stageWindows(bad, target, nil); err == nil {
		t.Error("zip-slip 을 받아들임")
	}
	left, _ = filepath.Glob(filepath.Join(dir, stagePrefix+"*"))
	if len(left) != 1 {
		t.Errorf("zip-slip 실패 뒤 임시 폴더가 남음: %v", left)
	}

	// 맨 exe 는 복사
	raw := filepath.Join(t.TempDir(), "sobi-1.2.0-windows.exe")
	os.WriteFile(raw, []byte("RAW"), 0o755)
	_, staged2, err := stageWindows(raw, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(staged2); string(b) != "RAW" || filepath.Base(staged2) != "sobi-master.exe" {
		t.Errorf("exe 복사: %q %s", b, staged2)
	}

	if _, _, err := stageWindows(filepath.Join(dir, "x.dmg"), target, nil); err == nil {
		t.Error("지원하지 않는 확장자")
	}
}

func TestSwapExecutable(t *testing.T) {
	setup := func() (target, staged string) {
		dir := t.TempDir()
		target = filepath.Join(dir, "app.exe")
		staged = filepath.Join(dir, "stage", "app.exe")
		os.MkdirAll(filepath.Dir(staged), 0o755)
		os.WriteFile(target, []byte("OLD"), 0o755)
		os.WriteFile(staged, []byte("NEW"), 0o755)
		return
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }

	// 성공
	target, staged := setup()
	started := false
	old, err := swapExecutable(target, staged, func() error { started = true; return nil })
	if err != nil || !started {
		t.Fatalf("swap: %v started=%v", err, started)
	}
	if read(target) != "NEW" || read(old) != "OLD" || old != target+".old" {
		t.Errorf("target=%q old(%s)=%q", read(target), old, read(old))
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Error("새 exe 가 임시 폴더에 남음")
	}

	// 이미 .old 가 있어도(지울 수 있으면) 덮어쓴다
	target, staged = setup()
	os.WriteFile(target+".old", []byte("ANCIENT"), 0o644)
	old, err = swapExecutable(target, staged, nil)
	if err != nil || old != target+".old" || read(old) != "OLD" || read(target) != "NEW" {
		t.Errorf("기존 .old: %v %s %q", err, old, read(old))
	}

	// 시작 실패 → 모두 되돌림
	target, staged = setup()
	if _, err := swapExecutable(target, staged, func() error { return errors.New("실행 안 됨") }); err == nil {
		t.Fatal("start 실패인데 성공")
	}
	if read(target) != "OLD" || read(staged) != "NEW" {
		t.Errorf("되돌림 실패: target=%q staged=%q", read(target), read(staged))
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Error("되돌린 뒤 .old 가 남음")
	}

	// 새 exe 가 없으면(두 번째 이름 바꾸기 실패) 옛 exe 를 되돌린다
	target, staged = setup()
	os.Remove(staged)
	if _, err := swapExecutable(target, staged, nil); err == nil {
		t.Fatal("새 exe 가 없는데 성공")
	}
	if read(target) != "OLD" {
		t.Errorf("옛 exe 를 되돌리지 못함: %q", read(target))
	}
}

func TestRemoveOldExecutables(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "sobi-master.exe")
	for _, n := range []string{"sobi-master.exe", "sobi-master.exe.old", "sobi-master.exe.old-123", "other.exe.old", "notes.txt"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	removeOldExecutables(exe)
	for n, want := range map[string]bool{
		"sobi-master.exe": true, "sobi-master.exe.old": false, "sobi-master.exe.old-123": false,
		"other.exe.old": true, "notes.txt": true,
	} {
		_, err := os.Stat(filepath.Join(dir, n))
		if (err == nil) != want {
			t.Errorf("%s 존재=%v, 기대=%v", n, err == nil, want)
		}
	}
}

func TestWindowsInstallTarget(t *testing.T) {
	if _, err := windowsInstallTarget(filepath.Join(t.TempDir(), "app.bin")); err == nil {
		t.Error(".exe 가 아닌데 성공")
	}
	// 임시 폴더 안은 거절
	if _, err := windowsInstallTarget(filepath.Join(os.TempDir(), "x", "app.exe")); err == nil {
		t.Error("임시 폴더인데 성공")
	}
	// 임시 폴더가 아닌 쓸 수 있는 폴더는 통과 (작업 폴더)
	wd, _ := os.Getwd()
	if hasPathPrefixFold(wd, os.TempDir()) {
		t.Skip("작업 폴더가 임시 폴더 안")
	}
	exe := filepath.Join(wd, "app.exe")
	if got, err := windowsInstallTarget(exe); err != nil || got != exe {
		t.Errorf("쓸 수 있는 폴더: %q %v", got, err)
	}
	left, _ := filepath.Glob(filepath.Join(wd, stagePrefix+"*"))
	if len(left) != 0 {
		t.Errorf("확인용 임시 폴더가 남음: %v", left)
	}
}

func TestAutoInstallable(t *testing.T) {
	for _, c := range []struct {
		goos, name string
		want       bool
	}{
		{"darwin", "a.DMG", true}, {"darwin", "a.zip", false},
		{"windows", "a.zip", true}, {"windows", "A.EXE", true}, {"windows", "a.dmg", false},
		{"linux", "a.zip", false},
	} {
		if got := autoInstallable(c.goos, c.name); got != c.want {
			t.Errorf("%s %s = %v", c.goos, c.name, got)
		}
	}
}
