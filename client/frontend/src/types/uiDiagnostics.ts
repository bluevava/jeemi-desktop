// Only structural diagnostics cross the bridge. Never send error messages,
// raw stacks, component props, URLs, route IDs, or runtime configuration.
export interface UIFailure {
  kind: "render" | "unhandled_error" | "unhandled_rejection";
  scope: "app" | "page" | "status" | "global";
  page: string;
  errorType: string;
  code: string;
  frames: { asset: string; line: number; column: number }[];
}
