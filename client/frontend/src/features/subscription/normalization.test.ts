import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { zhSubscriptionNormalization } from "../../i18n/locales/subscription-normalization";
import { normalizationErrorKey, normalizationMessage } from "./normalization";

describe("subscription conversion diagnostics", () => {
  it("shows fixed reasons and positions without echoing sensitive values", async () => {
    const i18n = i18next.createInstance();
    await i18n.init({ lng: "zh", resources: { zh: { translation: { subscription: { normalization: zhSubscriptionNormalization } } } } });
    const result = normalizationMessage(i18n.t.bind(i18n), { code: "certificate_pin_mismatch", line: 3, field: "private-value" });
    expect(result).toContain("第 3 行");
    expect(result).toContain("证书 PIN");
    expect(result).not.toContain("private-value");
    expect(normalizationMessage(i18n.t.bind(i18n), { code: "https://private", line: 0, field: "" })).not.toContain("private");
  });

  it("recognizes Wails conversion errors but never treats arbitrary error text as a translation key", () => {
    expect(normalizationErrorKey(new Error("subscription normalization: conflicting_alias (line 2, field certificate_pin)"), "fallback")).toBe("subscription.normalization.codes.conflicting_alias");
    expect(normalizationErrorKey("subscription normalization: new_unknown_code (line 2, field )", "fallback")).toBe("fallback");
    expect(normalizationErrorKey("password=private", "fallback")).toBe("fallback");
  });
});
