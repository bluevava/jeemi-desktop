import { FileTextOutlined } from "@ant-design/icons";
import { Button, Tag, Tooltip } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { ConfigurationViewer } from "../../components/configuration/ConfigurationViewer";
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
  const request = useRef(0);

  useEffect(() => () => { request.current += 1; }, []);

  const showConfiguration = async () => {
    const currentRequest = ++request.current;
    setOpen(true);
    setBusy(true);
    setError(false);
    setConfiguration(null);
    try {
      const next = await getRuntimeConfigurationText();
      if (request.current === currentRequest) setConfiguration(next);
    } catch {
      if (request.current === currentRequest) setError(true);
    } finally {
      if (request.current === currentRequest) setBusy(false);
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
      <ConfigurationViewer
        contents={configuration?.contents ?? ""}
        error={error ? t("home.runtimeConfiguration.error") : undefined}
        loading={busy}
        loadingLabel={t("home.runtimeConfiguration.loading")}
        onClose={() => { request.current += 1; setOpen(false); }}
        open={open}
        title={t("home.runtimeConfiguration.title")}
        metadata={configuration ? <>
          <Tag color={configuration.source === "active" ? "success" : "processing"}>
            {t(`home.runtimeConfiguration.sources.${configuration.source}`)}
          </Tag>
          <span>
            {t("home.runtimeConfiguration.core", {
              version: configuration.coreVersion || "—",
            })}
          </span>
          <code>{configuration.subscriptionRevision}</code>
        </> : null}
      />
    </>
  );
}
