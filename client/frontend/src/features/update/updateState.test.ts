import { describe, expect, it } from "vitest";
import { downloadPercent, updateErrorKey, updateInProgress } from "./updateState";
import type { JeemiUpdateState } from "../../types/appUpdate";

describe("Jeemi update presentation", () => {
  it("only exposes translated errors, without raw remote or local details", () => {
    expect(updateErrorKey(new Error("jeemi_update_checksum_failed"))).toBe("checksum_failed");
    expect(updateErrorKey("jeemi_update_location_unwritable")).toBe("location_unwritable");
    expect(updateErrorKey("https://private.example/?token=secret")).toBe("check_failed");
  });
  it("keeps update progress visible through verification and restart", () => {
    expect(updateInProgress("downloading")).toBe(true);
    expect(updateInProgress("verifying")).toBe(true);
    expect(updateInProgress("restarting")).toBe(true);
    expect(updateInProgress("failed")).toBe(false);
    expect(updateInProgress("cancelled")).toBe(false);
  });
  it("bounds progress including an unknown total", () => {
    const state = { total: 100, downloaded: 53 } as JeemiUpdateState;
    expect(downloadPercent(state)).toBe(53);
    expect(downloadPercent({ ...state, total: 0 })).toBe(0);
    expect(downloadPercent({ ...state, downloaded: 150 })).toBe(100);
  });
});
