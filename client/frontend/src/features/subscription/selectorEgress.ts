import type { SubscriptionSelector } from "../../types/subscription";
import { selectorSelection, type SelectorIndex } from "./selectorNavigation";

export interface SelectorEgress {
  names: string[];
  end: "node" | "balanced" | "relay" | "unavailable" | "cycle";
}

/** Follow selections, never the panel's filtered members or its browsing path. */
export function resolveSelectorEgress(
  root: SubscriptionSelector,
  selectors: SelectorIndex,
  selections: Readonly<Record<string, string>>,
  runtimeReady: boolean,
  rootName = root.name,
): SelectorEgress {
  let current = selectors.get(rootName) ?? root;
  let name = rootName;
  const names: string[] = [];
  const visited = new Set<string>();
  while (!visited.has(name)) {
    visited.add(name);
    if (current.type === "load-balance") return { names, end: "balanced" };
    if (current.type === "relay") return { names, end: "relay" };
    const selected = selectorSelection(current, selections, runtimeReady, name);
    if (!selected) return { names, end: "unavailable" };
    const member = current.members.find((item) => item.name === selected);
    // Live selections may contain provider nodes absent from the offline projection.
    if (!runtimeReady && !member) return { names, end: "unavailable" };
    names.push(selected);
    if (member && member.source !== "group") return { names, end: "node" };
    const child = selectors.get(selected);
    if (!child) return { names, end: member?.source === "group" ? "unavailable" : "node" };
    current = child;
    name = selected;
  }
  return { names, end: "cycle" };
}

export type DescribeSelectorEgress = (selector: SubscriptionSelector, name?: string) => string;

/** Shared groups are resolved once per projection/selection snapshot. */
export function createSelectorEgressDescription(
  selectors: SelectorIndex,
  selections: Readonly<Record<string, string>>,
  runtimeReady: boolean,
  labels: Record<Exclude<SelectorEgress["end"], "node">, string>,
): DescribeSelectorEgress {
  const cache = new Map<string, string>();
  return (selector, name = selector.name) => {
    const cached = cache.get(name);
    if (cached !== undefined) return cached;
    const { names, end } = resolveSelectorEgress(selector, selectors, selections, runtimeReady, name);
    const text = (end === "node" ? names : [...names, labels[end]]).join(" · ");
    cache.set(name, text);
    return text;
  };
}
