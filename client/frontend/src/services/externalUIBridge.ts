import type { ZashboardState } from "../types/externalUI";

export interface ExternalUIAPI {
  GetZashboardState(): Promise<ZashboardState>;
  CheckZashboardUpdates(): Promise<ZashboardState>;
  DownloadZashboardVersion(version: string): Promise<ZashboardState>;
  SelectZashboardVersion(version: string): Promise<ZashboardState>;
  CancelZashboardDownload(): Promise<void>;
  OpenZashboard(): Promise<void>;
}

function bridge(): ExternalUIAPI {
  const api = window.go?.desktop?.App;
  if (!api) throw new Error("external_ui:desktop_unavailable");
  return api;
}
export const getZashboardState = async () => bridge().GetZashboardState();
export const checkZashboardUpdates = async () => bridge().CheckZashboardUpdates();
export const downloadZashboardVersion = async (version: string) => bridge().DownloadZashboardVersion(version);
export const selectZashboardVersion = async (version: string) => bridge().SelectZashboardVersion(version);
export const cancelZashboardDownload = async () => bridge().CancelZashboardDownload();
export const openZashboard = async () => bridge().OpenZashboard();
