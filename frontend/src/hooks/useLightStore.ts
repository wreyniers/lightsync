import { useSyncExternalStore } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { Device, DeviceState, Color, Scene } from "@/lib/types";
import { DEFAULT_KELVIN } from "@/lib/types";
import {
  GetDevices,
  GetLightState,
  TurnOnLight,
  TurnOffLight,
  SetLightState,
  DiscoverLights,
  GetActiveScene,
  GetScene,
} from "../../wailsjs/go/main/App";
import { lights } from "../../wailsjs/go/models";

interface LightStoreState {
  devices: Device[];
  deviceOn: Record<string, boolean>;
  brightness: Record<string, number>;
  kelvin: Record<string, number>;
  color: Record<string, Color>;
  activeScene: Scene | null;
}

let state: LightStoreState = {
  devices: [],
  deviceOn: {},
  brightness: {},
  kelvin: {},
  color: {},
  activeScene: null,
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

async function fetchLightStates(deviceList: Device[]) {
  const results = await Promise.allSettled(
    deviceList.map((d) =>
      GetLightState(d.id).then((s) => ({ id: d.id, state: s }))
    )
  );
  const onOff = { ...state.deviceOn };
  const bright = { ...state.brightness };
  const temps = { ...state.kelvin };
  const colors = { ...state.color };
  for (const r of results) {
    if (r.status === "fulfilled") {
      const { id, state: s } = r.value;
      onOff[id] = s.on;
      // Skip brightness/kelvin/color while a user action or scene apply is in flight.
      // Slow devices (Elgato HTTP) can return stale state before the hardware
      // applies the change, which would clobber the optimistic store update.
      if (!recentlySetByUser(id) && !recentlyAppliedByScene(id)) {
        if (s.brightness != null) bright[id] = Math.round(s.brightness * 100);
        if (s.kelvin != null) temps[id] = s.kelvin;
        if (s.color != null) colors[id] = s.color;
      }
    }
  }
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors };
  emit();
}

async function refreshDevices() {
  try {
    const d = await GetDevices();
    const devs = d || [];

    const results = await Promise.allSettled(
      devs.map((dev) =>
        GetLightState(dev.id).then((s) => ({ id: dev.id, state: s }))
      )
    );
    const onOff = { ...state.deviceOn };
    const bright = { ...state.brightness };
    const temps = { ...state.kelvin };
    const colors = { ...state.color };
    for (const r of results) {
      if (r.status === "fulfilled") {
        const { id, state: s } = r.value;
        onOff[id] = s.on;
        if (!recentlySetByUser(id) && !recentlyAppliedByScene(id)) {
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
    }

    state = { ...state, devices: devs, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors };
    emit();

    if (devs.length > 0 && Object.keys(onOff).length < devs.length) {
      setTimeout(() => fetchLightStates(state.devices), 2000);
    }
  } catch { /* swallow – caller can retry */ }
}

async function discoverLights(): Promise<Device[]> {
  const result = await DiscoverLights();
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
    if (on) await TurnOnLight(deviceId);
    else await TurnOffLight(deviceId);
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
      await SetLightState(deviceId, s);
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
      await SetLightState(deviceId, s);
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
      await SetLightState(deviceId, s);
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
      await SetLightState(deviceId, s);
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
  state = { ...state, deviceOn: onOff, brightness: bright, kelvin: temps, color: colors, activeScene: scene };
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
      SetLightState(id, buildDeviceCommand(ds))
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
    const id = await GetActiveScene();
    if (!id) return;
    const scene = await GetScene(id);
    if (scene?.devices && Object.keys(scene.devices).length > 0) {
      setActiveSceneWithStates(scene as Scene, scene.devices);
    } else if (scene) {
      state = { ...state, activeScene: scene as Scene };
      emit();
    }
  } catch {
    /* ignore */
  }
}

function setActiveSceneOptimistic(scene: Scene) {
  if (scene?.devices && Object.keys(scene.devices).length > 0) {
    setActiveSceneWithStates(scene, scene.devices);
  } else if (scene) {
    state = { ...state, activeScene: scene };
    emit();
  }
}

export const lightActions = {
  refreshDevices,
  discoverLights,
  toggleLight,
  setBrightness,
  setKelvin,
  setTemperature,
  setColor,
  applySceneStates,
  setActiveSceneOptimistic,
  previewSceneStates,
  restoreLightStates,
  refreshLightStates: () => fetchLightStates(state.devices),
  hydrateActiveScene,
};

export function useLightStore(): LightStoreState {
  return useSyncExternalStore(subscribe, getSnapshot);
}

// When scene:active fires, apply preset states AND active scene in one atomic update.
// Layout reads both from the store, so no race and no second source.
function setupSceneActiveListener() {
  try {
    EventsOn("scene:active", (payload: unknown) => {
      if (!payload || !state.devices.length) return;
      const scene = typeof payload === "object" && payload !== null && "devices" in payload && payload.devices
        ? (payload as Scene)
        : null;
      if (scene?.devices && Object.keys(scene.devices).length > 0) {
        setActiveSceneWithStates(scene, scene.devices);
      } else if (scene) {
        state = { ...state, activeScene: scene };
        emit();
      }
    });
  } catch {
    setTimeout(setupSceneActiveListener, 500);
  }
}
setupSceneActiveListener();

// camera:state fires when webcam turns on/off. The backend then emits scene:active
// with the full scene — we apply that and do NOT refetch from hardware here.
// fetchLightStates would overwrite our optimistic update with stale hardware state
// (slow devices like Elgato haven't applied the change yet).
