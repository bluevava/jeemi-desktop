import type { AppLanguage } from "./resources";
import { readUIPreference } from "../lib/uiPreferences";

export const languageStorageKey = "jeemi.ui.language";

export function resolveLanguage(saved: string | null, systemLanguage: string): AppLanguage {
  if (saved === "zh-CN" || saved === "en-US") return saved;
  return systemLanguage.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
}

export function initialLanguage(): AppLanguage {
  return resolveLanguage(
    readUIPreference(languageStorageKey),
    typeof navigator === "undefined" ? "zh-CN" : navigator.language,
  );
}
