import type { ReactNode } from "react";

interface StatusPillProps {
  label: string;
  value: string;
  active?: boolean;
  warning?: boolean;
  icon?: ReactNode;
}

export function StatusPill({
  label,
  value,
  active = false,
  warning = false,
  icon,
}: StatusPillProps) {
  const stateClass = active ? "active" : warning ? "warning" : "idle";

  return (
    <div className={`status-pill ${stateClass}`}>
      {icon}
      <span className="status-pill-label">{label}</span>
      <span className="status-pill-value">{value}</span>
    </div>
  );
}
