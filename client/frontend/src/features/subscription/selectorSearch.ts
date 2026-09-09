import type {
  SubscriptionSelector,
  SubscriptionSelectorMember,
} from "../../types/subscription";

type SearchCondition = "any" | "all" | "exclude";

export function hasNodeNameSearch(query: string): boolean {
  return /[^|&!\s]/u.test(query);
}

function nodeNameMatcher(query: string): (name: string) => boolean {
  const conditions: Record<SearchCondition, string[]> = {
    any: [],
    all: [],
    exclude: [],
  };
  let condition: SearchCondition = "any";
  for (const token of query.split(/([|&!])/u)) {
    if (token === "|") condition = "any";
    else if (token === "&") condition = "all";
    else if (token === "!") condition = "exclude";
    else {
      const keyword = token.trim().toLowerCase();
      if (keyword) conditions[condition].push(keyword);
    }
  }

  // Operators label the following keyword rather than forming a Boolean AST.
  // Collect the union first, then intersect all required terms, then exclude.
  // Moving a labelled condition cannot reintroduce a previously rejected node.
  return (name) => {
    const normalisedName = name.toLowerCase();
    const includes = (keyword: string) => normalisedName.includes(keyword);
    return (
      (conditions.any.length === 0 || conditions.any.some(includes)) &&
      conditions.all.every(includes) &&
      !conditions.exclude.some(includes)
    );
  };
}

function isNode(member: SubscriptionSelectorMember): boolean {
  return member.source === "proxy" || member.source === "provider";
}

export function filterSelectorsByNodeName(
  selectors: SubscriptionSelector[],
  query: string,
): SubscriptionSelector[] {
  if (!hasNodeNameSearch(query)) {
    return selectors;
  }

  const matches = nodeNameMatcher(query);
  return selectors.flatMap((selector) => {
    const members = selector.members.filter(
      (member) => isNode(member) && matches(member.name),
    );
    return members.length > 0 ? [{ ...selector, members }] : [];
  });
}

export function delayTargetsForSearch(
  allTargets: string[],
  visibleSelectors: SubscriptionSelector[],
  query: string,
): string[] {
  if (!hasNodeNameSearch(query)) return allTargets;
  return Array.from(
    new Set(
      visibleSelectors.flatMap((selector) =>
        selector.members
          .filter(isNode)
          .map((member) => member.name),
      ),
    ),
  );
}
