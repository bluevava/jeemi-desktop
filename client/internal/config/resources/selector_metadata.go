package resources

import "strings"

const defaultSelectorTestURL = "http://www.google.com/generate_204"

var selectorEmojiValues = []string{
	"🐱", "🐶", "🐕", "🐩", "🐈", "🦊", "🐼", "🐯", "🐰", "🐇",
	"🐻", "🦁", "🐸", "🐧", "🐣", "🐤", "🐥", "🐔", "🦆", "🦅",
	"🦉", "🦜", "🦢", "🦩", "🦄", "🐳", "🐬", "🦭", "🐟", "🐠",
	"🦈", "🐙", "🦑", "🦀", "🦞", "🐢", "🐨", "🐮", "🐷", "🐵",
	"🐹", "🐭", "🐺", "🦝", "🐴", "🦓", "🦒", "🐘", "🦏", "🦛",
	"🐑", "🐐", "🦌", "🐿️", "🦔", "🦦", "🦥", "🦋", "🐝", "🦖", "🐲",
	"🚀", "⚡", "🌐", "🛡️", "🔒", "🎮", "🎬", "🎵", "💬", "💻",
	"📱", "☁️", "🏠", "⭐", "🔥", "💎", "🌟", "🌈", "🍀", "🍉",
	"🍎", "🍭", "🎯", "🎲", "🎧", "📺", "✈️", "🚢", "🏎️", "🛰️",
}

var selectorEmojiSet = func() map[string]struct{} {
	result := make(map[string]struct{}, len(selectorEmojiValues))
	for _, value := range selectorEmojiValues {
		result[value] = struct{}{}
	}
	return result
}()

// IsSupportedSelectorEmoji lets other user-facing metadata editors reuse the
// same controlled Emoji catalog without duplicating its validation rules.
func IsSupportedSelectorEmoji(value string) bool {
	_, ok := selectorEmojiSet[strings.TrimSpace(value)]
	return ok
}

func strategyGroupDisplayName(group StrategyGroup) string {
	name := strings.TrimSpace(group.Name)
	if group.Kind == StrategyGroupKindSelector && group.Emoji != "" {
		return group.Emoji + " " + name
	}
	return name
}

// SelectorDisplayName returns the exact name emitted to proxy-groups.
func SelectorDisplayName(group StrategyGroup) string { return strategyGroupDisplayName(group) }
