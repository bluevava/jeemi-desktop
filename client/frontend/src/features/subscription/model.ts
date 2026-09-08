import type { LocalConfigSummary } from "../../types/localConfig";
import type { SubscriptionSummary } from "../../types/subscription";

export function formatSubscriptionBytes(
  bytes: number,
  locale: string,
): string {
  const safeBytes = Number.isFinite(bytes) && bytes > 0 ? bytes : 0;
  if (safeBytes < 1024) {
    return `${safeBytes} B`;
  }
  const units = ["KiB", "MiB", "GiB", "TiB", "PiB"];
  let value = safeBytes / 1024;
  let unit = units[0];
  for (let index = 1; index < units.length && value >= 1024; index += 1) {
    value /= 1024;
    unit = units[index];
  }
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(value)} ${unit}`;
}

export function subscriptionUsedBytes(subscription: SubscriptionSummary): number {
  const { uploadBytes, downloadBytes } = subscription.remoteProfile;
  const used = uploadBytes + downloadBytes;
  return Number.isSafeInteger(used) && used > 0 ? used : 0;
}

export function subscriptionUsagePercent(
  subscription: SubscriptionSummary,
): number {
  const total = subscription.remoteProfile.totalBytes;
  if (!Number.isFinite(total) || total <= 0) {
    return 0;
  }
  return Math.min(100, Math.max(0, (subscriptionUsedBytes(subscription) / total) * 100));
}

export function formatSubscriptionUpdateAge(
  timestamp: string,
  locale: string,
  now = Date.now(),
): string {
  const value = Date.parse(timestamp);
  if (!Number.isFinite(value)) {
    return "—";
  }
  const seconds = Math.round((value - now) / 1000);
  const absolute = Math.abs(seconds);
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: "always" });
  if (absolute < 60) {
    return formatter.format(seconds, "second");
  }
  if (absolute < 3600) {
    return formatter.format(Math.round(seconds / 60), "minute");
  }
  if (absolute < 86400) {
    return formatter.format(Math.round(seconds / 3600), "hour");
  }
  return formatter.format(Math.round(seconds / 86400), "day");
}

export function formatSubscriptionExpiry(
  expiresAt: number,
  locale: string,
): string | null {
  if (!Number.isSafeInteger(expiresAt) || expiresAt <= 0) {
    return null;
  }
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(
    new Date(expiresAt * 1000),
  );
}

export function associatedLocalConfigName(
  subscription: SubscriptionSummary,
  configs: LocalConfigSummary[],
): string | null {
  if (!subscription.localConfigId) {
    return null;
  }
  return (
    configs.find((config) => config.id === subscription.localConfigId)?.name ??
    null
  );
}
