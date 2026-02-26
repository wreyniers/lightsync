import { useRef, useEffect, type RefObject } from "react";
import { createPortal } from "react-dom";
import iro from "@jaames/iro";
import type { Device, Color, LightMode } from "@/lib/types";
import { SegmentedControl } from "@/components/ui/SegmentedControl";

// iro.ColorPicker is typed as a plain function but IS constructible at runtime.
type IroPicker = iro.ColorPicker;
const IroPickerCtor = iro.ColorPicker as unknown as new (
  parent: HTMLElement,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  props: Record<string, any>
) => IroPicker;

// ─── Color wheel (hue + saturation only — brightness is on the card) ─────────

interface ColorWheelProps {
  color: Color | undefined;
  brightness: number; // 0–100, used as the `b` component when emitting onChange
  onChange: (c: Color) => void;
}

export function ColorWheel({ color, brightness, onChange }: ColorWheelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const pickerRef = useRef<IroPicker | null>(null);
  const suppressRef = useRef(false);
  // Track current brightness in a ref so the mount-time handler always sends
  // the latest value without needing to re-initialise iro.
  const brightnessRef = useRef(brightness);

  useEffect(() => {
    brightnessRef.current = brightness;
  }, [brightness]);

  // Initialise iro once on mount (component is only rendered when mode = color).
  useEffect(() => {
    if (!containerRef.current) return;

    const initial = color
      ? { h: color.h, s: color.s * 100, v: 100 }
      : { h: 0, s: 100, v: 100 };

    const picker = new IroPickerCtor(containerRef.current, {
      width: 200,
      color: initial,
      borderWidth: 0,
      layout: [{ component: iro.ui.Wheel }],
    });

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const handleChange = (c: any) => {
      if (suppressRef.current) return;
      const { h, s } = c.hsv;
      onChange({ h, s: s / 100, b: brightnessRef.current / 100 });
    };

    picker.on("color:change", handleChange);
    pickerRef.current = picker;

    return () => {
      picker.off("color:change", handleChange);
      pickerRef.current = null;
      // Clear iro's appended DOM so a Strict Mode remount (or a future re-open)
      // doesn't accumulate duplicate wheels/sliders in the same container.
      if (containerRef.current) containerRef.current.innerHTML = "";
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Sync hue/saturation when changed externally (e.g. scene applied).
  useEffect(() => {
    if (!pickerRef.current || !color) return;
    const current = pickerRef.current.color.hsv;
    const ch = current.h ?? 0;
    const cs = current.s ?? 0;
    if (Math.abs(ch - color.h) < 0.5 && Math.abs(cs - color.s * 100) < 0.5) return;
    suppressRef.current = true;
    pickerRef.current.color.hsv = { h: color.h, s: color.s * 100, v: 100 };
    suppressRef.current = false;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [color?.h, color?.s]);

  return <div ref={containerRef} />;
}

// ─── Kelvin slider (temperature only — brightness is on the card) ─────────────

interface KelvinSliderProps {
  kelvin: number;
  device?: Device;
  onChange: (k: number) => void;
}

export function KelvinSlider({ kelvin, device, onChange }: KelvinSliderProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const pickerRef = useRef<IroPicker | null>(null);
  const lastEmitted = useRef(kelvin);

  // Initialise iro once on mount (component is only rendered when mode = kelvin).
  useEffect(() => {
    if (!containerRef.current) return;

    const picker = new IroPickerCtor(containerRef.current, {
      width: 220,
      color: "#ffffff",
      borderWidth: 0,
      layout: [
        {
          component: iro.ui.Slider,
          options: {
            sliderType: "kelvin",
            sliderSize: 26,
            ...(device?.minKelvin ? { minTemperature: device.minKelvin } : {}),
            ...(device?.maxKelvin ? { maxTemperature: device.maxKelvin } : {}),
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
          } as any,
        },
      ],
    });
    picker.color.kelvin = kelvin;

    const handleChange = (c: { kelvin: number }) => {
      let k =
        device?.kelvinStep && device.kelvinStep > 1
          ? Math.round(c.kelvin / device.kelvinStep) * device.kelvinStep
          : Math.round(c.kelvin);
      if (device?.minKelvin) k = Math.max(device.minKelvin, k);
      if (device?.maxKelvin) k = Math.min(device.maxKelvin, k);
      lastEmitted.current = k;
      onChange(k);
    };

    picker.on("input:change", handleChange);
    pickerRef.current = picker;

    return () => {
      picker.off("input:change", handleChange);
      pickerRef.current = null;
      if (containerRef.current) containerRef.current.innerHTML = "";
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Sync kelvin when changed externally.
  useEffect(() => {
    if (!pickerRef.current || kelvin === lastEmitted.current) return;
    lastEmitted.current = kelvin;
    pickerRef.current.color.kelvin = kelvin;
  }, [kelvin]);

  return <div ref={containerRef} />;
}

// ─── ColorPanel ───────────────────────────────────────────────────────────────

interface ColorPanelProps {
  open: boolean;
  anchorRef: RefObject<HTMLButtonElement | null>;
  onClose: () => void;
  device: Device;
  mode: LightMode;
  color: Color | undefined;
  kelvin: number;
  /** Current brightness (0–100), forwarded to ColorWheel so it can attach the right `b` on color changes. */
  brightness: number;
  onModeSwitch: (mode: LightMode) => void;
  onKelvin: (k: number) => void;
  onColor: (c: Color) => void;
}

export function ColorPanel({
  open,
  anchorRef,
  onClose,
  device,
  mode,
  color,
  kelvin,
  brightness,
  onModeSwitch,
  onKelvin,
  onColor,
}: ColorPanelProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const hasBoth = device.supportsColor && device.supportsKelvin;

  // Position the panel below (or above) the anchor button.
  useEffect(() => {
    if (!open || !anchorRef.current || !panelRef.current) return;
    const anchor = anchorRef.current.getBoundingClientRect();
    const panel = panelRef.current;
    const panelWidth = panel.offsetWidth || 264;

    let top = anchor.bottom + 8;
    let left = anchor.left + anchor.width / 2 - panelWidth / 2;

    // Keep panel within horizontal viewport bounds.
    left = Math.max(8, Math.min(left, window.innerWidth - panelWidth - 8));

    // Flip above the anchor if not enough room below.
    if (top + 320 > window.innerHeight) {
      top = anchor.top - 8;
      panel.style.transform = "translateY(-100%)";
    } else {
      panel.style.transform = "";
    }

    panel.style.top = `${top}px`;
    panel.style.left = `${left}px`;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Close on click outside the panel or its trigger button.
  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (
        panelRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        anchorRef.current &&
        !anchorRef.current.contains(e.target as Node)
      ) {
        onClose();
      }
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open, onClose, anchorRef]);

  if (!open) return null;

  return createPortal(
    <div
      ref={panelRef}
      className="fixed z-50 bg-muted rounded-xl shadow-2xl p-4 color-picker-panel"
      style={{ top: 0, left: 0, width: 264 }}
    >
      {/* Mode toggle — only when device supports both color and temperature */}
      {hasBoth && (
        <div className="mb-4">
          <SegmentedControl<LightMode>
            options={[
              { value: "color", label: "Color" },
              { value: "kelvin", label: "Temperature" },
            ]}
            value={mode}
            onChange={(newMode) => {
              onModeSwitch(newMode);
              if (newMode === "color") {
                // Immediately apply the color the wheel will initialise with so the
                // light switches at once rather than waiting for the first drag.
                const c = color ?? { h: 0, s: 1, b: brightness / 100 };
                onColor({ h: c.h, s: c.s, b: brightness / 100 });
              }
            }}
          />
        </div>
      )}

      {/* Color wheel — hue + saturation, no brightness slider */}
      {device.supportsColor && mode === "color" && (
        <div className="flex flex-col items-center gap-2">
          <ColorWheel color={color} brightness={brightness} onChange={onColor} />
          {color && (
            <p className="text-xs text-muted-foreground">
              {Math.round(color.h)}°&thinsp;hue · {Math.round(color.s * 100)}%&thinsp;sat
            </p>
          )}
        </div>
      )}

      {/* Kelvin slider — temperature only, no brightness slider */}
      {device.supportsKelvin && mode === "kelvin" && (
        <div className="flex flex-col gap-1">
          <KelvinSlider kelvin={kelvin} device={device} onChange={onKelvin} />
          <div className="flex justify-between text-xs text-muted-foreground px-1">
            <span>{device.minKelvin ? `${device.minKelvin}K` : "warm"}</span>
            <span className="font-medium text-foreground">{kelvin}K</span>
            <span>{device.maxKelvin ? `${device.maxKelvin}K` : "cool"}</span>
          </div>
        </div>
      )}
    </div>,
    document.body
  );
}
