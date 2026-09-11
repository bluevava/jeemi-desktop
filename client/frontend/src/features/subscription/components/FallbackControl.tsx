import { Alert, App, Button, Modal, Select, Tooltip } from "antd";
import { useEffect, useRef, useState } from "react";
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
  onChange: (selection: FallbackSelection) => Promise<boolean>;
}

export function FallbackControl({
  subscription,
  projection,
  busy,
  onChange,
}: FallbackControlProps) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [editor, setEditor] = useState<{ subscriptionId: string; value: string } | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveFailed, setSaveFailed] = useState(false);
  const submitting = useRef(false);
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
  const selectedValue = selection.mode === "selector"
    ? `selector:${selection.selector}` : selection.mode;
  const locked = busy || saving;
  const validDraft = options.some((option) => option.value === editor?.value && !option.disabled);

  useEffect(() => {
    setEditor(null);
    setSaveFailed(false);
  }, [subscription?.id]);

  const save = async () => {
    if (!editor || !ready || locked || submitting.current || !validDraft) return;
    if (editor.value === selectedValue) {
      setEditor(null);
      return;
    }
    submitting.current = true;
    setSaving(true);
    setSaveFailed(false);
    try {
      const value = editor.value;
      const saved = await onChange(value.startsWith("selector:")
        ? { mode: "selector", selector: value.slice("selector:".length) }
        : { mode: value as "none" | "direct", selector: "" });
      if (saved) setEditor(null);
      else setSaveFailed(true);
    } catch {
      setSaveFailed(true);
    } finally {
      submitting.current = false;
      setSaving(false);
    }
  };

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
    <>
      <Tooltip title={t("subscription.fallback.configure")}>
        <Button
          aria-label={t("subscription.fallback.configure")}
          aria-haspopup="dialog"
          className={`subscription-fallback-trigger${selection.mode === "none" ? "" : " active"}`}
          disabled={locked || !ready}
          onClick={() => {
            if (!subscription) return;
            setSaveFailed(false);
            setEditor({ subscriptionId: subscription.id, value: selectedValue });
          }}
          type="text"
          icon={<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinejoin="round" aria-hidden="true">
            <path d="M16 12c-3-7-10-7-14 0 4 7 11 7 14 0Zm0 0 6-6v12l-6-6Z" />
            <circle cx="7" cy="11" r=".9" fill="currentColor" stroke="none" />
          </svg>}
        />
      </Tooltip>
      <Modal
        title={<span className="feature-title-with-help">
          {t("subscription.fallback.title")}
          <FeatureHelp compact topic="fallbackTraffic" />
        </span>}
        open={ready && editor !== null && editor.subscriptionId === subscription?.id}
        width={480}
        confirmLoading={saving}
        closable={!locked}
        keyboard={!locked}
        maskClosable={!locked}
        cancelButtonProps={{ disabled: locked }}
        okButtonProps={{ disabled: locked || !validDraft }}
        cancelText={t("common.cancel")}
        okText={t("common.save")}
        onCancel={() => { if (!locked) setEditor(null); }}
        onOk={() => void save()}
      >
        <div className="subscription-fallback-editor">
          <Select
            aria-label={t("subscription.fallback.title")}
            className="subscription-fallback-select"
            disabled={locked || !ready}
            loading={locked}
            onChange={(value) => {
              setEditor((current) => current ? { ...current, value } : current);
              setSaveFailed(false);
            }}
            options={options}
            optionFilterProp="label"
            showSearch
            value={editor?.value ?? selectedValue}
          />
          {saveFailed ? <Alert type="error" showIcon description={t("subscription.fallback.saveFailed")} /> : null}
        </div>
      </Modal>
    </>
  );
}
