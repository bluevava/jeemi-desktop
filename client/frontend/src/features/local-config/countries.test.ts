import { describe, expect, it } from "vitest";

import { COUNTRY_REGION_CODES, countryFlag } from "./countries";

describe("local selector country options", () => {
  it("provides unique searchable country and region codes", () => {
    expect(new Set(COUNTRY_REGION_CODES).size).toBe(COUNTRY_REGION_CODES.length);
    expect(COUNTRY_REGION_CODES).toContain("DE");
    expect(COUNTRY_REGION_CODES).toContain("TW");
    expect(COUNTRY_REGION_CODES).toContain("EU");
    expect(COUNTRY_REGION_CODES.every((code) => /^[A-Z]{2}$/u.test(code))).toBe(
      true,
    );
  });

  it("renders regional-indicator flags and reserves ZZ for unknown nodes", () => {
    expect(countryFlag("DE")).toBe("🇩🇪");
    expect(countryFlag("tw")).toBe("🇹🇼");
    expect(countryFlag("ZZ")).toBe("");
  });
});
