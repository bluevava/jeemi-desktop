import { EMOJI_OPTIONS } from "../../lib/emojiCatalog";

export const STRATEGY_GROUP_EMOJIS = EMOJI_OPTIONS.map((item) => item.value);

export const SELECTOR_TEST_URLS = [
  "http://www.google.com/generate_204",
  "https://www.google.com/generate_204",
  "http://connect.rom.miui.com/generate_204",
  "http://cp.cloudflare.com/generate_204",
  "https://time.tv.cctv.com/time.php",
] as const;

export const DEFAULT_SELECTOR_TEST_URL = SELECTOR_TEST_URLS[0];
