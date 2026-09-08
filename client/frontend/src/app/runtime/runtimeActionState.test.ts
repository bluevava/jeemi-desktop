import { describe, expect, it } from "vitest";

import {
  runtimeActionDisplayState,
  runtimeActionReachedTarget,
  runtimePrimaryAction,
} from "./runtimeActionState";

describe("runtime action presentation", () => {
  it("shows the requested transition before the next runtime snapshot arrives", () => {
    expect(runtimeActionDisplayState("running", "stop")).toBe("stopping");
    expect(runtimeActionDisplayState("stopped", "start")).toBe("starting");
    expect(runtimeActionDisplayState("running", "restart")).toBe("starting");
    expect(runtimeActionDisplayState("ready", null)).toBe("ready");
  });

  it("clears a failed action only after its requested outcome is observed", () => {
    expect(runtimeActionReachedTarget("stop", "stopped")).toBe(true);
    expect(runtimeActionReachedTarget("stop", "running")).toBe(false);
    expect(runtimeActionReachedTarget("start", "running")).toBe(true);
    expect(runtimeActionReachedTarget("restart", "failed")).toBe(false);
  });

  it("keeps the title bar primary action aligned with the homepage", () => {
    expect(runtimePrimaryAction("stopped", false)).toBe("start");
    expect(runtimePrimaryAction("running", true)).toBe("restart");
    expect(runtimePrimaryAction("failed", true)).toBe("restart");
    expect(runtimePrimaryAction("failed", false)).toBe("start");
  });
});
