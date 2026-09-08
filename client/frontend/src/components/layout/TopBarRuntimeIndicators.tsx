import { CloseCircleOutlined } from "@ant-design/icons";
import { Tooltip } from "antd";
import { useMemo, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";

import { useProxyOverview } from "../../app/runtime/ProxyOverviewContext";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";
import { useTraffic } from "../../app/runtime/TrafficContext";

const leadingFlagPattern = /^(\p{Regional_Indicator}{2})/u;

export function TopBarRuntimeIndicators() {
  const { t, i18n } = useTranslation();
  const { runtime, runtimePreferences } = useRuntimeStatus();
  const { samples: trafficSamples, streamState: trafficState } = useTraffic();
  const { subscriptionState, proxyRuntime } = useProxyOverview();
  const mode =
    runtimePreferences?.outboundMode || runtime?.mihomo.outboundMode || "rule";
  const projection = subscriptionState?.projection ?? null;
  const projectionIsActive = Boolean(
    projection &&
      runtime?.mihomo.state === "running" &&
      runtime.mihomo.controllerReady &&
      runtime.mihomo.subscriptionId === projection.subscriptionId &&
      runtime.mihomo.source.subscriptionRevision === projection.revisionId &&
      runtime.mihomo.source.localConfigId === projection.localConfigId &&
      runtime.mihomo.source.localConfigRevision ===
        projection.localConfigRevision &&
      runtime.mihomo.source.localScriptId === projection.localScriptId &&
      runtime.mihomo.source.localScriptRevision ===
        projection.localScriptRevision &&
      runtime.mihomo.source.ruleProviderOverrideRevision ===
        projection.ruleProviderOverrideRevision &&
      runtime.mihomo.source.fallbackOverrideRevision ===
        projection.fallbackOverrideRevision &&
      runtime.mihomo.source.geoDataFingerprint === projection.geoDataFingerprint,
  );
  const exits = useMemo(() => {
    if (!projectionIsActive || !projection || !proxyRuntime || mode === "direct") {
      return [];
    }
    if (mode === "global") {
      const node = resolveSelectedLeaf(
        "GLOBAL",
        proxyRuntime.selections,
        proxyRuntime.allProxies[0]?.name || "",
      );
      return node
        ? [
            {
              selector: t("subscription.selector.globalProxy"),
              node,
              flag: leadingCountryFlag(node),
            },
          ]
        : [];
    }
    return projection.selectors
      .filter((selector) => selector.referencedByRules)
      .map((selector) => {
        const node = resolveSelectedLeaf(
          selector.name,
          proxyRuntime.selections,
          selector.defaultSelection,
        );
        return {
          selector: selector.name,
          node,
          flag: leadingCountryFlag(node),
        };
      })
      .filter((item) => item.node);
  }, [mode, projection, projectionIsActive, proxyRuntime, t]);
  const traffic = trafficSamples.at(-1) ?? {
    up: 0,
    down: 0,
    upTotal: 0,
    downTotal: 0,
  };
  const twoRows = exits.length >= 5;
  const flagStyle = twoRows
    ? ({ "--selector-flag-columns": Math.ceil(exits.length / 2) } as CSSProperties)
    : undefined;

  return (
    <div className="top-bar-runtime-indicators">
      <Tooltip
        title={t(
          runtime?.mihomo.state === "running"
            ? `home.traffic.states.${trafficState}`
            : "home.traffic.states.offline",
        )}
      >
        <span
          aria-label={t("topBar.traffic", {
            up: formatTitleBarRate(traffic.up, i18n.language),
            down: formatTitleBarRate(traffic.down, i18n.language),
          })}
          className={`top-bar-traffic${trafficState === "live" ? "" : " stale"}`}
        >
          ↑ {formatTitleBarRate(traffic.up, i18n.language)}/ {formatTitleBarRate(traffic.down, i18n.language)}↓
        </span>
      </Tooltip>
      {exits.length > 0 ? (
        <div
          aria-label={t("topBar.selectorExits")}
          className={`top-bar-selector-flags${twoRows ? " two-rows" : ""}`}
          style={flagStyle}
        >
          {exits.map((exit) => (
            <Tooltip
              key={exit.selector}
              title={t("topBar.selectorExit", {
                selector: exit.selector,
                node: exit.node,
              })}
            >
              <span className="top-bar-selector-flag">
                {exit.flag || <CloseCircleOutlined />}
              </span>
            </Tooltip>
          ))}
        </div>
      ) : null}
    </div>
  );
}

export function leadingCountryFlag(name: string): string {
  return name.match(leadingFlagPattern)?.[1] ?? "";
}

export function resolveSelectedLeaf(
  selector: string,
  selections: Record<string, string>,
  fallback: string,
): string {
  const visited = new Set<string>([selector]);
  let current = selections[selector] || fallback;
  while (current && selections[current] && !visited.has(current)) {
    visited.add(current);
    current = selections[current];
  }
  return current;
}

export function formatTitleBarRate(bytes: number, locale: string): string {
  const safeBytes = Math.max(0, Number.isFinite(bytes) ? bytes : 0);
  const [divisor, unit] =
    safeBytes >= 1024 * 1024
      ? [1024 * 1024, "m"]
      : safeBytes >= 1024
        ? [1024, "k"]
        : [1, "b"];
  const value = safeBytes / divisor;
  return `${new Intl.NumberFormat(locale, {
    maximumFractionDigits: value >= 100 || divisor === 1 ? 0 : 1,
  }).format(value)} ${unit}`;
}
