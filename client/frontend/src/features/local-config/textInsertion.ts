export function insertAtSelection(
  text: string,
  value: string,
  start: number,
  end: number,
) {
  const from = Math.max(0, Math.min(start, text.length));
  const to = Math.max(from, Math.min(end, text.length));
  return {
    text: text.slice(0, from) + value + text.slice(to),
    caret: from + value.length,
  };
}
