import { afterEach, describe, expect, it, vi } from "vitest";
import { chooseSubscriptionSourceFile, updateSubscription } from "../../services/appBridge";
import { subscriptionSourceErrorKey } from "./fileSource";

afterEach(() => vi.unstubAllGlobals());

describe("file subscription source", () => {
  it("selects through the native dialog and submits only an opaque token", async () => {
    const selection = { cancelled: false, token: "native-choice", fileName: "source.yaml" };
    const choose = vi.fn().mockResolvedValue(selection);
    const update = vi.fn().mockResolvedValue({ subscriptions: [] });
    vi.stubGlobal("window", { go: { desktop: { App: { ChooseSubscriptionSourceFile: choose, UpdateSubscription: update } } } });
    const labels = { title: "Choose", filterName: "Config" };
    await expect(chooseSubscriptionSourceFile("file-sub", labels)).resolves.toEqual(selection);
    const input = { id: "file-sub", name: "File", sourceFileToken: selection.token, sourceUrl: "", description: "", autoUpdateIntervalMinutes: 0, iconKind: "" as const, icon: "" };
    await updateSubscription(input);
    expect(choose.mock.calls).toEqual([["file-sub", labels]]);
    expect(update.mock.calls).toEqual([[input]]);
    expect(JSON.stringify(update.mock.calls)).not.toContain("sourceFilePath");
  });

  it.each([
    ["subscription_file_source_missing", "fileSourceMissing"],
    ["subscription_file_unavailable", "fileSourceUnavailable"],
    ["subscription_file_changed", "fileSourceChanged"],
    ["subscription_file_selection_expired", "fileSelectionExpired"],
  ])("maps %s to a fixed localized explanation", (code, key) => {
    expect(subscriptionSourceErrorKey(code, "fallback")).toBe(`subscription.errors.${key}`);
    expect(subscriptionSourceErrorKey(new Error(code), "fallback")).toBe(`subscription.errors.${key}`);
  });

  it.each(["C:/private/user/source.yaml", "toString", "__proto__", "", null])("never displays arbitrary error values", (cause) => {
    expect(subscriptionSourceErrorKey(cause, "fallback")).toBe("fallback");
  });
});
