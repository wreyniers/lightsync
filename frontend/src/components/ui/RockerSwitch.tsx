interface RockerSwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  color?: string;
  className?: string;
}

export function RockerSwitch({ checked, onChange, color, className }: RockerSwitchProps) {
  const style = color ? ({ "--rs-light-color": color } as React.CSSProperties) : undefined;

  return (
    <label className={`rs${className ? ` ${className}` : ""}`} style={style}>
      <input
        className="rs__input"
        type="checkbox"
        role="switch"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="rs__surface">
        <span className="rs__surface-glare" />
      </span>
      <span className="rs__inner-shadow" />
      <span className="rs__inner">
        <span className="rs__inner-glare" />
      </span>
      <span className="rs__rocker-shadow" />
      <span className="rs__rocker-sides">
        <span className="rs__rocker-sides-glare" />
      </span>
      <span className="rs__rocker">
        <span className="rs__rocker-glare" />
      </span>
      <span className="rs__light">
        <span className="rs__light-inner" />
      </span>
    </label>
  );
}
