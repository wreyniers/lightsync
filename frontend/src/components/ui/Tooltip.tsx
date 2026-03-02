import type { ReactNode } from "react";

interface TooltipProps {
  content: string;
  children: ReactNode;
  /** Side the tooltip appears on. Defaults to "top". */
  side?: "top" | "bottom" | "left" | "right";
}

/**
 * Lightweight CSS-only tooltip.  Wrap any element to add a hover tooltip.
 * Uses group/peer pattern so no JS state is required.
 */
export function Tooltip({ content, children, side = "top" }: TooltipProps) {
  const posClasses: Record<string, string> = {
    top:    "bottom-full left-1/2 -translate-x-1/2 mb-2",
    bottom: "top-full left-1/2 -translate-x-1/2 mt-2",
    left:   "right-full top-1/2 -translate-y-1/2 mr-2",
    right:  "left-full top-1/2 -translate-y-1/2 ml-2",
  };

  return (
    <span className="relative inline-flex group/tip">
      {children}
      <span
        role="tooltip"
        className={`
          pointer-events-none absolute z-50 ${posClasses[side]}
          w-max max-w-[200px] rounded-lg px-2.5 py-1.5
          bg-popover border border-border/60
          text-[11px] leading-tight text-foreground shadow-lg
          opacity-0 group-hover/tip:opacity-100
          scale-95 group-hover/tip:scale-100
          transition-all duration-150
          whitespace-normal text-center
        `}
      >
        {content}
      </span>
    </span>
  );
}
