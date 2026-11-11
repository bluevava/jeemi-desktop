import catalog from "../../../internal/iconcatalog/emojis.json";

export const EMOJI_CATEGORIES = catalog;
export const EMOJI_OPTIONS = catalog.flatMap((category) =>
  category.items.map(([value, zh, en]) => ({
    value,
    zh,
    en,
    category: category.id,
  })),
);
export function filterEmojiOptions(category: string, query: string) {
  const term = query.trim().toLocaleLowerCase();
  return EMOJI_OPTIONS.filter(
    (item) =>
      (!category || item.category === category) &&
      (!term ||
        `${item.value} ${item.zh} ${item.en}`
          .toLocaleLowerCase()
          .includes(term)),
  );
}
