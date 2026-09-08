import type {
  SelectorSortMode,
  SubscriptionSelectorMember,
} from "../../types/subscription";
import type { ProxyDelayResult } from "./useProxyDelayQueue";

export function sortSelectorMembers(
  members: SubscriptionSelectorMember[],
  mode: SelectorSortMode,
  delayResults: Readonly<Record<string, ProxyDelayResult>>,
  locale: string,
): SubscriptionSelectorMember[] {
  if (mode === "default") return [...members];

  const indexed = members.map((member, index) => ({ member, index }));
  const collator = new Intl.Collator(locale, {
    numeric: true,
    sensitivity: "base",
  });
  indexed.sort((left, right) => {
    if (mode === "name") {
      return (
        collator.compare(left.member.name, right.member.name) ||
        left.index - right.index
      );
    }
    const leftDelay = delayRank(delayResults[left.member.name]);
    const rightDelay = delayRank(delayResults[right.member.name]);
    return (
      leftDelay.bucket - rightDelay.bucket ||
      leftDelay.delay - rightDelay.delay ||
      left.index - right.index
    );
  });
  return indexed.map(({ member }) => member);
}

function delayRank(result: ProxyDelayResult | undefined): {
  bucket: number;
  delay: number;
} {
  if (result?.status === "success" && result.delay > 0) {
    return { bucket: 0, delay: result.delay };
  }
  if (!result) return { bucket: 1, delay: 0 };
  return { bucket: 2, delay: 0 };
}
