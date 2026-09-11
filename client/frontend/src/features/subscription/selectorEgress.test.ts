import { describe, expect, it } from "vitest";
import type { SubscriptionSelector, SubscriptionSelectorMember } from "../../types/subscription";
import { createSelectorEgressDescription, resolveSelectorEgress } from "./selectorEgress";

function group(name: string, selected: string, source: SubscriptionSelectorMember["source"] = "group"): SubscriptionSelector {
  return {
    name, type: "select", icon: "", hidden: false, defaultSelection: selected,
    members: selected ? [{ name: selected, source, type: source, providerName: "" }] : [],
    providerNames: [], unresolvedProviderNames: [], referencedByRules: true,
  };
}
const google = group("Google", "Manual");
const manual = group("Manual", "Japan");
const japan = { ...group("Japan", "JP default", "proxy"), hidden: true };
const index = new Map([google, manual, japan].map((item) => [item.name, item]));
const live = { Google: "Manual", Manual: "Japan", Japan: "JP live" };

describe("selected egress paths", () => {
  it("follows every selected group, including hidden groups, to the live provider node", () => {
    expect(resolveSelectorEgress(google, index, live, true))
      .toEqual({ names: ["Manual", "Japan", "JP live"], end: "node" });
    expect(resolveSelectorEgress(manual, index, live, true))
      .toEqual({ names: ["Japan", "JP live"], end: "node" });
  });

  it("uses configuration defaults offline and never substitutes defaults for missing live data", () => {
    expect(resolveSelectorEgress(google, index, live, false))
      .toEqual({ names: ["Manual", "Japan", "JP default"], end: "node" });
    expect(resolveSelectorEgress(google, index, { Google: "Manual" }, true))
      .toEqual({ names: ["Manual"], end: "unavailable" });
  });

  it("ignores search-filtered members when resolving a header and preserves built-in exits", () => {
    expect(resolveSelectorEgress({ ...google, members: [] }, index, live, false).names)
      .toEqual(["Manual", "Japan", "JP default"]);
    const direct = group("Direct group", "DIRECT", "builtin");
    expect(resolveSelectorEgress(direct, new Map(), {}, false))
      .toEqual({ names: ["DIRECT"], end: "node" });
  });

  it.each(["url-test", "fallback"])("follows %s using its actual selection", (type) => {
    const automatic = { ...japan, type };
    expect(resolveSelectorEgress(google, new Map(index).set("Japan", automatic), live, true).names)
      .toEqual(["Manual", "Japan", "JP live"]);
  });

  it.each([["load-balance", "balanced"], ["relay", "relay"]] as const)(
    "does not invent a fixed exit for %s", (type, end) => {
      const child = { ...manual, type };
      expect(resolveSelectorEgress(google, new Map(index).set("Manual", child), live, true))
        .toEqual({ names: ["Manual"], end });
    },
  );

  it("terminates cycles and missing group references without presenting them as nodes", () => {
    const cycle = new Map(index).set("Manual", group("Manual", "Google"));
    expect(resolveSelectorEgress(google, cycle, {}, false))
      .toEqual({ names: ["Manual", "Google"], end: "cycle" });
    expect(resolveSelectorEgress(google, new Map(), live, true))
      .toEqual({ names: ["Manual"], end: "unavailable" });
    expect(resolveSelectorEgress(group("Missing", "Removed"), new Map(), {}, false).end)
      .toBe("unavailable");
  });

  it("does not follow a real node even if a stale group shares its name", () => {
    const node = group("Root", "Japan", "proxy");
    expect(resolveSelectorEgress(node, index, { Root: "Japan" }, true))
      .toEqual({ names: ["Japan"], end: "node" });
  });

  it("resolves the GLOBAL runtime name separately from its translated display name", () => {
    const global = group("🌐 全局代理", "Offline", "proxy");
    expect(resolveSelectorEgress(global, index, { ...live, GLOBAL: "Google" }, true, "GLOBAL").names)
      .toEqual(["Google", "Manual", "Japan", "JP live"]);
  });

  it("formats shared paths consistently and refreshes them when selections change", () => {
    const labels = { balanced: "按连接分配出口", relay: "顺序中继", unavailable: "—", cycle: "循环引用" };
    const describe = createSelectorEgressDescription(index, live, true, labels);
    expect(describe(google)).toBe("Manual · Japan · JP live");
    expect(describe(manual)).toBe("Japan · JP live");
    expect(createSelectorEgressDescription(index, { ...live, Japan: "JP new" }, true, labels)(google))
      .toBe("Manual · Japan · JP new");
  });

  it("traverses a deep configuration without recursive stack growth", () => {
    const groups = Array.from({ length: 2000 }, (_, i) => group(`Group ${i}`, `Group ${i + 1}`));
    groups.push(group("Group 2000", "Exit", "proxy"));
    const result = resolveSelectorEgress(groups[0], new Map(groups.map((item) => [item.name, item])), {}, false);
    expect(result.end).toBe("node");
    expect(result.names).toHaveLength(2001);
    expect(result.names.at(-1)).toBe("Exit");
  });
});
