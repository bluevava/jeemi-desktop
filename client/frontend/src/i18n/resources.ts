import { enUS } from "./locales/en-US";
import { zhCN } from "./locales/zh-CN";

export const resources = {
  "zh-CN": { translation: zhCN },
  "en-US": { translation: enUS },
} as const;

export type AppLanguage = keyof typeof resources;

export function flattenTranslationKeys(
  value: Record<string, unknown>,
  prefix = "",
): string[] {
  return Object.entries(value).flatMap(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key;
    return typeof child === "object" && child !== null
      ? flattenTranslationKeys(child as Record<string, unknown>, path)
      : [path];
  });
}
