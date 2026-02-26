import { cn } from "@/lib/utils";

export interface SegmentedOption<T extends string> {
  value: T;
  label: string;
  disabled?: boolean;
}

interface SegmentedControlProps<T extends string> {
  options: SegmentedOption<T>[];
  value: T;
  onChange: (value: T) => void;
  className?: string;
}

export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  className,
}: SegmentedControlProps<T>) {
  return (
    <div
      className={cn(
        "flex rounded-lg border border-border overflow-hidden text-xs",
        className
      )}
    >
      {options.map((opt, i) => (
        <div key={opt.value} className="contents">
          {i > 0 && <div className="w-px bg-border" />}
          <button
            type="button"
            onClick={() => !opt.disabled && onChange(opt.value)}
            disabled={opt.disabled}
            title={opt.disabled ? "Not available" : undefined}
            className={cn(
              "flex-1 px-3 py-1.5 transition-colors",
              value === opt.value
                ? "bg-primary/20 text-foreground font-medium"
                : opt.disabled
                ? "opacity-40 cursor-not-allowed text-muted-foreground"
                : "text-muted-foreground hover:bg-secondary"
            )}
          >
            {opt.label}
          </button>
        </div>
      ))}
    </div>
  );
}
