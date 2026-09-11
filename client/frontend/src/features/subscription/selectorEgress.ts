import type { SubscriptionSelector } from "../../types/subscription";
import { selectorSelection, type SelectorIndex } from "./selectorNavigation";

export interface SelectorEgress {
  names: string[];
  end: "node" | "builtin" | "balanced" | "relay" | "unavailable" | "cycle";
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
    if (member?.source === "builtin") return { names, end: "builtin" };
    if (member && member.source !== "group") return { names, end: "node" };
    const child = selectors.get(selected);
    if (!child) return { names, end: member?.source === "group" ? "unavailable" : "node" };
    current = child;
    name = selected;
  }
  return { names, end: "cycle" };
}

export type ResolveSelectorEgress = (selector: SubscriptionSelector, name?: string) => SelectorEgress;

/** Shared groups are resolved once per projection/selection snapshot. */
export function createSelectorEgressResolver(
  selectors: SelectorIndex,
  selections: Readonly<Record<string, string>>,
  runtimeReady: boolean,
): ResolveSelectorEgress {
  const cache = new Map<string, SelectorEgress>();
  return (selector, name = selector.name) => {
    const cached = cache.get(name);
    if (cached !== undefined) return cached;
    const result = resolveSelectorEgress(selector, selectors, selections, runtimeReady, name);
    cache.set(name, result);
    return result;
  };
}
