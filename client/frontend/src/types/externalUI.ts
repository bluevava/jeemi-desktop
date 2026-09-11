export interface ZashboardRelease {
  version: string;
  publishedAt: string;
  size: number;
  sha256: string;
}

export interface ZashboardState {
  installed: ZashboardRelease[];
  available: ZashboardRelease[];
  latestVersion: string;
  lastCheckedAt: string;
  phase: "" | "checking" | "downloading" | "installing";
  downloadingVersion: string;
  receivedBytes: number;
  totalBytes: number;
  lastError: string;
  selectedVersion: string;
  enabled: boolean;
}
