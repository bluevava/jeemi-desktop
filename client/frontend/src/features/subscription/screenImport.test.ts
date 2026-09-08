import { describe, expect, it } from "vitest";
import type { SubscriptionScreenImportResult } from "../../types/subscription";
import { screenImportErrorKey } from "./screenImport";

describe("screen QR import outcomes", () => {
  it("keeps cancellation quiet without requiring a replacement state", () => {
    expect(screenImportErrorKey({ cancelled: true })).toBeNull();
  });

  it.each([
    ["screen_capture_timeout", "screenTimeout"],
    ["screen_capture_unavailable", "screenUnavailable"],
    ["screen_capture_denied", "screenDenied"],
    ["screen_capture_failed", "screenCapture"],
    ["screen_capture_image", "screenImage"],
    ["screen_capture_too_large", "screenTooLarge"],
    ["screen_capture_busy", "screenBusy"],
    ["screen_qr_not_found", "screenNoQR"],
  ])("maps %s to an actionable localized message", (code, key) => {
    expect(screenImportErrorKey({ cancelled: false, errorCode: code })).toBe(`subscription.errors.${key}`);
  });

  it("does not expose unknown backend messages or accept incomplete success", () => {
    for (const result of [{ cancelled: false }, { cancelled: false, errorCode: "file:///private/screen.png" }, { cancelled: false, errorCode: "__proto__" }] satisfies SubscriptionScreenImportResult[]) {
      expect(screenImportErrorKey(result)).toBe("subscription.errors.screenImport");
    }
  });
});
