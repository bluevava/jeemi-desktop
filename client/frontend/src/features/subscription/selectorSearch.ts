import type { SubscriptionSelector } from "../../types/subscription";

export function filterSelectorsByName(
  selectors: SubscriptionSelector[],
  query: string,
): SubscriptionSelector[] {
  const normalisedQuery = query.trim().toLocaleLowerCase();
  if (!normalisedQuery) {
    return selectors;
  }

  return selectors.flatMap((selector) => {
    if (selector.name.toLocaleLowerCase().includes(normalisedQuery)) {
      return [selector];
    }

    const members = selector.members.filter((member) =>
      member.name.toLocaleLowerCase().includes(normalisedQuery),
    );
    return members.length > 0 ? [{ ...selector, members }] : [];
  });
}

export function delayTargetsForSearch(
  allTargets: string[],
  visibleSelectors: SubscriptionSelector[],
  query: string,
): string[] {
  if (!query.trim()) return allTargets;
  return Array.from(
    new Set(
      visibleSelectors.flatMap((selector) =>
        selector.members
          .filter(
            (member) => member.source === "proxy" || member.source === "provider",
          )
          .map((member) => member.name),
      ),
    ),
  );
}
