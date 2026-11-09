package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppVersionFromWailsJSON(t *testing.T) {
	if _, ok := parseVersion(appVersion); !ok || appVersion == "0.0.0" {
		t.Errorf("wails.json 의 info.productVersion 을 읽지 못함: %q", appVersion)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.1", "1.0.0", 1},
		{"1.2", "1.10.0", -1}, // 숫자로 비교 (문자열 비교면 반대)
		{"2.0.0", "1.99.99", 1},
		{"v1.1.0-beta", "1.1.0", 0}, // 꼬리표는 무시
		{"latest", "1.0.0", -1},     // 읽을 수 없는 건 가장 낮게
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestPickAsset(t *testing.T) {
	assets := []ghAsset{
		{Name: "sobi-1.1.0-windows.zip"},
		{Name: "sobi-1.1.0-macOS-arm64.dmg"},
		{Name: "sobi-1.1.0-macOS-universal.dmg"},
	}
	if a, ok := pickAsset(assets, "darwin", "amd64"); !ok || a.Name != "sobi-1.1.0-macOS-universal.dmg" {
		t.Errorf("유니버설 우선: %+v", a)
	}
	if a, ok := pickAsset(assets[:2], "darwin", "arm64"); !ok || a.Name != "sobi-1.1.0-macOS-arm64.dmg" {
		t.Errorf("같은 아키텍처: %+v", a)
	}
	if _, ok := pickAsset(assets[:2], "darwin", "amd64"); !ok {
		t.Error("맞는 게 없으면 아무 DMG 라도")
	}
	if a, ok := pickAsset(assets, "windows", "amd64"); !ok || a.Name != "sobi-1.1.0-windows.zip" {
		t.Errorf("윈도우: %+v", a)
	}
	if _, ok := pickAsset(nil, "darwin", "arm64"); ok {
		t.Error("파일이 없는데 골랐음")
	}
}

func TestBuildUpdateInfo(t *testing.T) {
	rels := []ghRelease{
		{Tag: "v1.1.0", Name: "1.1.0", Body: "- 기능 A\n", Published: "2026-11-20T01:00:00Z", HTMLURL: "https://x/1.1.0"},
		{Tag: "v1.2.0", Name: "1.2.0", Body: "- 기능 B", Published: "2026-12-01T01:00:00Z", HTMLURL: "https://x/1.2.0",
			Assets: []ghAsset{{Name: "sobi-1.2.0-macOS-universal.dmg", URL: "https://dl/1.2.0.dmg", Size: 100}}},
		{Tag: "v1.3.0", Draft: true},           // 초안은 건너뜀
		{Tag: "v1.4.0-rc1", Prerelease: true},  // 사전 배포도 건너뜀
		{Tag: "v1.0.0", Body: "지금 버전"},         // 같은 버전은 안 보임
		{Tag: "nightly", Body: "버전으로 읽을 수 없음"}, // 건너뜀
	}
	info, url := buildUpdateInfo("1.0.0", rels, "darwin", "arm64")
	if !info.Available || info.Latest != "1.2.0" || len(info.Notes) != 2 {
		t.Fatalf("새 버전: %+v", info)
	}
	if info.Notes[0].Version != "1.2.0" || info.Notes[1].Version != "1.1.0" || info.Notes[0].Date != "2026-12-01" {
		t.Errorf("최신 먼저 정렬·날짜: %+v", info.Notes)
	}
	if info.Notes[1].Notes != "- 기능 A" {
		t.Errorf("설명 앞뒤 공백 정리: %q", info.Notes[1].Notes)
	}
	if url != "https://dl/1.2.0.dmg" || info.AssetName != "sobi-1.2.0-macOS-universal.dmg" || info.AssetSize != 100 {
		t.Errorf("설치 파일: %s %+v", url, info)
	}
	if info.PageURL != "https://x/1.2.0" {
		t.Errorf("릴리스 페이지: %s", info.PageURL)
	}

	// 최신이면 Available=false, 빈 목록
	info, url = buildUpdateInfo("1.2.0", rels, "darwin", "arm64")
	if info.Available || info.Latest != "1.2.0" || len(info.Notes) != 0 || url != "" {
		t.Errorf("이미 최신: %+v %s", info, url)
	}
	// 최신 릴리스에 이 컴퓨터용 파일이 없으면 페이지 안내만
	info, url = buildUpdateInfo("1.0.0", rels[:1], "darwin", "arm64")
	if !info.Available || url != "" || info.AssetName != "" || info.PageURL != "https://x/1.1.0" {
		t.Errorf("파일 없음: %+v %s", info, url)
	}
}

func TestDownloadTo(t *testing.T) {
	body := strings.Repeat("sobi", 200000) // 800KB — 여러 번에 나눠 읽힌다
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", "800000")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "sobi.dmg")
	var calls int
	var last, total int64
	if err := downloadTo(context.Background(), srv.Client(), srv.URL+"/ok", dest, func(d, t int64) {
		calls++
		last, total = d, t
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != body {
		t.Fatalf("받은 내용이 다름 (%d 바이트, %v)", len(got), err)
	}
	if calls < 2 || last != 800000 || total != 800000 {
		t.Errorf("진행 알림: %d번, 마지막 %d/%d", calls, last, total)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part 파일이 남음")
	}

	// 실패하면 파일을 남기지 않는다
	bad := filepath.Join(dir, "missing.dmg")
	if err := downloadTo(context.Background(), srv.Client(), srv.URL+"/missing", bad, nil); err == nil {
		t.Error("404 인데 성공")
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Error("실패했는데 파일이 생김")
	}
}
