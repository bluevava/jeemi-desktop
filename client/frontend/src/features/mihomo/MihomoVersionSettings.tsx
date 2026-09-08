import { useEffect, useMemo, useRef, useState } from "react";
import {
  CloudDownloadOutlined,
  CloseOutlined,
  DeleteOutlined,
  FolderOpenOutlined,
  ImportOutlined,
  LinkOutlined,
  ReloadOutlined,
  StopOutlined,
} from "@ant-design/icons";
import {
  Alert,
  Button,
  Empty,
  Modal,
  Popconfirm,
  Select,
  Spin,
  Tag,
  Tooltip,
} from "antd";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../../components/help/FeatureHelp";
import { FeatureCard } from "../../components/layout/FeatureCard";
import {
  cancelMihomoDownload,
  checkMihomoUpdates,
  downloadMihomoVersion,
  getMihomoVersionState,
  importMihomoCore,
  openMihomoReleasesPage,
  openMihomoDirectory,
  removeMihomoVersion,
} from "../../services/appBridge";
import type { MihomoVersionManagerState } from "../../types/mihomo";

interface MihomoVersionSettingsProps {
  autoCheck?: boolean;
  draftVersion: string;
  onBusyChange: (busy: boolean) => void;
  onDraftVersionChange: (version: string) => void;
  onInitialLoadComplete?: () => void;
  onStateLoaded: (state: MihomoVersionManagerState) => void;
}

type Operation =
  | "check"
  | "import"
  | "open-releases"
  | "open"
  | `download:${string}`
  | `remove:${string}`
  | null;

export function formatBytes(bytes: number, locale: string): string {
  if (!Number.isFinite(bytes) || bytes < 0) {
    return "—";
  }
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  const units = ["KB", "MB", "GB"];
  let value = bytes / 1024;
  let unit = units[0];
  for (let index = 1; index < units.length && value >= 1024; index += 1) {
    value /= 1024;
    unit = units[index];
  }
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(value)} ${unit}`;
}

export function MihomoVersionSettings({
  autoCheck = false,
  draftVersion,
  onBusyChange,
  onDraftVersionChange,
  onInitialLoadComplete,
  onStateLoaded,
}: MihomoVersionSettingsProps) {
  const { i18n, t } = useTranslation();
  const [state, setState] = useState<MihomoVersionManagerState | null>(null);
  const [loading, setLoading] = useState(true);
  const [operation, setOperation] = useState<Operation>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [showAvailableVersions, setShowAvailableVersions] = useState(false);
  const [manualInstallOpen, setManualInstallOpen] = useState(false);
  const [importedVersion, setImportedVersion] = useState("");
  const cancellingVersion = useRef("");
  const autoCheckHandled = useRef(false);

  useEffect(() => {
    let active = true;
    void getMihomoVersionState()
      .then((nextState) => {
        if (!active) return;
        setState(nextState);
        setShowAvailableVersions(Boolean(nextState.lastCheckedAt));
        onStateLoaded(nextState);
      })
      .catch(() => {
        if (active) setErrorKey("settings.home.core.errors.load");
      })
      .finally(() => {
        if (active) {
          setLoading(false);
          onInitialLoadComplete?.();
        }
      });
    return () => {
      active = false;
    };
  }, [onInitialLoadComplete, onStateLoaded]);

  useEffect(() => {
    onBusyChange(operation !== null);
    return () => onBusyChange(false);
  }, [onBusyChange, operation]);

  const installedOptions = useMemo(
    () =>
      (state?.installedVersions ?? []).map((version) => ({
        label: `${version.version} · ${version.target}`,
        value: version.version,
      })),
    [state],
  );

  const updateState = (nextState: MihomoVersionManagerState) => {
    setState(nextState);
    onStateLoaded(nextState);
  };

  const checkUpdates = async () => {
    setErrorKey(null);
    setOperation("check");
    try {
      updateState(await checkMihomoUpdates());
      setShowAvailableVersions(true);
    } catch {
      setErrorKey("settings.home.core.errors.check");
    } finally {
      setOperation(null);
    }
  };

  useEffect(() => {
    if (!autoCheck || loading || autoCheckHandled.current) return;
    autoCheckHandled.current = true;
    void checkUpdates();
  }, [autoCheck, loading]);

  const openReleases = async () => {
    setErrorKey(null);
    setOperation("open-releases");
    try {
      await openMihomoReleasesPage();
    } catch {
      setErrorKey("settings.home.core.errors.openReleases");
    } finally {
      setOperation(null);
    }
  };

  const importCore = async () => {
    setErrorKey(null);
    setImportedVersion("");
    setOperation("import");
    try {
      const result = await importMihomoCore({
        title: t("settings.home.core.manual.dialogTitle"),
        filterName: t("settings.home.core.manual.fileFilter"),
      });
      if (result.cancelled) return;
      updateState(result.state);
      onDraftVersionChange(result.importedVersion);
      setImportedVersion(result.importedVersion);
    } catch {
      setErrorKey("settings.home.core.errors.import");
    } finally {
      setOperation(null);
    }
  };

  const download = async (version: string) => {
    setErrorKey(null);
    cancellingVersion.current = "";
    setOperation(`download:${version}`);
    try {
      const nextState = await downloadMihomoVersion(version);
      updateState(nextState);
      onDraftVersionChange(version);
    } catch {
      if (cancellingVersion.current !== version) {
        setErrorKey("settings.home.core.errors.download");
      }
    } finally {
      cancellingVersion.current = "";
      setOperation(null);
    }
  };

  const cancelDownload = async (version: string) => {
    cancellingVersion.current = version;
    try {
      await cancelMihomoDownload();
    } catch {
      setErrorKey("settings.home.core.errors.cancel");
    }
  };

  const remove = async (version: string) => {
    setErrorKey(null);
    setOperation(`remove:${version}`);
    try {
      const nextState = await removeMihomoVersion(version);
      updateState(nextState);
      if (draftVersion === version) {
        onDraftVersionChange(nextState.selectedVersion);
      }
    } catch {
      setErrorKey("settings.home.core.errors.remove");
    } finally {
      setOperation(null);
    }
  };

  const openDirectory = async () => {
    setErrorKey(null);
    setOperation("open");
    try {
      await openMihomoDirectory();
    } catch {
      setErrorKey("settings.home.core.errors.open");
    } finally {
      setOperation(null);
    }
  };

  const checkedAt = state?.lastCheckedAt
    ? new Intl.DateTimeFormat(i18n.language, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(state.lastCheckedAt))
    : t("settings.home.core.notChecked");

  return (
    <>
      <FeatureCard
        className="mihomo-version-card"
        headerAction={
          <Button
            disabled={operation !== null}
            icon={<ImportOutlined />}
            onClick={() => {
              setImportedVersion("");
              setManualInstallOpen(true);
            }}
            type="primary"
          >
            {t("settings.home.core.manual.open")}
          </Button>
        }
        helpTopic="coreVersion"
        title={t("settings.home.core.managerTitle")}
      >
        {errorKey ? (
          <Alert
            closable
            description={t(errorKey)}
            onClose={() => setErrorKey(null)}
            showIcon
            type="error"
          />
        ) : null}

      {loading ? (
        <div className="core-manager-loading">
          <Spin />
          <span>{t("settings.home.core.loading")}</span>
        </div>
      ) : (
        <>
          <div className="core-manager-toolbar">
            <div className="core-manager-facts">
              <div>
                <span>{t("settings.home.core.target")}</span>
                <strong>{state?.target ?? "—"}</strong>
              </div>
              <div>
                <span>{t("settings.home.core.lastChecked")}</span>
                <strong>{checkedAt}</strong>
              </div>
              <div>
                <span>{t("settings.home.core.latest")}</span>
                <strong>{state?.latestVersion || "—"}</strong>
              </div>
            </div>
            <div className="core-manager-actions">
              <Button
                icon={<ReloadOutlined />}
                loading={operation === "check"}
                onClick={() => void checkUpdates()}
              >
                {t("settings.home.core.check")}
              </Button>
              <Button
                icon={<FolderOpenOutlined />}
                loading={operation === "open"}
                onClick={() => void openDirectory()}
              >
                {t("settings.home.core.openDirectory")}
              </Button>
            </div>
          </div>

          <div className="core-selection-row">
            <div className="core-section-heading compact-heading">
              <div>
                <h3>{t("settings.home.core.selectedVersion")}</h3>
              </div>
              <FeatureHelp compact topic="coreVersion" />
            </div>
            <Select
              aria-label={t("settings.home.core.selectedVersion")}
              disabled={installedOptions.length === 0 || operation !== null}
              notFoundContent={t("settings.home.core.noInstalled")}
              onChange={onDraftVersionChange}
              options={installedOptions}
              placeholder={t("settings.home.core.selectPlaceholder")}
              value={draftVersion || undefined}
            />
          </div>

          {showAvailableVersions ? (
            <section className="core-version-section core-available-section">
              <div className="core-section-heading">
                <div>
                  <h3>{t("settings.home.core.availableTitle")}</h3>
                </div>
                <Tooltip title={t("settings.home.core.hideAvailable")}>
                  <Button
                    aria-label={t("settings.home.core.hideAvailable")}
                    icon={<CloseOutlined />}
                    onClick={() => setShowAvailableVersions(false)}
                    size="small"
                    type="text"
                  />
                </Tooltip>
              </div>
              {state?.availableVersions.length ? (
                <div className="core-version-list">
                  {state.availableVersions.map((version, index) => {
                    const isDownloading =
                      operation === `download:${version.version}`;
                    return (
                      <div className="core-version-item" key={version.version}>
                        <div className="core-version-copy">
                          <div className="core-version-title">
                            <strong>{version.version}</strong>
                            {index === 0 ? (
                              <Tag color="processing">
                                {t("settings.home.core.latestTag")}
                              </Tag>
                            ) : null}
                            {version.installed ? (
                              <Tag color="success">
                                {t("settings.home.core.installedTag")}
                              </Tag>
                            ) : null}
                          </div>
                          <span>
                            {version.assetName} ·{" "}
                            {formatBytes(
                              version.archiveSize,
                              i18n.language,
                            )}
                          </span>
                          <Tooltip title={version.sha256}>
                            <code>
                              SHA-256 {version.sha256.slice(0, 12)}…
                            </code>
                          </Tooltip>
                        </div>
                        {isDownloading ? (
                          <Button
                            danger
                            icon={<StopOutlined />}
                            onClick={() =>
                              void cancelDownload(version.version)
                            }
                          >
                            {cancellingVersion.current === version.version
                              ? t("settings.home.core.cancelling")
                              : t("settings.home.core.cancelDownload")}
                          </Button>
                        ) : (
                          <Button
                            disabled={version.installed || operation !== null}
                            icon={<CloudDownloadOutlined />}
                            onClick={() => void download(version.version)}
                            type={version.installed ? "default" : "primary"}
                          >
                            {version.installed
                              ? t("settings.home.core.installed")
                              : t("settings.home.core.download")}
                          </Button>
                        )}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <Empty
                  description={t("settings.home.core.availableEmpty")}
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              )}
            </section>
          ) : null}

          <section className="core-version-section">
            <div className="core-section-heading">
              <div>
                <h3>{t("settings.home.core.filesTitle")}</h3>
              </div>
              <FeatureHelp compact topic="coreFiles" />
            </div>
            {state?.installedVersions.length ? (
              <div className="core-version-list">
                {state.installedVersions.map((version) => (
                  <div className="core-version-item" key={version.version}>
                    <div className="core-version-copy">
                      <div className="core-version-title">
                        <strong>{version.version}</strong>
                        {version.selected ? (
                          <Tag color="processing">{t("settings.home.core.inUseTag")}</Tag>
                        ) : null}
                        {version.source === "manual" ? (
                          <Tag color="warning">{t("settings.home.core.manualTag")}</Tag>
                        ) : null}
                      </div>
                      <span>
                        {version.target} · {formatBytes(version.binarySize, i18n.language)}
                      </span>
                      <Tooltip title={version.executablePath}>
                        <code>{version.executablePath}</code>
                      </Tooltip>
                    </div>
                    <Popconfirm
                      cancelText={t("settings.cancel")}
                      description={t("settings.home.core.removeConfirmDescription", {
                        version: version.version,
                      })}
                      disabled={version.selected}
                      okButtonProps={{ danger: true }}
                      okText={t("settings.home.core.remove")}
                      onConfirm={() => void remove(version.version)}
                      title={t("settings.home.core.removeConfirmTitle")}
                    >
                      <Button
                        danger
                        disabled={version.selected || operation !== null}
                        icon={<DeleteOutlined />}
                        loading={operation === `remove:${version.version}`}
                      >
                        {t("settings.home.core.remove")}
                      </Button>
                    </Popconfirm>
                  </div>
                ))}
              </div>
            ) : (
              <Empty
                description={t("settings.home.core.noInstalled")}
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            )}
          </section>

          <div className="core-data-path">
            <span>{t("settings.home.core.dataDirectory")}</span>
            <code>{state?.dataDirectory || "—"}</code>
            <span>{t("settings.home.core.coreDirectory")}</span>
            <code>{state?.coreDirectory || "—"}</code>
          </div>
        </>
      )}
      </FeatureCard>

      <Modal
        centered
        footer={[
          <Button key="close" onClick={() => setManualInstallOpen(false)}>
            {t("common.close")}
          </Button>,
          <Button
            icon={<ImportOutlined />}
            key="import"
            loading={operation === "import"}
            onClick={() => void importCore()}
            type="primary"
          >
            {t("settings.home.core.manual.import")}
          </Button>,
        ]}
        mask={{ closable: false }}
        onCancel={() => setManualInstallOpen(false)}
        open={manualInstallOpen}
        title={t("settings.home.core.manual.title")}
        width={680}
      >
        <Alert
          description={t("settings.home.core.manual.warning")}
          showIcon
          type="warning"
        />
        {errorKey ? (
          <Alert
            className="manual-install-feedback"
            closable
            description={t(errorKey)}
            onClose={() => setErrorKey(null)}
            showIcon
            type="error"
          />
        ) : null}
        <ol className="manual-install-steps">
          <li>
            <span>{t("settings.home.core.manual.stepRepository")}</span>
            <Button
              icon={<LinkOutlined />}
              loading={operation === "open-releases"}
              onClick={() => void openReleases()}
              size="small"
              type="link"
            >
              {t("settings.home.core.manual.openRepository")}
            </Button>
          </li>
          <li>
            {t("settings.home.core.manual.stepDownload", {
              target: state?.target ?? "—",
            })}
          </li>
          <li>{t("settings.home.core.manual.stepImport")}</li>
        </ol>
        {importedVersion ? (
          <Alert
            description={t("settings.home.core.manual.imported", {
              version: importedVersion,
            })}
            showIcon
            type="success"
          />
        ) : null}
      </Modal>
    </>
  );
}
