import type {} from "./appBridge";

export async function initializeClient(language: string): Promise<void> {
  const app = window.go?.desktop?.App;
  // Browser development has no desktop service or managed configuration files.
  if (!app) return;
  if (!app.InitializeClient) throw new Error("Client startup bridge is unavailable");
  await app.InitializeClient(language);
}
