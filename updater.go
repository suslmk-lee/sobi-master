package main

// 앱 버전 표시와 새 버전 확인·내려받기.
//
// 새 버전은 GitHub Releases(공개 저장소)에 v1.2.3 태그로 올린다. 릴리스 설명이 '변경 사항',
// 첨부한 DMG 가 내려받을 파일이다(scripts/release.sh 참고). 앱은 공개 API 로 릴리스 목록을 읽어
// 지금 버전보다 새 것이 있으면 알리고, 사용자가 원하면 다운로드 폴더로 받아 연다.
// 앱이 직접 받은 파일에는 macOS 격리 표시(quarantine)가 붙지 않아 Gatekeeper 경고가 다시 뜨지 않는다.

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed wails.json
var wailsConfig []byte

// appVersion 은 빌드된 앱의 버전 — wails.json 의 info.productVersion (DMG 이름의 버전과 같다).
var appVersion = func() string {
	var c struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if json.Unmarshal(wailsConfig, &c) == nil && c.Info.ProductVersion != "" {
		return c.Info.ProductVersion
	}
	return "0.0.0"
}()

const (
	releasesAPI = "https://api.github.com/repos/suslmk-lee/sobi-master/releases?per_page=30"
	releasesWeb = "https://github.com/suslmk-lee/sobi-master/releases"
)

// ReleaseNote 는 지금 버전보다 새 릴리스 하나의 변경 사항.
type ReleaseNote struct {
	Version string `json:"version"` // "1.1.0"
	Title   string `json:"title"`   // 릴리스 제목
	Date    string `json:"date"`    // 올린 날 YYYY-MM-DD
	Notes   string `json:"notes"`   // 릴리스 설명(마크다운)
}

// UpdateInfo 는 업데이트 확인 결과.
type UpdateInfo struct {
	Current   string        `json:"current"`
	Latest    string        `json:"latest"`    // 가장 새 릴리스 버전 (릴리스가 없으면 Current)
	Available bool          `json:"available"` // Latest 가 Current 보다 새로운지
	Notes     []ReleaseNote `json:"notes"`     // Current 보다 새 릴리스들 (최신 먼저)
	AssetName string        `json:"assetName"` // 이 컴퓨터용 설치 파일 (없으면 "")
	AssetSize int64         `json:"assetSize"`
	PageURL   string        `json:"pageUrl"` // 최신 릴리스 페이지 (설치 파일이 없을 때 안내용)
}

type updateState struct {
	mu       sync.Mutex
	info     UpdateInfo
	assetURL string // DownloadUpdate 가 받을 주소 — 확인 결과로만 정해지게 프론트에 맡기지 않는다
	path     string // 마지막으로 받은 파일
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type ghRelease struct {
	Tag        string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  string    `json:"published_at"`
	HTMLURL    string    `json:"html_url"`
	Assets     []ghAsset `json:"assets"`
}

// parseVersion 은 "v1.2.3", "1.2", "1.2.3-beta" 를 [주, 부, 수] 로 읽는다.
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "v"), "V"))
	if i := strings.IndexAny(s, "-+ "); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if s == "" || len(parts) > 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// compareVersions 는 a<b 면 -1, 같으면 0, a>b 면 1. 읽을 수 없는 버전은 가장 낮게 본다.
func compareVersions(a, b string) int {
	va, oka := parseVersion(a)
	vb, okb := parseVersion(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for i := 0; i < 3; i++ {
		if va[i] != vb[i] {
			if va[i] < vb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// pickAsset 은 이 컴퓨터에 맞는 설치 파일을 고른다.
// macOS: 유니버설 DMG → 같은 아키텍처 DMG → 아무 DMG. Windows: windows 가 들어간 .exe/.zip.
func pickAsset(assets []ghAsset, goos, goarch string) (ghAsset, bool) {
	lower := func(a ghAsset) string { return strings.ToLower(a.Name) }
	find := func(ok func(string) bool) (ghAsset, bool) {
		for _, a := range assets {
			if ok(lower(a)) {
				return a, true
			}
		}
		return ghAsset{}, false
	}
	switch goos {
	case "darwin":
		archNames := map[string][]string{"arm64": {"arm64", "apple"}, "amd64": {"amd64", "x86_64", "intel"}}[goarch]
		for _, ok := range []func(string) bool{
			func(n string) bool { return strings.HasSuffix(n, ".dmg") && strings.Contains(n, "universal") },
			func(n string) bool {
				if !strings.HasSuffix(n, ".dmg") {
					return false
				}
				for _, an := range archNames {
					if strings.Contains(n, an) {
						return true
					}
				}
				return false
			},
			func(n string) bool { return strings.HasSuffix(n, ".dmg") },
		} {
			if a, found := find(ok); found {
				return a, true
			}
		}
	case "windows":
		return find(func(n string) bool {
			return (strings.HasSuffix(n, ".exe") || strings.HasSuffix(n, ".zip")) && strings.Contains(n, "win")
		})
	}
	return ghAsset{}, false
}

// buildUpdateInfo 는 릴리스 목록에서 지금 버전보다 새 것을 모으고 최신 릴리스의 설치 파일을 고른다.
// 초안·사전 배포(prerelease)·버전으로 읽을 수 없는 태그는 건너뛴다.
func buildUpdateInfo(current string, rels []ghRelease, goos, goarch string) (UpdateInfo, string) {
	info := UpdateInfo{Current: current, Latest: current, Notes: []ReleaseNote{}, PageURL: releasesWeb}
	var newer []ghRelease
	for _, r := range rels {
		if r.Draft || r.Prerelease {
			continue
		}
		if _, ok := parseVersion(r.Tag); !ok {
			continue
		}
		if compareVersions(r.Tag, current) > 0 {
			newer = append(newer, r)
		}
	}
	if len(newer) == 0 {
		return info, ""
	}
	sort.SliceStable(newer, func(i, j int) bool { return compareVersions(newer[i].Tag, newer[j].Tag) > 0 })
	for _, r := range newer {
		date := r.Published
		if len(date) >= 10 {
			date = date[:10]
		}
		info.Notes = append(info.Notes, ReleaseNote{
			Version: strings.TrimPrefix(strings.TrimPrefix(r.Tag, "v"), "V"),
			Title:   r.Name, Date: date, Notes: strings.TrimSpace(r.Body),
		})
	}
	latest := newer[0]
	info.Latest = info.Notes[0].Version
	info.Available = true
	if latest.HTMLURL != "" {
		info.PageURL = latest.HTMLURL
	}
	asset, ok := pickAsset(latest.Assets, goos, goarch)
	if !ok {
		return info, ""
	}
	info.AssetName, info.AssetSize = asset.Name, asset.Size
	return info, asset.URL
}

var updateClient = &http.Client{Timeout: 30 * time.Minute} // 내려받기까지 같이 쓴다(요청별 컨텍스트로 확인은 짧게)

// GetAppVersion 은 지금 앱의 버전.
func (a *App) GetAppVersion() string { return appVersion }

// CheckForUpdate 는 GitHub Releases 에서 새 버전이 있는지 확인한다.
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return UpdateInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sobi-app/"+appVersion)
	resp, err := updateClient.Do(req)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("업데이트 서버에 연결하지 못했습니다: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("업데이트 확인 실패 (HTTP %d)", resp.StatusCode)
	}
	var rels []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return UpdateInfo{}, fmt.Errorf("릴리스 정보를 읽지 못했습니다: %w", err)
	}
	info, assetURL := buildUpdateInfo(appVersion, rels, goruntime.GOOS, goruntime.GOARCH)
	a.update.mu.Lock()
	a.update.info, a.update.assetURL = info, assetURL
	a.update.mu.Unlock()
	return info, nil
}

// DownloadUpdate 는 마지막 확인에서 찾은 설치 파일을 다운로드 폴더로 받는다.
// 받는 동안 "update:progress" 이벤트(받은 바이트, 전체 바이트)를 보낸다. 받은 파일 경로를 돌려준다.
func (a *App) DownloadUpdate() (string, error) {
	a.update.mu.Lock()
	url, name := a.update.assetURL, a.update.info.AssetName
	a.update.mu.Unlock()
	if url == "" || name == "" {
		return "", fmt.Errorf("받을 설치 파일이 없습니다. 업데이트 확인을 다시 해 주세요")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(home, "Downloads", filepath.Base(name))
	var last time.Time
	err = downloadTo(context.Background(), updateClient, url, dest, func(done, total int64) {
		if a.ctx == nil {
			return
		}
		if now := time.Now(); done == total || now.Sub(last) > 100*time.Millisecond {
			last = now
			wruntime.EventsEmit(a.ctx, "update:progress", done, total)
		}
	})
	if err != nil {
		logIf("DownloadUpdate", err)
		return "", err
	}
	a.update.mu.Lock()
	a.update.path = dest
	a.update.mu.Unlock()
	return dest, nil
}

// OpenDownloadedUpdate 는 내려받은 설치 파일을 연다(macOS 는 DMG 가 마운트되며 Finder 창이 열린다).
func (a *App) OpenDownloadedUpdate() error {
	a.update.mu.Lock()
	path := a.update.path
	a.update.mu.Unlock()
	if path == "" {
		return fmt.Errorf("내려받은 파일이 없습니다")
	}
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// downloadTo 는 url 을 dest 로 받는다. 받는 중에는 dest.part 에 쓰고, 다 받은 뒤에 이름을 바꿔
// 중간에 끊겨도 반쯤 받은 파일이 설치 파일처럼 남지 않게 한다.
func downloadTo(ctx context.Context, client *http.Client, url, dest string, progress func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "sobi-app/"+appVersion)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("내려받기 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("내려받기 실패 (HTTP %d)", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	part := dest + ".part"
	f, err := os.Create(part)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(part)
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(part)
			return fmt.Errorf("내려받기 중 끊겼습니다: %w", rerr)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(part)
		return err
	}
	if total > 0 && done != total {
		os.Remove(part)
		return fmt.Errorf("파일을 끝까지 받지 못했습니다 (%d/%d 바이트)", done, total)
	}
	return os.Rename(part, dest)
}
