import { afterEach, describe, expect, it, vi } from "vitest";

import { initializeClient } from "./startupBridge";

afterEach(() => vi.unstubAllGlobals());

describe("startup bridge", () => {
  it("allows browser previews without a desktop bridge", async () => {
    vi.stubGlobal("window", {});
    await expect(initializeClient("zh-CN")).resolves.toBeUndefined();
  });

  it("waits for configuration recovery before completing startup", async () => {
    let complete!: () => void;
    const pending = new Promise<void>((resolve) => { complete = resolve; });
    const initialize = vi.fn(() => pending);
    vi.stubGlobal("window", { go: { desktop: { App: { InitializeClient: initialize } } } });
    let ready = false;
    const result = initializeClient("en-US").then(() => { ready = true; });
    await Promise.resolve();
    expect(ready).toBe(false);
    expect(initialize).toHaveBeenCalledWith("en-US");
    complete();
    await result;
    expect(ready).toBe(true);
  });

  it("does not continue when desktop initialization fails or is missing", async () => {
    vi.stubGlobal("window", { go: { desktop: { App: {} } } });
    await expect(initializeClient("zh-CN")).rejects.toThrow();
    vi.stubGlobal("window", { go: { desktop: { App: { InitializeClient: vi.fn().mockRejectedValue(new Error("declined")) } } } });
    await expect(initializeClient("zh-CN")).rejects.toThrow("declined");
  });
});
