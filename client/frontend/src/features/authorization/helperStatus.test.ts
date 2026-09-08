import { describe, expect, it } from "vitest";
import type { ProxyAuthorizationStatus } from "../../types/authorization";
import { helperHealth } from "./helperStatus";

const installed: ProxyAuthorizationStatus = { platform: "windows", kind: "service", present: true, ready: true, action: "", code: "ready", localTest: false, steps: [] };

describe("About helper status", () => {
  it("distinguishes installation from health and available updates", () => {
    expect(helperHealth(installed)).toBe("ready");
    expect(helperHealth({ ...installed, ready: false, action: "repair" })).toBe("failed");
    expect(helperHealth({ ...installed, ready: false, action: "update" })).toBe("update");
    expect(helperHealth({ ...installed, present: false, ready: false, action: "install" })).toBe("not_installed");
  });
});
