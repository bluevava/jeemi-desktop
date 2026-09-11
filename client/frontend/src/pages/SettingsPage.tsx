import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { CheckOutlined, CloseOutlined } from "@ant-design/icons";
import { Alert, Button, Empty, InputNumber, Segmented, Spin } from "antd";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";

import { navigationItems, type NavigationPage } from "../app/navigation";
import {
  homeSettingsSectionIds,
  resolveHomeSettingsSection,
} from "../app/homeSettingsNavigation";
import {
  usePreferences,
  type ThemeMode,
} from "../app/preferences/PreferencesContext";
import { useRuntimeStatus } from "../app/runtime/RuntimeStatusContext";
import { FeatureHelp, type HelpTopic } from "../components/help/FeatureHelp";
import { FeatureCard } from "../components/layout/FeatureCard";
import { MihomoVersionSettings } from "../features/mihomo/MihomoVersionSettings";
import { JeemiUpdateButton } from "../features/update/JeemiUpdateButton";
import { GeoDataSettings } from "../features/geodata/GeoDataSettings";
import { ZashboardSettings } from "../features/external-ui/ZashboardSettings";
import { selectZashboardVersion } from "../services/externalUIBridge";
import type { ZashboardState } from "../types/externalUI";
import { geoDataPreferencesEqual } from "../features/geodata/model";
import type { AppLanguage } from "../i18n/resources";
import {
  getManagedStorageDirectories,
  getSubscriptionPreferences,
  saveGeoDataPreferences,
  saveSubscriptionPreferences,
  selectMihomoVersion,
} from "../services/appBridge";
import type { MihomoVersionManagerState } from "../types/mihomo";
import {
  defaultGeoDataPreferences,
  type GeoDataPreferences,
  type GeoDataState,
} from "../types/geodata";
import type { ManagedStorageDirectories } from "../types/storage";
import type {
  ConnectionResetMode,
  SelectorDensity,
} from "../types/subscription";

interface SettingsRowProps {
  title: string;
  helpTopic?: HelpTopic;
  children: ReactNode;
}

function SettingsRow({ title, helpTopic, children }: SettingsRowProps) {
  return (
    <div className="settings-row">
      <div className="settings-row-copy">
        <div className="settings-label-line">
          <strong>{title}</strong>
          {helpTopic ? <FeatureHelp compact topic={helpTopic} /> : null}
        </div>
      </div>
      <div className="settings-row-control">{children}</div>
    </div>
  );
}

interface ManagedStorageCardProps {
  directory: string;
  error: boolean;
  loading: boolean;
  title: string;
}

function ManagedStorageCard({
  directory,
  error,
  loading,
  title,
}: ManagedStorageCardProps) {
  const { t } = useTranslation();

  return (
    <FeatureCard helpTopic="managedStorage" title={title}>
      <div className="managed-storage-setting">
        {error ? (
          <Alert
            description={t("settings.managedStorage.error")}
            showIcon
            type="error"
          />
        ) : loading ? (
          <span className="managed-storage-loading">
            <Spin size="small" />
            {t("settings.managedStorage.loading")}
          </span>
        ) : (
          <code>{directory || "—"}</code>
        )}
      </div>
    </FeatureCard>
  );
}

interface SettingsPageProps {
  page: NavigationPage;
}

export function SettingsPage({ page }: SettingsPageProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { theme, language, setTheme, setLanguage } = usePreferences();
  const { refresh } = useRuntimeStatus();
  const [draftTheme, setDraftTheme] = useState<ThemeMode>(theme);
  const [draftLanguage, setDraftLanguage] = useState<AppLanguage>(language);
  const [draftCoreVersion, setDraftCoreVersion] = useState("");
  const [persistedCoreVersion, setPersistedCoreVersion] = useState("");
  const [coreBusy, setCoreBusy] = useState(false);
  const [geoDataBusy, setGeoDataBusy] = useState(false);
  const [zashboardBusy, setZashboardBusy] = useState(false);
  const [draftZashboardVersion, setDraftZashboardVersion] = useState("");
  const [persistedZashboardVersion, setPersistedZashboardVersion] = useState("");
  const [zashboardSectionReady, setZashboardSectionReady] = useState(false);
  const zashboardDraftInitialised = useRef(false);
  const zashboardSectionRef = useRef<HTMLDivElement | null>(null);
  const [draftGeoDataPreferences, setDraftGeoDataPreferences] =
    useState<GeoDataPreferences>(defaultGeoDataPreferences);
  const [persistedGeoDataPreferences, setPersistedGeoDataPreferences] =
    useState<GeoDataPreferences>(defaultGeoDataPreferences);
  const [subscriptionBusy, setSubscriptionBusy] = useState(false);
  const [managedDirectories, setManagedDirectories] =
    useState<ManagedStorageDirectories | null>(null);
  const [managedDirectoriesBusy, setManagedDirectoriesBusy] = useState(false);
  const [managedDirectoriesError, setManagedDirectoriesError] = useState(false);
  const [draftSelectorDensity, setDraftSelectorDensity] =
    useState<SelectorDensity>("medium");
  const [draftDelayTestConcurrency, setDraftDelayTestConcurrency] =
    useState(8);
  const [draftConnectionResetMode, setDraftConnectionResetMode] =
    useState<ConnectionResetMode>("selector");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(false);
  const [mihomoSectionReady, setMihomoSectionReady] = useState(false);
  const [geoDataSectionReady, setGeoDataSectionReady] = useState(false);
  const coreDraftInitialised = useRef(false);
  const geoDataDraftInitialised = useRef(false);
  const mihomoSectionRef = useRef<HTMLDivElement | null>(null);
  const geoDataSectionRef = useRef<HTMLDivElement | null>(null);
  const navigationItem = navigationItems.find((item) => item.id === page)!;
  const pageLabel = t(navigationItem.labelKey);
  const settingsSearch = new URLSearchParams(location.search);
  const focusTarget =
    page === "home"
      ? resolveHomeSettingsSection(location.search, location.hash)
      : null;
  const autoCheck = settingsSearch.get("check") === "1";
  const homeSettingsLayoutReady = mihomoSectionReady && geoDataSectionReady && zashboardSectionReady;

  useEffect(() => {
    if (!focusTarget || !homeSettingsLayoutReady) return;
    const target =
      focusTarget === "mihomo"
        ? mihomoSectionRef.current
        : focusTarget === "zashboard" ? zashboardSectionRef.current : geoDataSectionRef.current;
    if (!target) return;

    let innerFrame = 0;
    const outerFrame = window.requestAnimationFrame(() => {
      innerFrame = window.requestAnimationFrame(() => {
        target.scrollIntoView({
          behavior: "smooth",
          block: "start",
          inline: "nearest",
        });
      });
    });
    return () => {
      window.cancelAnimationFrame(outerFrame);
      window.cancelAnimationFrame(innerFrame);
    };
  }, [focusTarget, homeSettingsLayoutReady, location.key]);

  const handleMihomoInitialLoadComplete = useCallback(() => {
    setMihomoSectionReady(true);
  }, []);

  const handleGeoDataInitialLoadComplete = useCallback(() => {
    setGeoDataSectionReady(true);
  }, []);

  const handleZashboardInitialLoadComplete = useCallback(() => setZashboardSectionReady(true), []);
  const handleZashboardStateLoaded = useCallback((state: ZashboardState) => {
    setPersistedZashboardVersion(state.selectedVersion);
    if (!zashboardDraftInitialised.current) {
      zashboardDraftInitialised.current = true;
      setDraftZashboardVersion(state.selectedVersion);
    }
  }, []);

  useEffect(() => {
    if (page !== "subscriptions") return;
    let active = true;
    setSubscriptionBusy(true);
    getSubscriptionPreferences()
      .then((preferences) => {
        if (active) {
          setDraftSelectorDensity(preferences.selectorDensity);
          setDraftDelayTestConcurrency(preferences.delayTestConcurrency);
          setDraftConnectionResetMode(preferences.connectionResetMode);
        }
      })
      .catch(() => {
        if (active) setSaveError(true);
      })
      .finally(() => {
        if (active) setSubscriptionBusy(false);
      });
    return () => {
      active = false;
    };
  }, [page]);

  useEffect(() => {
    if (page !== "subscriptions" && page !== "config") return;
    let active = true;
    setManagedDirectories(null);
    setManagedDirectoriesBusy(true);
    setManagedDirectoriesError(false);
    getManagedStorageDirectories()
      .then((directories) => {
        if (active) setManagedDirectories(directories);
      })
      .catch(() => {
        if (active) setManagedDirectoriesError(true);
      })
      .finally(() => {
        if (active) setManagedDirectoriesBusy(false);
      });
    return () => {
      active = false;
    };
  }, [page]);

  const handleCoreStateLoaded = useCallback(
    (state: MihomoVersionManagerState) => {
      setPersistedCoreVersion(state.selectedVersion);
      if (!coreDraftInitialised.current) {
        coreDraftInitialised.current = true;
        setDraftCoreVersion(state.selectedVersion);
      }
    },
    [],
  );

  const handleGeoDataStateLoaded = useCallback((state: GeoDataState) => {
    setPersistedGeoDataPreferences(state.preferences);
    if (!geoDataDraftInitialised.current) {
      geoDataDraftInitialised.current = true;
      setDraftGeoDataPreferences(state.preferences);
    }
  }, []);

  const confirm = async () => {
    setSaveError(false);
    setSaving(true);
    try {
      if (page === "home") {
        if (draftZashboardVersion && draftZashboardVersion !== persistedZashboardVersion) {
          await selectZashboardVersion(draftZashboardVersion);
          await refresh();
        }
        if (
          draftCoreVersion &&
          draftCoreVersion !== persistedCoreVersion
        ) {
          await selectMihomoVersion(draftCoreVersion);
          await refresh();
        }
        if (
          !geoDataPreferencesEqual(
            draftGeoDataPreferences,
            persistedGeoDataPreferences,
          )
        ) {
          await saveGeoDataPreferences(draftGeoDataPreferences);
          await refresh();
        }
        setTheme(draftTheme);
        setLanguage(draftLanguage);
      } else if (page === "subscriptions") {
        await saveSubscriptionPreferences({
          selectorDensity: draftSelectorDensity,
          delayTestConcurrency: draftDelayTestConcurrency,
          connectionResetMode: draftConnectionResetMode,
        });
      }
      navigate(navigationItem.path);
    } catch {
      setSaveError(true);
    } finally {
      setSaving(false);
    }
  };

  const cancel = () => {
    navigate(navigationItem.path);
  };

  return (
    <div className="page-stack settings-page">
      {saveError ? (
        <Alert
          closable
          description={t(
            page === "subscriptions"
              ? "settings.subscriptions.errors.save"
              : "settings.home.errors.save",
          )}
          onClose={() => setSaveError(false)}
          showIcon
          type="error"
        />
      ) : null}

      {page === "home" ? (
        <>
          <div className="feature-grid two-columns settings-grid">
            <FeatureCard title={t("settings.home.appearanceTitle")}>
              <SettingsRow title={t("settings.home.theme")}>
                <Segmented
                  aria-label={t("settings.home.theme")}
                  onChange={(value) => setDraftTheme(value as ThemeMode)}
                  options={[
                    { label: t("settings.home.light"), value: "light" },
                    { label: t("settings.home.dark"), value: "dark" },
                  ]}
                  value={draftTheme}
                />
              </SettingsRow>
              <SettingsRow title={t("settings.home.language")}>
                <Segmented
                  aria-label={t("settings.home.language")}
                  onChange={(value) =>
                    setDraftLanguage(value as AppLanguage)
                  }
                  options={[
                    { label: t("settings.home.chinese"), value: "zh-CN" },
                    { label: t("settings.home.english"), value: "en-US" },
                  ]}
                  value={draftLanguage}
                />
              </SettingsRow>
            </FeatureCard>

            <FeatureCard title={t("settings.home.updatesTitle")}>
              <SettingsRow
                helpTopic="appUpdate"
                title={t("settings.home.appUpdate")}
              >
                <JeemiUpdateButton />
              </SettingsRow>
            </FeatureCard>
          </div>

          <div
            className="settings-focus-target"
            id={homeSettingsSectionIds.mihomo}
            ref={mihomoSectionRef}
          >
            <MihomoVersionSettings
              autoCheck={focusTarget === "mihomo" && autoCheck}
              draftVersion={draftCoreVersion}
              onBusyChange={setCoreBusy}
              onDraftVersionChange={setDraftCoreVersion}
              onInitialLoadComplete={handleMihomoInitialLoadComplete}
              onStateLoaded={handleCoreStateLoaded}
            />
          </div>
          <div
            className="settings-focus-target"
            id={homeSettingsSectionIds.geodata}
            ref={geoDataSectionRef}
          >
            <GeoDataSettings
              autoCheck={focusTarget === "geodata" && autoCheck}
              draftPreferences={draftGeoDataPreferences}
              onBusyChange={setGeoDataBusy}
              onDraftPreferencesChange={setDraftGeoDataPreferences}
              onInitialLoadComplete={handleGeoDataInitialLoadComplete}
              onStateLoaded={handleGeoDataStateLoaded}
              selectedCoreVersion={persistedCoreVersion}
            />
          </div>
          <div className="settings-focus-target" id={homeSettingsSectionIds.zashboard} ref={zashboardSectionRef}>
            <ZashboardSettings draftVersion={draftZashboardVersion} onDraftVersionChange={setDraftZashboardVersion}
              onBusyChange={setZashboardBusy} onStateLoaded={handleZashboardStateLoaded}
              onInitialLoadComplete={handleZashboardInitialLoadComplete} />
          </div>
        </>
      ) : null}

      {page === "subscriptions" ? (
        <div className="feature-grid two-columns settings-grid">
          <FeatureCard title={t("settings.subscriptions.fetchTitle")}>
            <SettingsRow
              helpTopic="subscriptionFetch"
              title={t("settings.subscriptions.fetchMode")}
            >
              <Segmented
                aria-label={t("settings.subscriptions.fetchMode")}
                disabled
                options={[
                  {
                    label: t("settings.subscriptions.manual"),
                    value: "manual",
                  },
                  {
                    label: t("settings.subscriptions.onLaunch"),
                    value: "onLaunch",
                  },
                ]}
                value="manual"
              />
            </SettingsRow>
          </FeatureCard>

          <FeatureCard title={t("settings.subscriptions.displayTitle")}>
            <SettingsRow title={t("settings.subscriptions.displayMode")}>
              <Segmented
                aria-label={t("settings.subscriptions.displayMode")}
                disabled={subscriptionBusy}
                onChange={(value) =>
                  setDraftSelectorDensity(value as SelectorDensity)
                }
                options={[
                  {
                    label: t("settings.subscriptions.density.large"),
                    value: "large",
                  },
                  {
                    label: t("settings.subscriptions.density.medium"),
                    value: "medium",
                  },
                  {
                    label: t("settings.subscriptions.density.small"),
                    value: "small",
                  },
                ]}
                value={draftSelectorDensity}
              />
            </SettingsRow>
          </FeatureCard>

          <FeatureCard title={t("settings.subscriptions.speedTestTitle")}>
            <SettingsRow
              helpTopic="subscriptionDelayTest"
              title={t("settings.subscriptions.speedTestConcurrency")}
            >
              <InputNumber
                aria-label={t("settings.subscriptions.speedTestConcurrency")}
                disabled={subscriptionBusy}
                max={50}
                min={1}
                onChange={(value) =>
                  setDraftDelayTestConcurrency(
                    typeof value === "number" ? value : 8,
                  )
                }
                precision={0}
                value={draftDelayTestConcurrency}
              />
            </SettingsRow>
          </FeatureCard>

          <FeatureCard title={t("settings.subscriptions.connectionResetTitle")}>
            <SettingsRow
              helpTopic="subscriptionConnectionReset"
              title={t("settings.subscriptions.connectionResetMode")}
            >
              <Segmented
                aria-label={t("settings.subscriptions.connectionResetMode")}
                disabled={subscriptionBusy}
                onChange={(value) =>
                  setDraftConnectionResetMode(value as ConnectionResetMode)
                }
                options={(["off", "selector", "all"] as const).map(
                  (value) => ({
                    label: t(
                      `settings.subscriptions.connectionResetModes.${value}`,
                    ),
                    value,
                  }),
                )}
                value={draftConnectionResetMode}
              />
            </SettingsRow>
          </FeatureCard>

          <ManagedStorageCard
            directory={managedDirectories?.subscriptions ?? ""}
            error={managedDirectoriesError}
            loading={managedDirectoriesBusy}
            title={t("settings.subscriptions.storageTitle")}
          />
        </div>
      ) : null}

      {page === "config" ? (
        <div className="feature-grid two-columns settings-grid">
          <ManagedStorageCard
            directory={managedDirectories?.localConfigs ?? ""}
            error={managedDirectoriesError}
            loading={managedDirectoriesBusy}
            title={t("settings.config.storageTitle")}
          />
          <ManagedStorageCard
            directory={managedDirectories?.localScripts ?? ""}
            error={managedDirectoriesError}
            loading={managedDirectoriesBusy}
            title={t("settings.config.scriptStorageTitle")}
          />
        </div>
      ) : null}

      {page !== "home" && page !== "subscriptions" && page !== "config" ? (
        <FeatureCard
          className="settings-empty-card"
          title={t("settings.emptyTitle", { page: pageLabel })}
        >
          <Empty
            description={t("settings.emptyDescription", { page: pageLabel })}
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        </FeatureCard>
      ) : null}

      <nav
        aria-label={t("settings.actionBarLabel", { page: pageLabel })}
        className="settings-action-wrap"
      >
        <div className="settings-action-bar">
          <Button
            disabled={coreBusy || geoDataBusy || zashboardBusy || subscriptionBusy || saving}
            icon={<CloseOutlined />}
            onClick={cancel}
            size="large"
          >
            {t("settings.cancel")}
          </Button>
          <Button
            disabled={coreBusy || geoDataBusy || zashboardBusy || subscriptionBusy}
            icon={<CheckOutlined />}
            loading={saving}
            onClick={() => void confirm()}
            size="large"
            type="primary"
          >
            {t("settings.confirm")}
          </Button>
        </div>
      </nav>
    </div>
  );
}
