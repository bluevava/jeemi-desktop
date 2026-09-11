import { zhMacNetwork } from "../../i18n/locales/mac-network";
import type { MihomoRuntimeState } from "../../types/runtime";

import { zhAuthorization } from "../../i18n/locales/authorization";

export function platformFailureMessageKey(code: string): string | null {
  if (code.startsWith("authorization_")) {
    const key = code.slice("authorization_".length);
    return `authorization.status.${Object.hasOwn(zhAuthorization.status, key) ? key : "service_failed"}`;
  }
  if (code.startsWith("macos_network_")) {
    const key = code.slice("macos_network_".length);
    return `macNetwork.status.${Object.hasOwn(zhMacNetwork.status, key) ? key : "helper_failed"}`;
  }
  switch (code) {
    case "external_ui_prepare_failed": return "externalUI.prepareFailed";
    case "linux_core_privileges_blocked": return "runtime.platformErrors.corePrivilegesBlocked";
    case "linux_system_proxy_unavailable": return "runtime.platformErrors.systemProxyUnavailable";
    case "linux_tun_permission_required":
    case "linux_core_permission_required": return "runtime.platformErrors.corePermissionRequired";
    case "linux_core_authorization_unavailable": return "runtime.platformErrors.authorizationUnavailable";
    case "linux_core_authorizer_missing": return "runtime.platformErrors.authorizerMissing";
    case "linux_core_authentication_failed": return "runtime.platformErrors.authenticationFailed";
    case "linux_core_authorization_timeout": return "runtime.platformErrors.authorizationTimeout";
    case "linux_core_authorization_failed": return "runtime.platformErrors.authorizationFailed";
    case "linux_resolver_authorization_unavailable": return "runtime.platformErrors.resolverAuthorizationUnavailable";
    case "linux_resolver_authorization_conflict": return "runtime.platformErrors.resolverAuthorizationConflict";
    case "linux_resolver_authorization_failed": return "runtime.platformErrors.resolverAuthorizationFailed";
    case "linux_core_integrity_failed": return "runtime.platformErrors.coreIntegrityFailed";
    case "linux_core_authorization_unsafe_path": return "runtime.platformErrors.authorizationUnsafePath";
    case "linux_core_authorization_busy": return "runtime.platformErrors.authorizationBusy";
    case "linux_core_authorization_filesystem": return "runtime.platformErrors.authorizationFilesystem";
    case "linux_tun_device_unavailable": return "runtime.platformErrors.tunDeviceUnavailable";
    case "linux_core_not_executable": return "runtime.platformErrors.coreNotExecutable";
    default: return null;
  }
}

export function formatMemoryBytes(
  bytes: number | null,
  locale: string,
): string {
  if (bytes === null || !Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes === 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const unitIndex = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  const value = bytes / 1024 ** unitIndex;
  return `${new Intl.NumberFormat(locale, {
    maximumFractionDigits: value >= 100 ? 0 : 1,
  }).format(value)} ${units[unitIndex]}`;
}

export type RuntimeUptimeUnit = "second" | "minute" | "hour" | "day";

export interface RuntimeUptime {
  value: number;
  unit: RuntimeUptimeUnit;
}

interface MihomoUptimeVisibility {
  state: MihomoRuntimeState | undefined;
  controllerReady: boolean;
  transitioning: boolean;
  hasError: boolean;
}

export function shouldShowMihomoUptime({
  state,
  controllerReady,
  transitioning,
  hasError,
}: MihomoUptimeVisibility): boolean {
  return (
    state === "running" && controllerReady && !transitioning && !hasError
  );
}

export function runtimeUptime(
  startedAt: string,
  nowMilliseconds: number,
): RuntimeUptime | null {
  const startedAtMilliseconds = Date.parse(startedAt);
  if (
    !Number.isFinite(startedAtMilliseconds) ||
    !Number.isFinite(nowMilliseconds)
  ) {
    return null;
  }

  const elapsedSeconds = Math.max(
    0,
    Math.floor((nowMilliseconds - startedAtMilliseconds) / 1000),
  );
  if (elapsedSeconds < 60) {
    return { value: elapsedSeconds, unit: "second" };
  }
  if (elapsedSeconds < 60 * 60) {
    return { value: Math.floor(elapsedSeconds / 60), unit: "minute" };
  }
  if (elapsedSeconds < 24 * 60 * 60) {
    return { value: Math.floor(elapsedSeconds / (60 * 60)), unit: "hour" };
  }
  return {
    value: Math.floor(elapsedSeconds / (24 * 60 * 60)),
    unit: "day",
  };
}
