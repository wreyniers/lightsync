package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/icons"
	xdraw "golang.org/x/image/draw"
)

//go:embed build/appicon.png
var appIconPNG []byte

func (a *App) setupTray() {
	wailsApp := application.Get()
	systray := wailsApp.SystemTray.New()

	if runtime.GOOS == "darwin" {
		systray.SetTemplateIcon(icons.SystrayMacTemplate)
	} else {
		systray.SetIcon(trayIcon())
	}
	systray.SetTooltip("LightSync - Webcam Light Sync")

	menu := wailsApp.NewMenu()
	mShow := menu.Add("Show Window")
	menu.AddSeparator()
	mToggle := menu.Add("Pause Monitoring")
	menu.AddSeparator()
	mQuit := menu.Add("Quit")

	mShow.OnClick(func(*application.Context) {
		if a.mainWindow != nil {
			a.mainWindow.Show()
			a.mainWindow.SetAlwaysOnTop(true)
			a.mainWindow.SetAlwaysOnTop(false)
			a.mainWindow.Focus()
		}
	})

	mToggle.OnClick(func(*application.Context) {
		if a.webcamMon.IsEnabled() {
			a.webcamMon.SetEnabled(false)
			mToggle.SetLabel("Resume Monitoring")
			wailsApp.Event.Emit("monitoring:state", false)
		} else {
			a.webcamMon.SetEnabled(true)
			mToggle.SetLabel("Pause Monitoring")
			wailsApp.Event.Emit("monitoring:state", true)
		}
	})

	mQuit.OnClick(func(*application.Context) {
		wailsApp.Event.Emit("window:close-requested", nil)
	})

	if !a.webcamMon.IsEnabled() {
		mToggle.SetLabel("Resume Monitoring")
	}

	systray.SetMenu(menu)
}

func trayIcon() []byte {
	src, err := png.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		return appIconPNG
	}

	const size = 64
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)

	var buf bytes.Buffer
	_ = png.Encode(&buf, dst)
	pngData := buf.Bytes()

	ico := make([]byte, 0, 6+16+len(pngData))
	ico = append(ico, 0, 0)
	ico = append(ico, 1, 0)
	ico = append(ico, 1, 0)
	ico = append(ico, byte(size))
	ico = append(ico, byte(size))
	ico = append(ico, 0)
	ico = append(ico, 0)
	ico = append(ico, 1, 0)
	ico = append(ico, 32, 0)
	dataSize := uint32(len(pngData))
	ico = append(ico, byte(dataSize), byte(dataSize>>8), byte(dataSize>>16), byte(dataSize>>24))
	offset := uint32(6 + 16)
	ico = append(ico, byte(offset), byte(offset>>8), byte(offset>>16), byte(offset>>24))
	ico = append(ico, pngData...)

	return ico
}
