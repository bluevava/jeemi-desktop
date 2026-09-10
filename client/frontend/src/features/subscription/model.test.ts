import { describe, expect, it } from "vitest";

import type { LocalConfigSummary } from "../../types/localConfig";
import type { SubscriptionSummary } from "../../types/subscription";
import {
  associatedLocalConfigName,
  formatSubscriptionBytes,
  formatSubscriptionExpiry,
  formatSubscriptionUpdateAge,
  subscriptionUsagePercent,
  subscriptionUsedBytes,
} from "./model";

const subscription: SubscriptionSummary = {
  chainProxyGroupIds: [],
  chainProxyRevision: 0,
  id: "abcdef0123456789abcdef0123456789",
  name: "Test",
  description: "",
  sourceKind: "url",
  importMethod: "url",
  sourceLabel: "https://example.com",
  format: "yaml",
  iconKind: "",
  icon: "",
  localConfigId: "0123456789abcdef0123456789abcdef",
  localScriptId: "",
  ruleProviderOverrideRevision: 0,
  fallbackOverrideRevision: 0,
  fallback: { mode: "none", selector: "" },
  fallbackResetTarget: "",
  disabledRuleProviders: [],
  currentRevisionId: "a".repeat(64),
  revisionCount: 1,
  sizeBytes: 1536,
  createdAt: "2026-09-02T10:00:00Z",
  updatedAt: "2026-09-02T10:00:00Z",
  lastFetchedAt: "2026-09-02T10:00:00Z",
  remoteProfile: {
    hasTraffic: true,
    uploadBytes: 1024,
    downloadBytes: 2048,
    totalBytes: 8192,
    expiresAt: 1893456000,
    updateIntervalHours: 24,
    observedAt: "2026-09-02T10:00:00Z",
  },
  composition: {
    status: "ready",
    proxyCount: 3,
    selectorCount: 1,
    proxyProviderCount: 0,
    ruleProviderCount: 2,
  },
};

it("formats raw subscription size without changing its value", () => {
  expect(formatSubscriptionBytes(512, "en-US")).toBe("512 B");
  expect(formatSubscriptionBytes(1536, "en-US")).toBe("1.5 KiB");
});

it("derives bounded traffic and relative-time card values", () => {
  expect(subscriptionUsedBytes(subscription)).toBe(3072);
  expect(subscriptionUsagePercent(subscription)).toBe(37.5);
  expect(
    formatSubscriptionUpdateAge(
      subscription.lastFetchedAt,
      "en-US",
      Date.parse("2026-09-02T12:00:00Z"),
    ),
  ).toBe("2 hours ago");
  expect(formatSubscriptionExpiry(0, "en-US")).toBeNull();
  expect(formatSubscriptionExpiry(subscription.remoteProfile.expiresAt, "en-US")).toContain("2030");
});

it("clamps overused subscriptions to a full progress bar", () => {
  expect(
    subscriptionUsagePercent({
      ...subscription,
      remoteProfile: {
        ...subscription.remoteProfile,
        uploadBytes: 9000,
      },
    }),
  ).toBe(100);
});

describe("subscription local configuration association", () => {
  it("resolves the persisted id without creating a global selection", () => {
    const configs = [
      {
        id: subscription.localConfigId,
        name: "Office",
      } as LocalConfigSummary,
    ];
    expect(associatedLocalConfigName(subscription, configs)).toBe("Office");
    expect(
      associatedLocalConfigName({ ...subscription, localConfigId: "" }, configs),
    ).toBeNull();
  });
});
