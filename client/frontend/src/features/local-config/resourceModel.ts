import type {
  LocalConfigResourceState,
  RuleSetResource,
  StrategyGroupResource,
} from "../../types/localConfig";
import { DEFAULT_SELECTOR_TEST_URL } from "./strategyGroupEmoji";

export function emptyStrategyGroup(
  kind: "rule" | "selector" = "selector",
): StrategyGroupResource {
  return {
    id: "",
    name: "",
    description: "",
    kind,
    ruleOutput: "rule-set",
    ruleSetReferences: [],
    policy: { mode: kind === "rule" ? "direct" : "" },
    type: kind === "rule" ? "" : "select",
    emoji: "",
    icon: "",
    url: DEFAULT_SELECTOR_TEST_URL,
    interval: 300,
    tolerance: 50,
    strategy: "consistent-hashing",
    lazy: true,
    filter: { proxyTypes: [], namePatterns: [] },
    createdAt: "",
    updatedAt: "",
  };
}

export function emptyRuleSet(): RuleSetResource {
  return {
    id: "",
    name: "",
    description: "",
    sourceType: "inline",
    behavior: "classical",
    format: "yaml",
    url: "",
    interval: 86400,
    noResolve: false,
    payloadYaml: "",
    payload: [],
    createdAt: "",
    updatedAt: "",
  };
}

export function cloneStrategyGroup(
  item: StrategyGroupResource,
): StrategyGroupResource {
  return {
    ...item,
    ruleSetReferences: item.ruleSetReferences.map((reference) => ({
      ...reference,
    })),
    policy: { ...item.policy },
    filter: {
      proxyTypes: [...item.filter.proxyTypes],
      namePatterns: [...item.filter.namePatterns],
    },
  };
}

export function isRemoteRuleSet(
  id: string,
  state: LocalConfigResourceState,
): boolean {
  return state.ruleSets.find((item) => item.id === id)?.sourceType === "http";
}

export function canSaveStrategyGroup(
  draft: StrategyGroupResource,
  state: LocalConfigResourceState,
): boolean {
  if (!draft.name.trim()) {
    return false;
  }
  if (
    draft.ruleOutput === "inline" &&
    draft.ruleSetReferences.some((reference) =>
      isRemoteRuleSet(reference.ruleSetId, state),
    )
  ) {
    return false;
  }
  if (
    draft.ruleSetReferences.some((reference) =>
      state.strategyGroups.some(
        (group) =>
          group.id !== draft.id &&
          group.ruleSetReferences.some(
            (candidate) => candidate.ruleSetId === reference.ruleSetId,
          ),
      ),
    )
  ) {
    return false;
  }
  if (draft.kind === "selector") {
    return draft.type !== "";
  }
  return (
    draft.ruleSetReferences.length > 0 &&
    (draft.policy.mode === "direct" ||
      draft.policy.mode === "reject" ||
      draft.policy.mode === "proxy")
  );
}

export function policyLabel(
  item: StrategyGroupResource,
  translate: (key: string) => string,
): string {
  if (item.policy.mode === "proxy") {
    return translate("localConfig.resources.groups.policyProxy");
  }
  if (item.policy.mode === "direct") {
    return translate("localConfig.resources.groups.policyDirect");
  }
  if (item.policy.mode === "reject") {
    return translate("localConfig.resources.groups.policyReject");
  }
  return item.policy.mode;
}

export function cloneRuleSet(item: RuleSetResource): RuleSetResource {
  return {
    ...item,
    payload: [...item.payload],
  };
}

export function strategyGroupDisplayName(item: StrategyGroupResource): string {
  const name = item.name.trim();
  return item.emoji ? `${item.emoji} ${name}` : name;
}

export function canSaveRuleSet(draft: RuleSetResource): boolean {
  if (!draft.name.trim()) {
    return false;
  }
  if (draft.sourceType === "http") {
    return /^https?:\/\//iu.test(draft.url.trim());
  }
  return draft.payloadYaml.trim() !== "";
}

export function lines(value: string): string[] {
  return value
    .split(/\r?\n/u)
    .map((line) => line.trim())
    .filter(Boolean);
}

export function formatDate(value: string, language: string): string {
  if (!value) {
    return "";
  }
  return new Intl.DateTimeFormat(language, {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value));
}
