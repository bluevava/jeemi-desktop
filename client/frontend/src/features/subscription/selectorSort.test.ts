import { describe, expect, it } from "vitest";

import type { SubscriptionSelectorMember } from "../../types/subscription";
import { sortSelectorMembers } from "./selectorSort";

const members: SubscriptionSelectorMember[] = [
  { name: "Node 10", type: "ss", source: "proxy", providerName: "" },
  { name: "Node 2", type: "ss", source: "proxy", providerName: "" },
  { name: "Node A", type: "ss", source: "proxy", providerName: "" },
  { name: "Node B", type: "ss", source: "proxy", providerName: "" },
];

describe("selector node sorting", () => {
  it("preserves configuration order by default and uses natural name order", () => {
    expect(
      sortSelectorMembers(members, "default", {}, "en").map(
        (item) => item.name,
      ),
    ).toEqual(["Node 10", "Node 2", "Node A", "Node B"]);
    expect(
      sortSelectorMembers(members, "name", {}, "en").map(
        (item) => item.name,
      ),
    ).toEqual(["Node 2", "Node 10", "Node A", "Node B"]);
  });

  it("places measured delays first, unmeasured nodes next, and failures last", () => {
    const results = {
      "Node 10": {
        status: "success" as const,
        delay: 180,
        source: "manual" as const,
        testedAt: "now",
      },
      "Node 2": {
        status: "success" as const,
        delay: 40,
        source: "manual" as const,
        testedAt: "now",
      },
      "Node B": {
        status: "error" as const,
        delay: 0 as const,
        source: "manual" as const,
        testedAt: "now",
      },
    };
    expect(
      sortSelectorMembers(members, "delay", results, "en").map(
        (item) => item.name,
      ),
    ).toEqual(["Node 2", "Node 10", "Node A", "Node B"]);
  });
});
