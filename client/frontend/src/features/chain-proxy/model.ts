import { enChainProxy } from "../../i18n/locales/chain-proxy";
import { normalizationErrorKey } from "../subscription/normalization";

const errorCodes = new Set(Object.keys(enChainProxy.errors));
export function chainProxyErrorKey(error: unknown): string {
  const message = error instanceof Error ? error.message : typeof error === "string" ? error : "";
  const code = /chain_proxy:([a-z_]+)/.exec(message)?.[1];
  return code && errorCodes.has(code) ? `chainProxy.errors.${code}` : normalizationErrorKey(error, "chainProxy.errors.unknown");
}
