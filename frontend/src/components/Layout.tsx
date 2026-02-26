import { useEffect, useMemo, useState } from "react";
import { cn } from "@/lib/utils";
import { sceneSwatchBackground, liveSwatchBackground } from "@/lib/sceneColors";
import {
  Lightbulb,
  Film,
  Settings,
  Camera,
  CameraOff,
} from "lucide-react";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { Toggle } from "@/components/ui/Toggle";
import {
  IsMonitoringEnabled,
  SetMonitoringEnabled,
  GetCameraState,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

interface LayoutProps {
  children: React.ReactNode;
  currentPage: string;
  onNavigate: (page: string) => void;
}

const navItems = [
  { id: "lights", label: "Lights", icon: Lightbulb },
  { id: "scenes", label: "Scenes", icon: Film },
  { id: "settings", label: "Settings", icon: Settings },
];

export function Layout({ children, currentPage, onNavigate }: LayoutProps) {
  // Hydrate camera state on mount so UI shows correct value immediately; then
  // subscribe to camera:state for live updates. This avoids staged updates on load.
  const [cameraOn, setCameraOn] = useState(false);
  useEffect(() => {
    GetCameraState().then(setCameraOn).catch(() => {});
  }, []);
  useEffect(() => {
    const off = EventsOn("camera:state", (v: boolean) => setCameraOn(v));
    return () => off?.();
  }, []);

  const lightStore = useLightStore();
  const [monitoring, setMonitoring] = useState(true);

  useEffect(() => {
    IsMonitoringEnabled().then(setMonitoring).catch(() => {});
    lightActions.hydrateActiveScene();
  }, []);

  // Single source of truth: store holds activeScene + device states atomically.
  const activeScene = lightStore.activeScene ?? null;

  // Detect whether live light state has drifted from the active scene's preset.
  // Checks color/kelvin mode and value — brightness drift is intentionally ignored
  // to avoid noise from minor adjustments.
  const deviated = useMemo(() => {
    if (!activeScene) return false;
    for (const [id, sceneState] of Object.entries(activeScene.devices || {})) {
      const liveColor = lightStore.color[id];
      const liveKelvin = lightStore.kelvin[id];
      // Skip devices the store hasn't loaded yet.
      if (liveColor === undefined && liveKelvin === undefined) continue;
      const sceneHasColor = !!sceneState.color;
      const liveHasColor = !!liveColor;
      if (sceneHasColor !== liveHasColor) return true;
      if (sceneState.color && liveColor) {
        if (Math.abs(sceneState.color.h - liveColor.h) > 8) return true;
        if (Math.abs(sceneState.color.s - liveColor.s) > 0.08) return true;
      }
      if (sceneState.kelvin && liveKelvin) {
        if (Math.abs(sceneState.kelvin - liveKelvin) > 150) return true;
      }
    }
    return false;
  }, [activeScene, lightStore]);

  // Color swatches: when deviated show live light colors, otherwise scene colors.
  const sceneSwatch = useMemo(
    () => (activeScene ? sceneSwatchBackground(activeScene) : null),
    [activeScene]
  );

  const liveSwatch = useMemo(
    () =>
      liveSwatchBackground(
        lightStore.devices,
        lightStore.deviceOn,
        lightStore.color,
        lightStore.kelvin
      ),
    [lightStore]
  );

  const displaySwatch = deviated ? liveSwatch : sceneSwatch;
  const displayName = activeScene
    ? deviated ? "Custom" : (activeScene.name ?? "")
    : "No Scene";
  const sceneIsActive = !!activeScene;

  const handleToggleMonitoring = (enabled: boolean) => {
    SetMonitoringEnabled(enabled);
    setMonitoring(enabled);
  };

  return (
    <div className="flex h-screen">
      <aside className="w-64 bg-card flex flex-col">
        <div className="p-6">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-lg bg-primary/20 flex items-center justify-center">
              <Lightbulb className="h-5 w-5 text-primary" />
            </div>
            <div>
              <h1 className="text-lg font-bold tracking-tight">LightSync</h1>
              <p className="text-xs text-muted-foreground">Webcam to Lights</p>
            </div>
          </div>
        </div>

        <div className="px-3 mb-4 space-y-2">
          {/* Camera status card */}
          <div
            className={cn(
              "flex items-center gap-3 rounded-xl px-4 py-3",
              cameraOn ? "bg-success/10" : "bg-muted"
            )}
          >
            <div className={cn(
              "h-8 w-8 shrink-0 rounded-lg flex items-center justify-center",
              cameraOn ? "bg-success/20" : "bg-background/40"
            )}>
              {cameraOn ? (
                <Camera className="h-4 w-4 text-success" />
              ) : (
                <CameraOff className="h-4 w-4 text-muted-foreground" />
              )}
            </div>
            <div className="min-w-0">
              <p className={cn("text-sm font-semibold leading-tight", cameraOn ? "text-success" : "text-muted-foreground")}>
                {cameraOn ? "Camera Active" : "Camera Off"}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">
                {cameraOn ? "Monitoring webcam" : "Not monitoring"}
              </p>
            </div>
          </div>

          {/* Scene status card */}
          <div
            className={cn(
              "flex items-center gap-3 rounded-xl px-4 py-3",
              sceneIsActive ? "bg-primary/10" : "bg-muted"
            )}
          >
            <div className={cn(
              "h-8 w-8 shrink-0 rounded-lg flex items-center justify-center overflow-hidden",
              !displaySwatch && (sceneIsActive ? "bg-primary/20" : "bg-background/40")
            )}>
              {displaySwatch ? (
                <div className="h-8 w-8 rounded-lg" style={{ background: displaySwatch }} />
              ) : (
                <Film className={cn("h-4 w-4", sceneIsActive ? "text-primary" : "text-muted-foreground")} />
              )}
            </div>
            <div className="min-w-0">
              <p className={cn("text-sm font-semibold leading-tight truncate", sceneIsActive ? "text-primary" : "text-muted-foreground")}>
                {displayName}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">
                {sceneIsActive ? (deviated ? "Modified from preset" : "Scene active") : "No scene running"}
              </p>
            </div>
          </div>
        </div>

        <nav className="flex-1 px-3 space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = currentPage === item.id;
            return (
              <button
                key={item.id}
                onClick={() => onNavigate(item.id)}
                className={cn(
                  "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                  active
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-secondary hover:text-foreground"
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </button>
            );
          })}
        </nav>

        <div className="flex items-center justify-between px-6 py-3">
          <span className="text-sm font-medium text-muted-foreground">Webcam Monitoring</span>
          <Toggle checked={monitoring} onChange={handleToggleMonitoring} />
        </div>

        <div className="p-4">
          <p className="text-xs text-muted-foreground text-center">
            LightSync v1.0.0
          </p>
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <div className="p-8">{children}</div>
      </main>
    </div>
  );
}
