import type { RuntimeStatus } from "./runtime";

export type GeoDataKind = "geoip-mmdb" | "geoip-dat" | "geosite" | "asn";
export type GeoIPMode = "mmdb" | "dat";
export type GeoDataLoader = "memconservative" | "standard";
export type GeoDataSource = "official" | "custom";
export type GeoDataHealthStatus = "ready" | "pending" | "missing" | "invalid";

export interface GeoDataHealth {
  status: GeoDataHealthStatus;
  missingKinds: GeoDataKind[];
  invalidKinds: GeoDataKind[];
}

export interface GeoDataURLs {
  geoIpMmdb: string;
  geoIpDat: string;
  geoSite: string;
  asn: string;
}

export interface GeoDataPreferences {
  geoIpMode: GeoIPMode;
  loader: GeoDataLoader;
  source: GeoDataSource;
  customUrls: GeoDataURLs;
}

export interface GeoDataAssetState {
  kind: GeoDataKind;
  canonicalName: string;
  installed: boolean;
  active: boolean;
  pending: boolean;
  sha256: string;
  activeSha256: string;
  availableSha256: string;
  size: number;
  installedAt: string;
  source: string;
  sourceUrl: string;
  validatedCoreVersion: string;
  publisherVerified: boolean;
}

export interface GeoDataState {
  directory: string;
  runtimeDirectory: string;
  preferences: GeoDataPreferences;
  activeGeoIpMode: GeoIPMode;
  activeLoader: GeoDataLoader;
  lastCheckedAt: string;
  downloadingKind: GeoDataKind | "all" | "";
  restartRequired: boolean;
  health: GeoDataHealth;
  assets: GeoDataAssetState[];
}

export interface GeoDataDialogLabels {
  title: string;
  filterName: string;
}

export interface GeoDataDialogImportResult {
  cancelled: boolean;
  state: GeoDataState;
}

export type GeoDataApplyResult = RuntimeStatus;

export const defaultGeoDataPreferences: GeoDataPreferences = {
  geoIpMode: "mmdb",
  loader: "memconservative",
  source: "official",
  customUrls: {
    geoIpMmdb:
      "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb",
    geoIpDat:
      "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat",
    geoSite:
      "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
    asn:
      "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/GeoLite2-ASN.mmdb",
  },
};
