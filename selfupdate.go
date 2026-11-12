package main

// 내려받은 새 버전을 앱이 직접 설치하고 다시 시작한다 (macOS).
//
//  1. 내려받은 DMG 를 화면에 안 보이게 마운트해 안의 sobi.app 을 꺼낸다.
//  2. 꺼낸 앱이 같은 앱(번들 ID)이고 받으려던 버전이며 서명이 온전한지 확인한다.
//  3. 지금 앱이 있는 폴더(보통 /Applications) 안의 숨은 임시 폴더로 복사해 둔다 —
//     같은 디스크에 있어야 바꿔치기가 이름 바꾸기 한 번으로 끝난다.
//  4. 작은 셸 도우미를 띄워 두고 앱을 종료한다. 도우미는 앱이 완전히 끝나길(종료 전 백업 포함) 기다렸다가
//     옛 앱을 비켜 두고 새 앱을 제자리에 놓은 뒤 연다. 새 앱을 놓지 못하면 옛 앱을 되돌려 놓고 연다.
//
// 3번까지 실패하면 앱은 종료하지 않고 오류를 돌려준다(화면은 설치 파일을 직접 여는 방법으로 넘어간다).
// 앱이 직접 받은 파일이라 격리 표시(quarantine)가 없어 새 앱도 Gatekeeper 경고 없이 열린다.
// 같은 경로에 같은 이름으로 놓으므로 Dock 에 고정한 아이콘도 그대로 새 앱을 가리킨다.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sobi/internal/store"
)

const (
	// stagePrefix 는 앱 폴더 안에 새 앱을 복사해 두는 숨은 임시 폴더의 이름 앞부분.
	stagePrefix = ".sobi-update-"
	// updateMarker 는 설치를 시작했다는 표시 파일(설정 폴더). 다시 켜진 앱이 성공·실패를 알려 주는 데 쓴다.
	updateMarker = "update-pending.json"
	// swapWaitTenths 는 도우미가 앱이 끝나길 기다리는 한도(0.1초 단위) — 종료 전 백업이 있어 넉넉히 3분.
	swapWaitTenths = 1800
)

// swapScript 는 앱이 끝난 뒤 옛 앱과 새 앱을 바꾸고 다시 여는 도우미.
// 인자: $1 앱 PID, $2 지금 앱 경로, $3 새 앱(임시 폴더 안), $4 임시 폴더, $5 내려받은 DMG, $6 여는 명령, $7 기다리는 한도.
const swapScript = `
pid="$1"; target="$2"; staged="$3"; stage="$4"; dmg="$5"; opener="$6"; limit="$7"
say() { echo "$(date '+%Y/%m/%d %H:%M:%S') 업데이트 도우미: $*"; }
n=0
while kill -0 "$pid" 2>/dev/null; do
  n=$((n+1))
  if [ "$n" -gt "$limit" ]; then say "앱이 끝나지 않아 설치를 그만둡니다"; exit 1; fi
  sleep 0.1
done
old="$stage-old"
if mv "$target" "$old"; then
  if mv "$staged" "$target"; then
    say "설치했습니다 ($target)"
    rm -rf "$old" "$stage"
    rm -f "$dmg"
  else
    say "새 앱을 옮기지 못해 되돌립니다"
    mv "$old" "$target"
  fi
else
  say "지금 앱을 비키지 못했습니다"
fi
exec "$opener" "$target"
`

// updateMarkerData 는 설치를 시작할 때 남기는 표시.
type updateMarkerData struct {
	From string `json:"from"`
	To   string `json:"to"`
	At   string `json:"at"`
}

// UpdateNotice 는 업데이트 뒤 처음 켜졌을 때 화면에 한 번 알릴 결과.
type UpdateNotice struct {
	Kind string `json:"kind"` // "" 없음, "done" 새 버전으로 바뀜, "failed" 바뀌지 않음
	From string `json:"from"`
	To   string `json:"to"`
}

// bundleFromExecutable 은 실행 파일 경로(…/sobi.app/Contents/MacOS/sobi)에서 앱 번들 경로를 찾는다.
// 바꿔 놓을 수 없는 곳(설치 파일 안, 격리 실행 위치)에서 실행 중이면 오류.
func bundleFromExecutable(exe string) (string, error) {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(bundle, ".app") {
		return "", fmt.Errorf("앱 번들(.app)에서 실행 중이 아닙니다")
	}
	if strings.Contains(bundle, "/AppTranslocation/") {
		return "", fmt.Errorf("격리된 위치에서 실행 중입니다. 앱을 Applications 폴더로 옮긴 뒤 다시 열어 주세요")
	}
	if strings.HasPrefix(bundle, "/Volumes/") {
		return "", fmt.Errorf("설치 파일(DMG) 안에서 실행 중입니다. Applications 폴더에 설치한 앱에서 업데이트해 주세요")
	}
	return bundle, nil
}

// installTarget 은 새 버전으로 바꿔 놓을 지금 앱 번들 경로. 이 컴퓨터에서 자동 설치를 못 하면 오류.
func installTarget() (string, error) {
	if goruntime.GOOS != "darwin" {
		return "", fmt.Errorf("자동 설치는 macOS 에서만 됩니다")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	bundle, err := bundleFromExecutable(exe)
	if err != nil {
		return "", err
	}
	// 같은 폴더에 임시 폴더를 만들 수 있어야 바꿔 놓을 수 있다
	probe, err := os.MkdirTemp(filepath.Dir(bundle), stagePrefix)
	if err != nil {
		return "", fmt.Errorf("%s 폴더에 쓸 수 없습니다", filepath.Dir(bundle))
	}
	os.RemoveAll(probe)
	return bundle, nil
}

// plistValue 는 앱 번들의 Info.plist 에서 값 하나를 읽는다.
func plistValue(bundle, key string) (string, error) {
	out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :"+key, filepath.Join(bundle, "Contents", "Info.plist")).Output()
	if err != nil {
		return "", fmt.Errorf("%s 의 %s 를 읽지 못했습니다", filepath.Base(bundle), key)
	}
	return strings.TrimSpace(string(out)), nil
}

// findApp 은 마운트한 DMG 에서 설치할 앱을 고른다: 지금 앱과 같은 이름 → 하나뿐인 .app.
func findApp(dir, want string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var apps []string
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".app") { // Applications 바로가기(심볼릭 링크)는 IsDir 가 아니다
			if e.Name() == want {
				return filepath.Join(dir, e.Name()), nil
			}
			apps = append(apps, e.Name())
		}
	}
	if len(apps) != 1 {
		return "", fmt.Errorf("설치 파일에서 앱을 찾지 못했습니다")
	}
	return filepath.Join(dir, apps[0]), nil
}

// stageUpdate 는 DMG 안의 앱을 확인하고 target 옆 임시 폴더로 복사한다. 돌려주는 값: 임시 폴더, 그 안의 새 앱.
func stageUpdate(dmg, target, wantVersion string) (stage, staged string, err error) {
	mnt, err := os.MkdirTemp("", "sobi-dmg-")
	if err != nil {
		return "", "", err
	}
	defer os.Remove(mnt)
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-noautoopen", "-mountpoint", mnt, dmg).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("설치 파일을 열지 못했습니다: %s", strings.TrimSpace(string(out)))
	}
	defer func() {
		if exec.Command("hdiutil", "detach", mnt, "-quiet").Run() != nil {
			exec.Command("hdiutil", "detach", mnt, "-force", "-quiet").Run()
		}
	}()

	src, err := findApp(mnt, filepath.Base(target))
	if err != nil {
		return "", "", err
	}
	wantID, err := plistValue(target, "CFBundleIdentifier")
	if err != nil {
		return "", "", err
	}
	if id, err := plistValue(src, "CFBundleIdentifier"); err != nil {
		return "", "", err
	} else if id != wantID {
		return "", "", fmt.Errorf("설치 파일의 앱이 이 앱과 다릅니다 (%s)", id)
	}
	if v, err := plistValue(src, "CFBundleShortVersionString"); err != nil {
		return "", "", err
	} else if compareVersions(v, wantVersion) != 0 {
		return "", "", fmt.Errorf("설치 파일의 버전(%s)이 받으려던 버전(%s)과 다릅니다", v, wantVersion)
	}
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", src).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("설치 파일의 앱 서명이 온전하지 않습니다: %s", strings.TrimSpace(string(out)))
	}

	stage, err = os.MkdirTemp(filepath.Dir(target), stagePrefix)
	if err != nil {
		return "", "", fmt.Errorf("%s 폴더에 쓸 수 없습니다: %w", filepath.Dir(target), err)
	}
	staged = filepath.Join(stage, filepath.Base(target))
	if out, err := exec.Command("ditto", src, staged).CombinedOutput(); err != nil {
		os.RemoveAll(stage)
		return "", "", fmt.Errorf("새 앱을 복사하지 못했습니다: %s", strings.TrimSpace(string(out)))
	}
	return stage, staged, nil
}

// startSwapHelper 는 앱이 끝난 뒤 바꿔치기할 도우미를 앱과 떨어진 프로세스로 띄운다.
func startSwapHelper(pid int, target, staged, stage, dmg, opener string, waitTenths int, logPath string) error {
	cmd := exec.Command("/bin/sh", "-c", swapScript, "sobi-update",
		strconv.Itoa(pid), target, staged, stage, dmg, opener, strconv.Itoa(waitTenths))
	if logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			cmd.Stdout, cmd.Stderr = f, f
			defer f.Close() // 도우미는 자기 쪽 파일 설명자로 계속 쓴다
		}
	}
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("설치 도우미를 띄우지 못했습니다: %w", err)
	}
	return cmd.Process.Release()
}

func markerPath() (string, error) {
	p, err := store.LogPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), updateMarker), nil
}

// readUpdateNotice 는 설치 표시가 있으면 지금 버전과 견줘 결과를 만들고 표시를 지운다.
func readUpdateNotice(path, current string) UpdateNotice {
	b, err := os.ReadFile(path)
	if err != nil {
		return UpdateNotice{}
	}
	os.Remove(path)
	var m updateMarkerData
	if json.Unmarshal(b, &m) != nil || m.To == "" {
		return UpdateNotice{}
	}
	// 너무 오래된 표시(설치 직후 다시 켜지지 않음)는 알리지 않는다
	if at, err := time.Parse(time.RFC3339, m.At); err == nil && time.Since(at) > 24*time.Hour {
		return UpdateNotice{}
	}
	n := UpdateNotice{Kind: "failed", From: m.From, To: m.To}
	if compareVersions(current, m.To) >= 0 {
		n.Kind = "done"
	}
	return n
}

// cleanupUpdateLeftovers 는 지난 설치가 남긴 임시 폴더(바꿔치기가 실패했을 때)와 내려받은 파일을 치운다.
func cleanupUpdateLeftovers(bundleDir, cacheDir string, now time.Time) {
	if bundleDir != "" {
		matches, _ := filepath.Glob(filepath.Join(bundleDir, stagePrefix+"*"))
		for _, m := range matches {
			if fi, err := os.Lstat(m); err == nil && now.Sub(fi.ModTime()) > 10*time.Minute {
				os.RemoveAll(m)
			}
		}
	}
	if cacheDir != "" {
		os.RemoveAll(cacheDir)
	}
}

// updateCacheDir 는 자동 설치용 DMG 를 받아 두는 곳 (macOS: ~/Library/Caches/sobi/updates).
func updateCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sobi", "updates"), nil
}

// startUpdateHousekeeping 은 앱이 켜질 때 지난 업데이트의 결과를 읽고 남은 파일을 치운다.
func (a *App) startUpdateHousekeeping() {
	if p, err := markerPath(); err == nil {
		n := readUpdateNotice(p, appVersion)
		if n.Kind != "" {
			log.Printf("업데이트 결과: %s (%s → %s)", n.Kind, n.From, n.To)
		}
		a.update.mu.Lock()
		a.update.notice = n
		a.update.mu.Unlock()
	}
	go func() {
		bundleDir := ""
		if b, err := installTarget(); err == nil {
			bundleDir = filepath.Dir(b)
		}
		cache, _ := updateCacheDir()
		cleanupUpdateLeftovers(bundleDir, cache, time.Now())
	}()
}

// TakeUpdateNotice 는 업데이트 뒤 처음 켜졌을 때 한 번만 결과를 돌려준다(이후에는 빈 값).
func (a *App) TakeUpdateNotice() UpdateNotice {
	a.update.mu.Lock()
	defer a.update.mu.Unlock()
	n := a.update.notice
	a.update.notice = UpdateNotice{}
	return n
}

// InstallUpdate 는 내려받은 새 버전을 설치하고 앱을 다시 시작한다.
// 설치 준비(확인·복사)까지 마치면 돌아온 뒤 곧 앱이 종료되고, 새 버전이 저절로 열린다.
func (a *App) InstallUpdate() error {
	a.update.mu.Lock()
	dmg, info := a.update.path, a.update.info
	a.update.mu.Unlock()
	if dmg == "" {
		return fmt.Errorf("내려받은 파일이 없습니다")
	}
	if !info.AutoInstall {
		return fmt.Errorf("이 컴퓨터에서는 자동 설치를 할 수 없습니다")
	}
	err := a.installUpdate(dmg, info.Latest)
	logIf("InstallUpdate", err)
	return err
}

func (a *App) installUpdate(dmg, version string) error {
	target, err := installTarget()
	if err != nil {
		return err
	}
	stage, staged, err := stageUpdate(dmg, target, version)
	if err != nil {
		return err
	}
	logPath, _ := store.LogPath()
	if err := startSwapHelper(os.Getpid(), target, staged, stage, dmg, "/usr/bin/open", swapWaitTenths, logPath); err != nil {
		os.RemoveAll(stage)
		return err
	}
	if p, err := markerPath(); err == nil {
		b, _ := json.Marshal(updateMarkerData{From: appVersion, To: version, At: time.Now().Format(time.RFC3339)})
		logIf("업데이트 표시 남기기", os.WriteFile(p, b, 0o644))
	}
	log.Printf("업데이트 설치: %s → %s, 앱을 다시 시작합니다", appVersion, version)
	// 화면이 '다시 시작합니다'를 그릴 틈을 주고 종료한다(종료 전 백업이 돈 뒤 프로세스가 끝난다)
	go func() {
		time.Sleep(600 * time.Millisecond)
		if a.ctx != nil {
			wruntime.Quit(a.ctx)
		}
	}()
	return nil
}
