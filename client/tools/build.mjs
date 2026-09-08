import {
  chmodSync,
  copyFileSync,
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parseArgs } from "node:util";
import {
  clientDirectory,
  descendant,
  metadata,
  records,
  repositoryDirectory,
  run,
  sha256,
  targetFor,
  validateBinary,
} from "./release-common.mjs";
import { packageMac } from "./macos-package.mjs";

export function buildPlan(target, version, digest = "<helper-sha256>") {
  const tags =
    target.platform === "macos"
      ? ["jeemi_local_test"]
      : target.platform === "linux"
        ? ["webkit2_41"]
        : [];
  const helperFlags =
    "-s -w" + (target.platform === "windows" ? " -H windowsgui" : "");
  let flags = "-s -w -X jeemi/internal/application.appVersion=" + version;
  if (target.platform !== "macos")
    flags +=
      " -X jeemi/internal/platform/" +
      (target.platform === "windows" ? "winauth" : "coreauth") +
      ".helperSHA256=" +
      digest;
  return {
    tags,
    helper: [
      "build",
      "-trimpath",
      "-ldflags",
      helperFlags,
      ...(tags.includes("jeemi_local_test")
        ? ["-tags", "jeemi_local_test"]
        : []),
      "-o",
    ],
    gui: [
      "build",
      "-clean",
      "-trimpath",
      "-s",
      "-skipbindings",
      "-platform",
      target.host.replace("win32", "windows") + "/" + target.architecture,
      "-ldflags",
      flags,
      ...(tags.length ? ["-tags", tags.join(",")] : []),
    ],
  };
}

export function promote(stage, output, root) {
  descendant(stage, root);
  descendant(output, root);
  const backup = output + ".previous";
  if (existsSync(backup))
    throw new Error("存在上次发布备份，请先确认恢复状态：" + backup);
  if (existsSync(output)) renameSync(output, backup);
  try {
    renameSync(stage, output);
  } catch (error) {
    if (existsSync(backup) && !existsSync(output)) renameSync(backup, output);
    throw error;
  }
  if (existsSync(backup)) rmSync(descendant(backup, root), { recursive: true });
}

export function build(
  { platform, architecture, dryRun = false },
  execute = run,
) {
  const target = targetFor(platform, architecture);
  const info = metadata();
  const plan = buildPlan(target, info.version);
  if (dryRun) {
    console.log(
      JSON.stringify(
        {
          target: target.directory,
          version: info.version,
          steps: [
            "宿主生成绑定及前端",
            "构建授权助手并验证",
            "注入版本与助手摘要后构建 GUI",
            "验证、归档并原子替换",
          ],
          ...plan,
        },
        null,
        2,
      ),
    );
    return;
  }
  if (process.platform !== target.host)
    throw new Error("必须在目标操作系统构建。");
  const nativeArch = process.arch === "x64" ? "amd64" : process.arch;
  if (platform !== "windows" && architecture !== nativeArch)
    throw new Error("Linux/macOS 公开构建使用对应 CPU 的原生环境。");
  const env = {
    GOOS: target.host.replace("win32", "windows"),
    GOARCH: architecture,
    CGO_ENABLED: platform === "macos" ? "1" : "0",
  };
  if (platform === "macos") env.MACOSX_DEPLOYMENT_TARGET = "13.0";
  if (platform === "linux")
    execute("pkg-config", ["--exists", "gtk+-3.0", "webkit2gtk-4.1"]);
  // Generate bindings with host tools before enabling cross-compilation flags.
  execute("wails", ["generate", "module"]);
  execute(process.execPath, [
    join(clientDirectory, "tools/frontend-build.mjs"),
  ]);
  const intermediate = join(
    clientDirectory,
    "build",
    platform === "macos" ? "macos-helper" : platform + "-helper",
    architecture,
  );
  mkdirSync(intermediate, { recursive: true });
  const helper = join(intermediate, target.helper);
  const resource = join(
    clientDirectory,
    "cmd/jeemi-authorizer",
    "helper_windows_" + architecture + ".syso",
  );
  if (platform === "windows" && existsSync(resource))
    throw new Error("授权助手资源已存在，请等待其他构建完成。");
  try {
    if (platform === "windows") {
      execute("go", ["run", "./cmd/windows-resource", resource, architecture]);
    }
    execute("go", [...plan.helper, helper, "./cmd/jeemi-authorizer"], { env });
  } finally {
    if (platform === "windows" && existsSync(resource)) rmSync(resource);
  }
  validateBinary(readFileSync(helper), platform, architecture);
  if (
    platform === "windows" &&
    !readFileSync(helper).includes(Buffer.from("requireAdministrator"))
  )
    throw new Error("助手缺少管理员清单。");
  const helperDigest = sha256(helper);
  execute("wails", buildPlan(target, info.version, helperDigest).gui, {
    env: { ...env, CGO_ENABLED: platform === "windows" ? "0" : "1" },
  });
  const bin = join(clientDirectory, "build/bin");
  const gui = join(bin, target.executable);
  const executable =
    platform === "macos" ? join(gui, "Contents/MacOS/Jeemi") : gui;
  validateBinary(readFileSync(executable), platform, architecture);
  if (
    platform === "windows" &&
    !["asInvoker", "requestedExecutionLevel"].every((value) => {
      const data = readFileSync(gui);
      return (
        data.includes(Buffer.from(value)) ||
        data.includes(Buffer.from(value, "utf16le"))
      );
    })
  )
    throw new Error("GUI 缺少普通权限清单。");
  const mac =
    platform === "macos"
      ? packageMac(gui, helper, info.version, execute)
      : undefined;
  const linux =
    platform === "linux"
      ? {
          distribution: readFileSync("/etc/os-release", "utf8"),
          webkit: "4.1",
          dependencies: execute("readelf", ["-d", executable], {
            capture: true,
          })
            .split("\n")
            .filter((line) => line.includes("NEEDED")),
          symbolVersions:
            execute("readelf", ["--version-info", executable], {
              capture: true,
            })
              .match(/GLIBC_[\d.]+/g)
              ?.filter(
                (value, index, array) => array.indexOf(value) === index,
              ) ?? [],
        }
      : undefined;
  const releaseRoot = join(repositoryDirectory, "Bin/jeemi");
  mkdirSync(releaseRoot, { recursive: true });
  const stage = mkdtempSync(join(releaseRoot, "." + target.directory + "-"));
  try {
    cpSync(gui, join(stage, target.executable), {
      recursive: true,
      verbatimSymlinks: true,
    });
    if (platform !== "macos") copyFileSync(helper, join(stage, target.helper));
    if (platform === "linux")
      for (const name of [target.executable, target.helper])
        chmodSync(join(stage, name), 0o755);
    const manifest = {
      product: "Jeemi",
      version: info.version,
      platform,
      architecture,
      wailsPlatform:
        target.host.replace("win32", "windows") + "/" + architecture,
      createdAt: new Date().toISOString(),
      files: records(stage),
      authorizationHelper: {
        file: target.helper,
        sha256: mac
          ? sha256(join(gui, "Contents/MacOS/jeemi-authorizer"))
          : helperDigest,
        ...(platform === "windows"
          ? { executionLevel: "requireAdministrator" }
          : {}),
      },
      ...(platform === "windows" ? { windowsExecutionLevel: "asInvoker" } : {}),
      ...(mac ? { macosNetworkHelper: mac } : {}),
      ...(linux ? { linuxRuntime: linux } : {}),
    };
    writeFileSync(
      join(stage, "release.json"),
      JSON.stringify(manifest, null, 2) + "\n",
    );
    writeFileSync(
      join(stage, "SHA256SUMS"),
      records(stage)
        .map((file) => file.sha256 + "  " + file.path)
        .join("\n") + "\n",
    );
    promote(stage, join(releaseRoot, target.directory), releaseRoot);
  } catch (error) {
    if (existsSync(stage))
      rmSync(descendant(stage, releaseRoot), { recursive: true });
    throw error;
  }
  console.log("已生成：" + join(releaseRoot, target.directory));
}

if (
  process.argv[1] &&
  pathToFileURL(resolve(process.argv[1])).href === import.meta.url
) {
  try {
    const { values } = parseArgs({
      options: {
        platform: { type: "string" },
        arch: { type: "string" },
        "dry-run": { type: "boolean", default: false },
      },
    });
    build({
      platform: values.platform,
      architecture: values.arch,
      dryRun: values["dry-run"],
    });
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
