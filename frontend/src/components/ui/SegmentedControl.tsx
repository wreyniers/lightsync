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
        "flex bg-background/50 rounded-lg p-0.5 gap-0.5 text-xs",
        className
      )}
    >
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          onClick={() => !opt.disabled && onChange(opt.value)}
          disabled={opt.disabled}
          title={opt.disabled ? "Not available" : undefined}
          className={cn(
            "flex-1 px-3 py-1.5 rounded-md transition-all",
            value === opt.value
              ? "bg-primary text-primary-foreground font-medium shadow-sm"
              : opt.disabled
              ? "opacity-40 cursor-not-allowed text-muted-foreground"
              : "text-muted-foreground hover:text-foreground"
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
