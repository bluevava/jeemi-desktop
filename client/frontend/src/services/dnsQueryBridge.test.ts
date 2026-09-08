import { afterEach, describe, expect, it, vi } from "vitest";

import { cancelDNSQuery, dnsErrorCode, getDNSQueryPreferences, queryDNS } from "./dnsQueryBridge";

afterEach(() => vi.unstubAllGlobals());

describe("DNS query bridge", () => {
  it("never displays arbitrary errors containing a private DoH endpoint", () => {
    expect(dnsErrorCode(new Error("POST https://dns.example/private?token=secret failed"))).toBe("query_failed");
    expect(dnsErrorCode("proxy_invalid")).toBe("proxy_invalid");
    expect(dnsErrorCode(new Error("nxdomain"))).toBe("nxdomain");
    expect(dnsErrorCode("preferences_load_failed")).toBe("preferences_load_failed");
    expect(dnsErrorCode("preferences_save_failed")).toBe("preferences_save_failed");
  });

  it.each(["https://resolver.example/private?token=saved", ""])("loads the saved custom address without querying or writing (%s)", async (customDNS) => {
    const load = vi.fn().mockResolvedValue({ customDNS });
    const query = vi.fn();
    vi.stubGlobal("window", { go: { desktop: { App: { GetDNSQueryPreferences: load, QueryDNS: query } } } });
    await expect(getDNSQueryPreferences()).resolves.toEqual({ customDNS });
    expect(load).toHaveBeenCalledOnce();
    expect(query).not.toHaveBeenCalled();
  });

  it("passes bounded input and scopes cancellation to the current request", async () => {
    const query = vi.fn().mockResolvedValue({ domain: "example.com" });
    const cancel = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("window", { go: { desktop: { App: { QueryDNS: query, CancelDNSQuery: cancel } } } });
    const input = { id: "request-1", domain: "https://example.com/path?x=1#section", proxyDNS: "1.1.1.1", directDNS: "223.5.5.5", customDNS: "https://dns.example/dns-query", customProxy: true };
    await queryDNS(input);
    await cancelDNSQuery(input.id);
    expect(query).toHaveBeenCalledWith(input);
    expect(cancel).toHaveBeenCalledWith("request-1");
  });

  it("does not simulate DNS success in the browser preview", async () => {
    vi.stubGlobal("window", {});
    await expect(getDNSQueryPreferences()).rejects.toThrow("bridge_unavailable");
    await expect(queryDNS({ id: "preview", domain: "example.com", proxyDNS: "1.1.1.1", directDNS: "1.1.1.1", customDNS: "1.1.1.1", customProxy: true })).rejects.toThrow("bridge_unavailable");
  });
});
