import type { ProxyAuthorizationStatus } from "../../types/authorization";
import { platformFailureMessageKey } from "../runtime/runtimePresentation";

export interface AuthorizationView {
  open: boolean;
  busy: boolean;
  status: ProxyAuthorizationStatus | null;
  errorKey: string | null;
}
export const idleAuthorization: AuthorizationView = { open: false, busy: false, status: null, errorKey: null };

export function authorizationErrorKey(error: unknown): string {
  // Extract only a known error code; raw native output never reaches the UI.
  const code = String(error).match(/(?:macos_network_|linux_core_|linux_resolver_|authorization_)[a-z_]+/)?.[0] ?? "";
  if (code.includes("cleanup")) return "authorization.cleanupFailed";
  return platformFailureMessageKey(code) ?? "authorization.actionFailed";
}

interface Operations {
  inspect: () => Promise<ProxyAuthorizationStatus>;
  setup: () => Promise<ProxyAuthorizationStatus>;
  settings: () => Promise<void>;
  cancel: () => Promise<void>;
}

// A single pending intent spans polling, native authentication and settings.
// Cancelling invalidates all late replies, including successful installations.
export class AuthorizationFlow {
  private sequence = 0;
  private pending: { id: number; promise: Promise<boolean>; resolve: (ready: boolean) => void } | null = null;
  private reading = false;
  private working = false;
  private queuedAction: number | null = null;
  private view = idleAuthorization;
  constructor(private readonly operations: Operations, private readonly publish: (view: AuthorizationView) => void) {}

  private update(view: AuthorizationView) { this.view = view; this.publish(view); }
  request(): Promise<boolean> {
    if (this.pending) return this.pending.promise;
    const id = ++this.sequence;
    let resolve!: (ready: boolean) => void;
    const promise = new Promise<boolean>(done => { resolve = done; });
    this.pending = { id, promise, resolve };
    void this.check();
    return promise;
  }
  private accept(id: number, status: ProxyAuthorizationStatus) {
    if (this.pending?.id !== id) return;
    if (status.ready) {
      const pending = this.pending;
      this.pending = null;
      this.update(idleAuthorization);
      pending.resolve(true);
    } else this.update({ open: true, busy: false, status, errorKey: null });
  }
  async check() {
    if (!this.pending || this.reading || this.working) return;
    const id = this.pending.id;
    this.reading = true;
    try { this.accept(id, await this.operations.inspect()); }
    catch { if (this.pending?.id === id) this.update({ ...this.view, open: true, busy: false, errorKey: "authorization.inspectionFailed" }); }
    finally {
      this.reading = false;
      const queued = this.queuedAction;
      this.queuedAction = null;
      if (queued !== null && this.pending?.id === queued) { void this.perform(); return; }
      // A new request may have arrived while an old read was being cancelled.
      if (this.pending && this.pending.id !== id) void this.check();
    }
  }
  async perform() {
    if (!this.pending || this.working || !this.view.status?.action) return;
    // A user click during a background read must not silently disappear.
    if (this.reading) {
      this.queuedAction = this.pending.id;
      this.update({ ...this.view, busy: true });
      return;
    }
    const id = this.pending.id;
    const action = this.view.status.action;
    this.working = true;
    this.update({ ...this.view, busy: true, errorKey: null });
    try {
      if (action === "settings" || action === "applications") {
        await this.operations.settings();
        if (this.pending?.id === id) this.update({ ...this.view, busy: false });
      } else this.accept(id, await this.operations.setup());
    } catch (error) {
      if (this.pending?.id !== id) return;
      if (/macos_network_cancelled|core authorization cancelled|context canceled/.test(String(error))) this.cancel();
      else this.update({ ...this.view, busy: false, errorKey: authorizationErrorKey(error) });
    } finally {
      this.working = false;
      if (this.pending && this.pending.id !== id) void this.check();
    }
  }
  cancel() {
    const pending = this.pending;
    this.pending = null;
    this.queuedAction = null;
    ++this.sequence;
    this.update(idleAuthorization);
    pending?.resolve(false);
    if (this.working) void this.operations.cancel().catch(() => undefined);
  }
}
