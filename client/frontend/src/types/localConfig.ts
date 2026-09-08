export type MergeStrategy =
  | "replace"
  | "merge"
  | "prepend"
  | "append"
  | "merge_by_name"
  | "append_before_terminal";

export type ConflictPolicy = "error" | "use_local";

export type ConfigValueKind =
  | "scalar"
  | "mapping"
  | "sequence"
  | "named_mapping"
  | "named_sequence"
  | "rules";

export type ConfigEditorKind =
  | "boolean"
  | "number"
  | "string"
  | "enum"
  | "yaml";

export interface ConfigCatalogField {
  id: string;
  path: string;
  kind: ConfigValueKind;
  editor: ConfigEditorKind;
  options: string[];
  exampleYaml: string;
  strategies: MergeStrategy[];
  defaultStrategy: MergeStrategy | "";
  defaultConflictPolicy: ConflictPolicy | "";
  locked: boolean;
  sensitive: boolean;
  documentationUrl: string;
  hidden: boolean;
  scope: "" | "all_proxies";
  targetPath: string;
  applicableTypes: string[] | null;
}

export interface ConfigCatalogCategory {
  id: string;
  documentationUrl: string;
  fields: ConfigCatalogField[];
}

export interface ConfigCatalog {
  version: string;
  categories: ConfigCatalogCategory[];
}

export interface LocalConfigField {
  path: string;
  valueYaml: string;
  strategy: MergeStrategy;
  conflictPolicy: ConflictPolicy | "";
}

export interface StrategyGroupFilter {
  proxyTypes: string[];
  namePatterns: string[];
}

export interface StrategyGroupResource {
  id: string;
  name: string;
  description: string;
  kind: "rule" | "selector";
  ruleOutput: "rule-set" | "inline";
  ruleSetReferences: RuleSetReference[];
  policy: {
    mode: "" | "proxy" | "direct" | "reject";
  };
  type: "" | "select" | "url-test" | "fallback" | "load-balance";
  emoji: string;
  icon: string;
  url: string;
  interval: number;
  tolerance: number;
  strategy: "" | "round-robin" | "consistent-hashing" | "sticky-sessions";
  lazy: boolean;
  filter: StrategyGroupFilter;
  createdAt: string;
  updatedAt: string;
}

export interface RuleSetReference {
  ruleSetId: string;
}

export interface RuleSetResource {
  id: string;
  name: string;
  description: string;
  sourceType: "inline" | "http";
  behavior: "domain" | "ipcidr" | "classical";
  format: "yaml" | "text" | "mrs";
  url: string;
  interval: number;
  noResolve: boolean;
  payloadYaml: string;
  payload: string[];
  createdAt: string;
  updatedAt: string;
}

export interface LocalConfigResourceState {
  directory: string;
  revision: number;
  updatedAt: string;
  strategyGroups: StrategyGroupResource[];
  ruleSets: RuleSetResource[];
}

export interface LocalConfigResourcePlan {
  version: number;
  match: { mode: "none" | "direct" | "selector"; selectorId: string };
  strategyGroupIds: string[];
  disabledStrategyGroupIds: string[];
  rulesDisabled: boolean;
  ruleStrategy: "prepend" | "append_before_terminal" | "replace";
  defaultProxySelectorId: string;
}

export function defaultLocalConfigResourcePlan(): LocalConfigResourcePlan {
  return {
    version: 1,
    match: { mode: "none", selectorId: "" },
    strategyGroupIds: [],
    disabledStrategyGroupIds: [],
    rulesDisabled: false,
    ruleStrategy: "append_before_terminal",
    defaultProxySelectorId: "",
  };
}

export interface SaveLocalConfigInput {
  id: string;
  name: string;
  description: string;
  fields: LocalConfigField[];
  resourcePlan: LocalConfigResourcePlan;
}

export interface LocalConfigSummary {
  id: string;
  name: string;
  description: string;
  schemaVersion: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
  enabledFieldCount: number;
  ruleProviderCount: number;
  strategyGroupCount: number;
  staticValidationStatus: "static_passed" | string;
}

export interface LocalConfig extends LocalConfigSummary {
  fields: LocalConfigField[];
  overlayYaml: string;
  resourcePlan: LocalConfigResourcePlan;
}

export interface LocalConfigState {
  directory: string;
  configs: LocalConfigSummary[];
}

export interface LocalConfigPreview {
  schemaVersion: string;
  enabledFieldCount: number;
  proxyOverrideCount: number;
  overlayYaml: string;
  redactedOverlayYaml: string;
  mergePlan: Array<{
    path: string;
    strategy: MergeStrategy;
    conflictPolicy: ConflictPolicy | "";
  }>;
  fields: LocalConfigField[];
  resourcePlan: LocalConfigResourcePlan;
}
