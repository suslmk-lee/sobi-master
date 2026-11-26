package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 자동 업데이트로 다시 열린 경우(--wait-pid N): 옛 앱이 종료 전 백업까지 끝내길 기다린 뒤 시작한다.
	// DB 를 두 프로세스가 동시에 열지 않게 하기 위해서다.
	if pid, ok := parseWaitPID(os.Args[1:]); ok {
		waitForProcessExit(pid, waitPIDLimit)
	}

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "소비마스터 — 우리집 가계부",
		Width:  1180,
		Height: 1360, // 800 × 1.7
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 6, G: 6, B: 7, A: 1},
		Windows:          &windows.Options{Theme: windows.Dark},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
