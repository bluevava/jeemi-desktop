import type { UIFailure } from "../../types/uiDiagnostics";
import { persistUIFailure } from "../../services/uiRecoveryBridge";

export function diagnosticPage(path: string): string {
  const page = path.replace(/^#/, "").split(/[?#]/, 1)[0];
  if (/^\/(home|subscriptions|config|connections|logs|tools)(\/settings)?$/.test(page)) {
    return page.slice(1).replace("/", "_");
  }
  if (/^\/config\/scripts\/(new|[^/]+\/edit)$/.test(page)) return "script_editor";
  if (/^\/config\/(new|[^/]+\/edit)$/.test(page)) return "config_editor";
  return "unknown";
}

export function describeUIFailure(
  error: unknown,
  kind: UIFailure["kind"],
  scope: UIFailure["scope"],
  path: string,
  assets: ReadonlySet<string> = new Set(),
): UIFailure {
  const result: UIFailure = {
    kind, scope, page: diagnosticPage(path), errorType: "unknown", code: "unknown", frames: [],
  };
  // Even reading an arbitrary rejected value may invoke a throwing getter.
  try {
    if (!(error instanceof Error)) return result;
    if (["Error", "TypeError", "RangeError", "ReferenceError", "SyntaxError", "DOMException"].includes(error.name)) {
      result.errorType = error.name;
    }
    const message = typeof error.message === "string" ? error.message.slice(0, 2048) : "";
    if (/Maximum update depth|Too many re-renders|Minified React error #(185|301)\b/i.test(message)) result.code = "render_loop";
    else if (/Rendered (more|fewer) hooks|order of Hooks|Minified React error #(300|310|311)\b/i.test(message)) result.code = "hook_order";
    else if (/Objects are not valid as a React child|Minified React error #31\b/i.test(message)) result.code = "invalid_child";
    else if (/Cannot (read|set) propert|undefined is not|is not a function/i.test(message)) result.code = "invalid_value";
    else if (/Maximum call stack|too much recursion/i.test(message)) result.code = "stack_overflow";
    else if (/dynamically imported module|Loading chunk/i.test(message)) result.code = "asset_load";
    // Only Vite release asset basenames + source positions survive. No URL,
    // query, arbitrary source path or error text is retained.
    const stack = typeof error.stack === "string" ? error.stack.slice(0, 16000) : "";
    for (const match of stack.matchAll(/\/assets\/(index-[A-Za-z0-9_-]{8,32}\.js):(\d{1,7}):(\d{1,7})(?=[)\s]|$)/g)) {
      if (!assets.has(match[1])) continue;
      result.frames.push({ asset: match[1], line: Number(match[2]), column: Number(match[3]) });
      if (result.frames.length === 5) break;
    }
  } catch {
    // Diagnostics must never become a second source of UI failures.
  }
  return result;
}

let remainingReports = 20;
const reported = new Map<string, Promise<boolean>>();

function currentReleaseAssets(): Set<string> {
  const assets = new Set<string>();
  if (typeof document === "undefined") return assets;
  for (const script of document.scripts) {
    if (!script.src) continue;
    try {
      const url = new URL(script.src);
      const match = url.pathname.match(/^\/assets\/(index-[A-Za-z0-9_-]{8,32}\.js)$/);
      if (url.origin === window.location.origin && match) assets.add(match[1]);
    } catch {
      // A malformed script element must not prevent a structural report.
    }
  }
  return assets;
}

export function reportUIFailure(
  error: unknown, kind: UIFailure["kind"], scope: UIFailure["scope"],
): Promise<boolean> {
  try {
    const input = describeUIFailure(error, kind, scope, window.location.hash, currentReleaseAssets());
    const key = JSON.stringify(input);
    const previous = reported.get(key);
    if (previous) return previous;
    if (remainingReports <= 0) return Promise.resolve(false);
    remainingReports--;
    const result = Promise.resolve().then(() => persistUIFailure(input)).catch(() => false);
    reported.set(key, result);
    return result;
  } catch {
    return Promise.resolve(false);
  }
}

export function installUIErrorReporting(): () => void {
  const onError = (event: ErrorEvent) => {
    void reportUIFailure(event.error, "unhandled_error", "global");
  };
  const onRejection = (event: PromiseRejectionEvent) => {
    void reportUIFailure(event.reason, "unhandled_rejection", "global");
  };
  window.addEventListener("error", onError);
  window.addEventListener("unhandledrejection", onRejection);
  return () => {
    window.removeEventListener("error", onError);
    window.removeEventListener("unhandledrejection", onRejection);
  };
}
