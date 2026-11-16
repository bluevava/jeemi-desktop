// Store offsets instead of one object per hit: a short query can match millions
// of times in a subscription. The source text and its offsets remain unchanged.
export function findConfigurationMatches(text: string, query: string): Uint32Array {
  if (!query || !text) return new Uint32Array(0);
  const literal = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const pattern = new RegExp(literal, "giu");
  let ranges = new Uint32Array(256);
  let length = 0;
  for (let match = pattern.exec(text); match; match = pattern.exec(text)) {
    if (length === ranges.length) {
      const larger = new Uint32Array(ranges.length * 2);
      larger.set(ranges);
      ranges = larger;
    }
    ranges[length++] = match.index;
    ranges[length++] = pattern.lastIndex;
  }
  return ranges.subarray(0, length);
}

export function nextConfigurationMatch(current: number, count: number, direction: 1 | -1): number {
  if (count === 0) return -1;
  if (current < 0) return direction === 1 ? 0 : count - 1;
  return (current + direction + count) % count;
}

export function visibleConfigurationMatches(ranges: Uint32Array, active: number): number[] {
  const count = ranges.length / 2;
  // Keep the DOM bounded for common characters; every result still participates
  // in counting and navigation, including results beyond the highlighted batch.
  return count <= 800
    ? Array.from({ length: count }, (_, index) => index)
    : active >= 0 && active < count ? [active] : [];
}

interface SearchKey {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  isComposing: boolean;
}

export function configurationSearchAction(event: SearchKey, inputFocused: boolean, hasQuery: boolean) {
  if (event.isComposing || event.altKey) return null;
  const modified = event.ctrlKey || event.metaKey;
  if (modified && event.key.toLowerCase() === "f") return "focus";
  if ((modified && event.key.toLowerCase() === "g") || event.key === "F3" ||
      (!modified && event.key === "Enter" && inputFocused)) {
    return event.shiftKey ? "previous" : "next";
  }
  if (!modified && event.key === "Escape" && hasQuery) return "clear";
  return null;
}
