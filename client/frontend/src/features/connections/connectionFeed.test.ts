import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { subscribeTraffic } from "../../lib/mihomo/client";
import { observeConnections } from "./connectionFeed";

const session = { id: "session", baseUrl: "http://127.0.0.1:49090", secret: "test-secret" };
const snapshot = (id: string) => ({ connections: [{ id, metadata: {}, chains: ["Node", "Selector"] }] });

class FakeWebSocket {
  static CLOSING = 2;
  static instances: FakeWebSocket[] = [];
  readyState = 0;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  constructor(public url: string) { FakeWebSocket.instances.push(this); }
  close() { this.readyState = 3; this.onclose?.(); }
  emit(value: unknown) { this.onmessage?.({ data: JSON.stringify(value) }); }
}

function deferredRequest(ignoreAbort = false) {
  let resolve!: (value: Response) => void;
  let reject!: (reason: unknown) => void;
  let signal!: AbortSignal;
  const result = new Promise<Response>((yes, no) => { resolve = yes; reject = no; });
  const fetch = vi.fn((_url: string, init: RequestInit) => {
    signal = init.signal as AbortSignal;
    if (!ignoreAbort) signal.addEventListener("abort", () => reject(new Error("cancelled")), { once: true });
    return result;
  });
  vi.stubGlobal("fetch", fetch);
  return { fetch, resolve, reject, get signal() { return signal; } };
}

beforeEach(() => {
  vi.useFakeTimers();
  FakeWebSocket.instances = [];
  vi.stubGlobal("WebSocket", FakeWebSocket);
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("page-owned connection feed", () => {
  it("cancels the first request and ignores late messages when the page leaves", async () => {
    const request = deferredRequest(true);
    const handlers = { onMessage: vi.fn(), onStateChange: vi.fn() };
    const stop = observeConnections(session, handlers);
    const socket = FakeWebSocket.instances[0];
    expect(request.fetch.mock.calls.map(([url]) => new URL(url).pathname)).toEqual(["/connections"]);
    expect(new URL(socket.url).pathname).toBe("/connections");
    expect(new URL(socket.url).searchParams.get("interval")).toBe("1000");

    const stateCalls = handlers.onStateChange.mock.calls.length;
    stop();
    stop();
    expect(request.signal.aborted).toBe(true);
    expect(socket.readyState).toBe(3);
    request.resolve(new Response(JSON.stringify(snapshot("late-http"))));
    socket.onopen?.();
    socket.emit(snapshot("late-stream"));
    socket.onerror?.();
    await vi.advanceTimersByTimeAsync(20000);
    expect(handlers.onMessage).not.toHaveBeenCalled();
    expect(handlers.onStateChange).toHaveBeenCalledTimes(stateCalls);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(request.fetch).toHaveBeenCalledTimes(1);
  });

  it("does not let a slow initial response overwrite newer live results", async () => {
    const request = deferredRequest(true);
    const onMessage = vi.fn();
    const stop = observeConnections(session, { onMessage });
    FakeWebSocket.instances[0].emit(snapshot("live"));
    expect(request.signal.aborted).toBe(true);
    request.resolve(new Response(JSON.stringify(snapshot("old-http"))));
    await vi.advanceTimersByTimeAsync(0);
    expect(onMessage).toHaveBeenCalledTimes(1);
    expect(onMessage.mock.calls[0][0].connections[0].id).toBe("live");
    stop();
  });

  it("clears pending reconnects without stopping the shared total-traffic stream", async () => {
    const request = deferredRequest();
    const traffic = vi.fn();
    const stopTraffic = subscribeTraffic(session, { onMessage: traffic });
    const stop = observeConnections(session, { onMessage: vi.fn() });
    const [trafficSocket, connectionSocket] = FakeWebSocket.instances;
    connectionSocket.close();
    stop();
    await vi.advanceTimersByTimeAsync(20000);

    expect(request.signal.aborted).toBe(true);
    expect(request.fetch).toHaveBeenCalledTimes(1);
    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(trafficSocket.readyState).not.toBe(3);
    expect(new URL(trafficSocket.url).pathname).toBe("/traffic");
    trafficSocket.emit({ up: 12, down: 34, upTotal: 56, downTotal: 78 });
    expect(traffic).toHaveBeenCalledWith({ up: 12, down: 34, upTotal: 56, downTotal: 78 });
    stopTraffic();
  });

  it("uses the first snapshot while connecting and keeps stream health independent of REST", async () => {
    const request = deferredRequest();
    const handlers = { onMessage: vi.fn(), onStateChange: vi.fn() };
    const stop = observeConnections(session, handlers);
    request.resolve(new Response(JSON.stringify(snapshot("first-http"))));
    await vi.advanceTimersByTimeAsync(0);
    expect(handlers.onMessage.mock.calls[0][0].connections[0].id).toBe("first-http");
    expect(handlers.onStateChange).toHaveBeenLastCalledWith("connecting");
    FakeWebSocket.instances[0].onopen?.();
    FakeWebSocket.instances[0].emit(snapshot("live"));
    expect(handlers.onMessage.mock.lastCall?.[0].connections[0].id).toBe("live");
    expect(handlers.onStateChange).toHaveBeenLastCalledWith("live");
    stop();
  });

  it("still receives live data when the optional initial request fails", async () => {
    const request = deferredRequest();
    const handlers = { onMessage: vi.fn(), onStateChange: vi.fn() };
    const stop = observeConnections(session, handlers);
    FakeWebSocket.instances[0].onopen?.();
    request.reject(new Error("first snapshot failed"));
    await vi.advanceTimersByTimeAsync(0);
    FakeWebSocket.instances[0].emit(snapshot("live"));
    expect(handlers.onStateChange).toHaveBeenLastCalledWith("live");
    expect(handlers.onMessage.mock.lastCall?.[0].connections[0].id).toBe("live");
    stop();
  });
});
