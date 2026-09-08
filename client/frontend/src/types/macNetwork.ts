export interface MacNetworkAuthorizationStatus {
  present: boolean;
	localTest: boolean;
  applicable: boolean;
  ready: boolean;
  code: string;
  action: "" | "register" | "repair" | "settings" | "applications";
  steps: { id: string; complete: boolean }[];
}
