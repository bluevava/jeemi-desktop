export const ruleMatchTypes = [
  "domain",
  "ip",
  "processName",
  "processPath",
] as const;
export type RuleMatchType = (typeof ruleMatchTypes)[number];

export interface RuleSetEntryInput {
  ruleSetId: string;
  name: string;
  matchType: RuleMatchType;
  value: string;
  caseInsensitive: boolean;
  expectedRevision: number;
}

export interface RuleSetEntryPreview {
  valid: boolean;
  line: string;
  yamlLine: string;
  errorCode: string;
}

export const ruleSetEntryErrorCodes = [
  "resourcesChanged",
  "missingRuleSet",
  "invalidName",
  "duplicateName",
  "remoteRuleSet",
  "incompatibleBehavior",
  "invalidPayload",
  "invalidValue",
  "invalidDomain",
  "invalidIP",
  "invalidProcessName",
  "invalidMatchType",
] as const;
