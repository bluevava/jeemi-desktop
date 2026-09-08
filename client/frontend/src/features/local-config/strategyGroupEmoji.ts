// Kept in sync with the controlled selector Emoji catalog validated by the Go
// resource store. These values match the Jeeyio rule editor reference set.
export const STRATEGY_GROUP_EMOJIS = [
  "🐱", "🐶", "🐕", "🐩", "🐈", "🦊", "🐼", "🐯", "🐰", "🐇",
  "🐻", "🦁", "🐸", "🐧", "🐣", "🐤", "🐥", "🐔", "🦆", "🦅",
  "🦉", "🦜", "🦢", "🦩", "🦄", "🐳", "🐬", "🦭", "🐟", "🐠",
  "🦈", "🐙", "🦑", "🦀", "🦞", "🐢", "🐨", "🐮", "🐷", "🐵",
  "🐹", "🐭", "🐺", "🦝", "🐴", "🦓", "🦒", "🐘", "🦏", "🦛",
  "🐑", "🐐", "🦌", "🐿️", "🦔", "🦦", "🦥", "🦋", "🐝", "🦖", "🐲",
  "🚀", "⚡", "🌐", "🛡️", "🔒", "🎮", "🎬", "🎵", "💬", "💻",
  "📱", "☁️", "🏠", "⭐", "🔥", "💎", "🌟", "🌈", "🍀", "🍉",
  "🍎", "🍭", "🎯", "🎲", "🎧", "📺", "✈️", "🚢", "🏎️", "🛰️",
] as const;

export const SELECTOR_TEST_URLS = [
  "http://www.google.com/generate_204",
  "https://www.google.com/generate_204",
  "http://connect.rom.miui.com/generate_204",
  "http://cp.cloudflare.com/generate_204",
  "https://time.tv.cctv.com/time.php",
] as const;

export const DEFAULT_SELECTOR_TEST_URL = SELECTOR_TEST_URLS[0];
