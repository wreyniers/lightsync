import { useEffect, useMemo, useState, type ReactNode } from "react";
import { cn } from "@/lib/utils";
import { sceneSwatchBackground, liveSwatchBackground } from "@/lib/sceneColors";
import {
  Lightbulb,
  Film,
  Settings,
  Camera,
  CameraOff,
  Pencil,
  Play,
  Square,
} from "lucide-react";
import { ScreenSyncSidebarWidget } from "@/components/screensync/ScreenSyncSidebarWidget";
import { APP_VERSION } from "@/lib/types";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { App } from "@bindings";
import { Events } from "@wailsio/runtime";

interface LayoutProps {
  children: ReactNode;
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
    App.GetCameraState().then(setCameraOn).catch(() => {});
  }, []);
  useEffect(() => {
    const off = Events.On("camera:state", (e) => setCameraOn(e.data));
    return () => off?.();
  }, []);

  const lightStore = useLightStore();
  const { lastScene } = lightStore;
  useEffect(() => {
    lightActions.hydrateActiveScene();
    lightActions.hydrateLastScene();
  }, []);

  // Single source of truth: store holds activeScene + device states atomically.
  const activeScene = lightStore.activeScene;

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
  }, [activeScene, lightStore.color, lightStore.kelvin]);

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
    [lightStore.devices, lightStore.deviceOn, lightStore.color, lightStore.kelvin]
  );

  // Show last scene in the sidebar when no scene is currently active.
  const displayScene = activeScene ?? lastScene;
  const lastSwatch = useMemo(
    () => (lastScene && !activeScene ? sceneSwatchBackground(lastScene) : null),
    [lastScene, activeScene]
  );
  const displaySwatch = activeScene ? (deviated ? liveSwatch : sceneSwatch) : lastSwatch;
  const displayName = activeScene
    ? deviated ? "Custom" : (activeScene.name ?? "")
    : lastScene
    ? lastScene.name
    : "No Scene";
  const sceneIsActive = !!activeScene;

  const handlePlayScene = async () => {
    const scene = displayScene;
    if (!scene) return;
    lightActions.setActiveSceneOptimistic(scene);
    try {
      await App.ActivateScene(scene.id);
    } catch (e) {
      console.error("Failed to activate scene:", e);
      lightActions.clearActiveScene();
    }
  };

  const handleStopScene = async () => {
    await lightActions.stopActiveScene(activeScene ?? null);
  };

  return (
    <div className="flex h-screen">
      <aside className="w-64 bg-card flex flex-col">
        <div className="p-4">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl bg-primary/15 flex items-center justify-center shrink-0">
              <Lightbulb className="h-5 w-5 text-primary" />
            </div>
            <div className="min-w-0">
              <h1 className="text-lg font-bold tracking-tight">
                LightSync <span className="text-[10px] font-normal text-muted-foreground/70">v{APP_VERSION}</span>
              </h1>
              <p className="text-xs text-muted-foreground">Lights that follow your activity</p>
            </div>
          </div>
        </div>

        <div className="px-3 mb-4 space-y-2">
          {/* Camera status card */}
          <div
            className={cn(
              "flex items-center gap-3 rounded-xl px-4 py-3 transition-colors",
              cameraOn ? "bg-success/10" : "bg-muted"
            )}
          >
            <div className={cn(
              "h-8 w-8 shrink-0 rounded-xl flex items-center justify-center",
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
              "group flex items-center gap-3 rounded-xl px-4 py-3 transition-colors",
              "bg-muted"
            )}
          >
            {/* Play / Stop button — replaces the Film icon */}
            <button
              type="button"
              onClick={sceneIsActive ? handleStopScene : (displayScene ? handlePlayScene : undefined)}
              disabled={!displayScene}
              title={sceneIsActive ? "Stop scene" : (displayScene ? "Play scene" : "No scene")}
              className={cn(
                "h-8 w-8 shrink-0 rounded-xl flex items-center justify-center overflow-hidden transition-colors",
                sceneIsActive
                  ? "bg-background/40 hover:bg-destructive/20 group/playbtn"
                  : displayScene
                  ? "bg-background/40 hover:bg-background/60"
                  : "bg-background/40 cursor-default"
              )}
            >
              {displaySwatch && !sceneIsActive ? (
                <div className="h-8 w-8 rounded-lg relative flex items-center justify-center" style={{ background: displaySwatch }}>
                  <Play className="h-3 w-3 text-white drop-shadow" />
                </div>
              ) : displaySwatch && sceneIsActive ? (
                <div className="h-8 w-8 rounded-lg relative flex items-center justify-center" style={{ background: displaySwatch }}>
                  <Square className="h-3 w-3 text-white drop-shadow fill-white" />
                </div>
              ) : sceneIsActive ? (
                <Square className="h-4 w-4 text-foreground fill-foreground" />
              ) : displayScene ? (
                <Play className={cn("h-4 w-4", "text-muted-foreground")} />
              ) : (
                <Film className="h-4 w-4 text-muted-foreground/40" />
              )}
            </button>
            <div className="min-w-0 flex-1">
              <p className={cn("text-sm font-semibold leading-tight truncate", sceneIsActive ? "text-foreground" : (displayScene ? "text-foreground/70" : "text-muted-foreground"))}>
                {displayName}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">
                {sceneIsActive
                  ? (deviated ? "Modified from preset" : "Scene active")
                  : displayScene
                  ? "Press play to start"
                  : "No scene configured"}
              </p>
            </div>
            {displayScene && (
              <button
                type="button"
                onClick={() => {
                  lightActions.requestEditScene((activeScene ?? lastScene)!.id);
                  onNavigate("scenes");
                }}
                title="Edit scene"
                className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-white/10"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
            )}
          </div>

        </div>

        <nav className="flex-1 px-3 space-y-0.5">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = currentPage === item.id;
            return (
              <button
                key={item.id}
                onClick={() => onNavigate(item.id)}
                className={cn(
                  "w-full flex items-center gap-2.5 rounded-xl px-3 py-2.5 text-sm transition-colors",
                  active
                    ? "bg-muted text-foreground font-medium"
                    : "text-foreground/45 hover:text-foreground/70 hover:bg-muted/30"
                )}
              >
                <Icon className="h-4 w-4 shrink-0" />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        <div className="px-3 pb-3">
          <ScreenSyncSidebarWidget />
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <div className="p-8">{children}</div>
      </main>
    </div>
  );
}
