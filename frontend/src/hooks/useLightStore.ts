import { useSyncExternalStore } from "react";
import { Events } from "@wailsio/runtime";
import { App } from "@bindings";
import * as lights from "@bindings/internal/lights/models.js";
import type { Device, DeviceState, Color, Scene } from "@/lib/types";
import { DEFAULT_KELVIN, SCREEN_SYNC_TRIGGER } from "@/lib/types";

interface LightStoreState {
  loading: boolean;
  devices: Device[];
  deviceOn: Record<string, boolean>;
  brightness: Record<string, number>;
  kelvin: Record<string, number>;
  color: Record<string, Color>;
  activeScene: Scene | null;
  /** The last scene that was activated; shown in the sidebar when no scene is currently active. */
  lastScene: Scene | null;
  /** Set by the sidebar edit button; consumed (and cleared) by Scenes on mount. */
  pendingEditSceneId: string | null;
}

let state: LightStoreState = {
  loading: true,
  devices: [],
  deviceOn: {},
  brightness: {},
  kelvin: {},
  color: {},
  activeScene: null,
  lastScene: null,
  pendingEditSceneId: null,
};

const listeners = new Set<() => void>();

// Debounce timers for API calls — one per device.
// The optimistic store update fires immediately; the actual HTTP call is held
// back and only sent after the user has stopped changing for SEND_DEBOUNCE_MS.
// This prevents flooding slow devices (Elgato HTTP) with one request per pixel.
const sendTimers: Record<string, ReturnType<typeof setTimeout>> = {};
const SEND_DEBOUNCE_MS = 80;

function scheduleSend(deviceId: string, fn: () => Promise<void>) {
  clearTimeout(sendTimers[deviceId]);
  sendTimers[deviceId] = setTimeout(() => { fn(); }, SEND_DEBOUNCE_MS);
}

// Tracks the last time a user action explicitly set a device's light state.
// fetchLightStates skips overwriting brightness/kelvin/color for devices that
// were updated recently — this prevents a slow device (e.g. Elgato over HTTP)
// from returning its pre-change state and clobbering the optimistic update.
const userSetAt: Record<string, number> = {};
const USER_SET_GRACE_MS = 2000;

// Tracks when a scene was last applied. fetchLightStates skips devices that
// were part of a recently applied scene — hardware may not have caught up yet.
const sceneAppliedAt: Record<string, number> = {};
const SCENE_APPLIED_GRACE_MS = 4000;

// When Screen Sync is running, we push colors from screensync:colors into the store
// so the Lights panel updates. Fetch must not overwrite these devices.
const screenSyncDeviceIds = new Set<string>();

function markUserSet(deviceId: string) {
  userSetAt[deviceId] = Date.now();
}

function recentlySetByUser(deviceId: string): boolean {
  return Date.now() - (userSetAt[deviceId] ?? 0) < USER_SET_GRACE_MS;
}

function markSceneApplied(deviceIds: string[]) {
  const now = Date.now();
  for (const id of deviceIds) {
    sceneAppliedAt[id] = now;
  }
}

function recentlyAppliedByScene(deviceId: string): boolean {
  return Date.now() - (sceneAppliedAt[deviceId] ?? 0) < SCENE_APPLIED_GRACE_MS;
}

function emit() {
  state = { ...state };
  for (const l of listeners) l();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

function getSnapshot(): LightStoreState {
  return state;
}

// ── Actions ────────────────────────────────────────────────────────

type FetchedPair = { id: string; state: DeviceState };

/**
 * Merges fetched light states into the store. Skips brightness/kelvin/color for
 * devices recently set by user, recently applied by scene, or in Screen Sync.
 * Uses color/kelvin mutual exclusion (device is either in color or CT mode).
 */
function mergeFetchedLightStates(pairs: FetchedPair[]): void {
  const onOff = { ...state.deviceOn };
  const bright = { ...state.brightness };
  const temps = { ...state.kelvin };
  const colors = { ...state.color };
  for (const { id, state: s } of pairs) {
    onOff[id] = s.on;
    if (!recentlySetByUser(id) && !recentlyAppliedByScene(id) && !screenSyncDeviceIds.has(id)) {
      if (s.brightness != null) bright[id] = Math.round(s.brightness * 100);
      if (s.color != null) {
        colors[id] = s.color;
        delete temps[id];
      } else if (s.kelvin != null) {
        temps[id] = s.kelvin;
        delete colors[id];
      }
    }
  }
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors };
  emit();
}

async function fetchLightStates(deviceList: Device[]) {
  const results = await Promise.allSettled(
    deviceList.map((d) =>
      App.GetLightState(d.id).then((s) => ({ id: d.id, state: s } as FetchedPair))
    )
  );
  const pairs = results
    .filter((r): r is PromiseFulfilledResult<FetchedPair> => r.status === "fulfilled")
    .map((r) => r.value);
  mergeFetchedLightStates(pairs);
}

async function refreshDevices() {
  try {
    const d = await App.GetDevices();
    const devs = d || [];
    const results = await Promise.allSettled(
      devs.map((dev) =>
        App.GetLightState(dev.id).then((s) => ({ id: dev.id, state: s } as FetchedPair))
      )
    );
    const pairs = results
      .filter((r): r is PromiseFulfilledResult<FetchedPair> => r.status === "fulfilled")
      .map((r) => r.value);
    mergeFetchedLightStates(pairs);
    state = { ...state, devices: devs, loading: false };
    emit();

    if (devs.length > 0 && Object.keys(state.deviceOn).length < devs.length) {
      setTimeout(() => fetchLightStates(state.devices), 2000);
    }
  } catch {
    state = { ...state, loading: false };
    emit();
  }
}

async function discoverLights(): Promise<Device[]> {
  const result = await App.DiscoverLights();
  const devs = result.devices || [];
  state = { ...state, devices: devs };
  emit();
  await fetchLightStates(devs);
  return devs;
}

async function toggleLight(deviceId: string, on: boolean) {
  state = { ...state, deviceOn: { ...state.deviceOn, [deviceId]: on } };
  emit();
  try {
    if (on) await App.TurnOnLight(deviceId);
    else await App.TurnOffLight(deviceId);
  } catch {
    state = { ...state, deviceOn: { ...state.deviceOn, [deviceId]: !on } };
    emit();
  }
}

async function setBrightness(deviceId: string, value: number) {
  markUserSet(deviceId);
  state = { ...state, brightness: { ...state.brightness, [deviceId]: value } };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({
      on: true,
      brightness: value / 100,
      kelvin: state.kelvin[deviceId] || DEFAULT_KELVIN,
    });
    try {
      await App.SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set brightness:", e);
    }
  });
}

async function setKelvin(deviceId: string, value: number) {
  markUserSet(deviceId);
  const colors = { ...state.color };
  delete colors[deviceId];
  state = { ...state, kelvin: { ...state.kelvin, [deviceId]: value }, color: colors };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({
      on: true,
      brightness: (state.brightness[deviceId] || 80) / 100,
      kelvin: value,
    });
    try {
      await App.SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set temperature:", e);
    }
  });
}

// Combined kelvin + brightness update used by TemperaturePicker.
async function setTemperature(deviceId: string, kelvin: number, brightness: number) {
  markUserSet(deviceId);
  const colors = { ...state.color };
  delete colors[deviceId];
  state = {
    ...state,
    kelvin: { ...state.kelvin, [deviceId]: kelvin },
    brightness: { ...state.brightness, [deviceId]: Math.round(brightness * 100) },
    color: colors,
  };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({ on: true, brightness, kelvin });
    try {
      await App.SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set temperature:", e);
    }
  });
}

async function setColor(deviceId: string, color: Color) {
  markUserSet(deviceId);
  // b from the iro value slider IS the brightness; keep both in sync.
  state = {
    ...state,
    color: { ...state.color, [deviceId]: color },
    brightness: { ...state.brightness, [deviceId]: Math.round(color.b * 100) },
  };
  emit();
  scheduleSend(deviceId, async () => {
    const s = new lights.DeviceState({
      on: true,
      brightness: color.b,
      color: new lights.Color(color),
    });
    try {
      await App.SetLightState(deviceId, s);
    } catch (e) {
      console.error("Failed to set color:", e);
    }
  });
}

/** Apply device state from a scene. Clears color when using kelvin and vice versa. */
function applyDeviceStateFromScene(
  onOff: Record<string, boolean>,
  bright: Record<string, number>,
  temps: Record<string, number>,
  colors: Record<string, Color>,
  sceneDevices: Record<string, DeviceState>
) {
  for (const [id, ds] of Object.entries(sceneDevices)) {
    onOff[id] = ds.on;
    if (ds.brightness != null) bright[id] = Math.round(ds.brightness * 100);
    if (ds.color != null) {
      colors[id] = ds.color;
      delete temps[id]; // clear kelvin when using color (LightCard resolves mode)
    } else if (ds.kelvin != null) {
      temps[id] = ds.kelvin;
      delete colors[id]; // clear color when using kelvin
    }
  }
}

function applySceneStates(sceneDevices: Record<string, DeviceState>) {
  const onOff = { ...state.deviceOn };
  const bright = { ...state.brightness };
  const temps = { ...state.kelvin };
  const colors = { ...state.color };
  applyDeviceStateFromScene(onOff, bright, temps, colors, sceneDevices);
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors };
  emit();
}

/** Apply scene states and set active scene in one atomic update. Prevents Layout deviated race. */
function setActiveSceneWithStates(scene: Scene, sceneDevices: Record<string, DeviceState>) {
  const onOff = { ...state.deviceOn };
  const bright = { ...state.brightness };
  const temps = { ...state.kelvin };
  const colors = { ...state.color };
  applyDeviceStateFromScene(onOff, bright, temps, colors, sceneDevices);
  markSceneApplied(Object.keys(sceneDevices));
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors, activeScene: scene, lastScene: scene };
  emit();
}

/** Clear active scene from the store (does not modify lights). */
function clearActiveScene() {
  state = { ...state, activeScene: null };
  emit();
}

/** Set the last-known scene without marking it as active (shown on startup before the user hits play). */
function setLastScene(scene: Scene | null) {
  state = { ...state, lastScene: scene };
  emit();
}

function buildDeviceCommand(ds: DeviceState): InstanceType<typeof lights.DeviceState> {
  const brightness =
    typeof ds.brightness === "number" && ds.brightness <= 1
      ? ds.brightness
      : (ds.brightness ?? 80) / 100;
  return new lights.DeviceState({
    on: ds.on,
    brightness,
    kelvin: ds.kelvin ?? DEFAULT_KELVIN,
    color: ds.color ? new lights.Color(ds.color) : undefined,
  });
}

async function sendToDevices(deviceStates: Record<string, DeviceState>) {
  await Promise.allSettled(
    Object.entries(deviceStates).map(([id, ds]) =>
      App.SetLightState(id, buildDeviceCommand(ds))
    )
  );
}

/** Apply scene states to store and hardware for live preview. Does not activate the scene. */
async function previewSceneStates(sceneDevices: Record<string, DeviceState>) {
  applySceneStates(sceneDevices);
  await sendToDevices(sceneDevices);
}

/** Restore lights to a prior state. Updates the store and sends commands to hardware. */
async function restoreLightStates(
  states: Record<string, DeviceState>,
  deviceList: Device[]
) {
  applySceneStates(states);
  await sendToDevices(states);
  const ids = Object.keys(states);
  await fetchLightStates(deviceList.filter((d) => ids.includes(d.id)));
}

async function hydrateActiveScene() {
  try {
    const id = await App.GetActiveScene();
    if (!id) return;
    const scene = await App.GetScene(id);
    if (scene?.devices && Object.keys(scene.devices).length > 0) {
      setActiveSceneWithStates(scene as Scene, scene.devices);
    } else if (scene) {
      state = { ...state, activeScene: scene as Scene, lastScene: scene as Scene };
      emit();
    }
  } catch {
    /* ignore */
  }
}

/** Load the last-activated scene (from persisted store) as a "ready to play" scene.
 *  Does NOT activate it — the user must click play in the sidebar. */
async function hydrateLastScene() {
  try {
    const id = await App.GetLastSceneID();
    if (!id) return;
    // Don't overwrite an already-active scene.
    if (state.activeScene) return;
    const scene = await App.GetScene(id);
    if (scene) setLastScene(scene as Scene);
  } catch {
    /* ignore */
  }
}

function setActiveSceneOptimistic(scene: Scene) {
  if (scene?.devices && Object.keys(scene.devices).length > 0) {
    setActiveSceneWithStates(scene, scene.devices);
  } else if (scene) {
    state = { ...state, activeScene: scene, lastScene: scene };
    emit();
  }
}

async function setDeviceRoom(deviceId: string, room: string) {
  state = {
    ...state,
    devices: state.devices.map((d) =>
      d.id === deviceId ? { ...d, room: room || undefined } : d
    ),
  };
  emit();
  try {
    await App.SetDeviceRoom(deviceId, room);
  } catch (e) {
    console.error("Failed to set device room:", e);
    await refreshDevices();
  }
}

async function removeDevice(deviceId: string) {
  const deviceOn = { ...state.deviceOn };
  const brightness = { ...state.brightness };
  const kelvin = { ...state.kelvin };
  const color = { ...state.color };
  delete deviceOn[deviceId];
  delete brightness[deviceId];
  delete kelvin[deviceId];
  delete color[deviceId];
  state = {
    ...state,
    devices: state.devices.filter((d) => d.id !== deviceId),
    deviceOn,
    brightness,
    kelvin,
    color,
  };
  emit();
  try {
    await App.RemoveDevice(deviceId);
  } catch (e) {
    console.error("Failed to remove device:", e);
    await refreshDevices();
  }
}

function requestEditScene(sceneId: string) {
  state = { ...state, pendingEditSceneId: sceneId };
  emit();
}

function clearPendingEdit() {
  state = { ...state, pendingEditSceneId: null };
  emit();
}

/** Apply colors from Screen Sync engine to the store so the Lights panel updates live. */
function applyScreenSyncColors(deviceIds: string[], colors: Color[]) {
  if (deviceIds.length === 0 || colors.length === 0) return;
  deviceIds.forEach((id) => screenSyncDeviceIds.add(id));
  const bright = { ...state.brightness };
  const temps = { ...state.kelvin };
  const colorsMap = { ...state.color };
  const onOff = { ...state.deviceOn };
  // Single-color mode emits 1 color; multi-color emits N. Use colors[0] when only one.
  const isSingle = colors.length === 1;
  for (let i = 0; i < deviceIds.length; i++) {
    const id = deviceIds[i];
    const c = isSingle ? colors[0] : colors[i] ?? colors[0];
    bright[id] = Math.round((c.b ?? 0.5) * 100);
    colorsMap[id] = { h: c.h, s: c.s, b: c.b };
    delete temps[id];
    onOff[id] = true;
  }
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colorsMap };
  emit();
}

/** Clear Screen Sync device tracking when sync stops. */
function clearScreenSyncDevices() {
  screenSyncDeviceIds.clear();
}

/**
 * Stops the active scene: stops screen sync if applicable, deactivates the scene,
 * and clears the local store. Use this instead of duplicating stop logic.
 */
async function stopActiveScene(activeScene: { trigger?: string } | null): Promise<void> {
  try {
    if (activeScene?.trigger === SCREEN_SYNC_TRIGGER) {
      await App.StopScreenSync();
    }
    await App.DeactivateScene();
  } catch (e) {
    console.error("Failed to deactivate scene:", e);
  } finally {
    clearActiveScene();
  }
}

export const lightActions = {
  refreshDevices,
  discoverLights,
  requestEditScene,
  clearPendingEdit,
  applyScreenSyncColors,
  clearScreenSyncDevices,
  toggleLight,
  setBrightness,
  setKelvin,
  setTemperature,
  setColor,
  setDeviceRoom,
  removeDevice,
  applySceneStates,
  setActiveSceneOptimistic,
  clearActiveScene,
  stopActiveScene,
  setLastScene,
  previewSceneStates,
  restoreLightStates,
  refreshLightStates: () => fetchLightStates(state.devices),
  hydrateActiveScene,
  hydrateLastScene,
};

export function useLightStore(): LightStoreState {
  return useSyncExternalStore(subscribe, getSnapshot);
}

// When scene:active fires, apply preset states AND active scene in one atomic update.
// Also updates Screen Sync device tracking so we process scene:active once.
let sceneActiveUnsubscribe: (() => void) | undefined;
function setupSceneActiveListener() {
  sceneActiveUnsubscribe?.();
  sceneActiveUnsubscribe = undefined;
  try {
    const setDeviceIdsFromScene = (scene: { screenSync?: { deviceIds?: string[] } } | null) => {
      const ids = scene?.screenSync?.deviceIds;
      if (Array.isArray(ids) && ids.length > 0) {
        screenSyncCachedDeviceIds = ids;
      } else {
        screenSyncCachedDeviceIds = [];
      }
    };

    const off = Events.On("scene:active", (e) => {
      const payload = e.data;
      const scene =
        payload && typeof payload === "object" && "id" in payload
          ? (payload as Scene)
          : null;
      setDeviceIdsFromScene(scene);

      if (!payload || !state.devices.length) return;
      if (scene?.devices && Object.keys(scene.devices).length > 0) {
        setActiveSceneWithStates(scene, scene.devices);
      } else if (scene) {
        state = { ...state, activeScene: scene, lastScene: scene };
        emit();
      }
    });
    sceneActiveUnsubscribe = off;
  } catch {
    setTimeout(setupSceneActiveListener, 500);
  }
}

let screenSyncCachedDeviceIds: string[] = [];

function setupAppLastSceneListener() {
  Events.On("app:last-scene", (e) => {
    const payload = e.data;
    if (state.activeScene) return;
    const scene = typeof payload === "object" && payload !== null && "id" in payload
      ? (payload as Scene)
      : null;
    if (scene) setLastScene(scene);
  });
}

function setupScreenSyncToStoreBridge() {
  const hydrate = (sceneId: string) => {
    App.GetScene(sceneId)
      .then((scene: { screenSync?: { deviceIds?: string[] } }) => {
        const ids = scene?.screenSync?.deviceIds ?? [];
        screenSyncCachedDeviceIds = ids;
      })
      .catch(() => {});
  };
  App.GetScreenSyncState().then((s) => {
    if (s?.running && s?.sceneId) {
      if (state.activeScene?.id === s.sceneId) {
        const ids = state.activeScene?.screenSync?.deviceIds;
        if (Array.isArray(ids) && ids.length > 0) {
          screenSyncCachedDeviceIds = ids;
        }
      }
      hydrate(s.sceneId);
    }
  });
  Events.On("screensync:state", (e) => {
    const data = e.data as { running?: boolean; sceneId?: string };
    if (!data?.running || !data.sceneId) {
      screenSyncCachedDeviceIds = [];
      clearScreenSyncDevices();
      return;
    }
    if (state.activeScene?.id === data.sceneId && Array.isArray(state.activeScene?.screenSync?.deviceIds)) {
      screenSyncCachedDeviceIds = state.activeScene.screenSync.deviceIds;
    }
    hydrate(data.sceneId);
  });
  Events.On("screensync:colors", (e) => {
    const colors = e.data as Color[];
    if (screenSyncCachedDeviceIds.length > 0 && Array.isArray(colors) && colors.length > 0) {
      applyScreenSyncColors(screenSyncCachedDeviceIds, colors);
    }
  });
}

setupSceneActiveListener();
setupAppLastSceneListener();
setupScreenSyncToStoreBridge();


// camera:state fires when webcam turns on/off. The backend then emits scene:active
// with the full scene — we apply that and do NOT refetch from hardware here.
// fetchLightStates would overwrite our optimistic update with stale hardware state
// (slow devices like Elgato haven't applied the change yet).
