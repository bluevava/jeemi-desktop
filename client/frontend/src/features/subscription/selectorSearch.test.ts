import { describe, expect, it } from "vitest";

import type { SubscriptionSelector } from "../../types/subscription";
import { filterSelectorsByName } from "./selectorSearch";

const selector: SubscriptionSelector = {
  name: "Primary group",
  icon: "",
  hidden: false,
  type: "select",
  defaultSelection: "Tokyo node",
  members: [
    {
      name: "Tokyo node",
      type: "ss",
      source: "provider",
      providerName: "provider-alpha",
    },
    {
      name: "London node",
      type: "wireguard",
      source: "proxy",
      providerName: "",
    },
  ],
  providerNames: ["provider-alpha"],
  unresolvedProviderNames: [],
  referencedByRules: true,
};

describe("filterSelectorsByName", () => {
  it("matches selector names and retains all of their nodes", () => {
    expect(filterSelectorsByName([selector], "primary")).toEqual([selector]);
  });

  it("matches node names and keeps only matching nodes", () => {
    const result = filterSelectorsByName([selector], "tokyo");
    expect(result).toHaveLength(1);
    expect(result[0].members.map((member) => member.name)).toEqual([
      "Tokyo node",
    ]);
  });

  it("does not match protocol types or provider names", () => {
    expect(filterSelectorsByName([selector], "wireguard")).toEqual([]);
    expect(filterSelectorsByName([selector], "provider-alpha")).toEqual([]);
  });
});
