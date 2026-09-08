import { describe, expect, it } from "vitest";

import {
  formatMemoryBytes,
  platformFailureMessageKey,
  runtimeUptime,
  shouldShowMihomoUptime,
} from "./runtimePresentation";

describe("formatMemoryBytes", () => {
  it("formats process memory with binary units", () => {
    expect(formatMemoryBytes(0, "en-US")).toBe("0 B");
    expect(formatMemoryBytes(18 * 1024 * 1024, "en-US")).toBe("18 MiB");
    expect(formatMemoryBytes(1536 * 1024, "en-US")).toBe("1.5 MiB");
  });

  it("keeps unavailable measurements explicit", () => {
    expect(formatMemoryBytes(null, "en-US")).toBe("—");
    expect(formatMemoryBytes(-1, "en-US")).toBe("—");
  });
});

describe("runtimeUptime", () => {
  const startedAt = "2026-09-04T00:00:00Z";
  const after = (seconds: number) => Date.parse(startedAt) + seconds * 1000;

  it("selects a compact elapsed-time unit", () => {
    expect(runtimeUptime(startedAt, after(42))).toEqual({
      value: 42,
      unit: "second",
    });
    expect(runtimeUptime(startedAt, after(90))).toEqual({
      value: 1,
      unit: "minute",
    });
    expect(runtimeUptime(startedAt, after(3 * 60 * 60))).toEqual({
      value: 3,
      unit: "hour",
    });
    expect(runtimeUptime(startedAt, after(2 * 24 * 60 * 60))).toEqual({
      value: 2,
      unit: "day",
    });
  });

  it("handles clock skew and unavailable timestamps", () => {
    expect(runtimeUptime(startedAt, after(-10))).toEqual({
      value: 0,
      unit: "second",
    });
    expect(runtimeUptime("", after(10))).toBeNull();
    expect(runtimeUptime(startedAt, Number.NaN)).toBeNull();
  });
});

describe("shouldShowMihomoUptime", () => {
  it("shows only for a healthy, stable running process", () => {
    expect(
      shouldShowMihomoUptime({
        state: "running",
        controllerReady: true,
        transitioning: false,
        hasError: false,
      }),
    ).toBe(true);

    for (const input of [
      {
        state: "stopped" as const,
        controllerReady: false,
        transitioning: false,
        hasError: false,
      },
      {
        state: "running" as const,
        controllerReady: false,
        transitioning: false,
        hasError: false,
      },
      {
        state: "running" as const,
        controllerReady: true,
        transitioning: true,
        hasError: false,
      },
      {
        state: "running" as const,
        controllerReady: true,
        transitioning: false,
        hasError: true,
      },
    ]) {
      expect(shouldShowMihomoUptime(input)).toBe(false);
    }
  });
});

describe("macOS authorization diagnostics", () => {
  it("maps release prerequisites and network failures without exposing native text", () => {
    expect(platformFailureMessageKey("macos_network_signing_required")).toBe("macNetwork.status.signing_required");
    expect(platformFailureMessageKey("macos_network_recovery_failed")).toBe("macNetwork.status.recovery_failed");
    expect(platformFailureMessageKey("macos_network_future_error")).toBe("macNetwork.status.helper_failed");
  });
});
