import { afterEach, describe, expect, it, vi } from "vitest";
import { subscribeSubscriptionUpdates } from "./subscriptionUpdateBridge";

afterEach(() => vi.unstubAllGlobals());

describe("subscription background update notifications", () => {
  it("subscribes to invalidations and releases the native listener", () => {
    const unsubscribe = vi.fn();
    let handler: (() => void) | undefined;
    const eventsOn = vi.fn((event: string, callback: () => void) => {
      expect(event).toBe("jeemi:subscription-updated");
      handler = callback;
      return unsubscribe;
    });
    vi.stubGlobal("window", { runtime: { EventsOn: eventsOn } });
    const listener = vi.fn();
    const stop = subscribeSubscriptionUpdates(listener);
    handler?.();
    expect(listener).toHaveBeenCalledOnce();
    stop();
    expect(unsubscribe).toHaveBeenCalledOnce();
  });

  it("keeps component previews usable without the desktop runtime", () => {
    vi.stubGlobal("window", {});
    expect(() => subscribeSubscriptionUpdates(vi.fn())()).not.toThrow();
  });
});
