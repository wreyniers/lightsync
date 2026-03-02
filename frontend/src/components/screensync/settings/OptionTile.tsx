import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

interface OptionTileProps {
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
  /** Layout: "grid" = flex-col centered, "list" = full width justify-between */
  variant?: "grid" | "list";
  /** When true, tile is dimmed and non-clickable */
  disabled?: boolean;
  className?: string;
  title?: string;
}

/** Selectable option tile. Centralizes selected/unselected styling. */
export function OptionTile({
  selected,
  onClick,
  children,
  variant = "grid",
  disabled = false,
  className,
  title,
}: OptionTileProps) {
  return (
    <button
      type="button"
      onClick={disabled ? undefined : onClick}
      disabled={disabled}
      title={title}
      className={cn(
        "transition-all rounded-xl text-left",
        variant === "grid" && "flex flex-col gap-1 p-3",
        variant === "list" && "w-full flex items-center justify-between px-3 py-2.5",
        disabled && "bg-background/20 text-muted-foreground opacity-50 cursor-not-allowed",
        !disabled && selected && "bg-primary/15",
        !disabled && !selected && "bg-background/30 text-muted-foreground hover:bg-background/60 hover:text-foreground",
        className
      )}
    >
      {children}
    </button>
  );
}
