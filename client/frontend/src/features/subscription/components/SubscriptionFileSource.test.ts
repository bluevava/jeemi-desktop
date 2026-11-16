import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import type { SubscriptionSummary } from "../../../types/subscription";
import { SubscriptionCard } from "./SubscriptionCard";
import { SubscriptionFileSource } from "./SubscriptionFileSource";

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en-US" } }) }));
vi.mock("./SubscriptionAssociations", () => ({ SubscriptionAssociations: () => null }));

describe("file subscription controls", () => {
  it.each([false, true])("shows the source draft and respects busy=%s", (busy) => {
    const html = renderToStaticMarkup(createElement(SubscriptionFileSource, {
      fileName: "Selected file.yaml", changed: true, busy, onChoose: vi.fn(),
    }));
    expect(html).toContain("Selected file.yaml");
    expect(html).toContain("subscription.form.filePending");
    expect(html.includes('disabled=""')).toBe(busy);
  });

  it.each([false, true])("file card refresh is enabled unless busy=%s", (disabled) => {
    const subscription = {
      id: "file-sub", name: "File", sourceKind: "file", iconKind: "", icon: "", lastFetchedAt: "2026-11-12T00:00:00Z",
      remoteProfile: {}, composition: {}, chainProxyGroupIds: [], localConfigId: "", localScriptId: "",
    } as unknown as SubscriptionSummary;
    const html = renderToStaticMarkup(createElement(SubscriptionCard, {
      subscription, selected: true, locale: "en-US", busy: false, copying: false, pendingAssociation: null,
      chainGroupNames: {}, disabled, menu: {}, onCopy: vi.fn(), onBind: vi.fn(), onUnbind: vi.fn(), onRefresh: vi.fn(), onSelect: vi.fn(),
    }));
    const refresh = html.match(/<button[^>]*aria-label="subscription\.menu\.refreshFile"[^>]*>/)?.[0];
    expect(refresh).toBeDefined();
    expect(refresh!.includes('disabled=""')).toBe(disabled);
    expect(html).not.toContain("refreshUnavailable");
  });
});
