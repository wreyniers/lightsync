package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	popupWindow   *application.WebviewWindow
	popupURL      = "/#lights-popup"
	popupLoaded   bool
)

func (a *App) initPopupWindow() {
	wailsApp := application.Get()
	// Create with about:blank so it doesn't load the full SPA at startup.
	// Loading both main window and popup in parallel doubles GetLightState calls
	// before controller caches are warm. We load the popup URL when user opens it.
	popupWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "lights",
		Title:  "Lights",
		Width:  420,
		Height: 660,
		Hidden: true,
		URL:    "about:blank",
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
	})
}

// OpenLightsPopup shows the lights popup window. Called from the frontend.
func (a *App) OpenLightsPopup() {
	if popupWindow == nil {
		return
	}
	if !popupLoaded {
		popupWindow.SetURL(popupURL)
		popupLoaded = true
	}
	popupWindow.Show()
	popupWindow.Focus()
}
