export function moveSubscriptionCard(ids: readonly string[], sourceID: string, targetID: string): string[] | null {
  const source = ids.indexOf(sourceID);
  const target = ids.indexOf(targetID);
  if (source < 0 || target < 0 || source === target) return null;
  const next = [...ids];
  next.splice(source, 1);
  next.splice(target, 0, sourceID);
  return next;
}

export function applySubscriptionOrder<T extends { id: string }>(items: readonly T[], ids: readonly string[]): T[] {
  const remaining = new Map(items.map((item) => [item.id, item]));
  const ordered: T[] = [];
  for (const id of ids) {
    const item = remaining.get(id);
    if (item) {
      ordered.push(item);
      remaining.delete(id);
    }
  }
  return [...ordered, ...remaining.values()];
}
