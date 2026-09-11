import { useEffect, useState } from "react";
import { Alert, AutoComplete, Button, Select, Spin } from "antd";
import { CloudDownloadOutlined, ReloadOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { FeatureCard } from "../../components/layout/FeatureCard";
import { checkZashboardUpdates, downloadZashboardVersion } from "../../services/externalUIBridge";
import type { ZashboardState } from "../../types/externalUI";
import { useZashboardState } from "./useZashboardState";
import { ZashboardProgress } from "./ZashboardProgress";
import "./external-ui.css";

export interface ZashboardSettingsProps {
  draftVersion: string;
  onDraftVersionChange: (version: string) => void;
  onStateLoaded: (state: ZashboardState) => void;
  onInitialLoadComplete: () => void;
  onBusyChange: (busy: boolean) => void;
}

export function ZashboardSettings({ draftVersion, onDraftVersionChange, onStateLoaded, onInitialLoadComplete, onBusyChange }: ZashboardSettingsProps) {
  const { t } = useTranslation();
  const { state, setState, loaded, loadError, refresh } = useZashboardState(true);
  const [version, setVersion] = useState("");
  const [operation, setOperation] = useState(false);
  const [error, setError] = useState("");
  const [checked, setChecked] = useState(false);
  const busy = operation || Boolean(state?.phase);
  useEffect(() => { if (loaded) onInitialLoadComplete(); }, [loaded, onInitialLoadComplete]);
  useEffect(() => { if (state) onStateLoaded(state); }, [state, onStateLoaded]);
  useEffect(() => { onBusyChange(busy || !loaded); return () => onBusyChange(false); }, [busy, loaded, onBusyChange]);

  const check = async () => {
    setOperation(true); setError("");
    try { setState(await checkZashboardUpdates()); setChecked(true); }
    catch { setError("externalUI.checkFailed"); }
    finally { setOperation(false); void refresh().catch(() => undefined); }
  };
  const download = async () => {
    const tag = version.trim();
    if (tag && !/^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(tag)) { setError("externalUI.invalidVersion"); return; }
    setOperation(true); setError("");
    try {
      const next = await downloadZashboardVersion(tag);
      setState(next);
      onDraftVersionChange(tag || next.installed[0]?.version || "");
    } catch { setError("externalUI.downloadFailed"); }
    finally { setOperation(false); void refresh().catch(() => undefined); }
  };

  return <FeatureCard title={t("externalUI.title")} helpTopic="externalUI">
    <div className="zashboard-settings">
      {(error || loadError) && <Alert type="error" showIcon description={t(error || "externalUI.loadFailed")} />}
      {!loaded ? <Spin /> : <>
        <div className="zashboard-settings-row">
          <strong>{t("externalUI.selected")}</strong>
          <Select aria-label={t("externalUI.selected")} value={draftVersion || undefined} disabled={busy}
            placeholder={t("externalUI.notInstalled")} onChange={onDraftVersionChange}
            options={(state?.installed ?? []).map(item => ({ label: item.version, value: item.version }))} />
          <Button icon={<ReloadOutlined />} loading={state?.phase === "checking"} disabled={busy} onClick={() => void check()}>{t("externalUI.check")}</Button>
        </div>
        <div className="zashboard-settings-row">
          <strong>{t("externalUI.downloadVersion")}</strong>
          <AutoComplete aria-label={t("externalUI.downloadVersion")} value={version} disabled={busy}
            onChange={setVersion} placeholder={t("externalUI.versionPlaceholder")}
            options={(state?.available ?? []).map(item => ({ label: item.version, value: item.version }))} />
          <Button icon={<CloudDownloadOutlined />} disabled={busy} onClick={() => void download()}>{t("externalUI.download")}</Button>
        </div>
        <ZashboardProgress state={state} />
        {checked && state?.latestVersion && <Alert showIcon type={state.latestVersion === state.selectedVersion ? "success" : "info"}
          description={t(state.latestVersion === state.selectedVersion ? "externalUI.latest" : "externalUI.available", { version: state.latestVersion })} />}
        <span className="settings-description">{t("externalUI.selectionHint")}</span>
      </>}
    </div>
  </FeatureCard>;
}
