import { useEffect, useMemo, useRef, useState } from "react";
import {
  CloudDownloadOutlined,
  DeleteOutlined,
  FolderOpenOutlined,
  ImportOutlined,
  LinkOutlined,
  ReloadOutlined,
  StopOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import {
  Alert,
  Button,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Segmented,
  Spin,
  Tag,
  Tooltip,
} from "antd";
import { useTranslation } from "react-i18next";

import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { FeatureCard } from "../../components/layout/FeatureCard";
import { FeatureHelp } from "../../components/help/FeatureHelp";
import { formatBytes } from "../mihomo/MihomoVersionSettings";
import {
  cancelGeoDataDownload,
  checkGeoDataUpdates,
  cleanOldGeoDataRevisions,
  downloadAllGeoData,
  downloadGeoData,
  getGeoDataState,
  importGeoData,
  openGeoDataDirectory,
  openGeoDataReleasesPage,
} from "../../services/appBridge";
import type {
  GeoDataKind,
  GeoDataPreferences,
  GeoDataState,
  GeoDataURLs,
} from "../../types/geodata";
import {
  geoDataPreferencesEqual,
  geoDataURLFields,
  isSafeGeoDataURL,
  visibleGeoDataKinds,
} from "./model";

interface GeoDataSettingsProps {
  autoCheck?: boolean;
  draftPreferences: GeoDataPreferences;
  onBusyChange: (busy: boolean) => void;
  onDraftPreferencesChange: (preferences: GeoDataPreferences) => void;
  onInitialLoadComplete?: () => void;
  onStateLoaded: (state: GeoDataState) => void;
  selectedCoreVersion: string;
}

type Operation =
  | "check"
  | "download-all"
  | "apply"
  | "clean"
  | "open"
  | "open-releases"
  | `download:${GeoDataKind}`
  | `import:${GeoDataKind}`
  | null;

export function GeoDataSettings({
  autoCheck = false,
  draftPreferences,
  onBusyChange,
  onDraftPreferencesChange,
  onInitialLoadComplete,
  onStateLoaded,
  selectedCoreVersion,
}: GeoDataSettingsProps) {
  const { i18n, t } = useTranslation();
  const { refresh, runtime, restart } = useRuntimeStatus();
  const [state, setState] = useState<GeoDataState | null>(null);
  const [loading, setLoading] = useState(true);
  const [operation, setOperation] = useState<Operation>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [manualInstallOpen, setManualInstallOpen] = useState(false);
  const cancelling = useRef(false);
  const autoCheckHandled = useRef(false);

  useEffect(() => {
    let active = true;
    void getGeoDataState()
      .then((nextState) => {
        if (!active) return;
        setState(nextState);
        onStateLoaded(nextState);
      })
      .catch(() => {
        if (active) setErrorKey("settings.home.geoData.errors.load");
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

  const visibleKinds = useMemo(
    () => visibleGeoDataKinds(draftPreferences),
    [draftPreferences],
  );
  const draftDirty = Boolean(
    state && !geoDataPreferencesEqual(state.preferences, draftPreferences),
  );

  const updateState = (nextState: GeoDataState) => {
    setState(nextState);
    onStateLoaded(nextState);
  };

  const runStateOperation = async (
    nextOperation: Exclude<Operation, null>,
    error: string,
    action: () => Promise<GeoDataState>,
  ) => {
    setErrorKey(null);
    cancelling.current = false;
    setOperation(nextOperation);
    try {
      updateState(await action());
    } catch {
      if (!cancelling.current) setErrorKey(error);
    } finally {
      cancelling.current = false;
      setOperation(null);
    }
  };

  useEffect(() => {
    if (!autoCheck || loading || autoCheckHandled.current) return;
    autoCheckHandled.current = true;
    void runStateOperation(
      "check",
      "settings.home.geoData.errors.check",
      checkGeoDataUpdates,
    );
  }, [autoCheck, loading]);

  const handleImport = async (kind: GeoDataKind) => {
    setErrorKey(null);
    setOperation(`import:${kind}`);
    try {
      const result = await importGeoData(kind, {
        title: t("settings.home.geoData.importDialogTitle", {
          asset: t(`settings.home.geoData.assets.${kind}`),
        }),
        filterName: t("settings.home.geoData.importDialogFilter"),
      });
      if (!result.cancelled) updateState(result.state);
    } catch {
      setErrorKey("settings.home.geoData.errors.import");
    } finally {
      setOperation(null);
    }
  };

  const handleCancel = async () => {
    cancelling.current = true;
    try {
      await cancelGeoDataDownload();
    } catch {
      cancelling.current = false;
      setErrorKey("settings.home.geoData.errors.cancel");
    }
  };

  const handleApply = async () => {
    setErrorKey(null);
    setOperation("apply");
    try {
      await restart();
      await refresh();
      updateState(await getGeoDataState());
    } catch {
      setErrorKey("settings.home.geoData.errors.apply");
    } finally {
      setOperation(null);
    }
  };

  const openReleases = async () => {
    setErrorKey(null);
    setOperation("open-releases");
    try {
      await openGeoDataReleasesPage();
    } catch {
      setErrorKey("settings.home.geoData.errors.openReleases");
    } finally {
      setOperation(null);
    }
  };

  const updatePreference = <K extends keyof GeoDataPreferences>(
    key: K,
    value: GeoDataPreferences[K],
  ) => onDraftPreferencesChange({ ...draftPreferences, [key]: value });

  const updateURL = (field: keyof GeoDataURLs, value: string) =>
    onDraftPreferencesChange({
      ...draftPreferences,
      customUrls: { ...draftPreferences.customUrls, [field]: value },
    });

  const checkedAt = state?.lastCheckedAt
    ? new Intl.DateTimeFormat(i18n.language, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(state.lastCheckedAt))
    : t("settings.home.geoData.notChecked");
  const running = runtime?.mihomo.state === "running";
  const coreAvailable = Boolean(selectedCoreVersion);
  const downloading = Boolean(state?.downloadingKind) ||
    operation === "download-all" ||
    operation?.startsWith("download:") === true;

  return (
    <>
      <FeatureCard
        className="geodata-manager-card"
        headerAction={
          <Button
            disabled={operation !== null}
            icon={<ImportOutlined />}
            onClick={() => setManualInstallOpen(true)}
            type="primary"
          >
            {t("settings.home.geoData.manual.open")}
          </Button>
        }
        helpTopic="geoData"
        title={t("settings.home.geoData.managerTitle")}
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
          <span>{t("settings.home.geoData.loading")}</span>
        </div>
      ) : (
        <>
          <div className="geodata-preferences">
            <div className="geodata-preference">
              <span className="feature-title-with-help">
                {t("settings.home.geoData.geoIpMode")}
                <FeatureHelp compact topic="geoDataFormat" />
              </span>
              <Segmented
                aria-label={t("settings.home.geoData.geoIpMode")}
                disabled={operation !== null}
                onChange={(value) => updatePreference("geoIpMode", value as "mmdb" | "dat")}
                options={[
                  { label: "MetaDB / MMDB", value: "mmdb" },
                  { label: "DAT", value: "dat" },
                ]}
                value={draftPreferences.geoIpMode}
              />
            </div>
            <div className="geodata-preference">
              <span className="feature-title-with-help">
                {t("settings.home.geoData.loader")}
                <FeatureHelp compact topic="geoDataLoader" />
              </span>
              <Segmented
                aria-label={t("settings.home.geoData.loader")}
                disabled={operation !== null}
                onChange={(value) => updatePreference("loader", value as "memconservative" | "standard")}
                options={[
                  {
                    label: t("settings.home.geoData.loaders.memconservative"),
                    value: "memconservative",
                  },
                  {
                    label: t("settings.home.geoData.loaders.standard"),
                    value: "standard",
                  },
                ]}
                value={draftPreferences.loader}
              />
            </div>
            <div className="geodata-preference">
              <span className="feature-title-with-help">
                {t("settings.home.geoData.source")}
                <FeatureHelp compact topic="geoDataSource" />
              </span>
              <Segmented
                aria-label={t("settings.home.geoData.source")}
                disabled={operation !== null}
                onChange={(value) => updatePreference("source", value as "official" | "custom")}
                options={[
                  {
                    label: t("settings.home.geoData.sources.official"),
                    value: "official",
                  },
                  {
                    label: t("settings.home.geoData.sources.custom"),
                    value: "custom",
                  },
                ]}
                value={draftPreferences.source}
              />
            </div>
          </div>

          {draftPreferences.source === "custom" ? (
            <div className="geodata-custom-urls">
              {geoDataURLFields.map(({ field, kind }) => (
                <label key={kind}>
                  <span>{t(`settings.home.geoData.assets.${kind}`)}</span>
                  <Input
                    disabled={operation !== null}
                    onChange={(event) => updateURL(field, event.target.value)}
                    status={!isSafeGeoDataURL(draftPreferences.customUrls[field]) ? "error" : undefined}
                    value={draftPreferences.customUrls[field]}
                  />
                </label>
              ))}
            </div>
          ) : null}

          {draftDirty ? (
            <Alert
              description={t("settings.home.geoData.confirmBeforeOperations")}
              showIcon
              type="info"
            />
          ) : null}

          {!coreAvailable ? (
            <Alert
              description={t("settings.home.geoData.coreRequired")}
              showIcon
              type="info"
            />
          ) : null}

          {state?.restartRequired ? (
            <Alert
              action={
                running ? (
                  <Button
                    loading={operation === "apply"}
                    onClick={() => void handleApply()}
                    size="small"
                    type="primary"
                  >
                    {t("settings.home.geoData.restartNow")}
                  </Button>
                ) : undefined
              }
              description={t(
                running
                  ? "settings.home.geoData.pendingRunning"
                  : "settings.home.geoData.pendingStopped",
              )}
              showIcon
              type="warning"
            />
          ) : null}

          <div className="geodata-manager-toolbar">
            <div className="geodata-manager-facts">
              <div>
                <span>{t("settings.home.geoData.lastChecked")}</span>
                <strong>{checkedAt}</strong>
              </div>
              <div>
                <span>{t("settings.home.geoData.activeMode")}</span>
                <strong>
                  {state?.activeGeoIpMode === "dat" ? "DAT" : "MetaDB / MMDB"}
                </strong>
              </div>
              <div>
                <span>{t("settings.home.geoData.activeLoader")}</span>
                <strong>
                  {t(
                    `settings.home.geoData.loaders.${state?.activeLoader ?? "memconservative"}`,
                  )}
                </strong>
              </div>
            </div>
            <div className="geodata-manager-actions">
              {downloading ? (
                <Button danger icon={<StopOutlined />} onClick={() => void handleCancel()}>
                  {cancelling.current
                    ? t("settings.home.geoData.cancelling")
                    : t("settings.home.geoData.cancel")}
                </Button>
              ) : (
                <>
                  <Button
                    disabled={draftDirty || operation !== null}
                    icon={<ReloadOutlined />}
                    loading={operation === "check"}
                    onClick={() =>
                      void runStateOperation(
                        "check",
                        "settings.home.geoData.errors.check",
                        checkGeoDataUpdates,
                      )
                    }
                  >
                    {t("settings.home.geoData.check")}
                  </Button>
                  <Button
                    disabled={draftDirty || !coreAvailable || operation !== null}
                    icon={<CloudDownloadOutlined />}
                    onClick={() =>
                      void runStateOperation(
                        "download-all",
                        "settings.home.geoData.errors.download",
                        downloadAllGeoData,
                      )
                    }
                    type="primary"
                  >
                    {t("settings.home.geoData.updateAll")}
                  </Button>
                </>
              )}
              <Button
                disabled={operation !== null}
                icon={<FolderOpenOutlined />}
                loading={operation === "open"}
                onClick={() => {
                  setOperation("open");
                  void openGeoDataDirectory()
                    .catch(() => setErrorKey("settings.home.geoData.errors.open"))
                    .finally(() => setOperation(null));
                }}
              >
                {t("settings.home.geoData.openDirectory")}
              </Button>
              <Popconfirm
                cancelText={t("settings.cancel")}
                description={t("settings.home.geoData.cleanConfirmDescription")}
                okText={t("settings.home.geoData.clean")}
                onConfirm={() =>
                  runStateOperation(
                    "clean",
                    "settings.home.geoData.errors.clean",
                    cleanOldGeoDataRevisions,
                  )
                }
                title={t("settings.home.geoData.cleanConfirmTitle")}
              >
                <Button disabled={operation !== null} icon={<DeleteOutlined />}>
                  {t("settings.home.geoData.clean")}
                </Button>
              </Popconfirm>
            </div>
          </div>

          <section className="geodata-assets-section">
            <div className="core-section-heading">
              <div>
                <h3>{t("settings.home.geoData.assetsTitle")}</h3>
              </div>
            </div>
            {visibleKinds.length ? (
              <div className="geodata-asset-list">
                {visibleKinds.map((kind) => {
                  const asset = state?.assets.find((item) => item.kind === kind);
                  const hasUpdate = Boolean(
                    asset?.availableSha256 &&
                      asset.availableSha256 !== asset.sha256,
                  );
                  const installedAt = asset?.installedAt
                    ? new Intl.DateTimeFormat(i18n.language, {
                        dateStyle: "medium",
                        timeStyle: "short",
                      }).format(new Date(asset.installedAt))
                    : "";
                  return (
                    <div className="geodata-asset" key={kind}>
                      <div className="geodata-asset-copy">
                        <div className="geodata-asset-title">
                          <strong>{t(`settings.home.geoData.assets.${kind}`)}</strong>
                          <code>{asset?.canonicalName ?? "—"}</code>
                          {asset?.active ? (
                            <Tag color="success">{t("settings.home.geoData.active")}</Tag>
                          ) : asset?.pending ? (
                            <Tag color="warning">{t("settings.home.geoData.pending")}</Tag>
                          ) : null}
                          {hasUpdate ? (
                            <Tag color="processing">{t("settings.home.geoData.updateAvailable")}</Tag>
                          ) : null}
                          {asset?.publisherVerified ? (
                            <Tag>{t("settings.home.geoData.publisherVerified")}</Tag>
                          ) : null}
                        </div>
                        {asset?.installed ? (
                          <>
                            <span>
                              {formatBytes(asset.size, i18n.language)} · {t(`settings.home.geoData.sources.${asset.source === "official" ? "official" : asset.source === "import" ? "import" : asset.source === "existing" ? "existing" : "custom"}`)}
                              {asset.validatedCoreVersion
                                ? ` · mihomo ${asset.validatedCoreVersion}`
                                : ""}
                              {installedAt ? ` · ${installedAt}` : ""}
                            </span>
                            <Tooltip title={asset.sha256}>
                              <code>SHA-256 {asset.sha256.slice(0, 12)}…</code>
                            </Tooltip>
                          </>
                        ) : (
                          <span>{t("settings.home.geoData.notInstalled")}</span>
                        )}
                      </div>
                      <div className="geodata-asset-actions">
                        <Button
                          disabled={draftDirty || !coreAvailable || operation !== null}
                          icon={<SyncOutlined />}
                          loading={operation === `download:${kind}`}
                          onClick={() =>
                            void runStateOperation(
                              `download:${kind}`,
                              "settings.home.geoData.errors.download",
                              () => downloadGeoData(kind),
                            )
                          }
                        >
                          {asset?.installed
                            ? t("settings.home.geoData.update")
                            : t("settings.home.geoData.download")}
                        </Button>
                        <Button
                          disabled={draftDirty || !coreAvailable || operation !== null}
                          icon={<ImportOutlined />}
                          loading={operation === `import:${kind}`}
                          onClick={() => void handleImport(kind)}
                        >
                          {t("settings.home.geoData.import")}
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </section>

          <div className="core-data-path geodata-data-path">
            <span>{t("settings.home.geoData.dataDirectory")}</span>
            <code>{state?.directory || "—"}</code>
            <span>{t("settings.home.geoData.runtimeDirectory")}</span>
            <code>{state?.runtimeDirectory || "—"}</code>
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
        ]}
        mask={{ closable: false }}
        onCancel={() => setManualInstallOpen(false)}
        open={manualInstallOpen}
        title={t("settings.home.geoData.manual.title")}
        width={720}
      >
        <Alert
          description={t("settings.home.geoData.manual.warning")}
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
            <span>{t("settings.home.geoData.manual.stepRepository")}</span>
            <Button
              icon={<LinkOutlined />}
              loading={operation === "open-releases"}
              onClick={() => void openReleases()}
              size="small"
              type="link"
            >
              {t("settings.home.geoData.manual.openRepository")}
            </Button>
          </li>
          <li>{t("settings.home.geoData.manual.stepDownload")}</li>
          <li>{t("settings.home.geoData.manual.stepImport")}</li>
        </ol>
        {!coreAvailable ? (
          <Alert
            description={t("settings.home.geoData.coreRequired")}
            showIcon
            type="info"
          />
        ) : null}
        <div className="manual-geodata-actions">
          {visibleKinds.map((kind) => (
            <Button
              disabled={draftDirty || !coreAvailable || operation !== null}
              icon={<ImportOutlined />}
              key={kind}
              loading={operation === `import:${kind}`}
              onClick={() => void handleImport(kind)}
            >
              {t("settings.home.geoData.manual.importAsset", {
                asset: t(`settings.home.geoData.assets.${kind}`),
              })}
            </Button>
          ))}
        </div>
      </Modal>
    </>
  );
}
