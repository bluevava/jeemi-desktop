import { LoadingOutlined, ThunderboltOutlined } from "@ant-design/icons";
import { Tooltip } from "antd";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { ProxyDelayResult } from "../useProxyDelayQueue";

interface SelectorDelayBadgeProps {
  result?: ProxyDelayResult;
  busy: boolean;
  disabled: boolean;
  label: string;
  title: string;
  idle?: ReactNode;
  onClick: () => void;
}

/** Nodes test delay; nested groups use the same badge to browse their members. */
export function SelectorDelayBadge({
  result, busy, disabled, label, title, idle = <ThunderboltOutlined />, onClick,
}: SelectorDelayBadgeProps) {
  const { t } = useTranslation();
  const delayClass = result
    ? result.status === "error" ? " error"
      : result.delay <= 200 ? " fast" : result.delay <= 500 ? " medium" : " slow"
    : "";
  return (
    <Tooltip title={title}>
      <span className="selector-node-delay-wrap">
        <button
          type="button"
          aria-label={label}
          className={`selector-node-delay${delayClass}`}
          disabled={disabled}
          onClick={onClick}
        >
          {busy ? <LoadingOutlined spin />
            : result?.status === "success" ? `${result.delay} ms`
              : result?.status === "error" ? t("subscription.delay.failed") : idle}
        </button>
      </span>
    </Tooltip>
  );
}
