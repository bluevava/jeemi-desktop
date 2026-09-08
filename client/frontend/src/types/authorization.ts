export interface ProxyAuthorizationStatus {
  platform: string;
  kind: "service" | "capabilities" | "application";
  ready: boolean;
  present: boolean;
  localTest: boolean;
  code: string;
  action: "" | "install" | "authorize" | "repair" | "update" | "settings" | "applications";
  steps: { id: string; complete: boolean }[];
}
