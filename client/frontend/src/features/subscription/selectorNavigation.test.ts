import { describe, expect, it } from "vitest";
import type { SubscriptionSelector } from "../../types/subscription";
import { resolveSelectorPath, selectorMemberCounts, selectorSelection } from "./selectorNavigation";

function group(name: string, children: string[] = [], hidden = false): SubscriptionSelector {
  return {
    name, type: "select", icon: "", hidden, defaultSelection: children[0] ?? "",
    providerNames: [], unresolvedProviderNames: [], referencedByRules: false,
    members: children.map((name) => ({ name, type: "group", source: "group", providerName: "" })),
  };
}
const tiktok = group("TikTok", ["Japan"]);
const japan = group("Japan", ["Automatic"]);
const automatic = { ...group("Automatic", [], true), type: "url-test" };
const index = new Map([tiktok, japan, automatic].map((item) => [item.name, item]));

describe("nested selector navigation", () => {
  it("follows real references into hidden groups without flattening them", () => {
    expect(resolveSelectorPath(tiktok, ["Japan", "Automatic"], index)).toEqual([tiktok, japan, automatic]);
  });
  it("stops at removed groups and unrelated references after refresh", () => {
    expect(resolveSelectorPath(tiktok, ["Automatic"], index)).toEqual([tiktok]);
    expect(resolveSelectorPath(tiktok, ["Japan", "Removed"], index)).toEqual([tiktok, japan]);
  });
  it("does not follow cycles or a node with the same name as a group", () => {
    const cyclicJapan = group("Japan", ["TikTok"]);
    expect(resolveSelectorPath(tiktok, ["Japan", "TikTok", "Japan"], new Map(index).set("Japan", cyclicJapan)))
      .toEqual([tiktok, cyclicJapan]);
    const nodeRoot = { ...tiktok, members: [{ name: "Japan", source: "proxy" as const, type: "ss", providerName: "" }] };
    expect(resolveSelectorPath(nodeRoot, ["Japan"], index)).toEqual([nodeRoot]);
  });
  it("separates member counts and never presents an offline default as live state", () => {
    const mixed = { ...tiktok, members: [...tiktok.members,
      { name: "Node", source: "proxy" as const, type: "ss", providerName: "" },
      { name: "DIRECT", source: "builtin" as const, type: "direct", providerName: "" },
    ] };
    expect(selectorMemberCounts(mixed)).toEqual({ count: 3, groups: 1, nodes: 1, actions: 1 });
    expect(selectorSelection(tiktok, {}, false)).toBe("Japan");
    expect(selectorSelection(tiktok, {}, true)).toBe("");
    expect(selectorSelection(tiktok, { TikTok: "Japan" }, true)).toBe("Japan");
    expect(selectorSelection(tiktok, { GLOBAL: "Node" }, true, "GLOBAL")).toBe("Node");
  });
});
