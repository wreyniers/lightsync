import { useRef, useState, useEffect, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Lightbulb, Info, Palette } from "lucide-react";
import { ColorPanel } from "@/components/ui/ColorPanel";
import type { Device, Color, LightMode } from "@/lib/types";
import { kelvinToCSS, hsbToCSS } from "@/lib/utils";

interface LightCardProps {
  device: Device;
  on: boolean;
  /** Current brightness, 0–100. */
  brightness: number;
  kelvin: number;
  color: Color | undefined;
  mode: LightMode;
  onToggle: (on: boolean) => void;
  /**
   * Called when the user drags brightness on the card while NOT in color mode.
   * The parent is responsible for calling setTemperature or setBrightness as appropriate.
   */
  onBrightness: (value: number) => void;
  onModeSwitch: (mode: LightMode) => void;
  onKelvin: (k: number) => void;
  onColor: (c: Color) => void;
  /** When true the palette button is shown but locked — no panel opens. */
  colorLocked?: boolean;
}

function accentColor(color: Color | undefined, kelvin: number | undefined): string {
  if (color) return hsbToCSS(color.h, color.s, 1);
  if (kelvin) return kelvinToCSS(kelvin);
  return "rgb(253, 224, 136)";
}

function DeviceInfoRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex justify-between gap-2">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="text-right break-all">{value}</span>
    </div>
  );
}

function DeviceInfoTooltip({ device }: { device: Device }) {
  const [visible, setVisible] = useState(false);
  const [pos, setPos] = useState({ top: 0, left: 0 });
  const anchorRef = useRef<HTMLDivElement>(null);
  const minBright = device.brand === "elgato" ? 3 : 0;
  const tempRange =
    device.supportsKelvin && device.minKelvin && device.maxKelvin
      ? `${device.minKelvin.toLocaleString()}K – ${device.maxKelvin.toLocaleString()}K${device.kelvinStep && device.kelvinStep > 1 ? ` (step: ${device.kelvinStep}K)` : ""}`
      : device.supportsKelvin
      ? "Supported"
      : "Not supported";

  function handleMouseEnter() {
    if (anchorRef.current) {
      const r = anchorRef.current.getBoundingClientRect();
      setPos({ top: r.top + window.scrollY, left: r.left + r.width / 2 + window.scrollX });
    }
    setVisible(true);
  }

  // Close tooltip if the component unmounts while visible
  useEffect(() => () => setVisible(false), []);

  return (
    <div
      ref={anchorRef}
      className="pointer-events-auto"
      onMouseEnter={handleMouseEnter}
      onMouseLeave={() => setVisible(false)}
      onClick={(e) => e.stopPropagation()}
    >
      <Info className="h-3 w-3 text-muted-foreground/70 hover:text-muted-foreground cursor-help transition-colors" />
      {visible && createPortal(
        <div
          className="pointer-events-none fixed z-[9999] w-64 rounded-lg bg-muted text-popover-foreground shadow-lg p-3 space-y-1.5 text-xs"
          style={{ top: pos.top - 8, left: pos.left, transform: "translate(-50%, -100%)" }}
        >
          <div className="font-medium text-foreground mb-2">Device info</div>
          <DeviceInfoRow label="IP" value={<span className="font-mono">{device.lastIp}</span>} />
          {device.model && <DeviceInfoRow label="Model" value={device.model} />}
          {device.firmwareVersion && <DeviceInfoRow label="Firmware" value={device.firmwareVersion} />}
          <DeviceInfoRow label="Brightness" value={`${minBright}% – 100%`} />
          <DeviceInfoRow label="Color" value={device.supportsColor ? "Full RGB" : "Not supported"} />
          <DeviceInfoRow label="Temperature" value={tempRange} />
        </div>,
        document.body
      )}
    </div>
  );
}

export function LightCard({
  device,
  on,
  brightness,
  kelvin,
  color,
  mode,
  onToggle,
  onBrightness,
  onModeSwitch,
  onKelvin,
  onColor,
  colorLocked = false,
}: LightCardProps) {
  const [panelOpen, setPanelOpen] = useState(false);
  const paletteRef = useRef<HTMLButtonElement>(null);

  // Close the color panel whenever the light is turned off.
  useEffect(() => {
    if (!on) setPanelOpen(false);
  }, [on]);
  const minBrightness = device.brand === "elgato" ? 3 : 0;
  const hasColorControl = device.supportsColor || device.supportsKelvin;
  const lightAccent = accentColor(color, kelvin);
  const lightAccentRing = lightAccent.replace(/^rgb\(/, "rgba(").replace(/\)$/, ", 0.5)");

  function handleBrightness(newValue: number) {
    if (color) {
      // Color mode: update brightness while preserving hue + saturation.
      onColor({ h: color.h, s: color.s, b: newValue / 100 });
    } else {
      // Kelvin / plain-brightness mode — parent picks the right action.
      onBrightness(newValue);
    }
  }

  return (
    <>
      <div className="relative overflow-hidden rounded-xl bg-card transition-colors">
        {/* Brightness fill — visual indicator, fills left-to-right */}
        <div
          aria-hidden
          className="absolute inset-y-0 left-0 pointer-events-none transition-[width] duration-75"
          style={{
            width: `${brightness}%`,
            background: lightAccent,
            opacity: on ? 0.35 : 0.07,
          }}
        />

        {/* Card content — z-20 sits above the range input; pointer-events-none lets
            drags reach the range input, while interactive children opt back in */}
        <div className="relative z-20 pointer-events-none flex items-center gap-3 px-4 py-3">
          {/* Icon circle — doubles as the on/off toggle; always full opacity so hover is visible */}
          <button
            type="button"
            title={on ? "Turn off" : "Turn on"}
            onClick={(e) => {
              e.stopPropagation();
              onToggle(!on);
            }}
            className="pointer-events-auto h-10 w-10 shrink-0 rounded-full flex items-center justify-center transition-all duration-200 focus:outline-none focus:ring-2 hover:scale-110 hover:brightness-125"
            style={{ background: on ? "rgba(255,255,255,0.12)" : "rgba(255,255,255,0.06)", ["--tw-ring-color" as string]: lightAccentRing } as CSSProperties}
          >
            <Lightbulb className={`h-5 w-5 text-white transition-opacity duration-200 ${on ? "opacity-100" : "opacity-30"}`} />
          </button>

          {/* Name, brightness, palette — dimmed when off */}
          <div className={`flex flex-1 items-center gap-3 min-w-0 transition-opacity duration-200 ${on ? "opacity-100" : "opacity-15"}`}>
          {/* Name + model */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-1">
              <p className="font-medium text-sm leading-tight truncate">{device.name}</p>
              <DeviceInfoTooltip device={device} />
            </div>
            {device.model && (
              <p className="text-xs text-muted-foreground mt-0.5 truncate">{device.model}</p>
            )}
          </div>

          {/* Brightness — vertically centred in the flex row */}
          <span className="text-xs text-muted-foreground tabular-nums shrink-0">
            {brightness}%
          </span>

          {/* Palette button — opens color / temperature panel; locked when a global override is active or light is off */}
          {hasColorControl && (
            <button
              ref={paletteRef}
              type="button"
              title={
                !on ? "Turn on the light to adjust color"
                : colorLocked ? "Color controlled by global override"
                : "Adjust color / temperature"
              }
              disabled={!on}
              onClick={colorLocked || !on ? undefined : (e) => {
                e.stopPropagation();
                setPanelOpen((v) => !v);
              }}
              className={`pointer-events-auto h-10 w-10 shrink-0 rounded-full flex items-center justify-center transition-colors duration-200 focus:outline-none ${
                colorLocked || !on ? "cursor-not-allowed" : "focus:ring-2"
              }`}
              style={{
                background: panelOpen ? "rgba(255,255,255,0.14)" : "rgba(255,255,255,0.08)",
                ...(!colorLocked && on && { ["--tw-ring-color" as string]: lightAccentRing }),
              } as CSSProperties}
            >
              <Palette className="h-5 w-5 text-white" />
            </button>
          )}
          </div>{/* end dim wrapper */}

        </div>

        {/* Transparent range input — full-card drag surface for brightness; disabled when light is off */}
        <input
          type="range"
          min={minBrightness}
          max={100}
          value={brightness}
          disabled={!on}
          onChange={(e) => handleBrightness(Number(e.target.value))}
          className={`absolute inset-0 w-full h-full appearance-none opacity-0 z-10 ${on ? "cursor-ew-resize" : "cursor-not-allowed"}`}
          aria-label={`${device.name} brightness`}
        />
      </div>

      {/* ColorPanel rendered via portal — not clipped by card's overflow:hidden */}
      {hasColorControl && (
        <ColorPanel
          open={panelOpen}
          anchorRef={paletteRef}
          onClose={() => setPanelOpen(false)}
          device={device}
          mode={mode}
          color={color}
          kelvin={kelvin}
          brightness={brightness}
          onModeSwitch={onModeSwitch}
          onKelvin={onKelvin}
          onColor={onColor}
        />
      )}
    </>
  );
}
