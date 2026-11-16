import { normalizationErrorKey } from "./normalization";

const fileErrors: Record<string, string> = {
  subscription_file_source_missing: "subscription.errors.fileSourceMissing",
  subscription_file_unavailable: "subscription.errors.fileSourceUnavailable",
  subscription_file_changed: "subscription.errors.fileSourceChanged",
  subscription_file_selection_expired: "subscription.errors.fileSelectionExpired",
};

export function subscriptionSourceErrorKey(cause: unknown, fallback: string): string {
  const message = cause instanceof Error ? cause.message : typeof cause === "string" ? cause : "";
  return Object.hasOwn(fileErrors, message) ? fileErrors[message] : normalizationErrorKey(cause, fallback);
}
