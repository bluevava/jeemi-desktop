import type { UIFailure } from "../types/uiDiagnostics";

export async function persistUIFailure(input: UIFailure): Promise<boolean> {
  const binding = window.go?.desktop?.App?.ReportUIFailure;
  if (!binding) return false;
  await binding(input);
  return true;
}

export function canOpenUIDiagnostics(): boolean {
  return (
    typeof window !== "undefined" &&
    Boolean(window.go?.desktop?.App?.OpenUIDiagnosticsDirectory)
  );
}

export async function openUIDiagnosticsDirectory(): Promise<void> {
  const binding = window.go?.desktop?.App?.OpenUIDiagnosticsDirectory;
  if (!binding) throw new Error("UI diagnostics unavailable");
  await binding();
}

export async function reloadInterface(
  browserConfirmation: string,
): Promise<void> {
  const binding = window.go?.desktop?.App?.ReloadInterface;
  if (binding) await binding();
  else if (window.confirm(browserConfirmation)) window.location.reload();
}
