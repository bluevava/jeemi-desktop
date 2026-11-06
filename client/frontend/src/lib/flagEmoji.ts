import { COUNTRY_REGION_CODES } from "./countryFlags";

const countryCodes = new Set(COUNTRY_REGION_CODES.map((code) => code.toLowerCase()));
const subdivisionFlags: Readonly<Record<string, string>> = {
  gbeng: "gb-eng",
  gbsct: "gb-sct",
  gbwls: "gb-wls",
};
const flagPattern = /\p{Regional_Indicator}{2}\uFE0F?|\u{1F3F4}[\u{E0061}-\u{E007A}]+\u{E007F}/gu;

export interface FlagTextPart {
  text: string;
  code?: string;
}

/** Only known country/region flags resolve to bundled assets. */
export function flagEmojiCode(emoji: string): string | undefined {
  if (/^\p{Regional_Indicator}{2}\uFE0F?$/u.test(emoji)) {
    const code = [...emoji].slice(0, 2).map((letter) =>
      String.fromCharCode(letter.codePointAt(0)! - 0x1f1e6 + 97),
    ).join("");
    return countryCodes.has(code) ? code : undefined;
  }
  if (/^\u{1F3F4}[\u{E0061}-\u{E007A}]+\u{E007F}$/u.test(emoji)) {
    const tag = [...emoji].slice(1, -1).map((letter) =>
      String.fromCharCode(letter.codePointAt(0)! - 0xe0000),
    ).join("");
    return subdivisionFlags[tag];
  }
  return undefined;
}

export function splitFlagText(text: string): FlagTextPart[] {
  const parts: FlagTextPart[] = [];
  let cursor = 0;
  for (const match of text.matchAll(flagPattern)) {
    const code = flagEmojiCode(match[0]);
    if (!code) continue;
    if (match.index > cursor) parts.push({ text: text.slice(cursor, match.index) });
    parts.push({ text: match[0], code });
    cursor = match.index + match[0].length;
  }
  if (cursor < text.length || parts.length === 0) parts.push({ text: text.slice(cursor) });
  return parts;
}

export function leadingFlagEmoji(text: string): string {
  const first = splitFlagText(text)[0];
  return first?.code ? first.text : "";
}
