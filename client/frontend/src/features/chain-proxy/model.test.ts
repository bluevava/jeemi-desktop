import { describe, expect, it } from "vitest";
import { chainProxyErrorKey } from "./model";

describe("chain proxy errors", () => {
  it("localizes known codes through candidate validation context", () => {
    expect(chainProxyErrorKey(new Error('subscription "Test": chain_proxy:dns_conflict'))).toBe("chainProxy.errors.dns_conflict");
    expect(chainProxyErrorKey("chain_proxy:revision_conflict")).toBe("chainProxy.errors.revision_conflict");
  });
  it("never exposes arbitrary credentials or URLs as UI messages", () => {
    expect(chainProxyErrorKey("https://source.invalid/?token=private anytls://password@host")).toBe("chainProxy.errors.unknown");
    expect(chainProxyErrorKey("chain_proxy:unrecognized")).toBe("chainProxy.errors.unknown");
  });
  it("preserves subscription parser diagnostic localization", () => {
    expect(chainProxyErrorKey("subscription normalization: invalid_uri (line 1, field server)")).toBe("subscription.normalization.codes.invalid_uri");
  });
});
