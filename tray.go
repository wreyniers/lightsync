package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	goruntime "runtime"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	xdraw "golang.org/x/image/draw"
)

//go:embed build/appicon.png
var appIconPNG []byte

func (a *App) setupTray() {
	go func() {
		goruntime.LockOSThread()
		systray.Run(a.onTrayReady, a.onTrayExit)
	}()
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon())
	systray.SetTitle("LightSync")
	systray.SetTooltip("LightSync - Webcam Light Sync")

	mShow := systray.AddMenuItem("Show Window", "Show LightSync window")
	systray.AddSeparator()
	mToggle := systray.AddMenuItem("Pause Monitoring", "Pause webcam monitoring")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit LightSync")

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				runtime.WindowShow(a.ctx)
				runtime.WindowSetAlwaysOnTop(a.ctx, true)
				runtime.WindowSetAlwaysOnTop(a.ctx, false)
			case <-mToggle.ClickedCh:
				if a.webcamMon.IsEnabled() {
					a.webcamMon.SetEnabled(false)
					mToggle.SetTitle("Resume Monitoring")
					runtime.EventsEmit(a.ctx, "monitoring:state", false)
				} else {
					a.webcamMon.SetEnabled(true)
					mToggle.SetTitle("Pause Monitoring")
					runtime.EventsEmit(a.ctx, "monitoring:state", true)
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				runtime.Quit(a.ctx)
			}
		}
	}()
}

func (a *App) onTrayExit() {}

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
