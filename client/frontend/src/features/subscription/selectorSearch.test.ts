import { describe, expect, it } from "vitest";

import type { SubscriptionSelector } from "../../types/subscription";
import { delayTargetsForSearch, filterSelectorsByName } from "./selectorSearch";

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

describe("batch delay targets for quick search", () => {
  const allTargets = ["Tokyo node", "London node", "Unlisted node"];

  it.each(["", " \n "])("tests all nodes for an empty search (%j)", (query) => {
    expect(delayTargetsForSearch(allTargets, [], query)).toEqual(allTargets);
  });

  it("tests only matching node names, including provider members", () => {
    const query = "  TOKYO  ";
    expect(
      delayTargetsForSearch(
        allTargets,
        filterSelectorsByName([selector], query),
        query,
      ),
    ).toEqual(["Tokyo node"]);
  });

  it("tests a matched selector's real nodes once, excluding groups and built-ins", () => {
    const query = "primary";
    const withExtraMembers: SubscriptionSelector = {
      ...selector,
      members: [
        ...selector.members,
        {
          name: "Nested group",
          type: "select",
          source: "group",
          providerName: "",
        },
        { name: "DIRECT", type: "direct", source: "builtin", providerName: "" },
      ],
    };
    expect(
      delayTargetsForSearch(
        allTargets,
        filterSelectorsByName([withExtraMembers, selector], query),
        query,
      ),
    ).toEqual(["Tokyo node", "London node"]);
  });

  it("does not fall back to all nodes when the search has no matches", () => {
    const query = "missing";
    expect(
      delayTargetsForSearch(
        allTargets,
        filterSelectorsByName([selector], query),
        query,
      ),
    ).toEqual([]);
  });

  it("keeps a started batch unchanged when the next search changes", () => {
    const started = delayTargetsForSearch(
      allTargets,
      filterSelectorsByName([selector], "tokyo"),
      "tokyo",
    );
    expect(
      delayTargetsForSearch(
        allTargets,
        filterSelectorsByName([selector], "london"),
        "london",
      ),
    ).toEqual(["London node"]);
    expect(started).toEqual(["Tokyo node"]);
    expect(selector.members).toHaveLength(2);
  });
});
