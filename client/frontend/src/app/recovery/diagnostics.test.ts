import { afterEach, describe, expect, it, vi } from "vitest";
import {
  describeUIFailure,
  diagnosticPage,
  installUIErrorReporting,
  reportUIFailure,
} from "./diagnostics";

afterEach(() => vi.unstubAllGlobals());

describe("structural UI diagnostics", () => {
  it("omits messages, URLs, identifiers and credentials while retaining release source positions", () => {
    const error = new TypeError("Cannot read properties of SECRET-PASSWORD");
    error.stack =
      "TypeError: SECRET-PASSWORD\n at fn (wails://wails.localhost/assets/index-Abc123_x.js:10:23)\n at http://localhost/?token=CONTROL-SECRET:2:3";
    const result = describeUIFailure(
      error,
      "render",
      "page",
      "#/config/private-id/edit?token=SECRET",
      new Set(["index-Abc123_x.js"]),
    );
    expect(result).toEqual({
      kind: "render",
      scope: "page",
      page: "config_editor",
      errorType: "TypeError",
      code: "invalid_value",
      frames: [{ asset: "index-Abc123_x.js", line: 10, column: 23 }],
    });
    expect(JSON.stringify(result)).not.toMatch(/SECRET|private-id|localhost/);
    expect(describeUIFailure(error, "render", "page", "#/home").frames).toEqual(
      [],
    );
  });

  it("classifies production React errors and safely handles hostile rejected values", () => {
    expect(
      describeUIFailure(
        new Error("Minified React error #185;"),
        "render",
        "app",
        "#/home",
      ).code,
    ).toBe("render_loop");
    const error = new Error();
    Object.defineProperty(error, "message", {
      get: () => {
        throw new Error("getter");
      },
    });
    expect(() =>
      describeUIFailure(error, "render", "app", "unknown"),
    ).not.toThrow();
    expect(diagnosticPage("#/config/scripts/secret/edit")).toBe(
      "script_editor",
    );
    expect(diagnosticPage("#/subscriptions/settings")).toBe(
      "subscriptions_settings",
    );
    expect(diagnosticPage("#/unknown/secret")).toBe("unknown");
  });

  it("deduplicates reports and contains bridge failures without recursive rejections", async () => {
    const binding = vi.fn().mockRejectedValue(new Error("native unavailable"));
    vi.stubGlobal("window", {
      location: { hash: "#/logs" },
      go: { desktop: { App: { ReportUIFailure: binding } } },
    });
    const error = new SyntaxError("test");
    expect(await reportUIFailure(error, "render", "page")).toBe(false);
    expect(await reportUIFailure(error, "render", "page")).toBe(false);
    expect(binding).toHaveBeenCalledTimes(1);
  });

  it("records asynchronous failures without replacing the current interface and removes listeners", async () => {
    const target = new EventTarget();
    const binding = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal(
      "window",
      Object.assign(target, {
        location: { hash: "#/tools" },
        go: { desktop: { App: { ReportUIFailure: binding } } },
      }),
    );
    const remove = installUIErrorReporting();
    const event = Object.assign(new Event("unhandledrejection"), {
      reason: new RangeError("Maximum call stack size exceeded"),
    });
    target.dispatchEvent(event);
    await Promise.resolve();
    expect(binding).toHaveBeenCalledTimes(1);
    remove();
    target.dispatchEvent(
      Object.assign(new Event("error"), { error: new TypeError("different") }),
    );
    await Promise.resolve();
    expect(binding).toHaveBeenCalledTimes(1);
  });
});
