import { afterEach, describe, expect, it, vi } from "vitest";

import {
  closeConnections,
  closeConnection,
  getConnections,
  getProxyRuntimeState,
  getProxySelections,
  getRuleProviderRuntimeState,
  selectProxy,
  subscribeLogs,
  subscribeTraffic,
  testProxyDelay,
} from "./client";

const session = {
  id: "session",
  baseUrl: "http://127.0.0.1:49090",
  secret: "secret",
};

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("mihomo client", () => {
  it("reads selector values and the latest mihomo proxy history", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() =>
        Promise.resolve(new Response(
          JSON.stringify({
            proxies: {
              GLOBAL: {
                all: ["Node A", "Main", "DIRECT"],
                now: "Node A",
                type: "Selector",
              },
              Main: { all: ["Node A"], now: "Node A", type: "Selector" },
              "Node A": {
                type: "ss",
                history: [
                  { time: "2026-09-02T12:00:00Z", delay: 90 },
                  { time: "2026-09-02T12:01:00Z", delay: 62 },
                ],
              },
            },
          }),
          { status: 200 },
        )),
      ),
    );
    await expect(getProxySelections(session)).resolves.toEqual({
      GLOBAL: "Node A",
      Main: "Node A",
    });
    await expect(getProxyRuntimeState(session)).resolves.toEqual({
      selections: { GLOBAL: "Node A", Main: "Node A" },
      selectorNames: ["GLOBAL", "Main"],
      allProxies: [{ name: "Node A", type: "ss" }],
      delays: [{ name: "Node A", status: "success", delay: 62 }],
    });
  });

  it("selects a proxy with bearer authentication", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    await selectProxy(session, "Main Group", "Node A");
    expect(fetchMock).toHaveBeenCalledWith(
      "http://127.0.0.1:49090/proxies/Main%20Group",
      expect.objectContaining({
        method: "PUT",
        headers: expect.objectContaining({ Authorization: "Bearer secret" }),
      }),
    );
  });

  it("tests a named proxy through mihomo with a bounded generate-204 request", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ delay: 128 }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(testProxyDelay(session, "Tokyo / A")).resolves.toBe(128);
    const [requestURL, requestInit] = fetchMock.mock.calls[0];
    const url = new URL(requestURL);
    expect(url.pathname).toBe("/proxies/Tokyo%20%2F%20A/delay");
    expect(url.searchParams.get("timeout")).toBe("8000");
    expect(url.searchParams.get("expected")).toBe("200-299");
    expect(requestInit.headers.Authorization).toBe("Bearer secret");
  });

  it("reads rule-provider update timestamps from the running core", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            providers: {
              ads: {
                ruleCount: 321,
                updatedAt: "2026-09-02T12:03:04Z",
                vehicleType: "HTTP",
              },
              pending: { updatedAt: "0001-01-01T00:00:00Z" },
            },
          }),
          { status: 200 },
        ),
      ),
    );

    await expect(getRuleProviderRuntimeState(session)).resolves.toEqual({
      ads: {
        name: "ads",
        ruleCount: 321,
        updatedAt: "2026-09-02T12:03:04Z",
        vehicleType: "HTTP",
      },
      pending: {
        name: "pending",
        ruleCount: 0,
        updatedAt: "",
        vehicleType: "",
      },
    });
  });

  it("normalises connection snapshots and closes one or a filtered set", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            uploadTotal: 12,
            downloadTotal: 34,
            connections: [
              {
                id: "connection/1",
                metadata: {
                  host: "example.com",
                  destinationPort: "443",
                  process: "browser.exe",
                },
                upload: 5,
                download: 9,
                chains: ["Node A"],
              },
            ],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    const snapshot = await getConnections(session);
    expect(snapshot.connections[0]).toMatchObject({
      id: "connection/1",
      metadata: { host: "example.com", process: "browser.exe" },
      chains: ["Node A"],
    });
    await closeConnection(session, "connection/1");
    await closeConnections(session, ["connection/2", "connection/3"], 2);
    expect(fetchMock.mock.calls[1][0]).toBe(
      "http://127.0.0.1:49090/connections/connection%2F1",
    );
    expect(fetchMock.mock.calls[1][1]).toMatchObject({ method: "DELETE" });
    expect(fetchMock.mock.calls.slice(2).map((call) => call[0])).toEqual([
      "http://127.0.0.1:49090/connections/connection%2F2",
      "http://127.0.0.1:49090/connections/connection%2F3",
    ]);
  });

  it("uses the required token query only for the local traffic WebSocket", () => {
    class FakeWebSocket {
      static CLOSING = 2;
      static instances: FakeWebSocket[] = [];
      readyState = 0;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: ((event: { data: string }) => void) | null = null;
      onopen: (() => void) | null = null;

      constructor(public url: string) {
        FakeWebSocket.instances.push(this);
      }

      close() {
        this.readyState = 3;
      }
    }
    vi.stubGlobal("WebSocket", FakeWebSocket);
    const onMessage = vi.fn();
    const onStateChange = vi.fn();
    const stop = subscribeTraffic(session, { onMessage, onStateChange });
    const socket = FakeWebSocket.instances[0];
    const url = new URL(socket.url);
    expect(url.protocol).toBe("ws:");
    expect(url.hostname).toBe("127.0.0.1");
    expect(url.pathname).toBe("/traffic");
    expect(url.searchParams.get("token")).toBe("secret");
    socket.onopen?.();
    socket.onmessage?.({
      data: JSON.stringify({ up: 12, down: 34, upTotal: 56, downTotal: 78 }),
    });
    expect(onStateChange).toHaveBeenLastCalledWith("live");
    expect(onMessage).toHaveBeenCalledWith({
      up: 12,
      down: 34,
      upTotal: 56,
      downTotal: 78,
    });
    const stateCallsBeforeStop = onStateChange.mock.calls.length;
    const messageCallsBeforeStop = onMessage.mock.calls.length;
    stop();
    expect(socket.readyState).toBe(3);
    socket.onopen?.();
    socket.onerror?.();
    socket.onmessage?.({
      data: JSON.stringify({ up: 1, down: 2, upTotal: 3, downTotal: 4 }),
    });
    expect(onStateChange).toHaveBeenCalledTimes(stateCallsBeforeStop);
    expect(onMessage).toHaveBeenCalledTimes(messageCallsBeforeStop);
  });

  it("subscribes to structured mihomo logs at the requested level", () => {
    class FakeWebSocket {
      static CLOSING = 2;
      static instances: FakeWebSocket[] = [];
      readyState = 0;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: ((event: { data: string }) => void) | null = null;
      onopen: (() => void) | null = null;

      constructor(public url: string) {
        FakeWebSocket.instances.push(this);
      }

      close() {
        this.readyState = 3;
      }
    }
    vi.stubGlobal("WebSocket", FakeWebSocket);
    const onMessage = vi.fn();
    const stop = subscribeLogs(session, "warning", { onMessage });
    const socket = FakeWebSocket.instances[0];
    const url = new URL(socket.url);
    expect(url.pathname).toBe("/logs");
    expect(url.searchParams.get("level")).toBe("warning");
    expect(url.searchParams.get("format")).toBe("structured");
    expect(url.searchParams.get("token")).toBe("secret");

    socket.onmessage?.({
      data: JSON.stringify({
        time: "12:34:56",
        level: "warn",
        message: "[DNS] lookup failed",
        fields: [{ host: "example.com" }],
      }),
    });
    expect(onMessage).toHaveBeenCalledWith({
      time: "12:34:56",
      level: "warning",
      message: "[DNS] lookup failed",
      fields: [{ host: "example.com" }],
      receivedAt: expect.any(Number),
    });

    socket.onmessage?.({
      data: JSON.stringify({ type: "warn", payload: "legacy frame" }),
    });
    expect(onMessage).toHaveBeenLastCalledWith({
      time: "",
      level: "warning",
      message: "legacy frame",
      fields: [],
      receivedAt: expect.any(Number),
    });
    stop();
  });
});
