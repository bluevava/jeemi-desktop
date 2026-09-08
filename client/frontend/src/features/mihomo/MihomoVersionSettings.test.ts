import { describe, expect, it } from "vitest";

import { formatBytes } from "./MihomoVersionSettings";

describe("formatBytes", () => {
  it("formats binary sizes without changing the underlying value", () => {
    expect(formatBytes(1024, "en-US")).toBe("1 KB");
    expect(formatBytes(18 * 1024 * 1024, "en-US")).toBe("18 MB");
  });

  it("rejects invalid sizes", () => {
    expect(formatBytes(-1, "en-US")).toBe("—");
  });
});
