import { chmodSync, copyFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import { clientDirectory, run } from "./release-common.mjs";

// Ad-hoc builds reuse the existing CDHash-pinned authorization mode.
// Both the GUI and helper must be built with the same jeemi_local_test tag.
export function packageMac(app, helper, version, execute = run) {
  const service = "com.jeemi.Jeemi.NetworkHelper.LocalTest";
  const bundledHelper = join(app, "Contents/MacOS/jeemi-authorizer");
  copyFileSync(helper, bundledHelper);
  chmodSync(bundledHelper, 0o755);
  const daemons = join(app, "Contents/Library/LaunchDaemons");
  mkdirSync(daemons, { recursive: true });
  const plist = join(daemons, service + ".plist");
  copyFileSync(
    join(clientDirectory, "build/darwin/com.jeemi.Jeemi.NetworkHelper.plist"),
    plist,
  );
  for (const command of [
    "Set :Label " + service,
    "Delete :MachServices:com.jeemi.Jeemi.NetworkHelper",
    "Add :MachServices:" + service + " bool true",
  ])
    execute("/usr/libexec/PlistBuddy", ["-c", command, plist]);
  const info = join(app, "Contents/Info.plist");
  for (const [key, expected] of [
    ["CFBundleExecutable", "Jeemi"],
    ["CFBundleVersion", version],
    ["CFBundleShortVersionString", version],
  ]) {
    const actual = execute(
      "/usr/libexec/PlistBuddy",
      ["-c", "Print :" + key, info],
      { capture: true },
    ).trim();
    if (actual !== expected)
      throw new Error("macOS 应用清单与发布版本不一致：" + key);
  }
  execute("/usr/libexec/PlistBuddy", [
    "-c",
    "Set :LSMinimumSystemVersion 13.0",
    info,
  ]);
  execute("/usr/libexec/PlistBuddy", [
    "-c",
    "Add :JeemiLocalNetworkTesting bool true",
    info,
  ]);
  for (const [path, identifier] of [
    [bundledHelper, service],
    [app, "com.jeemi.desktop"],
  ]) {
    execute("/usr/bin/codesign", [
      "--force",
      "--sign",
      "-",
      "--identifier",
      identifier,
      path,
    ]);
  }
  execute("/usr/bin/codesign", ["--verify", "--deep", "--strict", app]);
  return {
    service,
    minimumOS: "13.0",
    signing: "ad-hoc",
    authorization: "local-test-pinned",
    notarized: false,
  };
}
