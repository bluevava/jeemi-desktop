import { describe, expect, it } from "vitest";

import { defaultGeoDataPreferences } from "../../types/geodata";
import {
  geoDataPreferencesEqual,
  isSafeGeoDataURL,
  visibleGeoDataKinds,
} from "./model";

describe("GEO data settings model", () => {
  it("shows exactly one GeoIP family alongside GeoSite and ASN", () => {
    expect(visibleGeoDataKinds(defaultGeoDataPreferences)).toEqual([
      "geoip-mmdb",
      "geosite",
      "asn",
    ]);
    expect(
      visibleGeoDataKinds({
        ...defaultGeoDataPreferences,
        geoIpMode: "dat",
      }),
    ).toEqual(["geoip-dat", "geosite", "asn"]);
  });

  it("detects changes to nested custom URLs", () => {
    const copy = {
      ...defaultGeoDataPreferences,
      customUrls: { ...defaultGeoDataPreferences.customUrls },
    };
    expect(geoDataPreferencesEqual(defaultGeoDataPreferences, copy)).toBe(true);
    copy.customUrls.asn = "https://example.test/asn.mmdb";
    expect(geoDataPreferencesEqual(defaultGeoDataPreferences, copy)).toBe(false);
  });

  it("only accepts HTTPS URLs without embedded user information", () => {
    expect(isSafeGeoDataURL("https://example.test/geosite.dat")).toBe(true);
    expect(isSafeGeoDataURL("http://example.test/geosite.dat")).toBe(false);
    expect(isSafeGeoDataURL("https://user@example.test/geosite.dat")).toBe(false);
    expect(isSafeGeoDataURL("not a URL")).toBe(false);
  });
});
