package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var (
	popupWindow *application.WebviewWindow
	popupURL    = "/#lights-popup"
)

// initPopupWindow is intentionally a no-op. The popup window is created lazily
// on first open to avoid having two WebView2 controller instances at startup.
// Multiple WebView2 instances cause a cross-process deadlock (AppHangXProcB1)
// when Wails tears them down on Windows.
func (a *App) initPopupWindow() {}

// OpenLightsPopup shows the lights popup window. Created on first call;
// recreated after being closed (Wails 3 destroys the window on close).
func (a *App) OpenLightsPopup() {
	if popupWindow == nil {
		wailsApp := application.Get()
		popupWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:   "lights",
			Title:  "Lights",
			Width:  420,
			Height: 660,
			URL:    popupURL,
			Windows: application.WindowsWindow{
				Theme: application.Dark,
			},
		})
		popupWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			popupWindow = nil
		})
	}
	popupWindow.Show()
	popupWindow.Focus()
}
