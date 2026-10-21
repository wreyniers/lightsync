package store

import "testing"

func TestEffectiveScreenSyncConfig_nilSceneAppliesPortable(t *testing.T) {
	p := &ScreenSyncPortableDefaults{
		SaturationBoost: floatPtr(1.1),
		WhiteBias:       floatPtr(-0.2),
	}
	out := EffectiveScreenSyncConfig(nil, p)
	if !approxEq(out.SaturationBoost, 1.1) {
		t.Fatalf("SaturationBoost got %v", out.SaturationBoost)
	}
	if !approxEq(out.WhiteBias, -0.2) {
		t.Fatalf("WhiteBias got %v", out.WhiteBias)
	}
	def := DefaultScreenSyncConfig()
	if approxEq(out.PaletteStability, def.PaletteStability) == false {
		t.Fatalf("expected factory palette stability when not overridden")
	}
}

func TestEffectiveScreenSyncConfig_sceneFactoryFieldUpgraded(t *testing.T) {
	def := DefaultScreenSyncConfig()
	scene := def
	scene.CaptureMode = CaptureModeMonitor
	scene.MonitorIndex = 2
	scene.Region = CaptureRect{X: 1, Y: 2, Width: 100, Height: 80}
	scene.DeviceIDs = []string{"a", "b"}

	p := &ScreenSyncPortableDefaults{
		SaturationBoost: floatPtr(1.15),
	}
	out := EffectiveScreenSyncConfig(&scene, p)
	if !approxEq(out.SaturationBoost, 1.15) {
		t.Fatalf("expected global default saturation when scene still at factory")
	}
	if out.MonitorIndex != 2 {
		t.Fatalf("capture must stay from scene, monitorIndex=%d", out.MonitorIndex)
	}
}

func TestEffectiveScreenSyncConfig_sceneCustomPreserved(t *testing.T) {
	scene := DefaultScreenSyncConfig()
	scene.SaturationBoost = 1.5
	scene.DeviceIDs = []string{"x"}

	p := &ScreenSyncPortableDefaults{
		SaturationBoost: floatPtr(1.15),
	}
	out := EffectiveScreenSyncConfig(&scene, p)
	if !approxEq(out.SaturationBoost, 1.5) {
		t.Fatalf("expected scene saturation preserved, got %v", out.SaturationBoost)
	}
}
