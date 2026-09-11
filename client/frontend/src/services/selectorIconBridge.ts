export interface SelectorIconAPI {
  GetSelectorIcon(id: string, address: string): Promise<string>;
  CancelSelectorIcon(id: string): Promise<void>;
}

export async function getSelectorIcon(id: string, address: string): Promise<string> {
  const method = window.go?.desktop?.App?.GetSelectorIcon;
  if (!method) throw new Error("selector_icon_unavailable");
  return method(id, address);
}

export async function cancelSelectorIcon(id: string): Promise<void> {
  await window.go?.desktop?.App?.CancelSelectorIcon?.(id);
}
