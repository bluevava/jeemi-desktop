import { beforeEach, describe, expect, it, vi } from "vitest";
import { selectProxy } from "../../lib/mihomo/client";
import { rememberRuntimeProxySelection } from "../../services/appBridge";
import { selectAndRememberProxy } from "./proxySelection";

vi.mock("../../lib/mihomo/client", () => ({ selectProxy: vi.fn() }));
vi.mock("../../services/appBridge", () => ({
  rememberRuntimeProxySelection: vi.fn(),
}));

const session = {
  id: "session-a",
  baseUrl: "http://127.0.0.1:12345",
  secret: "test-secret",
};
beforeEach(() => vi.resetAllMocks());

describe("remembering a direct mihomo selection", () => {
  it("saves only after REST success and keeps the originating session and subscription", async () => {
    const order: string[] = [];
    vi.mocked(selectProxy).mockImplementation(async () => {
      order.push("select");
    });
    vi.mocked(rememberRuntimeProxySelection).mockImplementation(async () => {
      order.push("save");
    });
    const saved = await selectAndRememberProxy(
      session,
      "subscription-a",
      "🌐 Main",
      "🇯🇵 Node",
      () => {
        order.push("display");
      },
    );
    expect(saved).toBe(true);
    expect(order).toEqual(["select", "display", "save"]);
    expect(rememberRuntimeProxySelection).toHaveBeenCalledWith(
      "session-a",
      "subscription-a",
      "🌐 Main",
      "🇯🇵 Node",
    );
  });

  it("does not remember or display a rejected core selection", async () => {
    vi.mocked(selectProxy).mockRejectedValue(new Error("core rejected"));
    const display = vi.fn();
    await expect(
      selectAndRememberProxy(
        session,
        "subscription-a",
        "Main",
        "Node",
        display,
      ),
    ).rejects.toThrow("core rejected");
    expect(display).not.toHaveBeenCalled();
    expect(rememberRuntimeProxySelection).not.toHaveBeenCalled();
  });

  it("reports persistence failure separately while retaining the completed selection", async () => {
    vi.mocked(rememberRuntimeProxySelection).mockRejectedValue(
      new Error("disk unavailable"),
    );
    const display = vi.fn();
    expect(
      await selectAndRememberProxy(
        session,
        "subscription-a",
        "Main",
        "Node",
        display,
      ),
    ).toBe(false);
    expect(display).toHaveBeenCalledOnce();
    expect(selectProxy).toHaveBeenCalledOnce();
  });
});
