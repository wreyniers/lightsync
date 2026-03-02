package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	ensureSingleInstance()

	app := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "LightSync",
		Description: "Webcam and screen sync for smart lights",
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{},
		Services: []application.Service{
			application.NewService(app),
		},
	})

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "main",
		Title:         "LightSync",
		Width:         2200,
		Height:        1440,
		DisableResize: false,
		MaxWidth:      2200,
		MaxHeight:     1440,
		Frameless:     false,
		BackgroundColour: application.NewRGB(10, 10, 15),
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
	})

	// Intercept close so the frontend can ask: minimize to tray or quit.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if app.quitConfirmed {
			return // allow close to proceed
		}
		wailsApp.Event.Emit("window:close-requested", nil)
		e.Cancel()
	})

	app.setMainWindow(mainWindow)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
