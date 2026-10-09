import { useRef } from "react";
import { App } from "antd";
import { useTranslation } from "react-i18next";

import { copySubscriptionSource, reorderSubscriptions } from "../../services/appBridge";
import type { SubscriptionState, SubscriptionSummary } from "../../types/subscription";
import type { SubscriptionAssociationKind } from "./components/SubscriptionAssociations";
import { unbindSubscriptionAssociation } from "./cardActions";

export function useSubscriptionCardActions({ busyAction, setBusyAction, onSaved, onReordered, onError }: {
  busyAction: string | null;
  setBusyAction: (action: string | null) => void;
  onSaved: (state: SubscriptionState) => void;
  onReordered: (ids: string[]) => void;
  onError: (key: string | null) => void;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const pending = useRef(false);

  const copySource = async (subscription: SubscriptionSummary) => {
    if (busyAction || pending.current) return;
    pending.current = true;
    setBusyAction(`copy:${subscription.id}`);
    onError(null);
    try {
      await copySubscriptionSource(subscription.id);
      void message.success(t("subscription.card.sourceCopied"));
    } catch {
      onError("subscription.errors.copySource");
    } finally {
      pending.current = false;
      setBusyAction(null);
    }
  };

  const unbind = async (subscription: SubscriptionSummary, kind: SubscriptionAssociationKind) => {
    if (busyAction || pending.current) return;
    pending.current = true;
    setBusyAction(`unbind:${subscription.id}:${kind}`);
    onError(null);
    try {
      onSaved(await unbindSubscriptionAssociation(subscription, kind));
    } catch {
      onError("subscription.errors.unbind");
    } finally {
      pending.current = false;
      setBusyAction(null);
    }
  };

  const reorder = async (ids: string[]) => {
    if (busyAction || pending.current) return;
    pending.current = true;
    setBusyAction("reorder");
    onError(null);
    try {
      await reorderSubscriptions(ids);
      onReordered(ids);
    } catch {
      onError("subscription.errors.reorder");
    } finally {
      pending.current = false;
      setBusyAction(null);
    }
  };

  return { copySource, unbind, reorder };
}
