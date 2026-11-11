import type {
  LocalConfigResourcePlan,
  LocalConfigResourceState,
  StrategyGroupResource,
} from "../../types/localConfig";

export function strategyGroupEnabled(
  plan: LocalConfigResourcePlan,
  group: StrategyGroupResource,
): boolean {
  return !plan.rulesDisabled && !group.disabled &&
    !plan.disabledStrategyGroupIds.includes(group.id);
}

export function requiresProxySelector(
  plan: LocalConfigResourcePlan,
  resources: LocalConfigResourceState,
): boolean {
  return resources.strategyGroups.some((group) =>
    plan.strategyGroupIds.includes(group.id) &&
    strategyGroupEnabled(plan, group) &&
    group.kind === "rule" && group.policy.mode === "proxy" &&
    group.ruleSetReferences.some((reference) =>
      !resources.ruleSets.find((rule) => rule.id === reference.ruleSetId)?.disabled,
    ),
  );
}

export function localMatchStatus(
  plan: LocalConfigResourcePlan,
  resources: LocalConfigResourceState,
) {
  const selected = resources.strategyGroups.find((group) =>
    group.kind === "selector" && group.id === plan.match.selectorId &&
    plan.strategyGroupIds.includes(group.id),
  );
  const missingSelector = plan.match.mode === "selector" &&
    (!selected || !strategyGroupEnabled(plan, selected));
  const needsMatch = plan.ruleStrategy === "replace" && plan.match.mode === "none";
  return { selected, missingSelector, needsMatch,
    invalid: !plan.rulesDisabled && (missingSelector || needsMatch) };
}
