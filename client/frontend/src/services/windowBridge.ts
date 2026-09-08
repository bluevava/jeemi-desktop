declare global {
  interface Window {
    runtime?: {
      EventsOn?: (
        eventName: string,
        callback: (...data: unknown[]) => void,
      ) => (() => void) | void;
      WindowMinimise?: () => void;
      WindowToggleMaximise?: () => void;
    };
  }
}

export const windowActivityEvent = "jeemi:window-activity";

export function usesNativeWindowControls(platform?: string): boolean {
  if (platform !== undefined) return platform === "darwin";
  // Reserve AppKit's buttons on the first paint, before Go bootstrap returns.
  // This hint affects layout only; the backend platform takes precedence.
  return typeof navigator !== "undefined" && /\bMacintosh\b/.test(navigator.userAgent);
}

export type WindowActivity = "visible" | "hidden";

export function parseWindowActivity(value: unknown): WindowActivity | null {
  return value === "visible" || value === "hidden" ? value : null;
}

export function subscribeWindowActivity(
  listener: (activity: WindowActivity) => void,
): () => void {
  const unsubscribe = window.runtime?.EventsOn?.(
    windowActivityEvent,
    (value: unknown) => {
      const activity = parseWindowActivity(value);
      if (activity) listener(activity);
    },
  );
  return typeof unsubscribe === "function" ? unsubscribe : () => undefined;
}

export const windowBridge = {
  minimise: () => window.runtime?.WindowMinimise?.(),
  toggleMaximise: () => window.runtime?.WindowToggleMaximise?.(),
};
