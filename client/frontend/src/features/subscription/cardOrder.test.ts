import { describe, expect, it } from "vitest";
import { applySubscriptionOrder, moveSubscriptionCard } from "./cardOrder";

describe("subscription card order", () => {
  it("moves a card forward and backward without changing other relative positions", () => {
    const ids = ["a", "b", "c", "d"];
    expect(moveSubscriptionCard(ids, "a", "c")).toEqual(["b", "c", "a", "d"]);
    expect(moveSubscriptionCard(ids, "d", "b")).toEqual(["a", "d", "b", "c"]);
    expect(ids).toEqual(["a", "b", "c", "d"]);
  });
  it("ignores cancellation, same-card drops, and stale or external IDs", () => {
    expect(moveSubscriptionCard(["a", "b"], "a", "a")).toBeNull();
    expect(moveSubscriptionCard(["a", "b"], "external", "b")).toBeNull();
    expect(moveSubscriptionCard(["a", "b"], "a", "deleted")).toBeNull();
  });
  it("applies a confirmed order while keeping fresh data and new cards at the end", () => {
    const first = { id: "a", refreshed: true };
    const second = { id: "b", refreshed: false };
    const added = { id: "new", refreshed: false };
    const next = applySubscriptionOrder([first, second, added], ["b", "deleted", "a"]);
    expect(next).toEqual([second, first, added]);
    expect(next[1]).toBe(first);
  });
});
