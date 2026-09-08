import type { JeemiUpdateState } from "../../types/appUpdate";

const errorCodes = [
  "check_failed", "no_release", "rate_limited", "invalid_release", "current_version_invalid",
  "asset_unavailable", "busy", "check_again", "download_failed", "checksum_failed",
  "package_invalid", "location_unwritable", "restart_failed", "replace_failed", "rollback_failed",
  "cancelled", "stop_failed",
] as const;

export function updateErrorKey(error: unknown) {
  const code = String(error instanceof Error ? error.message : error).replace(/^Error: /, "");
  return errorCodes.find((item) => code === `jeemi_update_${item}`) ?? "check_failed";
}

export function updateInProgress(phase: JeemiUpdateState["phase"]) {
  return phase === "downloading" || phase === "verifying" || phase === "restarting";
}

export function downloadPercent(state: JeemiUpdateState) {
  return state.total > 0 ? Math.max(0, Math.min(100, Math.floor(state.downloaded * 100 / state.total))) : 0;
}
