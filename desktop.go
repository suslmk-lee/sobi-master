package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"sobi/internal/app"
	"sobi/internal/store"
)

// App 은 Wails 바인딩 진입점. 업무 API 는 임베딩한 *app.App 의 메서드가 그대로
// 승격되어 바인딩되고(프론트엔드에서는 여전히 window.go.main.App), 여기에는
// 데스크톱에서만 의미가 있는 것(업데이트·타이틀바·파일 저장 대화상자)만 둔다.
type App struct {
	*app.App
	ctx context.Context
	// update 는 마지막 업데이트 확인 결과와 내려받은 파일 (updater.go).
	update updateState
}

func NewApp() *App {
	app.Version = appVersion
	return &App{App: app.New()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 오류를 설정 디렉토리의 sobi.log 에 기록한다 (Windows: %AppData%\sobi\sobi.log)
	if path, err := store.LogPath(); err == nil {
		if f, ferr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); ferr == nil {
			log.SetOutput(f)
		}
	}
	app.Start(a.App, ctx, app.Options{Backup: true, AutoPost: true})
	a.startUpdateHousekeeping()
}

func (a *App) shutdown(ctx context.Context) {
	app.Stop(a.App)
}

// logIf 는 err 가 있으면 로그에 남긴다.
func logIf(op string, err error) { app.LogIf(op, err) }

// ---- CSV 내보내기 ----

type ExportResult struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// ExportCSV 는 필터에 맞는 거래를 CSV(UTF-8 BOM, 엑셀 호환)로 저장한다.
func (a *App) ExportCSV(filter store.TxFilter) (ExportResult, error) {
	data, count, err := app.CSV(a.App, filter)
	if err != nil {
		return ExportResult{}, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "sobi_export.csv",
		Title:           "CSV 내보내기",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV 파일 (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil {
		return ExportResult{}, err
	}
	if path == "" { // 사용자가 취소
		return ExportResult{}, nil
	}
	if !strings.HasSuffix(strings.ToLower(path), ".csv") {
		path += ".csv"
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		logIf("ExportCSV(쓰기)", err)
		return ExportResult{}, err
	}
	return ExportResult{Path: path, Count: count}, nil
}
