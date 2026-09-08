/** Preserve supplied diagnostics; only the absence of a useful message needs a UI fallback. */
export function errorText(error: unknown, fallback: string): string {
  const message = error instanceof Error ? error.message : typeof error === "string" ? error : "";
  return message.trim() || fallback;
}
