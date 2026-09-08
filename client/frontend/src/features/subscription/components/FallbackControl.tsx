import { App, Select } from "antd";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import type {
  FallbackSelection,
  SubscriptionProjection,
  SubscriptionSummary,
} from "../../../types/subscription";
import { useSubscriptionPageStateField } from "../SubscriptionPageStateContext";

interface FallbackControlProps {
  subscription?: SubscriptionSummary;
  projection: SubscriptionProjection | null;
  busy: boolean;
  onChange: (selection: FallbackSelection) => Promise<void>;
}

export function FallbackControl({
  subscription,
  projection,
  busy,
  onChange,
}: FallbackControlProps) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [notices, setNotices] = useSubscriptionPageStateField("fallbackResetNotices");
  const ready =
    projection?.status === "ready" &&
    projection.subscriptionId === subscription?.id;
  const fallback = ready ? projection.fallback : null;
  const selection = subscription?.fallback ?? { mode: "none", selector: "" };
  const originalTarget = fallback
    ? fallback.originalTarget === "DIRECT"
      ? t("subscription.fallback.direct")
      : fallback.originalTarget || t("subscription.fallback.undefined")
    : t("subscription.fallback.unavailable");
  const selectors = fallback?.selectors ?? [];
  const options = [
    {
      value: "none",
      label: t("subscription.fallback.preserve", { target: originalTarget }),
    },
    { value: "direct", label: t("subscription.fallback.direct") },
    ...selectors.map((name) => ({ value: `selector:${name}`, label: name })),
    ...(selection.mode === "selector" && !selectors.includes(selection.selector)
      ? [{
          value: `selector:${selection.selector}`,
          label: selection.selector,
          disabled: true,
        }]
      : []),
  ];

  useEffect(() => {
    if (
      !subscription?.fallbackResetTarget ||
      notices[subscription.id] === subscription.fallbackOverrideRevision
    ) return;
    setNotices((current) => ({
      ...current,
      [subscription.id]: subscription.fallbackOverrideRevision,
    }));
    void message.warning({
      key: `fallback-reset:${subscription.id}:${subscription.fallbackOverrideRevision}`,
      content: t("subscription.fallback.reset", {
        selector: subscription.fallbackResetTarget,
      }),
      duration: 5,
    });
  }, [message, notices, setNotices, subscription, t]);

  return (
    <div className="subscription-fallback-control">
      <span className="feature-title-with-help">
        <span>{t("subscription.fallback.title")}</span>
        <FeatureHelp compact topic="fallbackTraffic" />
      </span>
      <Select
        size="small"
        aria-label={t("subscription.fallback.title")}
        className="subscription-fallback-select"
        disabled={busy || !ready}
        loading={busy}
        onChange={(value) =>
          void onChange(value.startsWith("selector:")
            ? { mode: "selector", selector: value.slice("selector:".length) }
            : { mode: value as "none" | "direct", selector: "" })
        }
        options={options}
        optionFilterProp="label"
        showSearch
        value={selection.mode === "selector"
          ? `selector:${selection.selector}`
          : selection.mode}
      />
    </div>
  );
}
