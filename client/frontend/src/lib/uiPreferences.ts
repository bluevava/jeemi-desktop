// UI preferences are optional. WebView storage can be denied or run out of
// space; either failure must leave the current in-memory interface usable.
export function readUIPreference(key: string): string | null {
  try {
    return globalThis.localStorage?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

export function writeUIPreference(key: string, value: string): void {
  try {
    globalThis.localStorage?.setItem(key, value);
  } catch {
    // Keep the preference for this session even if persistence is unavailable.
  }
}

export function initialTheme(): "light" | "dark" {
  const saved = readUIPreference("jeemi.ui.theme");
  if (saved === "light" || saved === "dark") return saved;
  try {
    return globalThis.matchMedia?.("(prefers-color-scheme: dark)").matches
      ? "dark" : "light";
  } catch {
    return "light";
  }
}
