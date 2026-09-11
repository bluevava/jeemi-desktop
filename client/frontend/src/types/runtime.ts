export type CoreState =
  | "not_installed"
  | "downloading"
  | "ready"
  | "starting"
  | "running"
  | "stopping"
  | "stopped"
  | "failed";

export interface PlatformInfo {
  os: "windows" | "darwin" | "linux" | string;
  architecture: string;
}

export interface CoreStatus {
  state: CoreState;
  version: string;
  controllerReady: boolean;
}

export interface RuntimeStatus {
  coreAuthorizationPending: boolean;
  platform: PlatformInfo;
  core: CoreStatus;
  clientStartedAt: string;
  proxyMode: string;
  systemProxy: boolean;
  tunEnabled: boolean;
  statusMessageKey: string;
  mihomo: MihomoRuntimeStatus;
  configuration: RuntimeConfigurationStatus;
  geoData: import("./geodata").GeoDataHealth;
  memory: RuntimeMemoryStatus;
}

export interface RuntimeMemoryStatus {
  clientBytes: number | null;
  webViewBytes: number | null;
  mihomoBytes: number | null;
  totalBytes: number | null;
}

export type RuntimeConfigurationState =
  | "idle"
  | "building"
  | "validating"
  | "ready"
  | "applying"
  | "applied"
  | "failed";

export interface RuntimeConfigurationStatus {
  chainProxyFingerprint: string;
  state: RuntimeConfigurationState;
  trigger: string;
  desiredFingerprint: string;
  appliedFingerprint: string;
  subscriptionId: string;
  subscriptionRevision: string;
  localConfigId: string;
  localConfigRevision: number;
  localScriptId: string;
  localScriptRevision: number;
  ruleProviderOverrideRevision: number;
  fallbackOverrideRevision: number;
  geoDataFingerprint: string;
  coreVersion: string;
  coreValidated: boolean;
  updatedAt: string;
  lastError: MihomoRuntimeFailure | null;
}

export interface RuntimeConfigurationText {
  chainProxyFingerprint: string;
  source: "active" | "resolved";
  contents: string;
  generationId: string;
  fingerprint: string;
  subscriptionId: string;
  subscriptionRevision: string;
  localConfigId: string;
  localConfigRevision: number;
  localScriptId: string;
  localScriptRevision: number;
  ruleProviderOverrideRevision: number;
  fallbackOverrideRevision: number;
  geoDataFingerprint: string;
  coreVersion: string;
  coreValidated: boolean;
  generatedAt: string;
}

export type MihomoRuntimeState =
  | "stopped"
  | "preparing"
  | "validating"
  | "starting"
  | "running"
  | "reloading"
  | "stopping"
  | "recovering"
  | "failed";

export interface MihomoRuntimeFailure {
  code: string;
  phase: string;
  message: string;
}

export interface MihomoControllerSession {
  id: string;
  baseUrl: string;
  secret: string;
}

export interface MihomoRuntimeStatus {
  state: MihomoRuntimeState;
  desiredRunning: boolean;
  coreVersion: string;
  pid: number;
  generationId: string;
  subscriptionId: string;
  source: {
    normalizationFingerprint: string;
    chainProxyFingerprint: string;
    subscriptionId: string;
    subscriptionRevision: string;
    localConfigId: string;
    localConfigRevision: number;
    localScriptId: string;
    localScriptRevision: number;
    ruleProviderOverrideRevision: number;
    fallbackOverrideRevision: number;
    geoDataFingerprint: string;
  };
  controllerReady: boolean;
  controllerSession: MihomoControllerSession | null;
  outboundMode: OutboundMode | "";
  proxyMode: string;
  systemProxy: boolean;
  tunEnabled: boolean;
  tunDevice: string;
  startedAt: string;
  restartAttempt: number;
  lastError: MihomoRuntimeFailure | null;
}

export interface BootstrapState {
  app: {
    name: string;
    version: string;
  };
  runtime: RuntimeStatus;
}

export type OutboundMode = "rule" | "global" | "direct";
export type ProxyMode = "system_proxy" | "tun";
export type ListenerType = "http" | "socks" | "mixed";
export type TunStack = "system" | "gvisor" | "mixed";
export type LogLevel = "silent" | "error" | "warning" | "info" | "debug";
export type FindProcessMode = "always" | "strict" | "off";
export type DnsEnhancedMode = "fake-ip" | "redir-host";
export type RuntimeMergeMode = "append" | "override";
export type RuntimeYamlField =
  | "dns.nameserver"
  | "dns.fake-ip-filter"
  | "dns.proxy-server-nameserver"
  | "dns.nameserver-policy"
  | "dns.proxy-server-nameserver-policy"
  | "hosts"
  | "rules.lan-bypass"
  | "tun.route-exclude-address";

export interface RuntimeYamlFragmentInput {
  field: RuntimeYamlField;
  contents: string;
}

export interface RuntimeYamlValidationIssue {
  code: string;
  message: string;
  line: number;
  column: number;
}

export interface RuntimeYamlFragmentValidation {
  field: RuntimeYamlField;
  valid: boolean;
  normalizedYaml: string;
  values: string[];
  issue: RuntimeYamlValidationIssue | null;
}

export interface RuntimePreferences {
  outboundMode: OutboundMode;
  proxyMode: ProxyMode;
  listenerType: ListenerType;
  listenPort: number;
  allowLan: boolean;
  tunStack: TunStack;
  logLevel: LogLevel;
  findProcessMode: FindProcessMode;
  externalUIEnabled: boolean;
  externalUIVersion: string;
  ipv6: boolean;
  dnsEnabled: boolean;
  dnsListen: string;
  dnsIpv6: boolean;
  dnsUseHosts: boolean;
  dnsEnhancedMode: DnsEnhancedMode;
  dnsFakeIpRange: string;
  dnsFakeIpRange6: string;
  tunAutoRoute: boolean;
  tunAutoDetectInterface: boolean;
  tunRouteExcludeAddress: string[];
  tunRouteExcludeAddressEnabled: boolean;
  tunRouteExcludeAddressMerge: RuntimeMergeMode;
  dnsNameservers: string[];
  dnsNameserverEnabled: boolean;
  dnsNameserverMerge: RuntimeMergeMode;
  dnsFakeIpFilter: string[];
  dnsFakeIpFilterEnabled: boolean;
  dnsFakeIpFilterMerge: RuntimeMergeMode;
  dnsProxyServerNameservers: string[];
  dnsProxyServerNameserverEnabled: boolean;
  dnsProxyServerNameserverMerge: RuntimeMergeMode;
  dnsNameserverPolicyYaml: string;
  dnsNameserverPolicyEnabled: boolean;
  dnsNameserverPolicyMerge: RuntimeMergeMode;
  dnsProxyServerNameserverPolicyYaml: string;
  dnsProxyServerNameserverPolicyEnabled: boolean;
  dnsProxyServerNameserverPolicyMerge: RuntimeMergeMode;
  hostsYaml: string;
  hostsMerge: RuntimeMergeMode;
  lanBypassRules: string[];
}
