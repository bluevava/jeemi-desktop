import type {
  GeoDataKind,
  GeoDataPreferences,
  GeoDataURLs,
} from "../../types/geodata";

export const geoDataURLFields: Array<{
  kind: GeoDataKind;
  field: keyof GeoDataURLs;
}> = [
  { kind: "geoip-mmdb", field: "geoIpMmdb" },
  { kind: "geoip-dat", field: "geoIpDat" },
  { kind: "geosite", field: "geoSite" },
  { kind: "asn", field: "asn" },
];

export function visibleGeoDataKinds(
  preferences: GeoDataPreferences,
): GeoDataKind[] {
  return [
    preferences.geoIpMode === "dat" ? "geoip-dat" : "geoip-mmdb",
    "geosite",
    "asn",
  ];
}

export function geoDataPreferencesEqual(
  left: GeoDataPreferences,
  right: GeoDataPreferences,
): boolean {
  return (
    left.geoIpMode === right.geoIpMode &&
    left.loader === right.loader &&
    left.source === right.source &&
    geoDataURLFields.every(
      ({ field }) => left.customUrls[field] === right.customUrls[field],
    )
  );
}

export function isSafeGeoDataURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    return (
      parsed.protocol === "https:" &&
      Boolean(parsed.hostname) &&
      parsed.username === "" &&
      parsed.password === ""
    );
  } catch {
    return false;
  }
}
