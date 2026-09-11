export interface SelectorPresentation {
  displayName: string;
  emoji: string;
  iconUrl: string | null;
}

export function shouldDisplaySelector(
  hidden: boolean,
  showHiddenSelectors: boolean,
): boolean {
  return showHiddenSelectors || !hidden;
}

const leadingEmojiPattern = /^((?:\p{Regional_Indicator}{2})|(?:[#*0-9]\uFE0F?\u20E3)|(?:\p{Extended_Pictographic}(?:\uFE0F|\uFE0E)?(?:\p{Emoji_Modifier})?(?:\u200D\p{Extended_Pictographic}(?:\uFE0F|\uFE0E)?(?:\p{Emoji_Modifier})?)*))\s*/u;

function remoteIconURL(value: string): string | null {
  if (!value) {
    return null;
  }
  try {
    const parsed = new URL(value);
    return (parsed.protocol === "http:" || parsed.protocol === "https:") && !parsed.username && !parsed.password
      ? parsed.href
      : null;
  } catch {
    return null;
  }
}

export function selectorPresentation(
  name: string,
  configuredIcon: string,
): SelectorPresentation {
  const trimmedName = name.trim();
  const trimmedIcon = configuredIcon.trim();
  const emojiMatch = trimmedName.match(leadingEmojiPattern);
  const useNameEmoji = trimmedIcon === "" && emojiMatch !== null;
  const nameWithoutEmoji = useNameEmoji
    ? trimmedName.slice(emojiMatch[0].length).trimStart()
    : trimmedName;

  return {
    displayName: nameWithoutEmoji || trimmedName,
    emoji: useNameEmoji ? emojiMatch[1] : "",
    iconUrl: remoteIconURL(trimmedIcon),
  };
}
