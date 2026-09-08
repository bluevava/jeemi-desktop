import { afterEach, describe, expect, it, vi } from "vitest";

import {
  parseWindowActivity,
  subscribeWindowActivity,
  windowActivityEvent,
} from "./windowBridge";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("window activity events", () => {
  it("accepts only the two native lifecycle states", () => {
    expect(parseWindowActivity("visible")).toBe("visible");
    expect(parseWindowActivity("hidden")).toBe("hidden");
    expect(parseWindowActivity(true)).toBeNull();
    expect(parseWindowActivity("background")).toBeNull();
  });

  it("subscribes to the native event and ignores malformed payloads", () => {
    const callbackRef: { current?: (value: unknown) => void } = {};
    const unsubscribe = vi.fn();
    const eventsOn = vi.fn(
      (eventName: string, listener: (value: unknown) => void) => {
        expect(eventName).toBe(windowActivityEvent);
        callbackRef.current = listener;
        return unsubscribe;
      },
    );
    vi.stubGlobal("window", { runtime: { EventsOn: eventsOn } });
    const listener = vi.fn();

    const stop = subscribeWindowActivity(listener);
    expect(callbackRef.current).toBeTypeOf("function");
    callbackRef.current!("hidden");
    callbackRef.current!(false);
    callbackRef.current!("visible");

    expect(listener.mock.calls).toEqual([["hidden"], ["visible"]]);
    stop();
    expect(unsubscribe).toHaveBeenCalledOnce();
  });
});
