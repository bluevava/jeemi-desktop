import { afterEach, describe, expect, it, vi } from "vitest";
import { copySubscriptionSource } from "../../services/appBridge";
import type { SubscriptionSummary } from "../../types/subscription";
import { unbindSubscriptionAssociation } from "./cardActions";

afterEach(() => vi.unstubAllGlobals());

describe("subscription card actions", () => {
  it("copies by ID without requesting a URL, path, or source body", async () => {
    const copy = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("window", { go: { desktop: { App: { CopySubscriptionSource: copy } } } });
    await expect(copySubscriptionSource("subscription-id")).resolves.toBeUndefined();
    expect(copy.mock.calls).toEqual([["subscription-id"]]);
  });

  it.each(["script", "config"] as const)("unbinds only the requested %s handler", async (kind) => {
    const script = vi.fn().mockResolvedValue({ subscriptions: [] });
    const config = vi.fn().mockResolvedValue({ subscriptions: [] });
    vi.stubGlobal("window", { go: { desktop: { App: { SetSubscriptionLocalScript: script, SetSubscriptionLocalConfig: config } } } });
    await unbindSubscriptionAssociation({ id: "sub" } as SubscriptionSummary, kind);
    expect((kind === "script" ? script : config).mock.calls).toEqual([["sub", ""]]);
    expect(kind === "script" ? config : script).not.toHaveBeenCalled();
  });

  it("uses the current library revision and the displayed subscription revision to unbind all chain groups", async () => {
    const get = vi.fn().mockResolvedValue({ revision: 17, groups: [] });
    const set = vi.fn().mockResolvedValue({ subscriptions: [] });
    vi.stubGlobal("window", { go: { desktop: { App: { GetChainProxyState: get, SetSubscriptionChainProxyGroups: set } } } });
    await unbindSubscriptionAssociation({ id: "sub", chainProxyRevision: 4 } as SubscriptionSummary, "chain");
    expect(get).toHaveBeenCalledOnce();
    expect(set.mock.calls).toEqual([["sub", [], 4, 17]]);
    set.mockRejectedValue(new Error("chain_proxy:revision_conflict"));
    await expect(unbindSubscriptionAssociation({ id: "sub", chainProxyRevision: 4 } as SubscriptionSummary, "chain")).rejects.toThrow("revision_conflict");
  });
});
