import type { SubscriptionScreenImportResult } from "../../types/subscription";

const screenErrorKeys: Record<string, string> = {
  screen_capture_timeout: "subscription.errors.screenTimeout",
  screen_capture_unavailable: "subscription.errors.screenUnavailable",
  screen_capture_denied: "subscription.errors.screenDenied",
  screen_capture_failed: "subscription.errors.screenCapture",
  screen_capture_image: "subscription.errors.screenImage",
  screen_capture_too_large: "subscription.errors.screenTooLarge",
  screen_capture_busy: "subscription.errors.screenBusy",
  screen_qr_not_found: "subscription.errors.screenNoQR",
};

// Only allow known localization keys, never display backend exception text or
// a screenshot path. A cancelled request must leave the current state intact.
export function screenImportErrorKey(result: SubscriptionScreenImportResult): string | null {
  if (result.cancelled) return null;
  if (result.errorCode) return Object.hasOwn(screenErrorKeys, result.errorCode)
    ? screenErrorKeys[result.errorCode]
    : "subscription.errors.screenImport";
  return result.state ? null : "subscription.errors.screenImport";
}
