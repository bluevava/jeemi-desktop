import type { SubscriptionSelector } from "../../types/subscription";

export type SelectorIndex = ReadonlyMap<string, SubscriptionSelector>;

export function selectorTypeKey(type: string): string {
  switch (type) {
    case "select": return "subscription.selector.groupTypes.select";
    case "url-test": return "subscription.selector.groupTypes.urlTest";
    case "fallback": return "subscription.selector.groupTypes.fallback";
    case "load-balance": return "subscription.selector.groupTypes.loadBalance";
    case "relay": return "subscription.selector.groupTypes.relay";
    default: return "subscription.selector.groupTypes.other";
  }
}

// Validate every edge against current membership. A refresh, filter or cycle
// must not leave the user inside an unrelated or infinitely nested group.
export function resolveSelectorPath(
  root: SubscriptionSelector,
  requested: readonly string[],
  selectors: SelectorIndex,
): SubscriptionSelector[] {
  const path = [root];
  const visited = new Set([root.name]);
  for (const name of requested) {
    const parent = path[path.length - 1];
    const child = selectors.get(name);
    if (!child || visited.has(name) || !parent.members.some(
      (member) => member.source === "group" && member.name === name,
    )) break;
    visited.add(name);
    path.push(child);
  }
  return path;
}

export function selectorMemberCounts(selector: SubscriptionSelector) {
  let nodes = 0;
  let groups = 0;
  let actions = 0;
  for (const member of selector.members) {
    if (member.source === "group") groups++;
    else if (member.source === "builtin") actions++;
    else nodes++;
  }
  return { count: selector.members.length, nodes, groups, actions };
}

export function selectorSelection(
  selector: SubscriptionSelector,
  selections: Readonly<Record<string, string>>,
  runtimeReady: boolean,
  name = selector.name,
): string {
  return runtimeReady ? (selections[name] ?? "") : selector.defaultSelection;
}
