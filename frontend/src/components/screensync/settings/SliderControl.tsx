import { Slider } from "@/components/ui/Slider";
import { Tooltip } from "@/components/ui/Tooltip";
import type { ReactNode } from "react";

interface SliderControlProps {
  label: string;
  /** Optional tooltip content for the label */
  tooltip?: string;
  value: ReactNode;
  sliderValue: number;
  sliderMin: number;
  sliderMax: number;
  sliderStep: number;
  onSliderChange: (value: number) => void;
  hint?: { left: ReactNode; right: ReactNode };
  /** Optional description paragraph below the slider */
  description?: ReactNode;
}

/** Standard slider control with label, value display, and optional hint row. */
export function SliderControl({
  label,
  tooltip,
  value,
  sliderValue,
  sliderMin,
  sliderMax,
  sliderStep,
  onSliderChange,
  hint,
  description,
}: SliderControlProps) {
  const labelEl = (
    <span className="text-sm font-medium cursor-help">
      {label}
    </span>
  );

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        {tooltip ? (
          <Tooltip content={tooltip} side="right">
            {labelEl}
          </Tooltip>
        ) : (
          labelEl
        )}
        <span className="text-xs text-muted-foreground font-mono">
          {value}
        </span>
      </div>
      <Slider
        value={sliderValue}
        min={sliderMin}
        max={sliderMax}
        step={sliderStep}
        onChange={onSliderChange}
      />
      {hint && (
        <div className="flex justify-between text-[10px] text-muted-foreground mt-1">
          <span>{hint.left}</span>
          <span>{hint.right}</span>
        </div>
      )}
      {description && (
        <p className="text-xs text-muted-foreground mt-2">{description}</p>
      )}
    </div>
  );
}
