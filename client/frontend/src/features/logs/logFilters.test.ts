import { describe, expect, it } from "vitest";

import type { MihomoLogEntry } from "../../lib/mihomo/client";
import {
  appendBoundedLogs,
  filterMihomoLogs,
  formatMihomoLogContent,
} from "./logFilters";

const entries: MihomoLogEntry[] = [
  {
    time: "10:20:30",
    level: "info",
    message: "[TCP] connected to 日本.example",
    fields: [{ network: "tcp" }],
    receivedAt: 1,
  },
  {
    time: "10:20:31",
    level: "error",
    message: "dial [failed]*",
    fields: [],
    receivedAt: 2,
  },
];

describe("mihomo log filters", () => {
  it("matches literal log content without treating punctuation as a pattern", () => {
    expect(filterMihomoLogs(entries, "日本", "all")).toEqual([entries[0]]);
    expect(filterMihomoLogs(entries, "[failed]*", "all")).toEqual([
      entries[1],
    ]);
    expect(filterMihomoLogs(entries, "TCP", "all")).toEqual([entries[0]]);
  });

  it("filters exact display levels independently from text search", () => {
    expect(filterMihomoLogs(entries, "", "error")).toEqual([entries[1]]);
    expect(filterMihomoLogs(entries, "connected", "error")).toEqual([]);
  });

  it("keeps structured fields visible and searchable", () => {
    expect(formatMihomoLogContent(entries[0])).toBe(
      '[TCP] connected to 日本.example {"network":"tcp"}',
    );
    expect(filterMihomoLogs(entries, '"network":"tcp"', "all")).toEqual([
      entries[0],
    ]);
  });

  it("retains only the newest entries within the configured bound", () => {
    expect(appendBoundedLogs([1, 2, 3], [4, 5], 3)).toEqual([3, 4, 5]);
    expect(appendBoundedLogs([1], [2], 0)).toEqual([]);
  });
});
