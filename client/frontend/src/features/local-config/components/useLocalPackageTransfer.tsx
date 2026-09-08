import { useRef, useState } from "react";
import { Alert, App, Modal } from "antd";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../../components/help/FeatureHelp";
import {
  exportLocalPackage,
  importLocalPackage,
  resolveLocalPackage,
} from "../../../services/appBridge";
import type {
  LocalPackageKind,
  LocalPackagePreview,
} from "../../../types/localPackage";

export function useLocalPackageTransfer(
  onBusyChange: (id: string | null) => void,
  onImported: () => Promise<void>,
) {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const active = useRef(false);
  const [preview, setPreview] = useState<LocalPackagePreview | null>(null);
  const [saving, setSaving] = useState(false);

  const showError = (error: unknown) =>
    modal.error({
      title: t("localPackage.failed"),
      content: (
        <div className="local-package-error">
          <p>{t("localPackage.unchanged")}</p>
          <pre>{error instanceof Error ? error.message : String(error)}</pre>
        </div>
      ),
      okText: t("common.close"),
    });

  const finish = () => {
    active.current = false;
    onBusyChange(null);
  };

  const exportPackage = async (kind: LocalPackageKind, id: string) => {
    if (active.current) return;
    active.current = true;
    onBusyChange(id);
    try {
      const result = await exportLocalPackage(kind, id, {
        title: t("localPackage.exportTitle"),
        filterName: t("localPackage.filter"),
      });
      if (!result.cancelled) void message.success(t("localPackage.exported"));
    } catch (error) {
      showError(error);
    } finally {
      finish();
    }
  };

  const importPackage = async (kind: LocalPackageKind, id: string) => {
    if (active.current) return;
    active.current = true;
    onBusyChange(id);
    try {
      const result = await importLocalPackage(kind, id, {
        title: t("localPackage.importTitle"),
        filterName: t("localPackage.filter"),
      });
      if (result.cancelled) finish();
      else setPreview(result);
    } catch (error) {
      showError(error);
      finish();
    }
  };

  const resolve = async (confirm: boolean) => {
    if (!preview || saving) return;
    setSaving(true);
    try {
      await resolveLocalPackage(preview.token, confirm);
      if (confirm) {
        void message.success(t("localPackage.imported"));
        await onImported();
      }
    } catch (error) {
      if (confirm) showError(error);
    } finally {
      setSaving(false);
      setPreview(null);
      finish();
    }
  };

  const names = (key: string, values: string[]) =>
    values.length > 0 ? (
      <section key={key}>
        <strong>{t(key, { count: values.length })}</strong>
        <ul>
          {values.map((name, index) => (
            <li key={`${index}:${name}`}>{name}</li>
          ))}
        </ul>
      </section>
    ) : null;

  return {
    exportPackage,
    importPackage,
    dialog: (
      <Modal
        open={preview !== null}
        title={
          <span className="local-package-title">
            {t("localPackage.confirmTitle")}
            <FeatureHelp compact topic="localPackage" />
          </span>
        }
        onOk={() => void resolve(true)}
        onCancel={() => void resolve(false)}
        okText={t("localPackage.overwrite")}
        cancelText={t("common.cancel")}
        okButtonProps={{ danger: true }}
        cancelButtonProps={{ disabled: saving }}
        confirmLoading={saving}
        closable={!saving}
        keyboard={!saving}
        mask={{ closable: false }}
        width={600}
      >
        {preview && (
          <div className="local-package-preview">
            <Alert
              type="warning"
              showIcon
              title={t("localPackage.replace", {
                target: preview.targetName,
                name: preview.importedName,
              })}
            />
            {preview.kind === "local-config" && (
              <>
                {names(
                  "localPackage.overwrittenGroups",
                  preview.overwrittenGroups,
                )}
                {names(
                  "localPackage.overwrittenRuleSets",
                  preview.overwrittenRuleSets,
                )}
                {preview.overwrittenGroups.length +
                  preview.overwrittenRuleSets.length ===
                  0 && <p>{t("localPackage.noConflicts")}</p>}
                {names("localPackage.addedGroups", preview.addedGroups)}
                {names("localPackage.addedRuleSets", preview.addedRuleSets)}
                {preview.affectedConfigs.length > 1 &&
                  names(
                    "localPackage.affectedConfigs",
                    preview.affectedConfigs,
                  )}
              </>
            )}
            {names(
              "localPackage.affectedSubscriptions",
              preview.affectedSubscriptions,
            )}
          </div>
        )}
      </Modal>
    ),
  };
}
