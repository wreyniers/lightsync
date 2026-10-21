import { type ReactNode } from "react";
import { Tooltip } from "@/components/ui/Tooltip";

interface SettingsSectionProps {
  title?: string;
  /** Custom header content (overrides title when provided) */
  header?: ReactNode;
  children: ReactNode;
  /** Optional tooltip content for the title */
  titleTooltip?: string;
}

/** Section card with optional header. Centralizes section styling. */
export function SettingsSection({ title, header, children, titleTooltip }: SettingsSectionProps) {
  const titleClass = "text-xs font-medium uppercase tracking-widest text-muted-foreground";
  return (
    <div className="rounded-xl bg-muted p-4 space-y-4">
      {header !== undefined ? (
        header
      ) : title ? (
        titleTooltip ? (
          <Tooltip content={titleTooltip} side="right">
            <p className={`${titleClass} cursor-help`}>{title}</p>
          </Tooltip>
        ) : (
          <p className={titleClass}>{title}</p>
        )
      ) : null}
      {children}
    </div>
  );
}
