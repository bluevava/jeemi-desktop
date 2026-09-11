interface PendingIcon {
  id: string;
  promise: Promise<string>;
  users: number;
  cancelled: boolean;
  finished: boolean;
}

const cancelled = () => new Error("selector_icon_cancelled");

// Share successful images and requests; Go owns persistent bytes and network IO.
export class SelectorIconCache {
  private readonly images = new Map<string, string>();
  private readonly pending = new Map<string, PendingIcon>();
  private readonly failures = new Map<string, { retryKey: string; time: number; error: unknown }>();
  private bytes = 0;

  constructor(
    private readonly fetch: (id: string, address: string) => Promise<string>,
    private readonly cancel: (id: string) => Promise<void>,
  ) {}

  load(address: string, signal: AbortSignal, retryKey = ""): Promise<string> {
    if (signal.aborted) return Promise.reject(cancelled());
    const cached = this.images.get(address);
    if (cached) {
      this.images.delete(address);
      this.images.set(address, cached);
      return Promise.resolve(cached);
    }
    const failed = this.failures.get(address);
    if (failed?.retryKey === retryKey && Date.now() - failed.time < 1000) return Promise.reject(failed.error);
    let request = this.pending.get(address);
    if (!request) {
      const next: PendingIcon = {
        id: crypto.randomUUID(), users: 0, cancelled: false, finished: false,
        promise: Promise.resolve(""),
      };
      next.promise = this.fetch(next.id, address).then((source) => {
        if (next.cancelled) throw cancelled();
        if (!/^data:image\/(png|jpeg|gif|webp|x-icon|svg\+xml);base64,/.test(source) || source.length > 1_400_000) {
          throw new Error("selector_icon_unavailable");
        }
        this.images.set(address, source);
        this.failures.delete(address);
        this.bytes += source.length * 2;
        while (this.images.size > 128 || this.bytes > 24 * 1024 * 1024) {
          const oldest = this.images.entries().next().value;
          if (!oldest) break;
          this.images.delete(oldest[0]);
          this.bytes -= oldest[1].length * 2;
        }
        return source;
      }).catch((error: unknown) => {
        if (!next.cancelled) {
          this.failures.delete(address);
          this.failures.set(address, { retryKey, time: Date.now(), error });
          if (this.failures.size > 256) this.failures.delete(this.failures.keys().next().value!);
        }
        throw error;
      }).finally(() => {
        next.finished = true;
        if (this.pending.get(address) === next) this.pending.delete(address);
      });
      this.pending.set(address, next);
      request = next;
    }
    const shared = request;
    shared.users++;
    return new Promise((resolve, reject) => {
      let released = false;
      const release = () => {
        if (released) return;
        released = true;
        signal.removeEventListener("abort", abort);
        shared.users--;
        if (shared.users === 0 && !shared.finished) {
          shared.cancelled = true;
          if (this.pending.get(address) === shared) this.pending.delete(address);
          void this.cancel(shared.id).catch(() => undefined);
        }
      };
      const abort = () => { release(); reject(cancelled()); };
      signal.addEventListener("abort", abort, { once: true });
      shared.promise.then(
        (source) => { if (!released) { release(); resolve(source); } },
        (error: unknown) => { if (!released) { release(); reject(error); } },
      );
    });
  }
}

export function waitForIconRetry(milliseconds: number, signal: AbortSignal): Promise<void> {
  if (signal.aborted) return Promise.reject(cancelled());
  return new Promise((resolve, reject) => {
    const abort = () => { clearTimeout(timer); reject(cancelled()); };
    const timer = setTimeout(() => { signal.removeEventListener("abort", abort); resolve(); }, milliseconds);
    signal.addEventListener("abort", abort, { once: true });
  });
}
