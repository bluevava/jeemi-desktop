import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  closeConnections,
  getConnections,
  getProxySelections,
  selectProxy,
  type MihomoConnection,
} from "../../lib/mihomo/client";
import { rememberRuntimeProxySelection } from "../../services/appBridge";
import { switchProxySelection } from "./proxySelection";

vi.mock("../../lib/mihomo/client", () => ({
  closeConnections: vi.fn(),
  getConnections: vi.fn(),
  getProxySelections: vi.fn(),
  selectProxy: vi.fn(),
}));
vi.mock("../../services/appBridge", () => ({
  rememberRuntimeProxySelection: vi.fn(),
}));

const session = {
  id: "session-a",
  baseUrl: "http://127.0.0.1:12345",
  secret: "test-secret",
};
const connection = (id: string, chains: string[]): MihomoConnection => ({
  id, chains, providerChains: [], upload: 0, download: 0,
  start: "", rule: "MATCH", rulePayload: "",
  metadata: {
    network: "tcp", type: "HTTP", host: "example.com",
    sourceIP: "127.0.0.1", sourcePort: "50000",
    destinationIP: "192.0.2.1", destinationPort: "443",
    process: "", processPath: "",
  },
});
const snapshot = {
  uploadTotal: 0, downloadTotal: 0, memory: 0,
  connections: [
    connection("main", ["Node A", "Main"]),
    connection("nested", ["Node A", "Automatic", "Main"]),
    connection("other", ["Node A", "Other"]),
    connection("direct", ["DIRECT"]),
  ],
};

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(getProxySelections).mockResolvedValue({ Main: "Node A" });
  vi.mocked(getConnections).mockResolvedValue(snapshot);
});

function expectNoMutation() {
  expect(selectProxy).not.toHaveBeenCalled();
  expect(rememberRuntimeProxySelection).not.toHaveBeenCalled();
  expect(getConnections).not.toHaveBeenCalled();
  expect(closeConnections).not.toHaveBeenCalled();
}

describe("switching a direct mihomo selection", () => {
  it.each(["off", "selector", "all"] as const)(
    "preserves connections on repeated selections in %s mode, including GLOBAL and nested groups",
    async (mode) => {
      const selections = {
        Main: "🇯🇵 Node", GLOBAL: "🇯🇵 Node", Nested: "Automatic", Direct: "DIRECT",
      };
      vi.mocked(getProxySelections).mockResolvedValue(selections);
      for (const [group, proxy] of Object.entries(selections)) {
        const display = vi.fn();
        expect(await switchProxySelection(
          session, "subscription-a", group, proxy, mode, display,
        )).toEqual({ changed: false });
        // Refresh the stale display using the confirmed core state only.
        expect(display).toHaveBeenCalledOnce();
      }
      expectNoMutation();
    },
  );

  it("checks the core, switches, displays, remembers, then resets connections in order", async () => {
    const order: string[] = [];
    vi.mocked(getProxySelections).mockImplementation(async () => {
      order.push("read");
      return { Main: "Node A" };
    });
    vi.mocked(selectProxy).mockImplementation(async () => { order.push("select"); });
    vi.mocked(rememberRuntimeProxySelection).mockImplementation(async () => { order.push("save"); });
    vi.mocked(getConnections).mockImplementation(async () => {
      order.push("connections");
      return snapshot;
    });
    vi.mocked(closeConnections).mockImplementation(async () => { order.push("reset"); });

    expect(await switchProxySelection(
      session, "subscription-a", "Main", "🇯🇵 Node", "selector",
      () => { order.push("display"); },
    )).toEqual({ changed: true, remembered: true, connectionsReset: true });
    expect(order).toEqual(["read", "select", "display", "save", "connections", "reset"]);
    expect(selectProxy).toHaveBeenCalledWith(session, "Main", "🇯🇵 Node");
    expect(rememberRuntimeProxySelection).toHaveBeenCalledWith(
      "session-a", "subscription-a", "Main", "🇯🇵 Node",
    );
    expect(closeConnections).toHaveBeenCalledWith(session, ["main", "nested"]);
  });

  it.each([
    { mode: "off", ids: [] },
    { mode: "selector", ids: ["main", "nested"] },
    { mode: "all", ids: ["main", "nested", "other", "direct"] },
  ] as const)("honors $mode mode for a real change", async ({ mode, ids }) => {
    await switchProxySelection(session, "subscription-a", "Main", "Node B", mode, vi.fn());
    if (mode === "off") {
      expect(getConnections).not.toHaveBeenCalled();
      expect(closeConnections).not.toHaveBeenCalled();
    } else {
      expect(closeConnections).toHaveBeenCalledExactlyOnceWith(session, ids);
    }
  });

  it("rechecks the core on the next click even before the displayed snapshot updates", async () => {
    let selected = "Node A";
    vi.mocked(getProxySelections).mockImplementation(async () => ({ Main: selected }));
    vi.mocked(selectProxy).mockImplementation(async (_session, _group, proxy) => { selected = proxy; });
    const first = await switchProxySelection(session, "subscription-a", "Main", "Node B", "all", vi.fn());
    const repeated = await switchProxySelection(session, "subscription-a", "Main", "Node B", "all", vi.fn());
    expect(first.changed).toBe(true);
    expect(repeated).toEqual({ changed: false });
    expect(selectProxy).toHaveBeenCalledOnce();
    expect(rememberRuntimeProxySelection).toHaveBeenCalledOnce();
    expect(closeConnections).toHaveBeenCalledOnce();
  });

  it("switches a parent from an automatic group to its current leaf node", async () => {
    vi.mocked(getProxySelections).mockResolvedValue({ Main: "Automatic", Automatic: "Node B" });
    const result = await switchProxySelection(session, "subscription-a", "Main", "Node B", "selector", vi.fn());
    expect(result.changed).toBe(true);
    expect(selectProxy).toHaveBeenCalledExactlyOnceWith(session, "Main", "Node B");
    expect(closeConnections).toHaveBeenCalledOnce();
  });

  it("matches complete names without trimming spaces", async () => {
    vi.mocked(getProxySelections).mockResolvedValue({ "🌐 Main": "🇯🇵 Node" });
    const result = await switchProxySelection(session, "subscription-a", "🌐 Main", " 🇯🇵 Node ", "off", vi.fn());
    expect(result.changed).toBe(true);
    expect(selectProxy).toHaveBeenCalledWith(session, "🌐 Main", " 🇯🇵 Node ");
  });

  it("does not guess a selection or reset connections when the live read fails", async () => {
    vi.mocked(getProxySelections).mockRejectedValue(new Error("read failed"));
    const display = vi.fn();
    await expect(switchProxySelection(
      session, "subscription-a", "Main", "Node B", "all", display,
    )).rejects.toThrow("read failed");
    expect(display).not.toHaveBeenCalled();
    expectNoMutation();
  });

  it("does not change a selector missing from the current core snapshot", async () => {
    vi.mocked(getProxySelections).mockResolvedValue({});
    await expect(switchProxySelection(
      session, "subscription-a", "Main", "Node B", "selector", vi.fn(),
    )).rejects.toThrow("selection is unavailable");
    expectNoMutation();
  });

  it("does not remember, display or reset a rejected core selection", async () => {
    vi.mocked(selectProxy).mockRejectedValue(new Error("core rejected"));
    const display = vi.fn();
    await expect(switchProxySelection(
      session, "subscription-a", "Main", "Node B", "all", display,
    )).rejects.toThrow("core rejected");
    expect(display).not.toHaveBeenCalled();
    expect(rememberRuntimeProxySelection).not.toHaveBeenCalled();
    expect(getConnections).not.toHaveBeenCalled();
    expect(closeConnections).not.toHaveBeenCalled();
  });

  it("retains the completed switch and connection reset if persistence fails", async () => {
    vi.mocked(rememberRuntimeProxySelection).mockRejectedValue(new Error("disk unavailable"));
    const display = vi.fn();
    expect(await switchProxySelection(
      session, "subscription-a", "Main", "Node B", "selector", display,
    )).toEqual({ changed: true, remembered: false, connectionsReset: true });
    expect(display).toHaveBeenCalledOnce();
    expect(closeConnections).toHaveBeenCalledExactlyOnceWith(session, ["main", "nested"]);
  });

  it("reports a reset failure separately from a successful node switch", async () => {
    vi.mocked(closeConnections).mockRejectedValue(new Error("connection closed"));
    const display = vi.fn();
    expect(await switchProxySelection(
      session, "subscription-a", "Main", "Node B", "selector", display,
    )).toEqual({ changed: true, remembered: true, connectionsReset: false });
    expect(display).toHaveBeenCalledOnce();
    expect(rememberRuntimeProxySelection).toHaveBeenCalledOnce();
  });
});
