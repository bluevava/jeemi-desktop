import { LoadingOutlined, ThunderboltOutlined } from "@ant-design/icons";
import { Tooltip } from "antd";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import type {
  SelectorSortMode,
  SubscriptionSelectorMember,
} from "../../../types/subscription";
import { sortSelectorMembers } from "../selectorSort";
import type { SelectorIndex } from "../selectorNavigation";
import type { DescribeSelectorEgress } from "../selectorEgress";
import type { ProxyDelayResult } from "../useProxyDelayQueue";
import { SelectorGroupMember } from "./SelectorGroupMember";

const emptyDelayResults: Readonly<Record<string, ProxyDelayResult>> = {};

interface SelectorNodeGridProps {
  describeEgress: DescribeSelectorEgress;
  selectorsByName: SelectorIndex;
  onOpenGroup: (name: string) => void;
  selectionEnabled: boolean;
  busyDelayNodes: ReadonlySet<string>;
  busySelection: string;
  currentSelection: string;
  delayQueueActive: boolean;
  delayResults: Readonly<Record<string, ProxyDelayResult>>;
  groupName: string;
  members: SubscriptionSelectorMember[];
  onSelect: (group: string, proxy: string) => Promise<void>;
  onTestDelay: (proxy: string) => Promise<void>;
  runtimeReady: boolean;
  sortMode: SelectorSortMode;
}

export function SelectorNodeGrid({
  describeEgress,
  selectorsByName,
  onOpenGroup,
  selectionEnabled,
  busyDelayNodes,
  busySelection,
  currentSelection,
  delayQueueActive,
  delayResults,
  groupName,
  members,
  onSelect,
  onTestDelay,
  runtimeReady,
  sortMode,
}: SelectorNodeGridProps) {
  const { t, i18n } = useTranslation();
  const sortingDelays = sortMode === "delay" ? delayResults : emptyDelayResults;
  const sortedMembers = useMemo(
    () => sortSelectorMembers(members, sortMode, sortingDelays, i18n.language),
    [members, sortMode, sortingDelays, i18n.language],
  );
  return (
    <div className="selector-node-grid">
      {sortedMembers.map((member) => {
        if (member.source === "group") {
          const selector = selectorsByName.get(member.name);
          return <SelectorGroupMember
            key={`group-${member.name}`}
            name={member.name}
            selector={selector}
            egress={selector ? describeEgress(selector) : "—"}
            selected={currentSelection === member.name}
            selectionDisabled={!runtimeReady || !selectionEnabled || busySelection === groupName}
            runtimeReady={runtimeReady}
            onSelect={() => void onSelect(groupName, member.name)}
            onOpen={() => onOpenGroup(member.name)}
          />;
        }
        const delaySupported =
          member.source === "proxy" || member.source === "provider";
        const result = delayResults[member.name];
        const delayBusy = busyDelayNodes.has(member.name);
        const delayClass = result
          ? result.status === "error"
            ? " error"
            : result.delay <= 200
              ? " fast"
              : result.delay <= 500
                ? " medium"
                : " slow"
          : "";
        return (
          <div
            className={`selector-node${
              currentSelection === member.name ? " default" : ""
            }${delaySupported ? "" : " no-delay"}`}
            key={`${member.source}-${member.providerName}-${member.name}`}
          >
            <button
              aria-pressed={currentSelection === member.name}
              className="selector-node-select"
              disabled={!runtimeReady || !selectionEnabled || busySelection === groupName}
              onClick={() => void onSelect(groupName, member.name)}
              type="button"
            >
              <strong title={member.name}>{member.name}</strong>
              <span>
                {member.type || t("subscription.selector.unknownType")}
                {member.providerName ? ` · ${member.providerName}` : ""}
              </span>
            </button>
            {delaySupported ? (
              <Tooltip
                title={t(
                  !runtimeReady
                    ? "subscription.delay.requiresCore"
                    : delayQueueActive
                      ? "subscription.delay.queueBusy"
                      : result?.source === "mihomo"
                        ? "subscription.delay.retestAutomatic"
                        : result
                          ? "subscription.delay.retestCached"
                          : "subscription.delay.test",
                )}
              >
                <span className="selector-node-delay-wrap">
                  <button
                    aria-label={t(
                      result
                        ? "subscription.delay.retestNode"
                        : "subscription.delay.testNode",
                      { name: member.name },
                    )}
                    className={`selector-node-delay${delayClass}`}
                    disabled={!runtimeReady || delayQueueActive}
                    onClick={() => void onTestDelay(member.name)}
                    type="button"
                  >
                    {delayBusy ? (
                      <LoadingOutlined spin />
                    ) : result?.status === "success" ? (
                      `${result.delay} ms`
                    ) : result?.status === "error" ? (
                      t("subscription.delay.failed")
                    ) : (
                      <ThunderboltOutlined />
                    )}
                  </button>
                </span>
              </Tooltip>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
