export interface FallbackSelection {
  mode: "none" | "direct" | "selector";
  selector: string;
}

export interface FallbackState {
  selection: FallbackSelection;
  originalTarget: string;
  selectors: string[];
  reset: boolean;
}

export type SubscriptionSourceKind = "url" | "file";
export type SubscriptionImportMethod =
  | "url"
  | "file"
  | "qr_image"
  | "qr_screen";
export type SubscriptionFormat = "yaml" | "json" | "txt";
export type SubscriptionIconKind = "" | "emoji" | "url";

export interface SubscriptionRemoteProfile {
  hasTraffic: boolean;
  uploadBytes: number;
  downloadBytes: number;
  totalBytes: number;
  expiresAt: number;
  updateIntervalHours: number;
  observedAt: string;
}

export interface SubscriptionCompositionSummary {
  status: string;
  proxyCount: number;
  selectorCount: number;
  proxyProviderCount: number;
  ruleProviderCount: number;
}

export interface SubscriptionSummary {
  fallback: FallbackSelection;
  fallbackResetTarget: string;
  id: string;
  name: string;
  description: string;
  sourceKind: SubscriptionSourceKind;
  importMethod: SubscriptionImportMethod;
  sourceLabel: string;
  format: SubscriptionFormat;
  iconKind: SubscriptionIconKind;
  icon: string;
  localConfigId: string;
  localScriptId: string;
  ruleProviderOverrideRevision: number;
  fallbackOverrideRevision: number;
  disabledRuleProviders: string[];
  currentRevisionId: string;
  revisionCount: number;
  sizeBytes: number;
  createdAt: string;
  updatedAt: string;
  lastFetchedAt: string;
  remoteProfile: SubscriptionRemoteProfile;
  composition: SubscriptionCompositionSummary;
}

export interface SubscriptionDetail extends SubscriptionSummary {
  sourceUrl: string;
}

export interface SubscriptionState {
  directory: string;
  subscriptions: SubscriptionSummary[];
  selectedSubscriptionId: string;
  preferences: SubscriptionPreferences;
  projection: SubscriptionProjection | null;
}

export interface SubscriptionFallbackResult {
  state: SubscriptionState;
  connectionResetFailed: boolean;
}

export type SelectorDensity = "large" | "medium" | "small";
export type ConnectionResetMode = "off" | "selector" | "all";
export type SelectorSortMode = "default" | "delay" | "name";
export type SelectorViewMode = "panel" | "tabs";

export interface SubscriptionPreferences {
  selectorDensity: SelectorDensity;
  delayTestConcurrency: number;
  connectionResetMode: ConnectionResetMode;
  selectorSortMode: SelectorSortMode;
  selectorViewMode: SelectorViewMode;
}

export interface SaveSubscriptionPreferencesInput {
  selectorDensity: SelectorDensity;
  delayTestConcurrency: number;
  connectionResetMode: ConnectionResetMode;
}

export interface SaveSubscriptionSelectorDisplayPreferencesInput {
  selectorSortMode: SelectorSortMode;
  selectorViewMode: SelectorViewMode;
}

export interface SubscriptionSelectorMember {
  name: string;
  type: string;
  source: "proxy" | "group" | "builtin" | "provider";
  providerName: string;
}

export interface SubscriptionSelector {
  name: string;
  icon: string;
  hidden: boolean;
  type: string;
  defaultSelection: string;
  members: SubscriptionSelectorMember[];
  providerNames: string[];
  unresolvedProviderNames: string[];
  referencedByRules: boolean;
}

export interface SubscriptionRuleProvider {
  name: string;
  type: string;
  behavior: string;
  format: string;
  enabled: boolean;
}

export interface SubscriptionProjection {
  fallback: FallbackState;
  subscriptionId: string;
  revisionId: string;
  localConfigId: string;
  localConfigRevision: number;
  localScriptId: string;
  localScriptRevision: number;
  ruleProviderOverrideRevision: number;
  fallbackOverrideRevision: number;
  geoDataFingerprint: string;
  status: string;
  coreValidationStatus: "not_run";
  summary: SubscriptionCompositionSummary;
  selectors: SubscriptionSelector[];
  proxies: SubscriptionSelectorMember[];
  ruleProviders: SubscriptionRuleProvider[];
  warnings: string[];
}

export interface ImportSubscriptionURLInput {
  name: string;
  description: string;
  sourceUrl: string;
  importMethod: "url";
  iconKind: SubscriptionIconKind;
  icon: string;
}

export interface UpdateSubscriptionInput {
  id: string;
  name: string;
  description: string;
  sourceUrl: string;
  iconKind: SubscriptionIconKind;
  icon: string;
}

export interface SubscriptionIconDetectionResult {
  found: boolean;
  iconKind: SubscriptionIconKind;
  icon: string;
}

export interface SubscriptionTextView {
  id: string;
  name: string;
  format: SubscriptionFormat;
  revisionId: string;
  contents: string;
}

export interface NativeDialogLabels {
  title: string;
  filterName: string;
}

export interface SubscriptionDialogImportResult {
  cancelled: boolean;
  state: SubscriptionState;
}

export interface SubscriptionScreenImportResult {
  cancelled: boolean;
  errorCode?: string;
  state?: SubscriptionState;
}

export interface ProxyDelayCacheScope {
  subscriptionId: string;
  subscriptionRevision: string;
  localConfigId: string;
  localConfigRevision: number;
  localScriptId: string;
  localScriptRevision: number;
}

export type ProxyDelayStatus = "success" | "error";
export type ProxyDelaySource = "manual" | "mihomo";

export interface ProxyDelayCacheEntry {
  name: string;
  status: ProxyDelayStatus;
  delay: number;
  source: ProxyDelaySource;
  testedAt: string;
}

export interface ProxyDelayCacheUpdateEntry {
  name: string;
  status: ProxyDelayStatus;
  delay: number;
  source: ProxyDelaySource;
}

export interface ProxyDelayCacheSnapshot {
  scope: ProxyDelayCacheScope;
  results: ProxyDelayCacheEntry[];
  updatedAt: string;
}

export interface ProxyDelayCacheUpdate {
  scope: ProxyDelayCacheScope;
  results: ProxyDelayCacheUpdateEntry[];
}

export interface SubscriptionRefreshConflict {
  token: string;
  subscriptionId: string;
  localConfigId: string;
  localScriptId: string;
  message: string;
}
export interface SubscriptionRefreshResult {
  state: SubscriptionState | null;
  conflict: SubscriptionRefreshConflict | null;
}
