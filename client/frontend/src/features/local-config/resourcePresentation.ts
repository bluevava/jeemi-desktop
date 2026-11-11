import { selectorPresentation } from "../subscription/selectorPresentation";

// Presentation only. Full stored names remain available for search and copying.
export function resourcePresentation(item: { name: string; emoji?: string }) {
  const leading = selectorPresentation(item.name, "");
  return {
    displayName: leading.displayName,
    emoji: item.emoji || leading.emoji,
  };
}

export function resourceDisplayName(item: { name: string; emoji?: string }) {
  const { displayName, emoji } = resourcePresentation(item);
  return emoji ? `${emoji} ${displayName}` : displayName;
}
