package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	ensureSingleInstance()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:         "LightSync",
		Width:         2200,
		Height:        1440,
		DisableResize: false,
		MaxWidth:      2200,
		MaxHeight:     1440,
		Frameless:     false,
		// Intercept the close button so the frontend can ask the user
		// whether to minimize to the system tray or quit entirely.
		// When QuitApp() sets quitConfirmed the flag lets the quit through.
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if app.quitConfirmed {
				return false // allow the quit to proceed
			}
			runtime.EventsEmit(ctx, "window:close-requested")
			return true // block; let the frontend dialog decide
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 15, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.Dark,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
