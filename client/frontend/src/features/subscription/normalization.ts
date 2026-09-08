import type { TFunction } from "i18next";
import { enSubscriptionNormalization } from "../../i18n/locales/subscription-normalization";
import type { SubscriptionNormalizationDiagnostic } from "../../types/subscription";

const knownCodes = new Set(Object.keys(enSubscriptionNormalization.codes));

export function normalizationMessage(t: TFunction, diagnostic: SubscriptionNormalizationDiagnostic): string {
  const code = knownCodes.has(diagnostic.code) ? diagnostic.code : "unknown";
  const message = t(`subscription.normalization.codes.${code}`);
  return diagnostic.line > 0
    ? t("subscription.normalization.position", { line: diagnostic.line, message })
    : message;
}

// Wails serializes errors as strings. Accept only the converter's fixed codes;
// never display an arbitrary error, URI, credential or source field name here.
export function normalizationErrorKey(cause: unknown, fallback: string): string {
  const message = cause instanceof Error ? cause.message : typeof cause === "string" ? cause : "";
  const match = /subscription normalization: ([a-z_]+) \(line \d+, field [a-z0-9_.-]*\)/.exec(message);
  return match && knownCodes.has(match[1]) ? `subscription.normalization.codes.${match[1]}` : fallback;
}
