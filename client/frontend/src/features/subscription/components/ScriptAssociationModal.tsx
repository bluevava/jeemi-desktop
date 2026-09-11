import { Alert, Modal, Select, Spin } from "antd";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";

interface ScriptAssociationModalProps {
  open: boolean;
  busy: boolean;
  targetName: string;
  error: string;
  value: string;
  options: { label: string; value: string }[];
  noScripts: boolean;
  onChange: (value: string) => void;
  onCancel: () => void;
  onSave: () => void;
}

export function ScriptAssociationModal({
  open, busy, targetName, error, value, options, noScripts, onChange, onCancel, onSave,
}: ScriptAssociationModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      cancelText={t("common.cancel")}
      cancelButtonProps={{ disabled: busy }}
      closable={!busy}
      keyboard={!busy}
      mask={{ closable: !busy }}
      confirmLoading={busy}
      okText={t(busy ? "subscription.scriptAssociation.saving" : "subscription.scriptAssociation.save")}
      onCancel={() => { if (!busy) onCancel(); }}
      onOk={() => { if (!busy) onSave(); }}
      title={
        <span className="subscription-modal-title">
          {t("subscription.scriptAssociation.title")}
          <FeatureHelp compact topic="subscriptionAssociation" />
        </span>
      }
    >
      <div className="subscription-association-form">
        {error ? <Alert description={error} showIcon type="error" /> : null}
        <strong className="subscription-association-target">{targetName}</strong>
        <Select
          aria-label={t("subscription.scriptAssociation.title")}
          aria-busy={busy}
          disabled={busy}
          onChange={onChange}
          options={options}
          value={value}
        />
        {busy ? (
          <div className="subscription-association-progress" role="status" aria-live="polite">
            <Spin />
            <span>
              <strong>{t("subscription.scriptAssociation.saving")}</strong>
              <span>{t("subscription.scriptAssociation.waiting")}</span>
            </span>
          </div>
        ) : null}
        {noScripts ? <Alert description={t("subscription.scriptAssociation.empty")} showIcon type="info" /> : null}
      </div>
    </Modal>
  );
}
