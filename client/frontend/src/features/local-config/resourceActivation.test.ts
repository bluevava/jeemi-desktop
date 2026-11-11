import { describe, expect, it } from "vitest";
import {
  defaultLocalConfigResourcePlan,
  type LocalConfigResourceState,
} from "../../types/localConfig";
import { emptyRuleSet, emptyStrategyGroup } from "./resourceModel";
import { localMatchStatus, requiresProxySelector, strategyGroupEnabled } from "./resourceActivation";

function fixture() {
  const selector = { ...emptyStrategyGroup(), id: "selector", name: "Jeemi", disabled: false };
  const rule = { ...emptyRuleSet(), id: "rule" };
  const group = {
    ...emptyStrategyGroup("rule"), id: "group", disabled: false,
    policy: { mode: "proxy" as const }, ruleSetReferences: [{ ruleSetId: rule.id }],
  };
  const resources: LocalConfigResourceState = {
    directory: "", revision: 1, updatedAt: "",
    strategyGroups: [selector, group], ruleSets: [rule],
  };
  const plan = defaultLocalConfigResourcePlan();
  plan.strategyGroupIds = [selector.id, group.id];
  plan.defaultProxySelectorId = selector.id;
  plan.ruleStrategy = "replace";
  plan.match = { mode: "selector", selectorId: selector.id };
  return { selector, rule, group, resources, plan };
}

describe("resource activation in the configuration editor", () => {
  it("rejects a shared-disabled MATCH selector under the first-release version", () => {
    const { selector, resources, plan } = fixture();
    expect(plan.version).toBe(1);
    expect(localMatchStatus(plan, resources).invalid).toBe(false);
    selector.disabled = true;
    expect(strategyGroupEnabled(plan, selector)).toBe(false);
    expect(localMatchStatus(plan, resources).invalid).toBe(true);
    expect(plan.match.selectorId).toBe(selector.id);
    selector.disabled = false;
    expect(localMatchStatus(plan, resources).invalid).toBe(false);
  });

  it("keeps the local reference switch effective after the shared switch is restored", () => {
    const { selector, resources, plan } = fixture();
    plan.disabledStrategyGroupIds = [selector.id];
    selector.disabled = true;
    selector.disabled = false;
    expect(strategyGroupEnabled(plan, selector)).toBe(false);
    expect(localMatchStatus(plan, resources).invalid).toBe(true);
    plan.rulesDisabled = true;
    expect(localMatchStatus(plan, resources).invalid).toBe(false);
    expect(requiresProxySelector(plan, resources)).toBe(false);
  });

  it("only requires a default proxy for groups with enabled rule sets", () => {
    const { rule, group, resources, plan } = fixture();
    expect(requiresProxySelector(plan, resources)).toBe(true);
    rule.disabled = true;
    expect(requiresProxySelector(plan, resources)).toBe(false);
    rule.disabled = false;
    group.disabled = true;
    expect(requiresProxySelector(plan, resources)).toBe(false);
    group.disabled = false;
    expect(requiresProxySelector(plan, resources)).toBe(true);
  });

  it("reports a missing replacement MATCH without an obsolete version gate", () => {
    const { resources, plan } = fixture();
    plan.match = { mode: "none", selectorId: "" };
    expect(localMatchStatus(plan, resources)).toMatchObject({ invalid: true, needsMatch: true });
    plan.match = { mode: "direct", selectorId: "" };
    expect(localMatchStatus(plan, resources).invalid).toBe(false);
  });
});
