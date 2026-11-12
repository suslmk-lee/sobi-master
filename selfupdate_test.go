package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"
)

func TestBundleFromExecutable(t *testing.T) {
	ok := map[string]string{
		"/Applications/sobi.app/Contents/MacOS/sobi":         "/Applications/sobi.app",
		"/Users/me/Applications/sobi.app/Contents/MacOS/sobi": "/Users/me/Applications/sobi.app",
	}
	for exe, want := range ok {
		if got, err := bundleFromExecutable(exe); err != nil || got != want {
			t.Errorf("%s → %q, %v", exe, got, err)
		}
	}
	for _, exe := range []string{
		"/usr/local/bin/sobi",                    // 번들이 아님 (go test 실행 파일 등)
		"/Applications/sobi/Contents/MacOS/sobi", // .app 이 아님
		"/private/var/folders/x/AppTranslocation/ABC/d/sobi.app/Contents/MacOS/sobi", // 격리 실행
		"/Volumes/sobi 1.3.0/sobi.app/Contents/MacOS/sobi",                           // DMG 안
	} {
		if got, err := bundleFromExecutable(exe); err == nil {
			t.Errorf("%s 는 자동 설치 불가여야 함: %q", exe, got)
		}
	}
}

func TestFindApp(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sobi.app"), 0o755)
	os.Symlink("/Applications", filepath.Join(dir, "Applications"))
	os.WriteFile(filepath.Join(dir, "처음 실행 안내.txt"), []byte("x"), 0o644)
	if got, err := findApp(dir, "sobi.app"); err != nil || got != filepath.Join(dir, "sobi.app") {
		t.Errorf("같은 이름: %q %v", got, err)
	}
	// 지금 앱 이름을 바꿔 두었어도 DMG 에 앱이 하나면 그걸 고른다
	if got, err := findApp(dir, "소비.app"); err != nil || got != filepath.Join(dir, "sobi.app") {
		t.Errorf("하나뿐인 앱: %q %v", got, err)
	}
	os.MkdirAll(filepath.Join(dir, "other.app"), 0o755)
	if _, err := findApp(dir, "소비.app"); err == nil {
		t.Error("앱이 둘인데 아무거나 고름")
	}
	if _, err := findApp(t.TempDir(), "sobi.app"); err == nil {
		t.Error("빈 DMG 인데 성공")
	}
}

func TestReadUpdateNotice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, updateMarker)
	write := func(m updateMarkerData) {
		b, _ := json.Marshal(m)
		os.WriteFile(path, b, 0o644)
	}
	now := time.Now().Format(time.RFC3339)

	if n := readUpdateNotice(path, "1.4.0"); n.Kind != "" {
		t.Errorf("표시가 없으면 알릴 것 없음: %+v", n)
	}
	write(updateMarkerData{From: "1.3.0", To: "1.4.0", At: now})
	if n := readUpdateNotice(path, "1.4.0"); n.Kind != "done" || n.From != "1.3.0" || n.To != "1.4.0" {
		t.Errorf("새 버전으로 켜짐: %+v", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("한 번 읽으면 표시를 지운다")
	}
	write(updateMarkerData{From: "1.3.0", To: "1.4.0", At: now})
	if n := readUpdateNotice(path, "1.3.0"); n.Kind != "failed" {
		t.Errorf("옛 버전으로 다시 켜짐 = 실패: %+v", n)
	}
	write(updateMarkerData{From: "1.3.0", To: "1.4.0", At: time.Now().Add(-48 * time.Hour).Format(time.RFC3339)})
	if n := readUpdateNotice(path, "1.3.0"); n.Kind != "" {
		t.Errorf("오래된 표시는 알리지 않음: %+v", n)
	}
	os.WriteFile(path, []byte("not json"), 0o644)
	if n := readUpdateNotice(path, "1.3.0"); n.Kind != "" {
		t.Errorf("망가진 표시: %+v", n)
	}
}

func TestCleanupUpdateLeftovers(t *testing.T) {
	apps := t.TempDir()
	cache := filepath.Join(t.TempDir(), "updates")
	oldStage := filepath.Join(apps, stagePrefix+"111")
	oldBackup := filepath.Join(apps, stagePrefix+"111-old")
	fresh := filepath.Join(apps, stagePrefix+"222")
	keep := filepath.Join(apps, "sobi.app")
	for _, d := range []string{oldStage, oldBackup, fresh, keep, cache} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(cache, "sobi-1.4.0.dmg"), []byte("x"), 0o644)
	past := time.Now().Add(-time.Hour)
	os.Chtimes(oldStage, past, past)
	os.Chtimes(oldBackup, past, past)

	cleanupUpdateLeftovers(apps, cache, time.Now())
	for _, gone := range []string{oldStage, oldBackup, cache} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s 가 남음", filepath.Base(gone))
		}
	}
	for _, stay := range []string{fresh, keep} { // 방금 만든 임시 폴더(설치 중일 수 있음)와 앱은 둔다
		if _, err := os.Stat(stay); err != nil {
			t.Errorf("%s 를 지움", filepath.Base(stay))
		}
	}
}

// swapFixture 는 도우미 시험용: 앱 폴더에 옛 앱, 그 안 임시 폴더에 새 앱, 내려받은 DMG, '앱'을 흉내 내는 프로세스.
type swapFixture struct {
	apps, target, stage, staged, dmg, opened, opener string
	app                                              *exec.Cmd
}

func newSwapFixture(t *testing.T) *swapFixture {
	t.Helper()
	if goruntime.GOOS == "windows" {
		t.Skip("도우미는 macOS 용 셸 스크립트")
	}
	f := &swapFixture{apps: t.TempDir()}
	f.target = filepath.Join(f.apps, "sobi.app")
	f.stage = filepath.Join(f.apps, stagePrefix+"abc")
	f.staged = filepath.Join(f.stage, "sobi.app")
	f.dmg = filepath.Join(t.TempDir(), "sobi-1.4.0.dmg")
	f.opened = filepath.Join(t.TempDir(), "opened")
	f.opener = filepath.Join(t.TempDir(), "opener.sh")
	os.MkdirAll(f.target, 0o755)
	os.MkdirAll(f.staged, 0o755)
	os.WriteFile(filepath.Join(f.target, "version"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(f.staged, "version"), []byte("new"), 0o644)
	os.WriteFile(f.dmg, []byte("dmg"), 0o644)
	// 앱 대신 연 경로를 적어 두는 '여는 명령'
	os.WriteFile(f.opener, []byte("#!/bin/sh\necho \"$1\" > '"+f.opened+"'\n"), 0o755)
	f.app = exec.Command("sleep", "30")
	if err := f.app.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.app.Process.Kill(); f.app.Wait() })
	return f
}

func (f *swapFixture) start(t *testing.T, waitTenths int) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "sobi.log")
	if err := startSwapHelper(f.app.Process.Pid, f.target, f.staged, f.stage, f.dmg, f.opener, waitTenths, logPath); err != nil {
		t.Fatal(err)
	}
}

// quit 은 앱이 끝난 것처럼 한다. 기다려 거둬 줘야 좀비로 남지 않아 도우미가 끝난 걸 안다.
func (f *swapFixture) quit() {
	f.app.Process.Kill()
	f.app.Wait()
}

func (f *swapFixture) waitOpened(t *testing.T) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(f.opened); err == nil && len(b) > 0 {
			return strings.TrimSpace(string(b))
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("도우미가 앱을 다시 열지 않음")
	return ""
}

func readVersion(t *testing.T, app string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(app, "version"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// 도우미는 앱이 끝날 때까지 손대지 않고, 끝나면 새 앱으로 바꾸고 정리한 뒤 같은 경로를 연다.
func TestSwapHelper(t *testing.T) {
	f := newSwapFixture(t)
	f.start(t, 300)

	time.Sleep(400 * time.Millisecond)
	if readVersion(t, f.target) != "old" {
		t.Fatal("앱이 아직 도는데 바꿔 버림")
	}
	if _, err := os.Stat(f.opened); err == nil {
		t.Fatal("앱이 아직 도는데 다시 엶")
	}

	f.quit()
	if got := f.waitOpened(t); got != f.target {
		t.Errorf("연 경로 %q", got)
	}
	if readVersion(t, f.target) != "new" {
		t.Error("새 앱으로 바뀌지 않음")
	}
	for _, gone := range []string{f.stage, f.stage + "-old", f.dmg} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s 가 남음", gone)
		}
	}
}

// 새 앱을 제자리에 놓지 못하면 옛 앱을 되돌려 놓고 연다. 내려받은 파일은 남긴다(직접 설치용).
func TestSwapHelperRollback(t *testing.T) {
	f := newSwapFixture(t)
	os.RemoveAll(f.staged) // 새 앱이 사라진 상황 → 옮기기 실패
	f.start(t, 300)
	f.quit()
	if got := f.waitOpened(t); got != f.target {
		t.Errorf("연 경로 %q", got)
	}
	if readVersion(t, f.target) != "old" {
		t.Error("옛 앱이 돌아오지 않음")
	}
	if _, err := os.Stat(f.dmg); err != nil {
		t.Error("실패했는데 설치 파일을 지움")
	}
}

// 앱이 한도 안에 끝나지 않으면 아무것도 바꾸지 않고 그만둔다.
func TestSwapHelperTimeout(t *testing.T) {
	f := newSwapFixture(t)
	f.start(t, 2) // 0.2초
	time.Sleep(800 * time.Millisecond)
	f.quit()
	time.Sleep(300 * time.Millisecond)
	if readVersion(t, f.target) != "old" {
		t.Error("기다림이 끝났는데 바꿈")
	}
	if _, err := os.Stat(f.opened); err == nil {
		t.Error("그만뒀는데 다시 엶")
	}
}

// fakeBundle 은 서명된 작은 앱 번들을 만든다(실행 파일은 /usr/bin/true 복사본).
func fakeBundle(t *testing.T, dir, id, version string, sign bool) string {
	t.Helper()
	app := filepath.Join(dir, "sobi.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>sobi</string>
<key>CFBundleIdentifier</key><string>` + id + `</string>
<key>CFBundleName</key><string>sobi</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>` + version + `</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "/usr/bin/true", filepath.Join(app, "Contents", "MacOS", "sobi")).CombinedOutput(); err != nil {
		t.Fatalf("cp: %s", out)
	}
	if sign {
		if out, err := exec.Command("codesign", "--force", "--deep", "--sign", "-", app).CombinedOutput(); err != nil {
			t.Fatalf("codesign: %s", out)
		}
	}
	return app
}

// 실제 DMG 를 만들어 마운트·확인·복사까지 해 본다 (macOS).
func TestStageUpdate(t *testing.T) {
	if goruntime.GOOS != "darwin" {
		t.Skip("macOS 전용")
	}
	if testing.Short() {
		t.Skip("DMG 를 만들어 15초쯤 걸린다")
	}
	for _, tool := range []string{"hdiutil", "codesign", "ditto"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " 없음")
		}
	}
	src := t.TempDir()
	fakeBundle(t, src, "com.example.sobi", "1.4.0", true)
	os.Symlink("/Applications", filepath.Join(src, "Applications"))
	dmg := filepath.Join(t.TempDir(), "sobi-1.4.0-macOS-universal.dmg")
	if out, err := exec.Command("hdiutil", "create", "-volname", "sobi test", "-srcfolder", src,
		"-fs", "HFS+", "-format", "UDZO", "-ov", dmg).CombinedOutput(); err != nil {
		t.Skipf("hdiutil create 불가: %s", out)
	}

	apps := t.TempDir()
	target := fakeBundle(t, apps, "com.example.sobi", "1.3.0", false)
	leftovers := func() []string {
		m, _ := filepath.Glob(filepath.Join(apps, stagePrefix+"*"))
		return m
	}

	// 버전이 다르거나 다른 앱이면 복사하지 않는다
	if _, _, err := stageUpdate(dmg, target, "1.4.1"); err == nil || !strings.Contains(err.Error(), "버전") {
		t.Errorf("버전 확인: %v", err)
	}
	other := fakeBundle(t, t.TempDir(), "com.example.other", "1.3.0", false)
	if _, _, err := stageUpdate(dmg, other, "1.4.0"); err == nil || !strings.Contains(err.Error(), "다릅니다") {
		t.Errorf("번들 ID 확인: %v", err)
	}
	if l := leftovers(); len(l) != 0 {
		t.Errorf("실패했는데 임시 폴더가 남음: %v", l)
	}

	stage, staged, err := stageUpdate(dmg, target, "1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(stage) != apps || filepath.Dir(staged) != stage || filepath.Base(staged) != "sobi.app" {
		t.Errorf("임시 폴더는 앱 옆에: %s %s", stage, staged)
	}
	if v, err := plistValue(staged, "CFBundleShortVersionString"); err != nil || v != "1.4.0" {
		t.Errorf("복사한 앱 버전 %q %v", v, err)
	}
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", staged).CombinedOutput(); err != nil {
		t.Errorf("복사하며 서명이 깨짐: %s", out)
	}
	// 끝나면 DMG 를 떼어 둔다
	if out, _ := exec.Command("hdiutil", "info").Output(); strings.Contains(string(out), dmg) {
		t.Error("DMG 가 마운트된 채 남음")
	}

	// 서명이 깨진 앱이 든 DMG 는 거절한다
	os.RemoveAll(stage)
	bad := t.TempDir()
	app := fakeBundle(t, bad, "com.example.sobi", "1.4.0", true)
	os.WriteFile(filepath.Join(app, "Contents", "MacOS", "sobi"), []byte("#!/bin/sh\n"), 0o755)
	badDMG := filepath.Join(t.TempDir(), "bad.dmg")
	if out, err := exec.Command("hdiutil", "create", "-volname", "bad", "-srcfolder", bad, "-fs", "HFS+", "-format", "UDZO", "-ov", badDMG).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil: %s", out)
	}
	if _, _, err := stageUpdate(badDMG, target, "1.4.0"); err == nil || !strings.Contains(err.Error(), "서명") {
		t.Errorf("서명 확인: %v", err)
	}
	if l := leftovers(); len(l) != 0 {
		t.Errorf("실패했는데 임시 폴더가 남음: %v", l)
	}
}
