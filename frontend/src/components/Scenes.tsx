import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import {
  Plus,
  Camera,
  CameraOff,
  Film,
  Play,
  X,
  Check,
  Lightbulb,
  MonitorPlay,
  Square,
} from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { LightCard } from "@/components/ui/LightCard";
import { ColorWheel, KelvinSlider } from "@/components/ui/ColorPanel";
import type { Device, DeviceState, Color, LightMode, ScreenSyncConfig, Scene as AppScene } from "@/lib/types";
import { DEFAULT_KELVIN, DEFAULT_SCREEN_SYNC_CONFIG, SCREEN_SYNC_TRIGGER } from "@/lib/types";
import { hueToKelvin, kelvinToHSB } from "@/lib/utils";
import { groupByRoom, UNASSIGNED_KEY } from "@/lib/brands";
import { getRoomIcon, sortedRoomKeys } from "@/lib/rooms";
import { sceneSwatchBackground } from "@/lib/sceneColors";
import { SceneRow } from "@/components/SceneRow";
import { ScreenSyncEditor } from "@/components/screensync/ScreenSyncEditor";
import { OptionTile, SettingsLabel, SettingsSection } from "@/components/screensync/settings";
import { SetupWizard } from "@/components/screensync/SetupWizard";
import { useLightStore, lightActions } from "@/hooks/useLightStore";
import { App } from "@bindings";
import { CreateSceneRequest } from "@bindings";
import * as lights from "@bindings/internal/lights/models.js";
import * as store from "@bindings/internal/store/models.js";
import { Events } from "@wailsio/runtime";

type Scene = store.Scene;

const DEFAULT_COLOR: Color = { h: 30, s: 1, b: 1 };
type GlobalMode = "none" | "color" | "kelvin";

// Keys that affect extraction/assignment and should apply immediately (no debounce).
const DISCRETE_SCREEN_SYNC_KEYS = new Set([
  "subMethod",
  "extractionMethod",
  "colorMode",
  "multiColorApproach",
  "assignmentStrategy",
  "captureMode",
  "monitorIndex",
  "speedPreset",
  "brightnessMode",
  "deviceIds",
]);

/** Build DeviceState from store values. Store uses brightness 0-100; DeviceState uses 0-1. */
function captureDeviceState(
  deviceOn: Record<string, boolean>,
  brightness: Record<string, number>,
  kelvin: Record<string, number>,
  color: Record<string, Color>,
  deviceId: string
): DeviceState {
  const on = deviceOn[deviceId] ?? false;
  const bright = brightness[deviceId] ?? 80;
  return {
    on,
    brightness: bright <= 1 ? bright : bright / 100,
    kelvin: kelvin[deviceId],
    color: color[deviceId],
  };
}

export function Scenes() {
  const { devices, deviceOn, brightness, kelvin, color, activeScene: storeActiveScene, pendingEditSceneId } = useLightStore();
  const [scenes, setScenes] = useState<Scene[]>([]);
  const [editing, setEditing] = useState<Scene | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newTrigger, setNewTrigger] = useState("");
  const [newDevices, setNewDevices] = useState<Record<string, DeviceState>>({});
  const [screenSyncConfig, setScreenSyncConfig] = useState<ScreenSyncConfig>(DEFAULT_SCREEN_SYNC_CONFIG);
  const [showWizard, setShowWizard] = useState(false);
  const [syncRunning, setSyncRunning] = useState(false);
  const [syncSceneId, setSyncSceneId] = useState("");
  const firstScreenSync = useRef(true);
  const liveUpdateTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Snapshot of light states before edit mode; restored when exiting.
  const [preEditLightStates, setPreEditLightStates] = useState<Record<string, DeviceState>>({});

  // Per-device mode overrides (keyed by device id).
  const [deviceModes, setDeviceModes] = useState<Record<string, LightMode>>({});

  // Global override — one mode at a time; brightness stays per-device.
  const [globalMode, setGlobalMode] = useState<GlobalMode>("none");
  const [globalColorValue, setGlobalColorValue] = useState<Color>(DEFAULT_COLOR);
  const [globalKelvin, setGlobalKelvin] = useState(DEFAULT_KELVIN);

  // True when the editor has unsaved changes relative to the saved scene.
  // Always true when creating (nothing to compare against).
  const isDirty = useMemo(() => {
    if (creating) return true;
    if (!editing) return false;

    if (newName.trim() !== editing.name) return true;
    if (newTrigger !== editing.trigger) return true;

    if (newTrigger === SCREEN_SYNC_TRIGGER) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      return JSON.stringify(screenSyncConfig) !== JSON.stringify((editing.screenSync as any) || DEFAULT_SCREEN_SYNC_CONFIG);
    }

    // Deep-compare device maps via JSON — both are plain serialisable objects.
    if (JSON.stringify(newDevices) !== JSON.stringify(editing.devices ?? {})) return true;

    // Reconstruct the saved global mode from the persisted fields.
    const savedGlobalMode: GlobalMode = editing.globalColor
      ? "color"
      : editing.globalKelvin != null
      ? "kelvin"
      : "none";
    if (globalMode !== savedGlobalMode) return true;
    if (globalMode === "color" && JSON.stringify(globalColorValue) !== JSON.stringify(editing.globalColor)) return true;
    if (globalMode === "kelvin" && globalKelvin !== editing.globalKelvin) return true;

    return false;
  }, [creating, editing, newName, newTrigger, newDevices, globalMode, globalColorValue, globalKelvin, screenSyncConfig]);

  const refresh = useCallback(() => {
    App.GetScenes()
      .then((s) => setScenes(s || []))
      .catch(() => {});
    lightActions.refreshDevices();
    lightActions.hydrateActiveScene();
    // Sync engine running state.
    App.GetScreenSyncState()
      .then((state) => {
        setSyncRunning(state.running);
        setSyncSceneId(state.sceneId ?? "");
      })
      .catch(() => {});
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  // Subscribe to screen sync engine state changes.
  // Use returned unsubscribe so we don't remove the sidebar widget's listener.
  useEffect(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const handler = (data: any) => {
      setSyncRunning(data?.running ?? false);
      setSyncSceneId(data?.sceneId ?? "");
    };
    const off = Events.On("screensync:state", (e) => handler(e.data));
    return () => { off?.(); };
  }, []);

  // Open the editor when the sidebar's edit button sets pendingEditSceneId.
  // Wait until scenes have loaded (non-empty) before matching, so we don't
  // clear the pending ID before the async GetScenes() fetch has resolved.
  useEffect(() => {
    if (!pendingEditSceneId || scenes.length === 0) return;
    const scene = scenes.find((s) => s.id === pendingEditSceneId);
    lightActions.clearPendingEdit();
    if (scene) startEdit(scene as Scene);
  }, [pendingEditSceneId, scenes]);

  // Live-apply Screen Sync config changes while the engine is running for the scene being edited.
  // Debounced at 300 ms so rapid slider moves don't flood the backend.
  useEffect(() => {
    const isLiveScene =
      syncRunning &&
      editing != null &&
      !creating &&
      editing.id === syncSceneId &&
      newTrigger === SCREEN_SYNC_TRIGGER;

    if (!isLiveScene) return;

    if (liveUpdateTimer.current) clearTimeout(liveUpdateTimer.current);
    liveUpdateTimer.current = setTimeout(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      editing?.id && App.UpdateScreenSyncConfig(editing.id, screenSyncConfig as any).catch(console.error);
    }, 300);

    return () => {
      if (liveUpdateTimer.current) clearTimeout(liveUpdateTimer.current);
    };
  }, [screenSyncConfig, syncRunning, syncSceneId, editing, creating, newTrigger]);

  const activeSceneId = storeActiveScene?.id ?? "";

  const startCreate = (initialTrigger?: string) => {
    const trigger = initialTrigger ?? "";
    setCreating(true);
    setNewName("");
    setNewTrigger(trigger);
    setNewDevices({});
    setDeviceModes({});
    setGlobalMode("none");
    setGlobalColorValue(DEFAULT_COLOR);
    setGlobalKelvin(DEFAULT_KELVIN);
    setEditing(null);
    setPreEditLightStates({});
    setScreenSyncConfig(DEFAULT_SCREEN_SYNC_CONFIG);
    if (trigger === SCREEN_SYNC_TRIGGER && firstScreenSync.current) {
      firstScreenSync.current = false;
      setShowWizard(true);
    }
  };

  const startEdit = (scene: Scene) => {
    setEditing(scene);
    setNewName(scene.name ?? "");
    setNewTrigger(scene.trigger ?? "");
    const devs = scene.devices || {};
    setNewDevices(devs);
    setDeviceModes({});
    setCreating(false);
    // Restore ScreenSync config.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    setScreenSyncConfig((scene.screenSync as any) || DEFAULT_SCREEN_SYNC_CONFIG);

    // Restore global override that was saved with the scene; fall back to none.
    if (scene.globalColor) {
      setGlobalMode("color");
      setGlobalColorValue(scene.globalColor);
      setGlobalKelvin(DEFAULT_KELVIN);
    } else if (scene.globalKelvin != null) {
      setGlobalMode("kelvin");
      setGlobalKelvin(scene.globalKelvin);
      setGlobalColorValue(DEFAULT_COLOR);
    } else {
      setGlobalMode("none");
      setGlobalColorValue(DEFAULT_COLOR);
      setGlobalKelvin(DEFAULT_KELVIN);
    }

    // Capture current light states to restore on exit. Editing never activates the scene.
    const captured: Record<string, DeviceState> = {};
    for (const id of Object.keys(devs)) {
      captured[id] = captureDeviceState(deviceOn, brightness, kelvin, color, id);
    }
    setPreEditLightStates(captured);

    // Apply scene to lights immediately for live preview (no ActivateScene).
    lightActions.previewSceneStates(devs);
  };

  const exitEdit = useCallback((restoreLights: boolean) => {
    const statesToRestore = restoreLights ? preEditLightStates : {};
    const deviceList = devices;

    // Close UI immediately so the user gets instant feedback
    setEditing(null);
    setCreating(false);
    setPreEditLightStates({});

    // Restore lights in background (don't block UI)
    if (Object.keys(statesToRestore).length > 0 && deviceList.length > 0) {
      lightActions.restoreLightStates(statesToRestore, deviceList).catch((e) => {
        console.error("Failed to restore light states on cancel:", e);
      });
    }
  }, [preEditLightStates, devices]);

  const cancelEdit = () => {
    exitEdit(true);
  };

  const toggleDeviceInScene = (deviceId: string) => {
    setNewDevices((prev) => {
      if (prev[deviceId]) {
        const next = { ...prev };
        delete next[deviceId];
        return next;
      }
      return { ...prev, [deviceId]: { on: true, brightness: 0.8 } };
    });
    // When adding a device in create mode, capture its current state for restore on exit.
    if (creating && !newDevices[deviceId] && !preEditLightStates[deviceId]) {
      setPreEditLightStates((p) => ({
        ...p,
        [deviceId]: captureDeviceState(deviceOn, brightness, kelvin, color, deviceId),
      }));
    }
  };

  const updateDeviceState = (deviceId: string, updates: Partial<DeviceState>) => {
    setNewDevices((prev) => ({
      ...prev,
      [deviceId]: { ...prev[deviceId], ...updates },
    }));
  };

  function switchDeviceMode(deviceId: string, mode: LightMode) {
    setDeviceModes((prev) => ({ ...prev, [deviceId]: mode }));
    if (mode === "kelvin") {
      // Clear color when switching to temperature.
      updateDeviceState(deviceId, { color: undefined });
    }
  }

  function getDeviceMode(deviceId: string): LightMode {
    if (deviceModes[deviceId]) return deviceModes[deviceId];
    const s = newDevices[deviceId];
    if (s?.color) return "color";
    if (s?.kelvin) return "kelvin";
    return "color";
  }

  const toModelDevices = (
    devs: Record<string, DeviceState>
  ): Record<string, lights.DeviceState> => {
    const result: Record<string, lights.DeviceState> = {};
    for (const [k, v] of Object.entries(devs)) {
      result[k] = new lights.DeviceState({
        ...v,
        color: v.color ? new lights.Color(v.color) : undefined,
      });
    }
    return result;
  };

  const handleSave = async () => {
    if (!newName.trim()) return;
    const finalDevices = { ...newDevices };

    if (globalMode === "color") {
      for (const id of Object.keys(finalDevices)) {
        if (finalDevices[id].on) {
          const dev = devices.find((d) => d.id === id);
          if (dev?.supportsColor) {
            // RGB device: preserve per-device brightness, override hue + saturation.
            finalDevices[id] = {
              ...finalDevices[id],
              color: { ...globalColorValue, b: finalDevices[id].brightness },
              kelvin: undefined,
            };
          } else if (dev?.supportsKelvin) {
            // Kelvin-only device: extrapolate CCT from the selected hue.
            const k = Math.max(
              dev.minKelvin ?? 2000,
              Math.min(dev.maxKelvin ?? 9000, hueToKelvin(globalColorValue.h))
            );
            finalDevices[id] = { ...finalDevices[id], kelvin: k, color: undefined };
          }
        }
      }
    } else if (globalMode === "kelvin") {
      for (const id of Object.keys(finalDevices)) {
        if (finalDevices[id].on) {
          // Preserve individual brightness; only override kelvin.
          finalDevices[id] = {
            ...finalDevices[id],
            kelvin: globalKelvin,
            color: undefined,
          };
        }
      }
    }

    const globalOverride = globalMode === "color"
      ? { globalColor: globalColorValue, globalKelvin: undefined }
      : globalMode === "kelvin"
      ? { globalColor: undefined, globalKelvin: globalKelvin }
      : { globalColor: undefined, globalKelvin: undefined };

    const isScreenSync = newTrigger === SCREEN_SYNC_TRIGGER;

    try {
      if (editing) {
        await App.UpdateScene(
          new store.Scene({
            ...editing,
            name: newName,
            trigger: newTrigger,
            devices: isScreenSync ? {} : toModelDevices(finalDevices),
            ...(isScreenSync ? {} : globalOverride),
            screenSync: isScreenSync ? screenSyncConfig : undefined,
          })
        );
      } else {
        await App.CreateScene(
          new CreateSceneRequest({
            name: newName,
            trigger: newTrigger,
            devices: isScreenSync ? {} : toModelDevices(finalDevices),
            ...(isScreenSync ? {} : globalOverride),
            screenSync: isScreenSync ? screenSyncConfig : undefined,
          })
        );
      }
      exitEdit(false);
      refresh();
    } catch (e) {
      console.error("Failed to save scene:", e);
    }
  };

  const handleDelete = async (id: string) => {
    try { await App.DeleteScene(id); refresh(); } catch (e) { console.error(e); }
  };

  const handleClone = async (id: string) => {
    try { await App.CloneScene(id); refresh(); } catch (e) { console.error(e); }
  };

  const handleActivate = async (id: string) => {
    const scene = scenes.find((s) => s.id === id);
    if (scene) lightActions.setActiveSceneOptimistic(scene as AppScene);
    try {
      await App.ActivateScene(id);
    } catch (e) {
      console.error("Failed to activate scene:", e);
      lightActions.refreshLightStates();
    }
  };

  const handleStopSync = async () => {
    try {
      await App.StopScreenSync();
      setSyncRunning(false);
    } catch (e) {
      console.error("Failed to stop screen sync:", e);
    }
  };

  const handleStop = async (sceneId: string) => {
    const scene = scenes.find((s) => s.id === sceneId);
    await lightActions.stopActiveScene(scene ?? null);
    if (scene?.trigger === SCREEN_SYNC_TRIGGER) {
      setSyncRunning(false);
    }
  };

  const anyDeviceSupportsColor = devices.some((d) => d.supportsColor);
  const anyDeviceSupportsKelvin = devices.some((d) => d.supportsKelvin);

  // Triggers already claimed by another scene (excluding the one currently being edited).
  const takenTriggers = useMemo(() => {
    const editingId = editing?.id;
    return new Set(
      scenes
        .filter((s) => s.trigger && s.id !== editingId)
        .map((s) => s.trigger)
    );
  }, [scenes, editing]);

  /** Push a global color value to all currently included+on devices for live preview.
   *  RGB-capable devices receive the full colour; kelvin-only devices get an
   *  approximated correlated colour temperature derived from the hue. */
  function previewGlobalColor(c: Color) {
    Object.entries(newDevices).forEach(([id, state]) => {
      if (!state.on) return;
      const dev = devices.find((d) => d.id === id);
      if (!dev) return;
      if (dev.supportsColor) {
        lightActions.setColor(id, { ...c, b: state.brightness });
      } else if (dev.supportsKelvin) {
        const k = Math.max(
          dev.minKelvin ?? 2000,
          Math.min(dev.maxKelvin ?? 9000, hueToKelvin(c.h))
        );
        lightActions.setTemperature(id, k, state.brightness);
      }
    });
  }
  function previewGlobalKelvin(k: number) {
    Object.entries(newDevices).forEach(([id, state]) => {
      if (!state.on) return;
      const dev = devices.find((d) => d.id === id);
      if (!dev) return;
      if (dev.supportsKelvin) {
        const clamped = Math.max(
          dev.minKelvin ?? 2000,
          Math.min(dev.maxKelvin ?? 9000, k)
        );
        lightActions.setTemperature(id, clamped, state.brightness);
      } else if (dev.supportsColor) {
        const hsb = kelvinToHSB(k);
        lightActions.setColor(id, { ...hsb, b: state.brightness });
      }
    });
  }
  const isEditing = creating || editing !== null;

  const grouped = groupByRoom(devices);
  const roomKeys = sortedRoomKeys(grouped);

  return (
    <div className="space-y-8">
      {/* Setup wizard */}
      {showWizard && (
        <SetupWizard
          config={screenSyncConfig}
          onChange={(patch) => setScreenSyncConfig((prev) => ({ ...prev, ...patch }))}
          onFinish={() => setShowWizard(false)}
          onSkip={() => setShowWizard(false)}
        />
      )}

      {!isEditing && (
        <div className="flex items-center justify-between">
          {syncRunning && (
            <div className="flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-success animate-pulse" />
              <span className="text-xs text-muted-foreground">Screen Sync active</span>
              <button
                type="button"
                onClick={handleStopSync}
                className="text-xs text-destructive hover:underline flex items-center gap-1"
              >
                <Square className="h-3 w-3" />
                Stop
              </button>
            </div>
          )}
          <div className="ml-auto">
            <Button onClick={() => startCreate()}>
              <Plus className="h-4 w-4" />
              New Scene
            </Button>
          </div>
        </div>
      )}

      {isEditing && (
        <Card className="space-y-6">
          {/* Header */}
          <div className="flex items-center justify-between gap-4">
            <div className="min-w-0 flex-1">
              <p className="text-xs font-medium uppercase tracking-widest text-muted-foreground mb-0.5">
                {creating ? "New Scene" : "Edit Scene"}
              </p>
              <input
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="Scene name…"
                className="text-xl font-bold bg-transparent focus:outline-none focus:ring-0 placeholder:text-muted-foreground/40 w-full min-w-0"
              />
            </div>
            <button
              type="button"
              onClick={cancelEdit}
              className="h-8 w-8 rounded-full flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-white/10 transition-colors shrink-0"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="space-y-5">
            {/* Trigger */}
            <div>
              <SettingsLabel>Trigger</SettingsLabel>
              <div className="flex gap-2">
                {([
                  { value: "", icon: Play, label: "Manual", taken: false },
                  { value: "camera_on", icon: Camera, label: "Camera On", taken: takenTriggers.has("camera_on") },
                  { value: "camera_off", icon: CameraOff, label: "Camera Off", taken: takenTriggers.has("camera_off") },
                  { value: SCREEN_SYNC_TRIGGER, icon: MonitorPlay, label: "Screen Sync", taken: false },
                ] as const).map(({ value: t, icon: Icon, label, taken }) => (
                  <OptionTile
                    key={t}
                    selected={newTrigger === t}
                    disabled={taken}
                    onClick={() => {
                      setNewTrigger(t);
                      if (t === SCREEN_SYNC_TRIGGER && creating && firstScreenSync.current) {
                        firstScreenSync.current = false;
                        setShowWizard(true);
                      }
                    }}
                    variant="grid"
                    className="flex-1 items-center gap-2 py-3.5"
                    title={taken ? "Already used by another scene" : undefined}
                  >
                    <Icon className="h-5 w-5" />
                    <span className="text-xs font-medium">{label}</span>
                  </OptionTile>
                ))}
              </div>
            </div>

            {/* Screen Sync config editor */}
            {newTrigger === SCREEN_SYNC_TRIGGER && (
              <ScreenSyncEditor
                config={screenSyncConfig}
                devices={devices}
                isRunning={syncRunning}
                canPlay={!creating && editing != null}
                onPlay={
                  (editing != null && editing.id)
                    ? (async () => {
                        const id = editing.id as string;
                        try {
                          // eslint-disable-next-line @typescript-eslint/no-explicit-any
                          await App.UpdateScreenSyncConfig(id, screenSyncConfig as any);
                          await App.ActivateScene(id);
                        } catch (e) {
                          console.error("Failed to start Screen Sync:", e);
                        }
                      })
                    : undefined
                }
                onStop={handleStopSync}
                onChange={(patch) => {
                  const next = { ...screenSyncConfig, ...patch };
                  setScreenSyncConfig(next);
                  // Apply discrete changes immediately so they take effect without waiting for debounce
                  const isLiveScene =
                    syncRunning &&
                    editing != null &&
                    !creating &&
                    editing.id === syncSceneId;
                  const patchKeys = Object.keys(patch);
                  const isDiscreteOnly =
                    patchKeys.length > 0 &&
                    patchKeys.every((k) => DISCRETE_SCREEN_SYNC_KEYS.has(k));
                  if (isLiveScene && isDiscreteOnly && editing?.id) {
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    App.UpdateScreenSyncConfig(editing.id, next as any).catch(console.error);
                  }
                }}
              />
            )}

            {/* Global Override — hidden for Screen Sync scenes */}
            {newTrigger !== SCREEN_SYNC_TRIGGER && (anyDeviceSupportsColor || anyDeviceSupportsKelvin) && (
              <SettingsSection title="Global Color">
                <div className="flex gap-2">
                  {(
                    [
                      { value: "none" as const, label: "Off" },
                      ...(anyDeviceSupportsColor ? [{ value: "color" as const, label: "Color" }] : []),
                      ...(anyDeviceSupportsKelvin ? [{ value: "kelvin" as const, label: "Temperature" }] : []),
                    ] as const
                  ).map(({ value, label }) => (
                    <OptionTile
                      key={value}
                      selected={globalMode === value}
                      onClick={() => {
                        setGlobalMode(value);
                        if (value === "color") previewGlobalColor(globalColorValue);
                        if (value === "kelvin") previewGlobalKelvin(globalKelvin);
                      }}
                      variant="list"
                      className="flex-1 min-w-0 justify-center"
                    >
                      <span className="text-sm font-medium">{label}</span>
                    </OptionTile>
                  ))}
                </div>

                {globalMode === "color" && (
                  <div className="flex flex-col items-center gap-2 pt-1">
                    <ColorWheel
                      color={globalColorValue}
                      brightness={100}
                      onChange={(c) => {
                        setGlobalColorValue(c);
                        previewGlobalColor(c);
                      }}
                    />
                    <p className="text-xs text-muted-foreground">
                      {Math.round(globalColorValue.h)}°&thinsp;hue · {Math.round(globalColorValue.s * 100)}%&thinsp;sat
                    </p>
                  </div>
                )}

                {globalMode === "kelvin" && (
                  <div className="flex flex-col items-center gap-1 pt-1">
                    <KelvinSlider
                      kelvin={globalKelvin}
                      onChange={(k) => {
                        setGlobalKelvin(k);
                        previewGlobalKelvin(k);
                      }}
                    />
                    <p className="text-center text-xs text-muted-foreground">{globalKelvin}K</p>
                  </div>
                )}
              </SettingsSection>
            )}

            {/* Per-device list — hidden for Screen Sync (handled in DevicesTab) */}
            {newTrigger !== SCREEN_SYNC_TRIGGER && (
              <SettingsSection title="Lights in Scene">
                {devices.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No lights discovered. Go to the Lights page to scan your network first.
                  </p>
                ) : (
                  <div className="space-y-5">
                    {roomKeys.map((roomKey) => {
                      const roomDevices = grouped[roomKey];
                      const RoomIcon = getRoomIcon(roomKey === UNASSIGNED_KEY ? undefined : roomKey);
                      const roomLabel = roomKey === UNASSIGNED_KEY ? "Unassigned" : roomKey;
                      return (
                        <div key={roomKey}>
                          <div className="flex items-center gap-2 mb-2">
                            <RoomIcon className="h-4 w-4 text-muted-foreground" />
                            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                              {roomLabel}
                            </span>
                          </div>
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                            {roomDevices.map((device) => {
                              const included = !!newDevices[device.id];
                              const devState = newDevices[device.id];
                              const devMode = getDeviceMode(device.id);

                              // When a global override is active, pass the global values
                              // to the card so it reflects the live preview color.
                              // For kelvin-only devices under a global color override, derive
                              // the equivalent Kelvin temperature from the hue instead.
                            const isKelvinOnly = device.supportsKelvin && !device.supportsColor;
                            const derivedKelvin = isKelvinOnly && globalMode === "color"
                              ? Math.min(
                                  device.maxKelvin ?? 9000,
                                  Math.max(device.minKelvin ?? 2000, hueToKelvin(globalColorValue.h))
                                )
                              : undefined;

                            const displayColor =
                              globalMode === "color" && !isKelvinOnly ? globalColorValue
                              : globalMode === "kelvin" ? undefined
                              : devState?.color;
                            const displayKelvin =
                              derivedKelvin ??
                              (globalMode === "kelvin" ? globalKelvin : (devState?.kelvin ?? DEFAULT_KELVIN));
                            const displayMode: LightMode =
                              globalMode === "color" && isKelvinOnly ? "kelvin"
                              : globalMode === "color" ? "color"
                              : globalMode === "kelvin" ? "kelvin"
                              : devMode;

                            if (!included) {
                              return (
                                <button
                                  key={device.id}
                                  type="button"
                                  onClick={() => toggleDeviceInScene(device.id)}
                                  className="flex items-center gap-3 rounded-xl bg-card/60 px-4 py-3 text-sm text-muted-foreground hover:text-foreground hover:bg-card transition-all w-full opacity-50 hover:opacity-100"
                                >
                                  <div className="h-10 w-10 rounded-full bg-white/5 flex items-center justify-center shrink-0">
                                    <Lightbulb className="h-5 w-5" />
                                  </div>
                                  <span className="flex-1 text-left font-medium">{device.name}</span>
                                  <Plus className="h-4 w-4 shrink-0" />
                                </button>
                              );
                            }

                            return (
                              <div key={device.id} className="relative group/scenecard">
                                <LightCard
                                  device={device}
                                  on={devState?.on ?? true}
                                  brightness={Math.round((devState?.brightness ?? 0.8) * 100)}
                                  kelvin={displayKelvin}
                                  color={displayColor}
                                  mode={displayMode}
                                  colorLocked={globalMode !== "none"}
                                  onToggle={(on) => {
                                    updateDeviceState(device.id, { on });
                                    lightActions.toggleLight(device.id, on);
                                  }}
                                  onBrightness={(value) => {
                                    const k = newDevices[device.id]?.kelvin;
                                    updateDeviceState(device.id, { brightness: value / 100 });
                                    if (k) {
                                      lightActions.setTemperature(device.id, k, value / 100);
                                    } else {
                                      lightActions.setBrightness(device.id, value);
                                    }
                                  }}
                                  onModeSwitch={(m) => switchDeviceMode(device.id, m)}
                                  onKelvin={(k) => {
                                    updateDeviceState(device.id, { kelvin: k, color: undefined });
                                    lightActions.setTemperature(
                                      device.id,
                                      k,
                                      newDevices[device.id]?.brightness ?? 0.8
                                    );
                                  }}
                                  onColor={(c) => {
                                    updateDeviceState(device.id, {
                                      color: c,
                                      brightness: c.b,
                                      kelvin: undefined,
                                    });
                                    lightActions.setColor(device.id, c);
                                  }}
                                />
                                {/* Remove from scene — appears on hover */}
                                <button
                                  type="button"
                                  title="Remove from scene"
                                  onClick={() => toggleDeviceInScene(device.id)}
                                  className="absolute -top-1.5 -right-1.5 z-30 h-5 w-5 rounded-full bg-card border border-border shadow-sm flex items-center justify-center opacity-0 group-hover/scenecard:opacity-100 transition-opacity"
                                >
                                  <X className="h-3 w-3 text-muted-foreground" />
                                </button>
                              </div>
                            );
                            })}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
            </SettingsSection>
            )}
          </div>

          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={cancelEdit}
              className="px-4 py-2 rounded-lg text-sm text-muted-foreground hover:text-foreground hover:bg-white/10 transition-colors"
            >
              Cancel
            </button>
            <Button onClick={handleSave} disabled={!newName.trim() || !isDirty}>
              <Check className="h-4 w-4" />
              {creating ? "Create Scene" : "Save Changes"}
            </Button>
          </div>
        </Card>
      )}

      {!isEditing && scenes.length === 0 && (
        <Card className="flex flex-col items-center justify-center py-16 text-center">
          <Film className="h-12 w-12 text-muted-foreground/40 mb-4" />
          <h3 className="text-lg font-semibold">No Scenes Yet</h3>
          <p className="text-sm text-muted-foreground mt-2 max-w-md">
            Create a scene to define what your lights should do when your camera turns on or off.
          </p>
          <Button onClick={() => startCreate()} className="mt-6">
            <Plus className="h-4 w-4" />
            Create Your First Scene
          </Button>
        </Card>
      )}

      {!isEditing && scenes.length > 0 && (
        <div className="space-y-2">
          {scenes.map((scene) => (
            <SceneRow
              key={scene.id ?? ""}
              scene={scene as AppScene}
              isActive={activeSceneId === scene.id}
              devices={devices}
              onActivate={() => scene.id && handleActivate(scene.id)}
              onStop={() => scene.id && handleStop(scene.id)}
              onEdit={() => startEdit(scene)}
              onDelete={() => scene.id && handleDelete(scene.id)}
              onClone={() => scene.id && handleClone(scene.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
