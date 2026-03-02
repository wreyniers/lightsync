interface SettingsLabelProps {
  children: React.ReactNode;
}

/** Subsection label for controls. */
export function SettingsLabel({ children }: SettingsLabelProps) {
  return (
    <p className="text-xs text-muted-foreground mb-2 block">
      {children}
    </p>
  );
}
