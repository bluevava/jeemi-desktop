import { afterEach, describe, expect, it, vi } from "vitest";
import { SelectorIconCache, waitForIconRetry } from "./selectorIconCache";

const image = "data:image/png;base64,aGVsbG8=";
const address = "https://assets.example/icon.png";
const signal = () => new AbortController().signal;

function pending<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

describe("selector icon cache", () => {
  afterEach(() => vi.useRealTimers());

  it("shares one download across cards and reuses success after navigation", async () => {
    const response = pending<string>();
    const fetch = vi.fn(() => response.promise);
    const cancel = vi.fn(async () => undefined);
    const cache = new SelectorIconCache(fetch, cancel);
    const first = cache.load(address, signal());
    const second = cache.load(address, signal());
    expect(fetch).toHaveBeenCalledTimes(1);
    response.resolve(image);
    await expect(first).resolves.toBe(image);
    await expect(second).resolves.toBe(image);
    await expect(cache.load(address, signal())).resolves.toBe(image);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(cancel).not.toHaveBeenCalled();
  });

  it("does not cache a failure and can recover after the proxy becomes ready", async () => {
    const fetch = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValue(image);
    const cache = new SelectorIconCache(fetch, vi.fn());
    await expect(cache.load(address, signal())).rejects.toThrow("offline");
    await expect(cache.load(address, signal())).rejects.toThrow("offline");
    expect(fetch).toHaveBeenCalledTimes(1);
    await expect(cache.load(address, signal(), "running")).resolves.toBe(image);
    await expect(cache.load(address, signal())).resolves.toBe(image);
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("cancels only when the last displayed copy releases its request", async () => {
    const response = pending<string>();
    const cancel = vi.fn(async () => undefined);
    const cache = new SelectorIconCache(() => response.promise, cancel);
    const first = new AbortController();
    const second = new AbortController();
    const firstLoad = cache.load(address, first.signal);
    const secondLoad = cache.load(address, second.signal);
    first.abort();
    await expect(firstLoad).rejects.toThrow("cancelled");
    expect(cancel).not.toHaveBeenCalled();
    response.resolve(image);
    await expect(secondLoad).resolves.toBe(image);
    expect(cancel).not.toHaveBeenCalled();
  });

  it("ignores late cancelled responses without deleting or poisoning a replacement request", async () => {
    const old = pending<string>();
    const replacement = pending<string>();
    const fetch = vi.fn().mockReturnValueOnce(old.promise).mockReturnValueOnce(replacement.promise);
    const cancel = vi.fn(async () => undefined);
    const cache = new SelectorIconCache(fetch, cancel);
    const controller = new AbortController();
    const first = cache.load(address, controller.signal);
    controller.abort();
    await expect(first).rejects.toThrow("cancelled");
    const second = cache.load(address, signal());
    old.resolve("data:image/png;base64,b2xk");
    await Promise.resolve();
    const third = cache.load(address, signal());
    expect(fetch).toHaveBeenCalledTimes(2);
    replacement.resolve(image);
    await expect(second).resolves.toBe(image);
    await expect(third).resolves.toBe(image);
    expect(cancel).toHaveBeenCalledTimes(1);
  });

  it("rejects remote or executable responses from the image bridge", async () => {
    for (const source of [address, "data:text/html;base64,aGVsbG8=", "data:image/png;base64," + "A".repeat(1_400_000)]) {
      const cache = new SelectorIconCache(async () => source, vi.fn());
      await expect(cache.load(address, signal())).rejects.toThrow("unavailable");
    }
  });

  it("bounds the process image cache while retaining recently used images", async () => {
    const fetch = vi.fn(async () => image);
    const cache = new SelectorIconCache(fetch, vi.fn());
    for (let index = 0; index < 128; index++) await cache.load(`${address}?${index}`, signal());
    await cache.load(`${address}?0`, signal());
    await cache.load(`${address}?128`, signal());
    await cache.load(`${address}?0`, signal());
    expect(fetch).toHaveBeenCalledTimes(129);
    await cache.load(`${address}?1`, signal());
    expect(fetch).toHaveBeenCalledTimes(130);
  });

  it("releases retry timers when the page or window becomes inactive", async () => {
    vi.useFakeTimers();
    const controller = new AbortController();
    const waiting = waitForIconRetry(5000, controller.signal);
    controller.abort();
    await expect(waiting).rejects.toThrow("cancelled");
    expect(vi.getTimerCount()).toBe(0);
  });
});
