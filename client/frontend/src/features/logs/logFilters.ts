import type {
  MihomoLogEntry,
  MihomoLogLevel,
} from "../../lib/mihomo/client";

export const mihomoLogBufferLimit = 500;

export type MihomoLogLevelFilter = "all" | MihomoLogLevel;

export function appendBoundedLogs<T>(
  current: readonly T[],
  incoming: readonly T[],
  limit = mihomoLogBufferLimit,
): T[] {
  if (limit <= 0 || (incoming.length === 0 && current.length === 0)) return [];
  return [...current, ...incoming].slice(-Math.max(0, limit));
}

export function formatMihomoLogContent(log: MihomoLogEntry): string {
  const fields = log.fields
    .map((field) => {
      if (typeof field === "string") return field;
      try {
        return JSON.stringify(field);
      } catch {
        return String(field);
      }
    })
    .filter(Boolean)
    .join(" ");
  return fields ? `${log.message} ${fields}`.trim() : log.message;
}

export function filterMihomoLogs<T extends MihomoLogEntry>(
  logs: readonly T[],
  query: string,
  level: MihomoLogLevelFilter,
): T[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  return logs.filter((log) => {
    if (level !== "all" && log.level !== level) return false;
    return (
      !normalizedQuery ||
      formatMihomoLogContent(log)
        .toLocaleLowerCase()
        .includes(normalizedQuery)
    );
  });
}
