import type { DNSQueryPreferences, DNSQueryRequest, DNSQueryResponse } from "../types/dnsQuery";

export function getDNSQueryPreferences(): Promise<DNSQueryPreferences> {
  const method = window.go?.desktop?.App?.GetDNSQueryPreferences;
  if (!method) return Promise.reject(new Error("bridge_unavailable"));
  return method();
}

export function queryDNS(input: DNSQueryRequest): Promise<DNSQueryResponse> {
  const method = window.go?.desktop?.App?.QueryDNS;
  if (!method) return Promise.reject(new Error("bridge_unavailable"));
  return method(input);
}

export async function cancelDNSQuery(id: string): Promise<void> {
  await window.go?.desktop?.App?.CancelDNSQuery?.(id);
}

const errorCodes = new Set([
  "invalid_domain", "invalid_server", "invalid_request", "busy", "cancelled",
  "timeout", "query_failed", "invalid_response", "nxdomain", "servfail",
  "refused", "dns_error", "proxy_invalid", "missing_proxy", "ambiguous_proxy",
  "direct_unavailable", "session_changed", "bridge_unavailable",
  "preferences_load_failed", "preferences_save_failed",
]);

// Never render raw transport errors: a custom DoH URL may contain credentials
// in its path or query. All backend failure details use stable, bounded codes.
export function dnsErrorCode(error: unknown): string {
  const code = error instanceof Error ? error.message : String(error);
  return errorCodes.has(code) ? code : "query_failed";
}
