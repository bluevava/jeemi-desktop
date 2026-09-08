import type { MacNetworkAuthorizationStatus } from "../types/macNetwork";
import type { ProxyAuthorizationStatus } from "../types/authorization";
import type { JeemiUpdateResult, JeemiUpdateState } from "../types/appUpdate";
import type {
  BootstrapState,
  RuntimeConfigurationText,
  RuntimePreferences,
  RuntimeStatus,
  RuntimeYamlFragmentInput,
  RuntimeYamlFragmentValidation,
} from "../types/runtime";
import type {
  MihomoCoreDialogImportResult,
  MihomoCoreDialogLabels,
  MihomoVersionManagerState,
} from "../types/mihomo";
import {
  defaultGeoDataPreferences,
  type GeoDataDialogImportResult,
  type GeoDataDialogLabels,
  type GeoDataKind,
  type GeoDataPreferences,
  type GeoDataState,
} from "../types/geodata";
import type { ManagedStorageDirectories } from "../types/storage";
import type {
  LocalPackageKind,
  LocalPackagePreview,
} from "../types/localPackage";
import type { UIFailure } from "../types/uiDiagnostics";
import type { AppLanguage } from "../i18n/resources";
import type { RuleSetEntryInput, RuleSetEntryPreview } from "../types/ruleSetEntry";
import type {
  ConfigCatalog,
  LocalConfig,
  LocalConfigPreview,
  LocalConfigState,
  LocalConfigResourceState,
  RuleSetResource,
  SaveLocalConfigInput,
  StrategyGroupResource,
} from "../types/localConfig";
import type {
  LocalScript,
  LocalScriptState,
  LocalScriptTestResult,
  SaveLocalScriptInput,
  TestLocalScriptInput,
} from "../types/localScript";
import type {
  SubscriptionRefreshResult,
  FallbackSelection,
  ImportSubscriptionURLInput,
  NativeDialogLabels,
  ProxyDelayCacheScope,
  ProxyDelayCacheSnapshot,
  ProxyDelayCacheUpdate,
  SaveSubscriptionPreferencesInput,
  SaveSubscriptionSelectorDisplayPreferencesInput,
  SubscriptionDetail,
  SubscriptionFallbackResult,
  SubscriptionDialogImportResult,
  SubscriptionScreenImportResult,
  SubscriptionIconDetectionResult,
  SubscriptionPreferences,
  SubscriptionState,
  SubscriptionTextView,
  UpdateSubscriptionInput,
} from "../types/subscription";

import type { DNSQueryPreferences, DNSQueryRequest, DNSQueryResponse } from "../types/dnsQuery";

declare global {
  interface Window {
    go?: {
      desktop?: {
        App?: {
          InitializeClient: (language: string) => Promise<void>;
          GetBootstrapState: () => Promise<BootstrapState>;
          GetDNSQueryPreferences: () => Promise<DNSQueryPreferences>;
          QueryDNS: (input: DNSQueryRequest) => Promise<DNSQueryResponse>;
          CancelDNSQuery: (id: string) => Promise<void>;
          OpenJeemiRepository: () => Promise<void>;
          CheckJeemiUpdates: () => Promise<JeemiUpdateResult>;
          GetJeemiUpdateState: () => Promise<JeemiUpdateState>;
          InstallJeemiUpdate: (version: string) => Promise<void>;
          CancelJeemiUpdate: () => Promise<void>;
          DismissJeemiUpdateResult: (id: string) => Promise<void>;
          OpenJeemiReleases: () => Promise<void>;
          OpenRuleDocumentation: () => Promise<void>;
          PreviewRuleSetEntry: (input: RuleSetEntryInput) => Promise<RuleSetEntryPreview>;
          SaveRuleSetEntry: (input: RuleSetEntryInput) => Promise<LocalConfigResourceState>;
          ReportUIFailure: (input: UIFailure) => Promise<void>;
          OpenUIDiagnosticsDirectory: () => Promise<void>;
          ReloadInterface: () => Promise<void>;
          RefreshRuntimeStatus: () => Promise<RuntimeStatus>;
          StartProxy: () => Promise<RuntimeStatus>;
          CancelCoreAuthorization: () => Promise<void>;
          GetProxyAuthorization: () => Promise<ProxyAuthorizationStatus>;
          GetAuthorizationHelperStatus: () => Promise<ProxyAuthorizationStatus>;
          SetupProxyAuthorization: () => Promise<ProxyAuthorizationStatus>;
          RemoveAuthorizationHelper: () => Promise<ProxyAuthorizationStatus>;
          OpenProxyAuthorizationSettings: () => Promise<void>;
          GetMacNetworkAuthorization: () => Promise<MacNetworkAuthorizationStatus>;
          SetupMacNetworkAuthorization: () => Promise<MacNetworkAuthorizationStatus>;
          OpenMacNetworkSettings: () => Promise<void>;
          OpenMacApplicationsDirectory: () => Promise<void>;
          RemoveMacNetworkHelper: () => Promise<MacNetworkAuthorizationStatus>;
          StopProxy: () => Promise<RuntimeStatus>;
          RestartProxy: () => Promise<RuntimeStatus>;
          GetRuntimeConfigurationText: () => Promise<RuntimeConfigurationText>;
          SelectRuntimeProxy: (group: string, proxy: string) => Promise<void>;
          RememberRuntimeProxySelection: (
            sessionId: string,
            subscriptionId: string,
            group: string,
            proxy: string,
          ) => Promise<void>;
          UpdateRuntimeRuleProvider: (name: string) => Promise<void>;
          UpdateRuntimeProxyProvider: (name: string) => Promise<void>;
          GetRuntimePreferences: () => Promise<RuntimePreferences>;
          GetDefaultLANBypassRules: () => Promise<string[]>;
          ValidateRuntimeYAMLFragment: (
            input: RuntimeYamlFragmentInput,
          ) => Promise<RuntimeYamlFragmentValidation>;
          SaveRuntimePreferences: (
            input: RuntimePreferences,
          ) => Promise<RuntimePreferences>;
          GetMihomoVersionState: () => Promise<MihomoVersionManagerState>;
          CheckMihomoUpdates: () => Promise<MihomoVersionManagerState>;
          DownloadMihomoVersion: (
            version: string,
          ) => Promise<MihomoVersionManagerState>;
          ImportMihomoCore: (
            labels: MihomoCoreDialogLabels,
          ) => Promise<MihomoCoreDialogImportResult>;
          OpenMihomoReleasesPage: () => Promise<void>;
          CancelMihomoDownload: () => Promise<void>;
          SelectMihomoVersion: (
            version: string,
          ) => Promise<MihomoVersionManagerState>;
          RemoveMihomoVersion: (
            version: string,
          ) => Promise<MihomoVersionManagerState>;
          OpenMihomoDirectory: () => Promise<void>;
          GetGeoDataState: () => Promise<GeoDataState>;
          SaveGeoDataPreferences: (
            input: GeoDataPreferences,
          ) => Promise<GeoDataState>;
          CheckGeoDataUpdates: () => Promise<GeoDataState>;
          DownloadGeoData: (kind: GeoDataKind) => Promise<GeoDataState>;
          DownloadAllGeoData: () => Promise<GeoDataState>;
          CancelGeoDataDownload: () => Promise<void>;
          ImportGeoData: (
            kind: GeoDataKind,
            labels: GeoDataDialogLabels,
          ) => Promise<GeoDataDialogImportResult>;
          ApplyGeoDataUpdates: () => Promise<RuntimeStatus>;
          CleanOldGeoDataRevisions: () => Promise<GeoDataState>;
          OpenGeoDataDirectory: () => Promise<void>;
          OpenGeoDataReleasesPage: () => Promise<void>;
          GetConfigCatalog: () => Promise<ConfigCatalog>;
          GetManagedStorageDirectories: () => Promise<ManagedStorageDirectories>;
          GetLocalConfigState: () => Promise<LocalConfigState>;
          GetLocalConfig: (id: string) => Promise<LocalConfig>;
          PreviewLocalConfig: (
            input: SaveLocalConfigInput,
          ) => Promise<LocalConfigPreview>;
          SaveLocalConfig: (
            input: SaveLocalConfigInput,
          ) => Promise<LocalConfig>;
          DeleteLocalConfig: (id: string) => Promise<LocalConfigState>;
          ExportLocalPackage: (
            kind: LocalPackageKind,
            id: string,
            labels: NativeDialogLabels,
          ) => Promise<{ cancelled: boolean }>;
          ImportLocalPackage: (
            kind: LocalPackageKind,
            id: string,
            labels: NativeDialogLabels,
          ) => Promise<LocalPackagePreview>;
          ResolveLocalPackage: (
            token: string,
            confirm: boolean,
          ) => Promise<void>;
          GetLocalScriptState: () => Promise<LocalScriptState>;
          GetLocalScript: (id: string) => Promise<LocalScript>;
          TestLocalScript: (
            input: TestLocalScriptInput,
          ) => Promise<LocalScriptTestResult>;
          SaveLocalScript: (
            input: SaveLocalScriptInput,
          ) => Promise<LocalScript>;
          DeleteLocalScript: (id: string) => Promise<LocalScriptState>;
          GetLocalConfigResources: () => Promise<LocalConfigResourceState>;
          SaveStrategyGroup: (
            input: StrategyGroupResource,
          ) => Promise<LocalConfigResourceState>;
          DeleteStrategyGroup: (
            id: string,
          ) => Promise<LocalConfigResourceState>;
          SaveRuleSet: (
            input: RuleSetResource,
          ) => Promise<LocalConfigResourceState>;
          DeleteRuleSet: (id: string) => Promise<LocalConfigResourceState>;
          GetSubscriptionState: () => Promise<SubscriptionState>;
          GetSubscription: (id: string) => Promise<SubscriptionDetail>;
          DetectSubscriptionIcon: (
            sourceUrl: string,
          ) => Promise<SubscriptionIconDetectionResult>;
          ImportSubscriptionURL: (
            input: ImportSubscriptionURLInput,
          ) => Promise<SubscriptionState>;
          ImportSubscriptionFile: (
            labels: NativeDialogLabels,
          ) => Promise<SubscriptionDialogImportResult>;
          ImportSubscriptionQRCodeImage: (
            labels: NativeDialogLabels,
          ) => Promise<SubscriptionDialogImportResult>;
          ImportSubscriptionQRCodeScreen: () => Promise<SubscriptionScreenImportResult>;
          CheckSubscriptionRefresh: (
            id: string,
          ) => Promise<SubscriptionRefreshResult>;
          ResolveSubscriptionRefresh: (
            token: string,
            detach: boolean,
          ) => Promise<SubscriptionState>;
          RefreshSubscription: (id: string) => Promise<SubscriptionState>;
          UpdateSubscription: (
            input: UpdateSubscriptionInput,
          ) => Promise<SubscriptionState>;
          DeleteSubscription: (id: string) => Promise<SubscriptionState>;
          GetSubscriptionText: (id: string) => Promise<SubscriptionTextView>;
          SetSubscriptionLocalConfig: (
            id: string,
            localConfigId: string,
          ) => Promise<SubscriptionState>;
          SetSubscriptionLocalScript: (
            id: string,
            localScriptId: string,
          ) => Promise<SubscriptionState>;
          SetSubscriptionFallback: (
            id: string,
            input: FallbackSelection,
            expectedRevision: number,
          ) => Promise<SubscriptionFallbackResult>;
          SetSubscriptionRuleProviderEnabled: (
            id: string,
            providerName: string,
            enabled: boolean,
          ) => Promise<SubscriptionState>;
          SelectSubscription: (id: string) => Promise<SubscriptionState>;
          GetSubscriptionPreferences: () => Promise<SubscriptionPreferences>;
          SaveSubscriptionPreferences: (
            input: SaveSubscriptionPreferencesInput,
          ) => Promise<SubscriptionPreferences>;
          SaveSubscriptionSelectorDisplayPreferences: (
            input: SaveSubscriptionSelectorDisplayPreferencesInput,
          ) => Promise<SubscriptionPreferences>;
          GetProxyDelayCache: (
            scope: ProxyDelayCacheScope,
          ) => Promise<ProxyDelayCacheSnapshot>;
          SaveProxyDelayCache: (
            update: ProxyDelayCacheUpdate,
          ) => Promise<ProxyDelayCacheSnapshot>;
          SetTrayLanguage: (language: AppLanguage) => Promise<void>;
          HideWindowToTray: () => Promise<void>;
        };
      };
    };
  }
}

const browserFallback: BootstrapState = {
  app: {
    name: "Jeemi",
    version: "0.1.0-dev",
  },
  runtime: {
    coreAuthorizationPending: false,
    platform: {
      os: "browser",
      architecture: "development",
    },
    core: {
      state: "not_installed",
      version: "",
      controllerReady: false,
    },
    clientStartedAt: new Date().toISOString(),
    proxyMode: "off",
    systemProxy: false,
    tunEnabled: false,
    statusMessageKey: "runtime.status.frameworkReady",
    mihomo: {
      state: "stopped",
      desiredRunning: false,
      coreVersion: "",
      pid: 0,
      generationId: "",
      subscriptionId: "",
      source: {
        normalizationFingerprint: "",
        subscriptionId: "",
        subscriptionRevision: "",
        localConfigId: "",
        localConfigRevision: 0,
        localScriptId: "",
        localScriptRevision: 0,
        ruleProviderOverrideRevision: 0,
        fallbackOverrideRevision: 0,
        geoDataFingerprint: "",
      },
      controllerReady: false,
      controllerSession: null,
      outboundMode: "",
      proxyMode: "off",
      systemProxy: false,
      tunEnabled: false,
      tunDevice: "",
      startedAt: "",
      restartAttempt: 0,
      lastError: null,
    },
    configuration: {
      state: "idle",
      trigger: "",
      desiredFingerprint: "",
      appliedFingerprint: "",
      subscriptionId: "",
      subscriptionRevision: "",
      localConfigId: "",
      localConfigRevision: 0,
      localScriptId: "",
      localScriptRevision: 0,
      ruleProviderOverrideRevision: 0,
      fallbackOverrideRevision: 0,
      geoDataFingerprint: "",
      coreVersion: "",
      coreValidated: false,
      updatedAt: "",
      lastError: null,
    },
    geoData: {
      status: "missing",
      missingKinds: ["geoip-mmdb", "geosite", "asn"],
      invalidKinds: [],
    },
    memory: {
      clientBytes: null,
      webViewBytes: null,
      mihomoBytes: null,
      totalBytes: null,
    },
  },
};

const browserMihomoFallback: MihomoVersionManagerState = {
  dataDirectory: "",
  coreDirectory: "",
  target: "browser-development",
  selectedVersion: "",
  latestVersion: "",
  lastCheckedAt: "",
  downloadingVersion: "",
  installedVersions: [],
  availableVersions: [],
};

const browserRuntimePreferencesFallback: RuntimePreferences = {
  outboundMode: "rule",
  proxyMode: "tun",
  listenerType: "mixed",
  listenPort: 7890,
  allowLan: false,
  tunStack: "mixed",
  logLevel: "silent",
  findProcessMode: "strict",
  ipv6: true,
  dnsEnabled: true,
  dnsListen: "127.0.0.1:1053",
  dnsIpv6: false,
  dnsUseHosts: false,
  dnsEnhancedMode: "fake-ip",
  dnsFakeIpRange: "198.18.0.1/16",
  dnsFakeIpRange6: "fdfe:dcba:9876::1/64",
  tunAutoRoute: true,
  tunAutoDetectInterface: true,
  tunRouteExcludeAddress: [],
  tunRouteExcludeAddressEnabled: false,
  tunRouteExcludeAddressMerge: "append",
  dnsNameservers: ["114.114.114.114", "223.5.5.5"],
  dnsNameserverEnabled: true,
  dnsNameserverMerge: "append",
  dnsFakeIpFilter: [
    "+.lan",
    "+.local",
    "+.home.arpa",
    "+.localdomain",
    "+.localhost",
  ],
  dnsFakeIpFilterEnabled: true,
  dnsFakeIpFilterMerge: "override",
  dnsProxyServerNameservers: [],
  dnsProxyServerNameserverEnabled: false,
  dnsProxyServerNameserverMerge: "append",
  dnsNameserverPolicyYaml: "",
  dnsNameserverPolicyEnabled: false,
  dnsNameserverPolicyMerge: "append",
  dnsProxyServerNameserverPolicyYaml: "",
  dnsProxyServerNameserverPolicyEnabled: false,
  dnsProxyServerNameserverPolicyMerge: "append",
  hostsYaml: "",
  hostsMerge: "append",
  lanBypassRules: [
    "DOMAIN,localhost,DIRECT",
    "DOMAIN-SUFFIX,lan,DIRECT",
    "DOMAIN-SUFFIX,local,DIRECT",
    "DOMAIN-SUFFIX,localdomain,DIRECT",
    "DOMAIN-SUFFIX,home.arpa,DIRECT",
    "IP-CIDR,0.0.0.0/8,DIRECT,no-resolve",
    "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
    "IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
    "IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
    "IP-CIDR,169.254.0.0/16,DIRECT,no-resolve",
    "IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
    "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
    "IP-CIDR,224.0.0.0/3,DIRECT,no-resolve",
    "IP-CIDR6,::/128,DIRECT,no-resolve",
    "IP-CIDR6,::1/128,DIRECT,no-resolve",
    "IP-CIDR6,fc00::/7,DIRECT,no-resolve",
    "IP-CIDR6,fe80::/10,DIRECT,no-resolve",
    "IP-CIDR6,ff00::/8,DIRECT,no-resolve",
  ],
};

const browserGeoDataFallback: GeoDataState = {
  directory: "",
  runtimeDirectory: "",
  preferences: defaultGeoDataPreferences,
  activeGeoIpMode: "mmdb",
  activeLoader: "memconservative",
  lastCheckedAt: "",
  downloadingKind: "",
  restartRequired: false,
  health: {
    status: "missing",
    missingKinds: ["geoip-mmdb", "geosite", "asn"],
    invalidKinds: [],
  },
  assets: (["geoip-mmdb", "geoip-dat", "geosite", "asn"] as const).map(
    (kind) => ({
      kind,
      canonicalName:
        kind === "geoip-mmdb"
          ? "geoip.metadb"
          : kind === "geoip-dat"
            ? "GeoIP.dat"
            : kind === "geosite"
              ? "GeoSite.dat"
              : "ASN.mmdb",
      installed: false,
      active: false,
      pending: false,
      sha256: "",
      activeSha256: "",
      availableSha256: "",
      size: 0,
      installedAt: "",
      source: "",
      sourceUrl: "",
      validatedCoreVersion: "",
      publisherVerified: false,
    }),
  ),
};

function appBinding() {
  return window.go?.desktop?.App;
}

export async function getBootstrapState(): Promise<BootstrapState> {
  const binding = appBinding();
  return binding ? binding.GetBootstrapState() : browserFallback;
}

export async function refreshRuntimeStatus(): Promise<RuntimeStatus> {
  const binding = appBinding();
  return binding ? binding.RefreshRuntimeStatus() : browserFallback.runtime;
}

export async function startProxy(): Promise<RuntimeStatus> {
  return requireAppBinding().StartProxy();
}

export async function cancelCoreAuthorization(): Promise<void> {
  return requireAppBinding().CancelCoreAuthorization();
}

export async function stopProxy(): Promise<RuntimeStatus> {
  return requireAppBinding().StopProxy();
}

export async function restartProxy(): Promise<RuntimeStatus> {
  return requireAppBinding().RestartProxy();
}

export async function getRuntimeConfigurationText(): Promise<RuntimeConfigurationText> {
  return requireAppBinding().GetRuntimeConfigurationText();
}

export async function selectRuntimeProxy(
  group: string,
  proxy: string,
): Promise<void> {
  return requireAppBinding().SelectRuntimeProxy(group, proxy);
}

export async function rememberRuntimeProxySelection(
  sessionId: string,
  subscriptionId: string,
  group: string,
  proxy: string,
): Promise<void> {
  return requireAppBinding().RememberRuntimeProxySelection(
    sessionId,
    subscriptionId,
    group,
    proxy,
  );
}

export async function updateRuntimeRuleProvider(name: string): Promise<void> {
  return requireAppBinding().UpdateRuntimeRuleProvider(name);
}

export async function updateRuntimeProxyProvider(name: string): Promise<void> {
  return requireAppBinding().UpdateRuntimeProxyProvider(name);
}

export async function getRuntimePreferences(): Promise<RuntimePreferences> {
  const binding = appBinding();
  return binding
    ? binding.GetRuntimePreferences()
    : browserRuntimePreferencesFallback;
}

export async function getDefaultLANBypassRules(): Promise<string[]> {
  const binding = appBinding();
  return binding
    ? binding.GetDefaultLANBypassRules()
    : [...browserRuntimePreferencesFallback.lanBypassRules];
}

export async function validateRuntimeYAMLFragment(
  input: RuntimeYamlFragmentInput,
): Promise<RuntimeYamlFragmentValidation> {
  return requireAppBinding().ValidateRuntimeYAMLFragment(input);
}

export async function saveRuntimePreferences(
  input: RuntimePreferences,
): Promise<RuntimePreferences> {
  return requireAppBinding().SaveRuntimePreferences(input);
}

export async function getMihomoVersionState(): Promise<MihomoVersionManagerState> {
  const binding = appBinding();
  return binding ? binding.GetMihomoVersionState() : browserMihomoFallback;
}

export async function checkMihomoUpdates(): Promise<MihomoVersionManagerState> {
  const binding = requireAppBinding();
  return binding.CheckMihomoUpdates();
}

export async function downloadMihomoVersion(
  version: string,
): Promise<MihomoVersionManagerState> {
  const binding = requireAppBinding();
  return binding.DownloadMihomoVersion(version);
}

export async function importMihomoCore(
  labels: MihomoCoreDialogLabels,
): Promise<MihomoCoreDialogImportResult> {
  return requireAppBinding().ImportMihomoCore(labels);
}

export async function openJeemiRepository(): Promise<void> {
  return requireAppBinding().OpenJeemiRepository();
}

export async function checkJeemiUpdates(): Promise<JeemiUpdateResult> {
  return requireAppBinding().CheckJeemiUpdates();
}

export async function getJeemiUpdateState(): Promise<JeemiUpdateState> {
  return requireAppBinding().GetJeemiUpdateState();
}

export async function installJeemiUpdate(version: string): Promise<void> {
  return requireAppBinding().InstallJeemiUpdate(version);
}

export async function cancelJeemiUpdate(): Promise<void> {
  return requireAppBinding().CancelJeemiUpdate();
}

export async function dismissJeemiUpdateResult(id: string): Promise<void> {
  return requireAppBinding().DismissJeemiUpdateResult(id);
}

export async function openJeemiReleases(): Promise<void> {
  return requireAppBinding().OpenJeemiReleases();
}

export async function openRuleDocumentation(): Promise<void> {
  return requireAppBinding().OpenRuleDocumentation();
}

export async function previewRuleSetEntry(input: RuleSetEntryInput): Promise<RuleSetEntryPreview> {
  return requireAppBinding().PreviewRuleSetEntry(input);
}

export async function saveRuleSetEntry(input: RuleSetEntryInput): Promise<LocalConfigResourceState> {
  return requireAppBinding().SaveRuleSetEntry(input);
}

export async function openMihomoReleasesPage(): Promise<void> {
  return requireAppBinding().OpenMihomoReleasesPage();
}

export async function cancelMihomoDownload(): Promise<void> {
  const binding = requireAppBinding();
  return binding.CancelMihomoDownload();
}

export async function selectMihomoVersion(
  version: string,
): Promise<MihomoVersionManagerState> {
  const binding = requireAppBinding();
  return binding.SelectMihomoVersion(version);
}

export async function removeMihomoVersion(
  version: string,
): Promise<MihomoVersionManagerState> {
  const binding = requireAppBinding();
  return binding.RemoveMihomoVersion(version);
}

export async function openMihomoDirectory(): Promise<void> {
  const binding = requireAppBinding();
  return binding.OpenMihomoDirectory();
}

export async function getGeoDataState(): Promise<GeoDataState> {
  const binding = appBinding();
  return binding ? binding.GetGeoDataState() : browserGeoDataFallback;
}

export async function saveGeoDataPreferences(
  input: GeoDataPreferences,
): Promise<GeoDataState> {
  return requireAppBinding().SaveGeoDataPreferences(input);
}

export async function checkGeoDataUpdates(): Promise<GeoDataState> {
  return requireAppBinding().CheckGeoDataUpdates();
}

export async function downloadGeoData(
  kind: GeoDataKind,
): Promise<GeoDataState> {
  return requireAppBinding().DownloadGeoData(kind);
}

export async function downloadAllGeoData(): Promise<GeoDataState> {
  return requireAppBinding().DownloadAllGeoData();
}

export async function cancelGeoDataDownload(): Promise<void> {
  return requireAppBinding().CancelGeoDataDownload();
}

export async function importGeoData(
  kind: GeoDataKind,
  labels: GeoDataDialogLabels,
): Promise<GeoDataDialogImportResult> {
  return requireAppBinding().ImportGeoData(kind, labels);
}

export async function applyGeoDataUpdates(): Promise<RuntimeStatus> {
  return requireAppBinding().ApplyGeoDataUpdates();
}

export async function cleanOldGeoDataRevisions(): Promise<GeoDataState> {
  return requireAppBinding().CleanOldGeoDataRevisions();
}

export async function openGeoDataDirectory(): Promise<void> {
  return requireAppBinding().OpenGeoDataDirectory();
}

export async function openGeoDataReleasesPage(): Promise<void> {
  return requireAppBinding().OpenGeoDataReleasesPage();
}

export async function getConfigCatalog(): Promise<ConfigCatalog> {
  return requireAppBinding().GetConfigCatalog();
}

export async function getManagedStorageDirectories(): Promise<ManagedStorageDirectories> {
  return requireAppBinding().GetManagedStorageDirectories();
}

export async function getLocalConfigState(): Promise<LocalConfigState> {
  return requireAppBinding().GetLocalConfigState();
}

export async function exportLocalPackage(
  kind: LocalPackageKind,
  id: string,
  labels: NativeDialogLabels,
): Promise<{ cancelled: boolean }> {
  return requireAppBinding().ExportLocalPackage(kind, id, labels);
}

export async function importLocalPackage(
  kind: LocalPackageKind,
  id: string,
  labels: NativeDialogLabels,
): Promise<LocalPackagePreview> {
  return requireAppBinding().ImportLocalPackage(kind, id, labels);
}

export async function resolveLocalPackage(
  token: string,
  confirm: boolean,
): Promise<void> {
  return requireAppBinding().ResolveLocalPackage(token, confirm);
}

export async function getLocalConfig(id: string): Promise<LocalConfig> {
  return requireAppBinding().GetLocalConfig(id);
}

export async function previewLocalConfig(
  input: SaveLocalConfigInput,
): Promise<LocalConfigPreview> {
  return requireAppBinding().PreviewLocalConfig(input);
}

export async function saveLocalConfig(
  input: SaveLocalConfigInput,
): Promise<LocalConfig> {
  return requireAppBinding().SaveLocalConfig(input);
}

export async function deleteLocalConfig(id: string): Promise<LocalConfigState> {
  return requireAppBinding().DeleteLocalConfig(id);
}

export async function getLocalScriptState(): Promise<LocalScriptState> {
  return requireAppBinding().GetLocalScriptState();
}

export async function getLocalScript(id: string): Promise<LocalScript> {
  return requireAppBinding().GetLocalScript(id);
}

export async function testLocalScript(
  input: TestLocalScriptInput,
): Promise<LocalScriptTestResult> {
  return requireAppBinding().TestLocalScript(input);
}

export async function saveLocalScript(
  input: SaveLocalScriptInput,
): Promise<LocalScript> {
  return requireAppBinding().SaveLocalScript(input);
}

export async function deleteLocalScript(id: string): Promise<LocalScriptState> {
  return requireAppBinding().DeleteLocalScript(id);
}

export async function getLocalConfigResources(): Promise<LocalConfigResourceState> {
  return requireAppBinding().GetLocalConfigResources();
}

export async function saveStrategyGroup(
  input: StrategyGroupResource,
): Promise<LocalConfigResourceState> {
  return requireAppBinding().SaveStrategyGroup(input);
}

export async function deleteStrategyGroup(
  id: string,
): Promise<LocalConfigResourceState> {
  return requireAppBinding().DeleteStrategyGroup(id);
}

export async function saveRuleSet(
  input: RuleSetResource,
): Promise<LocalConfigResourceState> {
  return requireAppBinding().SaveRuleSet(input);
}

export async function deleteRuleSet(
  id: string,
): Promise<LocalConfigResourceState> {
  return requireAppBinding().DeleteRuleSet(id);
}

export async function getSubscriptionState(): Promise<SubscriptionState> {
  return requireAppBinding().GetSubscriptionState();
}

export async function getSubscription(id: string): Promise<SubscriptionDetail> {
  return requireAppBinding().GetSubscription(id);
}

export async function detectSubscriptionIcon(
  sourceUrl: string,
): Promise<SubscriptionIconDetectionResult> {
  return requireAppBinding().DetectSubscriptionIcon(sourceUrl);
}

export async function importSubscriptionURL(
  input: ImportSubscriptionURLInput,
): Promise<SubscriptionState> {
  return requireAppBinding().ImportSubscriptionURL(input);
}

export async function importSubscriptionFile(
  labels: NativeDialogLabels,
): Promise<SubscriptionDialogImportResult> {
  return requireAppBinding().ImportSubscriptionFile(labels);
}

export async function importSubscriptionQRCodeImage(
  labels: NativeDialogLabels,
): Promise<SubscriptionDialogImportResult> {
  return requireAppBinding().ImportSubscriptionQRCodeImage(labels);
}

export async function importSubscriptionQRCodeScreen(): Promise<SubscriptionScreenImportResult> {
  return requireAppBinding().ImportSubscriptionQRCodeScreen();
}

export async function refreshSubscription(
  id: string,
): Promise<SubscriptionState> {
  return requireAppBinding().RefreshSubscription(id);
}

export async function updateSubscription(
  input: UpdateSubscriptionInput,
): Promise<SubscriptionState> {
  return requireAppBinding().UpdateSubscription(input);
}

export async function deleteSubscription(
  id: string,
): Promise<SubscriptionState> {
  return requireAppBinding().DeleteSubscription(id);
}

export async function getSubscriptionText(
  id: string,
): Promise<SubscriptionTextView> {
  return requireAppBinding().GetSubscriptionText(id);
}

export async function setSubscriptionLocalConfig(
  id: string,
  localConfigId: string,
): Promise<SubscriptionState> {
  return requireAppBinding().SetSubscriptionLocalConfig(id, localConfigId);
}

export async function setSubscriptionLocalScript(
  id: string,
  localScriptId: string,
): Promise<SubscriptionState> {
  return requireAppBinding().SetSubscriptionLocalScript(id, localScriptId);
}

export async function setSubscriptionRuleProviderEnabled(
  id: string,
  providerName: string,
  enabled: boolean,
): Promise<SubscriptionState> {
  return requireAppBinding().SetSubscriptionRuleProviderEnabled(
    id,
    providerName,
    enabled,
  );
}

export async function selectSubscription(
  id: string,
): Promise<SubscriptionState> {
  return requireAppBinding().SelectSubscription(id);
}

export async function getSubscriptionPreferences(): Promise<SubscriptionPreferences> {
  return requireAppBinding().GetSubscriptionPreferences();
}

export async function saveSubscriptionPreferences(
  input: SaveSubscriptionPreferencesInput,
): Promise<SubscriptionPreferences> {
  return requireAppBinding().SaveSubscriptionPreferences(input);
}

export async function saveSubscriptionSelectorDisplayPreferences(
  input: SaveSubscriptionSelectorDisplayPreferencesInput,
): Promise<SubscriptionPreferences> {
  return requireAppBinding().SaveSubscriptionSelectorDisplayPreferences(input);
}

export async function getProxyDelayCache(
  scope: ProxyDelayCacheScope,
): Promise<ProxyDelayCacheSnapshot> {
  return requireAppBinding().GetProxyDelayCache(scope);
}

export async function saveProxyDelayCache(
  update: ProxyDelayCacheUpdate,
): Promise<ProxyDelayCacheSnapshot> {
  return requireAppBinding().SaveProxyDelayCache(update);
}

export async function setTrayLanguage(language: AppLanguage): Promise<void> {
  const binding = appBinding();
  if (binding) {
    await binding.SetTrayLanguage(language);
  }
}

export async function hideWindowToTray(): Promise<void> {
  const binding = appBinding();
  if (binding) {
    await binding.HideWindowToTray();
  }
}

function requireAppBinding(): NonNullable<ReturnType<typeof appBinding>> {
  const binding = appBinding();
  if (!binding) {
    throw new Error("Wails backend is unavailable");
  }
  return binding;
}

export function getProxyAuthorization(): Promise<ProxyAuthorizationStatus> {
  return requireAppBinding().GetProxyAuthorization();
}
export function getAuthorizationHelperStatus(): Promise<ProxyAuthorizationStatus> {
  return requireAppBinding().GetAuthorizationHelperStatus();
}
export function setupProxyAuthorization(): Promise<ProxyAuthorizationStatus> {
  return requireAppBinding().SetupProxyAuthorization();
}
export function removeAuthorizationHelper(): Promise<ProxyAuthorizationStatus> {
  return requireAppBinding().RemoveAuthorizationHelper();
}
export function openProxyAuthorizationSettings(): Promise<void> {
  return requireAppBinding().OpenProxyAuthorizationSettings();
}

export async function setSubscriptionFallback(
  id: string,
  input: FallbackSelection,
  expectedRevision: number,
): Promise<SubscriptionFallbackResult> {
  return requireAppBinding().SetSubscriptionFallback(
    id,
    input,
    expectedRevision,
  );
}

export function getMacNetworkAuthorization(): Promise<MacNetworkAuthorizationStatus> {
  return requireAppBinding().GetMacNetworkAuthorization();
}
export function setupMacNetworkAuthorization(): Promise<MacNetworkAuthorizationStatus> {
  return requireAppBinding().SetupMacNetworkAuthorization();
}
export function openMacNetworkSettings(): Promise<void> {
  return requireAppBinding().OpenMacNetworkSettings();
}
export function openMacApplicationsDirectory(): Promise<void> {
  return requireAppBinding().OpenMacApplicationsDirectory();
}
export function removeMacNetworkHelper(): Promise<MacNetworkAuthorizationStatus> {
  return requireAppBinding().RemoveMacNetworkHelper();
}

export async function checkSubscriptionRefresh(
  id: string,
): Promise<SubscriptionRefreshResult> {
  return requireAppBinding().CheckSubscriptionRefresh(id);
}
export async function resolveSubscriptionRefresh(
  token: string,
  detach: boolean,
): Promise<SubscriptionState> {
  return requireAppBinding().ResolveSubscriptionRefresh(token, detach);
}
