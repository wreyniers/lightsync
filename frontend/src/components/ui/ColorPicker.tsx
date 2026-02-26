import { useRef, useEffect, useState } from "react";
import iro from "@jaames/iro";
import type { Color } from "@/lib/types";
import { hsbToCSS } from "@/lib/utils";

// iro.ColorPicker is typed as a plain function but IS constructible at runtime.
type IroPicker = iro.ColorPicker;
const IroPickerCtor = iro.ColorPicker as unknown as new (
  parent: HTMLElement,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  props: Record<string, any>
) => IroPicker;

interface ColorPickerProps {
  value: Color | null;
  onChange: (color: Color) => void;
  className?: string;
}

export function ColorPicker({ value, onChange, className }: ColorPickerProps) {
  const [open, setOpen] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const iroContainerRef = useRef<HTMLDivElement>(null);
  const pickerRef = useRef<IroPicker | null>(null);
  const suppressRef = useRef(false);

  // Initialise iro every time the popover opens; tear it down when it closes.
  useEffect(() => {
    if (!open || !iroContainerRef.current) return;

    const initial = value
      ? { h: value.h, s: value.s * 100, v: value.b * 100 }
      : { h: 0, s: 100, v: 100 };

    const picker = new IroPickerCtor(iroContainerRef.current, {
      width: 180,
      color: initial,
      borderWidth: 1,
      borderColor: "#f3f3f3",
      layout: [
        { component: iro.ui.Wheel },
        { component: iro.ui.Slider, options: { sliderType: "value" } },
      ],
    });

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const handleChange = (color: any) => {
      if (suppressRef.current) return;
      const { h, s, v } = color.hsv;
      onChange({ h, s: s / 100, b: v / 100 });
    };

    picker.on("color:change", handleChange);
    pickerRef.current = picker;

    return () => {
      picker.off("color:change", handleChange);
      pickerRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Keep iro in sync when value is updated externally (e.g. brightness slider).
  useEffect(() => {
    if (!pickerRef.current || !value || !open) return;
    suppressRef.current = true;
    pickerRef.current.color.hsv = { h: value.h, s: value.s * 100, v: value.b * 100 };
    suppressRef.current = false;
  }, [value, open]);

  // Close the popover on clicks outside the wrapper.
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

  const swatchCSS = value ? hsbToCSS(value.h, value.s, value.b) : undefined;

  return (
    <div
      ref={wrapperRef}
      className={`relative inline-block${className ? ` ${className}` : ""}`}
    >
      {/* Swatch trigger */}
      <button
        type="button"
        title="Pick color"
        onClick={() => setOpen((v) => !v)}
        className="h-8 w-8 rounded-full border-2 border-border hover:border-primary/60 transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
        style={{
          background: swatchCSS
            ?? "conic-gradient(red, yellow, lime, cyan, blue, magenta, red)",
        }}
      />

      {/* Floating iro picker panel */}
      {open && (
        <div className="absolute z-50 top-10 left-1/2 -translate-x-1/2 bg-card border border-border rounded-xl shadow-2xl p-4 color-picker-panel">
          <div ref={iroContainerRef} />
        </div>
      )}
    </div>
  );
}
