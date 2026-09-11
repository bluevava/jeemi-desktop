import { describe, expect, it } from "vitest";

import type { SubscriptionSelector } from "../../types/subscription";
import {
  delayTargetsForSearch,
  filterSelectorsByNodeName,
  filterSelectorsByName,
  hasNodeNameSearch,
} from "./selectorSearch";

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

describe("filterSelectorsByNodeName", () => {
  it("does not match selector names", () => {
    expect(filterSelectorsByNodeName([selector], "primary")).toEqual([]);
  });

  it("matches node names and keeps only matching nodes", () => {
    const result = filterSelectorsByNodeName([selector], "tokyo");
    expect(result).toHaveLength(1);
    expect(result[0].members.map((member) => member.name)).toEqual([
      "Tokyo node",
    ]);
  });

  it("does not match protocol types or provider names", () => {
    expect(filterSelectorsByNodeName([selector], "wireguard")).toEqual([]);
    expect(filterSelectorsByNodeName([selector], "provider-alpha")).toEqual([]);
  });

  it.each(["", " \n\t ", " | & ! "])(
    "keeps the unfiltered view when there are no keywords (%j)",
    (query) => {
      const selectors = [selector];
      expect(hasNodeNameSearch(query)).toBe(false);
      expect(filterSelectorsByNodeName(selectors, query)).toBe(selectors);
    },
  );

  it("does not include nested selectors or built-in actions in a node search", () => {
    const mixed: SubscriptionSelector = {
      ...selector,
      members: [
        ...selector.members,
        { name: "Tokyo group", type: "select", source: "group", providerName: "" },
        { name: "DIRECT", type: "direct", source: "builtin", providerName: "" },
      ],
    };
    expect(filterSelectorsByNodeName([mixed], "group|DIRECT")).toEqual([]);
    expect(
      filterSelectorsByNodeName([mixed], "!missing")[0].members,
    ).toEqual(selector.members);
  });

  it("preserves spaces inside a literal keyword", () => {
    expect(filterSelectorsByNodeName([selector], "  TOKYO node  ")[0].members)
      .toEqual([selector.members[0]]);
    expect(filterSelectorsByNodeName([selector], "Tokyo     node")).toEqual([]);
  });
});

const compoundSelector: SubscriptionSelector = {
  ...selector,
  name: "HK JP GM selector",
  members: [
    "HK GM 01",
    "JP gm 02",
    "HK GM EV 03",
    "JP GM ev 04",
    "HK 05",
    "US GM 06",
    "GM 07",
  ].map((name) => ({ name, type: "ss", source: "proxy", providerName: "" })),
};

describe("node conditions: union, intersection, then exclusion", () => {
  it.each([
    " hk | jp & gm & !ev",
    " hk& gm & !ev  | jp ",
    "hk|jp&gm&!ev",
    "hk     | jp         & gm  &   !   ev  ",
    "hk!ev|jp&gm",
    "hk!ev&gm|jp",
    "hk&gm|jp!ev",
    "hk|jp!ev&gm",
    "&gm!ev|hk|jp",
    "!ev&gm|jp|hk",
  ])("keeps the same result when labelled conditions move (%j)", (query) => {
    const result = filterSelectorsByNodeName([compoundSelector], query);
    expect(result.flatMap((group) => group.members.map((node) => node.name)))
      .toEqual(["HK GM 01", "JP gm 02"]);
  });

  it.each([
    ["hk|jp", ["HK GM 01", "JP gm 02", "HK GM EV 03", "JP GM ev 04", "HK 05"]],
    ["hk&gm", ["HK GM 01", "HK GM EV 03"]],
    ["&gm&jp!ev", ["JP gm 02"]],
    ["!ev!us", ["HK GM 01", "JP gm 02", "HK 05", "GM 07"]],
    ["hk|jp&gm&01!ev", ["HK GM 01"]],
    ["hk&gm!ev!01", []],
    ["hk|hk&gm&gm!ev!ev", ["HK GM 01"]],
    ["hk | ", ["HK GM 01", "HK GM EV 03", "HK 05"]],
  ])("combines single and multiple conditions (%j)", (query, expected) => {
    expect(
      filterSelectorsByNodeName([compoundSelector], query)
        .flatMap((group) => group.members.map((node) => node.name)),
    ).toEqual(expected);
  });
});

describe("batch delay targets for quick search", () => {
  const allTargets = ["Tokyo node", "London node", "Unlisted node"];

  it.each(["", " \n ", " | & ! "])("tests all nodes for an empty search (%j)", (query) => {
    expect(delayTargetsForSearch(allTargets, [], query)).toEqual(allTargets);
  });

  it("tests only matching node names, including provider members", () => {
    const query = "  TOKYO  ";
    expect(
      delayTargetsForSearch(
        allTargets,
        filterSelectorsByNodeName([selector], query),
        query,
      ),
    ).toEqual(["Tokyo node"]);
  });

  it("tests matched real nodes once, excluding groups and built-ins", () => {
    const query = "!missing";
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
        filterSelectorsByNodeName([withExtraMembers, selector], query),
        query,
      ),
    ).toEqual(["Tokyo node", "London node"]);
  });

  it("does not fall back to all nodes when the search has no matches", () => {
    const query = "missing";
    expect(
      delayTargetsForSearch(
        allTargets,
        filterSelectorsByNodeName([selector], query),
        query,
      ),
    ).toEqual([]);
  });

  it("keeps a started batch unchanged when the next search changes", () => {
    const started = delayTargetsForSearch(
      allTargets,
      filterSelectorsByNodeName([selector], "tokyo"),
      "tokyo",
    );
    expect(
      delayTargetsForSearch(
        allTargets,
        filterSelectorsByNodeName([selector], "london"),
        "london",
      ),
    ).toEqual(["London node"]);
    expect(started).toEqual(["Tokyo node"]);
    expect(selector.members).toHaveLength(2);
  });

  it("uses the compound search result and never reintroduces excluded nodes", () => {
    const query = "hk & gm & !ev | jp";
    expect(delayTargetsForSearch(
      compoundSelector.members.map((node) => node.name),
      filterSelectorsByNodeName([compoundSelector, compoundSelector], query),
      query,
    )).toEqual(["HK GM 01", "JP gm 02"]);
    expect(compoundSelector.members).toHaveLength(7);
  });
});

describe("selector name search", () => {
  const groups = ["Google Auto", "YouTube Auto", "Google Auto Backup", "YouTube Manual", "Other Auto"]
    .map((name) => ({ ...selector, name }));

  it.each([
    " Google | YouTube & Auto & !Backup ",
    "google&auto&!backup|youtube",
    "!backup & auto | google | youtube",
    "  google     |   youtube  &  auto ! backup  ",
  ])("reuses the same union, intersection and exclusion semantics (%j)", (query) => {
    const matched = filterSelectorsByName(groups, query);
    expect(matched).toEqual([groups[0], groups[1]]);
    expect(matched[0]).toBe(groups[0]);
    expect(matched[0].members).toBe(selector.members);
  });

  it("matches only group names, not nodes or protocols", () => {
    expect(filterSelectorsByName(groups, "Tokyo|wireguard|provider-alpha")).toEqual([]);
    expect(filterSelectorsByName(groups, "!backup")).toHaveLength(4);
    expect(filterSelectorsByName(groups, "&auto")).toHaveLength(4);
    expect(filterSelectorsByName(groups, " | & !  ")).toBe(groups);
  });

  it("composes with independent node search without restoring excluded groups or nodes", () => {
    const byNode = filterSelectorsByNodeName(groups, "Tokyo");
    expect(filterSelectorsByName(byNode, "YouTube !manual").map((item) => [item.name, item.members.map((node) => node.name)]))
      .toEqual([["YouTube Auto", ["Tokyo node"]]]);
    expect(delayTargetsForSearch(["Tokyo node", "London node"], byNode, "Tokyo")).toEqual(["Tokyo node"]);
  });
});
