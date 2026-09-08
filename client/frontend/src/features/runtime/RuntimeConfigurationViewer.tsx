import { FileTextOutlined } from "@ant-design/icons";
import { Alert, Button, Modal, Spin, Tag, Tooltip } from "antd";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { getRuntimeConfigurationText } from "../../services/appBridge";
import type { RuntimeConfigurationText } from "../../types/runtime";

interface RuntimeConfigurationViewerProps {
  hideTrigger?: boolean;
  openSignal?: number;
}

export function RuntimeConfigurationViewer({
  hideTrigger = false,
  openSignal = 0,
}: RuntimeConfigurationViewerProps = {}) {
  const { t } = useTranslation();
  const { runtime } = useRuntimeStatus();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  const [configuration, setConfiguration] =
    useState<RuntimeConfigurationText | null>(null);
  const disabled = !runtime?.configuration.subscriptionId;

  const showConfiguration = async () => {
    setOpen(true);
    setBusy(true);
    setError(false);
    try {
      setConfiguration(await getRuntimeConfigurationText());
    } catch {
      setConfiguration(null);
      setError(true);
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    if (openSignal > 0) void showConfiguration();
    // openSignal is an imperative request token; the loader intentionally
    // runs only when a new token is supplied.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openSignal]);

  return (
    <>
      {hideTrigger ? null : <Tooltip title={t("home.runtimeConfiguration.open")}>
        <span className="runtime-configuration-trigger-wrap">
          <Button
            aria-label={t("home.runtimeConfiguration.open")}
            className="runtime-configuration-trigger"
            disabled={disabled}
            icon={<FileTextOutlined />}
            onClick={() => void showConfiguration()}
            size="small"
            type="text"
          />
        </span>
      </Tooltip>}
      <Modal
        footer={null}
        onCancel={() => setOpen(false)}
        open={open}
        title={t("home.runtimeConfiguration.title")}
        width={920}
      >
        <div className="runtime-configuration-view">
          {busy ? (
            <div className="runtime-configuration-loading">
              <Spin />
              <span>{t("home.runtimeConfiguration.loading")}</span>
            </div>
          ) : error ? (
            <Alert
              description={t("home.runtimeConfiguration.error")}
              showIcon
              type="error"
            />
          ) : configuration ? (
            <>
              <div className="runtime-configuration-meta">
                <Tag color={configuration.source === "active" ? "success" : "processing"}>
                  {t(`home.runtimeConfiguration.sources.${configuration.source}`)}
                </Tag>
                <span>
                  {t("home.runtimeConfiguration.core", {
                    version: configuration.coreVersion || "—",
                  })}
                </span>
                <code>{configuration.subscriptionRevision}</code>
              </div>
              <pre>{configuration.contents}</pre>
            </>
          ) : null}
        </div>
      </Modal>
    </>
  );
}
