import { describe, expect, it, vi } from "vitest";
import type { ProxyAuthorizationStatus } from "../../types/authorization";
import { AuthorizationFlow, idleAuthorization, type AuthorizationView } from "./authorizationFlow";

const absent: ProxyAuthorizationStatus = { platform: "darwin", kind: "service", ready: false, present: false, localTest: true, code: "local_installation_required", action: "install", steps: [] };
const ready: ProxyAuthorizationStatus = { ...absent, ready: true, present: true, code: "ready", action: "" };
const flush = async () => { await Promise.resolve(); await Promise.resolve(); };
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(done => { resolve = done; }); return { promise, resolve }; }
function fixture() {
  const operations = { inspect: vi.fn(async () => absent), setup: vi.fn(async () => ready), settings: vi.fn(async () => undefined), cancel: vi.fn(async () => undefined) };
  let view: AuthorizationView = idleAuthorization;
  const flow = new AuthorizationFlow(operations, value => { view = value; });
  return { flow, operations, view: () => view };
}
describe("proxy authorization intent", () => {
  it("checks an outdated Windows helper only on start and waits for the update action", async () => {
    const f = fixture();
    const outdated: ProxyAuthorizationStatus = { ...absent, platform: "windows", localTest: false, present: true, code: "authorization_update_required", action: "update" };
    f.operations.inspect.mockResolvedValue(outdated);
    await f.flow.check(); await f.flow.perform();
    expect(f.operations.inspect).not.toHaveBeenCalled(); expect(f.operations.setup).not.toHaveBeenCalled();
    expect(f.view()).toEqual(idleAuthorization);
    const intent = f.flow.request(); await flush();
    expect(f.operations.inspect).toHaveBeenCalledTimes(1); expect(f.operations.setup).not.toHaveBeenCalled();
    expect(f.view()).toEqual({ open: true, busy: false, status: outdated, errorKey: null });
    f.operations.setup.mockResolvedValue({ ...outdated, ready: true, code: "ready", action: "" });
    await f.flow.perform(); expect(await intent).toBe(true);
    expect(f.operations.setup).toHaveBeenCalledTimes(1); expect(f.view()).toEqual(idleAuthorization);
  });
  it("retains an explicit action clicked during a background check", async () => {
    const f = fixture(); const intent = f.flow.request(); await flush();
    const read = deferred<ProxyAuthorizationStatus>(); f.operations.inspect.mockReturnValueOnce(read.promise);
    const check = f.flow.check(); await f.flow.perform(); expect(f.operations.setup).not.toHaveBeenCalled();
    read.resolve(absent); await check; expect(await intent).toBe(true); expect(f.operations.setup).toHaveBeenCalledTimes(1);
  });
  it("passes a healthy installation without opening a dialog or requesting permission", async () => {
    const f = fixture(); f.operations.inspect.mockResolvedValue(ready);
    expect(await f.flow.request()).toBe(true);
    expect(f.view().open).toBe(false); expect(f.operations.setup).not.toHaveBeenCalled();
  });
  it("shares duplicate starts and resumes once after installation", async () => {
    const f = fixture(); const first = f.flow.request(); expect(f.flow.request()).toBe(first);
    await flush(); expect(f.view().open).toBe(true); expect(f.operations.setup).not.toHaveBeenCalled();
    await f.flow.perform(); expect(await first).toBe(true);
    await f.flow.perform(); await f.flow.check(); expect(f.operations.setup).toHaveBeenCalledTimes(1);
  });
  it("cannot start from a successful reply arriving after close", async () => {
    const f = fixture(); const setup = deferred<ProxyAuthorizationStatus>(); f.operations.setup.mockReturnValue(setup.promise);
    const intent = f.flow.request(); await flush(); const work = f.flow.perform();
    f.flow.cancel(); expect(await intent).toBe(false);
    setup.resolve(ready); await work; expect(f.view().open).toBe(false); expect(f.operations.cancel).toHaveBeenCalledTimes(1);
  });
  it("never overwrites a new intent with an old check", async () => {
    const f = fixture(); const read = deferred<ProxyAuthorizationStatus>(); f.operations.inspect.mockReturnValueOnce(read.promise);
    const first = f.flow.request(); f.flow.cancel(); const second = f.flow.request();
    read.resolve(ready); await flush(); expect(await first).toBe(false);
    expect(f.view().status?.ready).toBe(false); f.flow.cancel(); expect(await second).toBe(false);
  });
  it("does not infer absence or elevate on inspection failure", async () => {
    const f = fixture(); f.operations.inspect.mockRejectedValue(new Error("failure"));
    const intent = f.flow.request(); await flush(); expect(f.view().errorKey).toBe("authorization.inspectionFailed");
    expect(f.view().status).toBe(null); expect(f.operations.setup).not.toHaveBeenCalled();
    f.flow.cancel(); expect(await intent).toBe(false);
  });
  it("waits for actual system approval and continues after rechecking", async () => {
    const f = fixture(); f.operations.inspect.mockResolvedValueOnce({ ...absent, present: true, action: "settings" });
    const intent = f.flow.request(); await flush(); await f.flow.perform();
    expect(f.operations.settings).toHaveBeenCalledTimes(1); expect(f.view().open).toBe(true);
    f.operations.inspect.mockResolvedValue(ready); await f.flow.check(); expect(await intent).toBe(true);
  });
  it("cancels the pending start when the native password prompt is cancelled", async () => {
    const f = fixture(); f.operations.setup.mockRejectedValue(new Error("macos_network_cancelled"));
    const intent = f.flow.request(); await flush(); await f.flow.perform();
    expect(await intent).toBe(false); expect(f.view().open).toBe(false);
  });
  it("keeps a stopped or broken installation eligible for repair", async () => {
    const f = fixture(); f.operations.inspect.mockResolvedValue({ ...absent, present: true, action: "repair", code: "unreachable" });
    const intent = f.flow.request(); await flush(); expect(f.view().status?.present).toBe(true);
    await f.flow.perform(); expect(await intent).toBe(true);
  });
});
