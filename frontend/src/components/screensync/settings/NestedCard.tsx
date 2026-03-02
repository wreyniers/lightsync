import type { ReactNode } from "react";

interface NestedCardProps {
  title?: string;
  children: ReactNode;
}

/** Subordinate card for nested settings (e.g. Identity Lock, Flow Track). */
export function NestedCard({ title, children }: NestedCardProps) {
  return (
    <div className="rounded-xl bg-background/20 p-3 space-y-3">
      {title && (
        <p className="text-xs font-medium text-muted-foreground">{title}</p>
      )}
      {children}
    </div>
  );
}
