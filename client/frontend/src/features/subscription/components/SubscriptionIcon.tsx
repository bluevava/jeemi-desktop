import { FileTextOutlined, LinkOutlined } from "@ant-design/icons";
import { useEffect, useState } from "react";

import type {
  SubscriptionIconKind,
  SubscriptionSourceKind,
} from "../../../types/subscription";

interface SubscriptionIconProps {
  className?: string;
  icon: string;
  iconKind: SubscriptionIconKind;
  sourceKind?: SubscriptionSourceKind;
}

export function SubscriptionIcon({
  className = "",
  icon,
  iconKind,
  sourceKind = "url",
}: SubscriptionIconProps) {
  const [failed, setFailed] = useState(false);

  useEffect(() => setFailed(false), [icon, iconKind]);

  return (
    <span
      aria-hidden="true"
      className={`subscription-icon ${className}`.trim()}
    >
      {iconKind === "emoji" && icon ? (
        <span className="subscription-icon-emoji">{icon}</span>
      ) : iconKind === "url" && icon && !failed ? (
        <img
          alt=""
          draggable={false}
          onError={() => setFailed(true)}
          referrerPolicy="no-referrer"
          src={icon}
        />
      ) : sourceKind === "file" ? (
        <FileTextOutlined />
      ) : (
        <LinkOutlined />
      )}
    </span>
  );
}
