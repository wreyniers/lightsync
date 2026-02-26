import { useRef, useEffect, useState } from "react";
import iro from "@jaames/iro";
import { kelvinToCSS } from "@/lib/utils";

type IroPicker = iro.ColorPicker;
const IroPickerCtor = iro.ColorPicker as unknown as new (
  parent: HTMLElement,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  props: Record<string, any>
) => IroPicker;

interface TemperaturePickerProps {
  kelvin: number;
  brightness: number; // 0–1
  /** Minimum brightness the device supports (0–1). Defaults to 0. */
  minBrightness?: number;
  /** Constrain the kelvin slider to a device-specific range. */
  minKelvin?: number;
  maxKelvin?: number;
  /** Snap kelvin to multiples of this step size. Defaults to 1. */
  kelvinStep?: number;
  onChange: (kelvin: number, brightness: number) => void;
  className?: string;
}

export function TemperaturePicker({
  kelvin,
  brightness,
  minBrightness = 0,
  minKelvin,
  maxKelvin,
  kelvinStep = 1,
  onChange,
  className,
}: TemperaturePickerProps) {
  const [open, setOpen] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const kelvinContainerRef = useRef<HTMLDivElement>(null);
  const brightnessContainerRef = useRef<HTMLDivElement>(null);

  // Two separate iro instances so they never share a color object.
  // The kelvin picker's V always resets to 1.0 on kelvin changes (iro
  // converts kelvin→RGB→HSV which produces max brightness). Since the
  // brightness picker has its own independent color object, those resets
  // are completely invisible to it.
  const kelvinPickerRef = useRef<IroPicker | null>(null);
  const brightnessPickerRef = useRef<IroPicker | null>(null);

  // Refs track the last values emitted to onChange and the stored brightness,
  // so the sync effect can skip React state echoes.
  const lastEmittedKelvin = useRef(kelvin);
  const lastEmittedBrightness = useRef(brightness);
  const storedBrightness = useRef(brightness);

  // ─── Mount iro when popover opens ────────────────────────────────────────
  useEffect(() => {
    if (!open || !kelvinContainerRef.current || !brightnessContainerRef.current) return;

    lastEmittedKelvin.current = kelvin;
    lastEmittedBrightness.current = brightness;
    storedBrightness.current = brightness;

    // ── Kelvin picker (one slider, reads color.kelvin only) ───────────────
    const kPicker = new IroPickerCtor(kelvinContainerRef.current, {
      width: 220,
      color: "#ffffff",
      borderWidth: 1,
      borderColor: "#f3f3f3",
      layout: [
        {
          component: iro.ui.Slider,
          options: {
            sliderType: "kelvin",
            sliderSize: 25,
            // Pass device-specific bounds when provided; otherwise use iro's
            // defaults (2200K–11000K) for the full warm-to-blue gradient.
            ...(minKelvin !== undefined ? { minTemperature: minKelvin } : {}),
            ...(maxKelvin !== undefined ? { maxTemperature: maxKelvin } : {}),
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          } as any,
        },
      ],
    });
    // Setting color.kelvin resets V→1.0 internally; that's fine because
    // we never read V from this picker.
    kPicker.color.kelvin = kelvin;

    // ── Brightness picker (one slider, reads color.value only) ───────────
    const bPicker = new IroPickerCtor(brightnessContainerRef.current, {
      width: 220,
      color: "#ffffff",
      borderWidth: 1,
      borderColor: "#f3f3f3",
      layout: [{ component: iro.ui.Slider, options: { sliderType: "value", sliderSize: 25 } }],
    });
    bPicker.color.value = Math.max(minBrightness, brightness) * 100;

    // input:change fires only on direct user interaction, never programmatically.
    // Each picker has exactly one slider, so no need to inspect `changes` —
    // we know which dimension changed by which picker fires.
    const kHandler = (color: { kelvin: number }) => {
      // Snap to step, then clamp to [minKelvin, maxKelvin] if provided.
      let newKelvin = kelvinStep > 1
        ? Math.round(color.kelvin / kelvinStep) * kelvinStep
        : Math.round(color.kelvin);
      if (minKelvin !== undefined) newKelvin = Math.max(minKelvin, newKelvin);
      if (maxKelvin !== undefined) newKelvin = Math.min(maxKelvin, newKelvin);
      lastEmittedKelvin.current = newKelvin;
      onChange(newKelvin, storedBrightness.current);
    };

    const bHandler = (color: { value: number }) => {
      const newBrightness = Math.max(minBrightness, Math.round(color.value) / 100);
      storedBrightness.current = newBrightness;
      lastEmittedBrightness.current = newBrightness;
      onChange(lastEmittedKelvin.current, newBrightness);
    };

    kPicker.on("input:change", kHandler);
    bPicker.on("input:change", bHandler);

    kelvinPickerRef.current = kPicker;
    brightnessPickerRef.current = bPicker;

    return () => {
      kPicker.off("input:change", kHandler);
      bPicker.off("input:change", bHandler);
      kelvinPickerRef.current = null;
      brightnessPickerRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // ─── Sync genuine external prop changes into iro ─────────────────────────
  // Skip props that are just our own onChange echoes bouncing back through
  // the parent's state update cycle.
  useEffect(() => {
    if (!open) return;

    if (kelvin !== lastEmittedKelvin.current && kelvinPickerRef.current) {
      lastEmittedKelvin.current = kelvin;
      // V reset doesn't matter — kelvin picker's V is never read.
      kelvinPickerRef.current.color.kelvin = kelvin;
    }

    if (brightness !== lastEmittedBrightness.current && brightnessPickerRef.current) {
      lastEmittedBrightness.current = brightness;
      storedBrightness.current = brightness;
      brightnessPickerRef.current.color.value = brightness * 100;
    }
  }, [kelvin, brightness, open]);

  // ─── Close on outside click ───────────────────────────────────────────────
  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  return (
    <div
      ref={wrapperRef}
      className={`relative inline-block${className ? ` ${className}` : ""}`}
    >
      <button
        type="button"
        title="Pick temperature"
        onClick={() => setOpen((v) => !v)}
        className="h-8 w-8 rounded-full border-2 border-border hover:border-primary/60 transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
        style={{ background: kelvinToCSS(kelvin) }}
      />

      {open && (
        <div className="absolute z-50 top-10 left-1/2 -translate-x-1/2 bg-card border border-border rounded-xl shadow-2xl p-4 color-picker-panel">
          <div className="mb-3 text-center">
            <span className="text-xs font-medium text-foreground">{kelvin}K</span>
            <span className="text-xs text-muted-foreground">
              {" "}· {Math.round(brightness * 100)}% brightness
            </span>
          </div>

          <div className="mb-1">
            <span className="text-xs text-muted-foreground px-1">Temperature</span>
            <div ref={kelvinContainerRef} />
            <div className="flex justify-between text-xs text-muted-foreground px-1">
              <span>{minKelvin !== undefined ? `${minKelvin}K` : "warm"}</span>
              <span>{maxKelvin !== undefined ? `${maxKelvin}K` : "cool"}</span>
            </div>
          </div>

          <div className="mt-3">
            <span className="text-xs text-muted-foreground px-1">Brightness</span>
            <div ref={brightnessContainerRef} />
          </div>
        </div>
      )}
    </div>
  );
}
