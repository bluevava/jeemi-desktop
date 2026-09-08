import type { ProxyAuthorizationStatus } from "../../types/authorization";

export type HelperHealth = "ready" | "failed" | "update" | "not_installed";

export function helperHealth(status: ProxyAuthorizationStatus): HelperHealth {
  if (!status.present) return "not_installed";
  if (status.action === "update") return "update";
  return status.ready ? "ready" : "failed";
}
