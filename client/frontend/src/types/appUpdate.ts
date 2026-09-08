export interface JeemiUpdateResult {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
}

export interface JeemiUpdateState {
  phase: "idle" | "checking" | "downloading" | "verifying" | "restarting" | "succeeded" | "failed" | "cancelled";
  version: string;
  downloaded: number;
  total: number;
  error: string;
  jobId: string;
}
