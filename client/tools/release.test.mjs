import assert from "node:assert/strict";
import { test } from "node:test";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { build, buildPlan, promote } from "./build.mjs";
import {
  descendant,
  metadata,
  releaseNotes,
  sha256,
  targetFor,
  validateBinary,
} from "./release-common.mjs";
import { archiveName, verifyArchives } from "./release.mjs";

function temporary(t) {
  const root = mkdtempSync(join(tmpdir(), "jeemi-release-test-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  return root;
}

test("all six plans bind the target and preserve platform permission setup", () => {
  for (const platform of ["windows", "linux", "macos"])
    for (const arch of ["amd64", "arm64"]) {
      const plan = buildPlan(
        targetFor(platform, arch),
        "1.2.3",
        "a".repeat(64),
      );
      assert.ok(
        plan.gui.includes(
          (platform === "macos" ? "darwin" : platform) + "/" + arch,
        ),
      );
      assert.match(plan.gui.join(" "), /application.appVersion=1.2.3/);
      assert.ok(plan.gui.includes("-skipbindings"));
      if (platform === "macos") {
        assert.ok(plan.helper.includes("jeemi_local_test"));
        assert.ok(plan.gui.includes("jeemi_local_test"));
      } else
        assert.match(
          plan.gui.join(" "),
          new RegExp(
            (platform === "windows" ? "winauth" : "coreauth") +
              "\\.helperSHA256=a{64}",
          ),
        );
    }
});

test("dry run does not execute build tools on any host", (t) => {
  t.mock.method(console, "log", () => {});
  for (const platform of ["windows", "linux", "macos"])
    for (const architecture of ["amd64", "arm64"]) {
      build({ platform, architecture, dryRun: true }, () =>
        assert.fail("executed build during dry run"),
      );
    }
});

test("binary checks reject wrong architecture, truncated headers and universal Mach-O", () => {
  for (const arch of ["amd64", "arm64"]) {
    const pe = Buffer.alloc(128);
    pe.write("MZ");
    pe.writeUInt32LE(64, 60);
    pe.write("PE\0\0", 64);
    pe.writeUInt16LE(arch === "amd64" ? 0x8664 : 0xaa64, 68);
    const elf = Buffer.alloc(20);
    Buffer.from([127, 69, 76, 70, 2, 1]).copy(elf);
    elf.writeUInt16LE(arch === "amd64" ? 62 : 183, 18);
    const mach = Buffer.alloc(8);
    mach.writeUInt32LE(0xfeedfacf);
    mach.writeUInt32LE(arch === "amd64" ? 0x01000007 : 0x0100000c, 4);
    for (const [platform, bytes] of [
      ["windows", pe],
      ["linux", elf],
      ["macos", mach],
    ]) {
      validateBinary(bytes, platform, arch);
      assert.throws(() =>
        validateBinary(bytes, platform, arch === "amd64" ? "arm64" : "amd64"),
      );
      assert.throws(() => validateBinary(bytes.subarray(0, 4), platform, arch));
    }
  }
  assert.throws(() =>
    validateBinary(
      Buffer.from([0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 2]),
      "macos",
      "amd64",
    ),
  );
});

test("promotion failure restores the previous complete release", (t) => {
  const root = temporary(t);
  const output = join(root, "Windows-amd64");
  mkdirSync(output);
  writeFileSync(join(output, "Jeemi.exe"), "healthy");
  assert.throws(() => promote(join(root, "missing-stage"), output, root));
  assert.equal(readFileSync(join(output, "Jeemi.exe"), "utf8"), "healthy");
  assert.equal(existsSync(output + ".previous"), false);
  assert.throws(() => descendant(root, root));
  assert.throws(() => descendant(join(root, "../other"), root));
});

test("release notes select an exact version and reject missing or empty notes", () => {
  assert.equal(
    releaseNotes(
      "# 更新\n## 1.2.30\nwrong\n## 1.2.3 - 2026-09-08\n\nfixed\n## 1.2.2\nold",
      "1.2.3",
    ),
    "fixed\n",
  );
  assert.throws(() => releaseNotes("## 1.2.30\nwrong", "1.2.3"));
  assert.throws(() => releaseNotes("## 1.2.3\n\n## 1.2.2\nold", "1.2.3"));
  assert.throws(() => metadata(undefined, "v999.0.0"));
});

test("release requires all six archives with correct checksums and signing claims", (t) => {
  const root = temporary(t);
  for (const platform of ["windows", "linux", "macos"])
    for (const architecture of ["amd64", "arm64"]) {
      const path = join(
        root,
        archiveName("1.2.3", targetFor(platform, architecture)),
      );
      writeFileSync(path, platform + architecture);
      writeFileSync(
        path + ".sha256",
        sha256(path) +
          "  " +
          archiveName("1.2.3", targetFor(platform, architecture)) +
          "\n",
      );
      writeFileSync(
        path + ".release.json",
        JSON.stringify({
          version: "1.2.3",
          platform,
          architecture,
          macosNetworkHelper: { signing: "ad-hoc", notarized: false },
        }),
      );
    }
  assert.equal(verifyArchives(root, "1.2.3").expected.length, 18);
  const changed = join(root, archiveName("1.2.3", targetFor("macos", "arm64")));
  writeFileSync(changed, "corrupted");
  assert.throws(() => verifyArchives(root, "1.2.3"), /摘要/);
});
