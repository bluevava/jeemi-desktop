import { ArrowDownOutlined, ArrowUpOutlined } from "@ant-design/icons";
import { useId, useMemo } from "react";
import { useTranslation } from "react-i18next";

import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { useTraffic } from "../../app/runtime/TrafficContext";
import { FeatureCard } from "../../components/layout/FeatureCard";
import type { MihomoTrafficSample } from "../../lib/mihomo/client";

const pointCount = 60;
const chartWidth = 640;
const chartHeight = 150;
const chartBottom = 142;

export function TrafficChartCard() {
  const { t, i18n } = useTranslation();
  const { runtime } = useRuntimeStatus();
  const { samples, streamState } = useTraffic();
  const session = runtime?.mihomo.controllerSession ?? null;
  const runtimeReady =
    runtime?.mihomo.state === "running" &&
    runtime.mihomo.controllerReady &&
    session !== null;
  const rawID = useId();
  const chartID = rawID.replace(/[^a-zA-Z0-9_-]/g, "");

  const current = samples.at(-1) ?? { up: 0, down: 0, upTotal: 0, downTotal: 0 };
  const paths = useMemo(() => chartPaths(samples), [samples]);
  const live = runtimeReady && streamState === "live";

  return (
    <FeatureCard
      className="runtime-traffic-card"
      helpTopic="trafficChart"
      title={t("home.traffic.title")}
      titleAddon={
        <span className="runtime-traffic-rates">
          <span className="upload">
            <ArrowUpOutlined />
            {formatRate(current.up, i18n.language)}
          </span>
          <span className="download">
            <ArrowDownOutlined />
            {formatRate(current.down, i18n.language)}
          </span>
        </span>
      }
    >
      <div
        aria-label={t("home.traffic.chartLabel")}
        className={`runtime-traffic-chart${live ? "" : " stale"}`}
        role="img"
      >
        <svg preserveAspectRatio="none" viewBox={`0 0 ${chartWidth} ${chartHeight}`}>
          <defs>
            <linearGradient id={`${chartID}-down`} x1="0" x2="0" y1="0" y2="1">
              <stop offset="0" stopColor="var(--color-accent)" stopOpacity="0.42" />
              <stop offset="1" stopColor="var(--color-accent)" stopOpacity="0.04" />
            </linearGradient>
            <linearGradient id={`${chartID}-up`} x1="0" x2="0" y1="0" y2="1">
              <stop offset="0" stopColor="var(--color-positive)" stopOpacity="0.24" />
              <stop offset="1" stopColor="var(--color-positive)" stopOpacity="0.02" />
            </linearGradient>
          </defs>
          <line className="traffic-grid-line" x1="0" x2={chartWidth} y1="75" y2="75" />
          <path d={paths.downArea} fill={`url(#${chartID}-down)`} />
          <path className="traffic-line download" d={paths.downLine} />
          <path d={paths.upArea} fill={`url(#${chartID}-up)`} />
          <path className="traffic-line upload" d={paths.upLine} />
        </svg>
        {!runtimeReady ? (
          <span className="runtime-traffic-empty">{t("home.traffic.requiresCore")}</span>
        ) : null}
      </div>
      <div className="runtime-traffic-legend">
        <span className="download">{t("home.traffic.download")}</span>
        <span className="upload">{t("home.traffic.upload")}</span>
        <span>
          {t("home.traffic.total", {
            down: formatBytes(current.downTotal, i18n.language),
            up: formatBytes(current.upTotal, i18n.language),
          })}
        </span>
      </div>
    </FeatureCard>
  );
}

function chartPaths(samples: MihomoTrafficSample[]) {
  const padded = [
    ...Array(Math.max(0, pointCount - samples.length)).fill({ up: 0, down: 0 }),
    ...samples.slice(-pointCount),
  ] as MihomoTrafficSample[];
  const maximum = Math.max(1, ...padded.flatMap((sample) => [sample.up, sample.down]));
  const downPoints = padded.map((sample, index) => ({
    x: (index / (pointCount - 1)) * chartWidth,
    y: chartBottom - (sample.down / maximum) * 126,
  }));
  const upPoints = padded.map((sample, index) => ({
    x: (index / (pointCount - 1)) * chartWidth,
    y: chartBottom - (sample.up / maximum) * 126,
  }));
  const downLine = smoothPath(downPoints);
  const upLine = smoothPath(upPoints);
  return {
    downLine,
    upLine,
    downArea: `${downLine} L ${chartWidth} ${chartBottom} L 0 ${chartBottom} Z`,
    upArea: `${upLine} L ${chartWidth} ${chartBottom} L 0 ${chartBottom} Z`,
  };
}

function smoothPath(points: Array<{ x: number; y: number }>): string {
  if (points.length === 0) return "";
  let path = `M ${points[0].x} ${points[0].y}`;
  for (let index = 1; index < points.length - 1; index += 1) {
    const point = points[index];
    const next = points[index + 1];
    path += ` Q ${point.x} ${point.y} ${(point.x + next.x) / 2} ${(point.y + next.y) / 2}`;
  }
  const last = points.at(-1)!;
  return `${path} L ${last.x} ${last.y}`;
}

function formatRate(bytes: number, locale: string): string {
  return `${formatBytes(bytes, locale)}/s`;
}

function formatBytes(bytes: number, locale: string): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = Math.max(0, bytes);
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${new Intl.NumberFormat(locale, {
    maximumFractionDigits: value >= 100 || unit === 0 ? 0 : 1,
  }).format(value)} ${units[unit]}`;
}
