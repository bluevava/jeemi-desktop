import { Button, Tooltip } from "antd";
import { useTranslation } from "react-i18next";

import type { SubscriptionSummary } from "../../../types/subscription";

export type SubscriptionAssociationKind = "script" | "config" | "chain";

interface SubscriptionAssociationsProps {
  subscription: SubscriptionSummary;
  localScriptName?: string;
  localConfigName?: string;
  chainGroupNames: Record<string, string>;
  disabled: boolean;
  pending: SubscriptionAssociationKind | null;
  onBind: (kind: SubscriptionAssociationKind) => void;
  onUnbind: (kind: SubscriptionAssociationKind) => void;
}

export function SubscriptionAssociations({
  subscription, localScriptName, localConfigName, chainGroupNames,
  disabled, pending, onBind, onUnbind,
}: SubscriptionAssociationsProps) {
  const { t } = useTranslation();
  const missing = t("subscription.card.resourceUnavailable");
  const chainIDs = subscription.chainProxyGroupIds ?? [];
  const rows: { kind: SubscriptionAssociationKind; label: string; bound: boolean; name: string }[] = [
    { kind: "script", label: t("subscription.card.handlerScript"), bound: !!subscription.localScriptId, name: localScriptName || missing },
    { kind: "config", label: t("subscription.card.handlerConfig"), bound: !!subscription.localConfigId, name: localConfigName || missing },
    { kind: "chain", label: t("subscription.card.handlerChain"), bound: chainIDs.length > 0, name: chainIDs.map((id) => chainGroupNames[id] || missing).join(" · ") },
  ];

  return (
    <div className="subscription-card-associations" onClick={(event) => event.stopPropagation()}>
      {rows.map(({ kind, label, bound, name }) => (
        <div className={`subscription-association-row${bound ? " bound" : ""}`} key={kind}>
          <span className="subscription-association-label">{label}</span>
          {bound ? (
            <Tooltip title={name}>
              <button
                aria-label={t("subscription.card.changeBinding", { type: label, name })}
                className="subscription-association-name"
                disabled={disabled}
                onClick={() => onBind(kind)}
                type="button"
              >
                {name}
              </button>
            </Tooltip>
          ) : <span className="subscription-association-space" />}
          <Button
            aria-label={t(bound ? "subscription.card.unbindType" : "subscription.card.bindType", { type: label })}
            className="subscription-association-action"
            disabled={disabled}
            loading={pending === kind}
            onClick={() => bound ? onUnbind(kind) : onBind(kind)}
            size="small"
            type="text"
          >
            {t(bound ? "subscription.card.unbind" : "subscription.card.bind")}
          </Button>
        </div>
      ))}
    </div>
  );
}
