import { describe, expect, it } from "vitest";

import { setSelectorExpanded } from "./SubscriptionPageStateContext";

describe("subscription page session state", () => {
  it("keeps expanded selectors isolated by subscription workspace", () => {
    const first = setSelectorExpanded({}, "subscription-a:rule", "Proxy", true);
    const second = setSelectorExpanded(
      first,
      "subscription-b:rule",
      "Automatic",
      true,
    );

    expect(second).toEqual({
      "subscription-a:rule": ["Proxy"],
      "subscription-b:rule": ["Automatic"],
    });
    expect(
      setSelectorExpanded(second, "subscription-a:rule", "Proxy", false),
    ).toEqual({ "subscription-b:rule": ["Automatic"] });
  });

  it("returns the existing state for an unchanged toggle", () => {
    const state = { "subscription-a:rule": ["Proxy"] };
    expect(
      setSelectorExpanded(state, "subscription-a:rule", "Proxy", true),
    ).toBe(state);
  });
});
