import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import type {
  SelectorSortMode,
  SubscriptionSelectorMember,
} from "../../../types/subscription";
import { sortSelectorMembers } from "../selectorSort";
import type { SelectorIndex } from "../selectorNavigation";
import type { ResolveSelectorEgress } from "../selectorEgress";
import type { ProxyDelayResult } from "../useProxyDelayQueue";
import { SelectorGroupMember } from "./SelectorGroupMember";
import { SelectorDelayBadge } from "./SelectorDelayBadge";

const emptyDelayResults: Readonly<Record<string, ProxyDelayResult>> = {};

interface SelectorNodeGridProps {
  resolveEgress: ResolveSelectorEgress;
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
  resolveEgress,
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
  const delayNodes = useMemo(() => new Map(members.map((member) => {
    if (member.source !== "group") {
      return [member.name, member.source === "builtin" ? undefined : member.name];
    }
    const selector = selectorsByName.get(member.name);
    const egress = selector ? resolveEgress(selector) : undefined;
    return [member.name, egress?.end === "node" ? egress.names.at(-1) : undefined];
  })), [members, selectorsByName, resolveEgress]);
  const sortingDelays = useMemo(() => {
    if (sortMode !== "delay") return emptyDelayResults;
    const results: Record<string, ProxyDelayResult> = {};
    for (const [name, node] of delayNodes) {
      if (node && delayResults[node]) results[name] = delayResults[node];
    }
    return results;
  }, [sortMode, delayNodes, delayResults]);
  const sortedMembers = useMemo(
    () => sortSelectorMembers(members, sortMode, sortingDelays, i18n.language),
    [members, sortMode, sortingDelays, i18n.language],
  );
  return (
    <div className="selector-node-grid">
      {sortedMembers.map((member) => {
        if (member.source === "group") {
          const selector = selectorsByName.get(member.name);
          const delayNode = delayNodes.get(member.name);
          return <SelectorGroupMember
            key={`group-${member.name}`}
            name={member.name}
            selector={selector}
            delay={delayNode ? delayResults[delayNode] : undefined}
            delayBusy={!!delayNode && busyDelayNodes.has(delayNode)}
            selected={currentSelection === member.name}
            selectionDisabled={!runtimeReady || !selectionEnabled || busySelection === groupName}
            onSelect={() => void onSelect(groupName, member.name)}
            onOpen={() => onOpenGroup(member.name)}
          />;
        }
        const delaySupported =
          member.source === "proxy" || member.source === "provider";
        const result = delayResults[member.name];
        const delayBusy = busyDelayNodes.has(member.name);
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
              <SelectorDelayBadge
                result={result}
                busy={delayBusy}
                disabled={!runtimeReady || delayQueueActive}
                onClick={() => void onTestDelay(member.name)}
                label={t(
                  result ? "subscription.delay.retestNode" : "subscription.delay.testNode",
                  { name: member.name },
                )}
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
              />
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
