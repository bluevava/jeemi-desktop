import { createHash } from "node:crypto";
import {
  appendFileSync,
  readFileSync,
  realpathSync,
  readdirSync,
  statSync,
} from "node:fs";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

export const clientDirectory = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "..",
);
export const repositoryDirectory = resolve(clientDirectory, "..");
export const platforms = {
  windows: {
    host: "win32",
    label: "Windows",
    executable: "Jeemi.exe",
    helper: "jeemi-authorizer.exe",
  },
  linux: {
    host: "linux",
    label: "Linux",
    executable: "Jeemi",
    helper: "jeemi-authorizer",
  },
  macos: {
    host: "darwin",
    label: "macOS",
    executable: "Jeemi.app",
    helper: "jeemi-authorizer",
  },
};

export function metadata(root = repositoryDirectory, tag = "") {
  const client = join(root, "client");
  const config = JSON.parse(readFileSync(join(client, "wails.json"), "utf8"));
  const pkg = JSON.parse(
    readFileSync(join(client, "frontend/package.json"), "utf8"),
  );
  const mod = readFileSync(join(client, "go.mod"), "utf8");
  const version = config.info.productVersion;
  if (!/^\d+\.\d+\.\d+$/.test(version))
    throw new Error("产品版本必须是三段数字，例如 0.1.0。");
  if (tag && tag !== "v" + version)
    throw new Error("版本标签与 client/wails.json 不一致。");
  const go =
    mod.match(/^toolchain go(\S+)$/m)?.[1] ?? mod.match(/^go (\S+)$/m)?.[1];
  const wails = mod.match(
    /^\s*github\.com\/wailsapp\/wails\/v2\s+(v\S+)/m,
  )?.[1];
  const pnpm = pkg.packageManager.match(/^pnpm@([\d.]+)$/)?.[1];
  const node = readFileSync(join(client, ".node-version"), "utf8").trim();
  if (![go, wails, pnpm, node].every(Boolean))
    throw new Error("工具版本声明不完整。");
  return { version, tag: "v" + version, go, wails, pnpm, node };
}

export function targetFor(platform, architecture) {
  const spec = platforms[platform];
  if (!spec || !["amd64", "arm64"].includes(architecture))
    throw new Error("目标必须是 windows/linux/macos 与 amd64/arm64。");
  return {
    ...spec,
    platform,
    architecture,
    directory: spec.label + "-" + architecture,
  };
}

export function run(
  command,
  args,
  { cwd = clientDirectory, env = {}, capture = false } = {},
) {
  const environment = { ...process.env };
  for (const key of ["GOOS", "GOARCH", "CGO_ENABLED"]) delete environment[key];
  const result = spawnSync(command, args, {
    cwd,
    env: { ...environment, ...env },
    encoding: "utf8",
    windowsHide: true,
    stdio: capture ? "pipe" : "inherit",
    timeout: 20 * 60 * 1000,
  });
  if (result.error || result.status !== 0)
    throw new Error(
      command +
        " 执行失败：" +
        (result.error?.message ?? result.stderr ?? result.status),
    );
  return result.stdout ?? "";
}

export function descendant(path, root) {
  const part = relative(resolve(root), resolve(path));
  if (
    !part ||
    part === ".." ||
    part.startsWith("..\\") ||
    part.startsWith("../") ||
    isAbsolute(part)
  )
    throw new Error("路径超出允许目录。");
  return resolve(path);
}

export function sha256(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

export function records(directory, prefix = "", ancestors = new Set()) {
  const result = [];
  const current = realpathSync(join(directory, prefix));
  if (ancestors.has(current)) throw new Error("发布目录包含循环链接。");
  const visited = new Set([...ancestors, current]);
  for (const name of readdirSync(join(directory, prefix)).sort()) {
    const path = join(directory, prefix, name);
    const key = (prefix ? prefix + "/" : "") + name;
    descendant(realpathSync(path), realpathSync(directory));
    const info = statSync(path);
    if (info.isDirectory()) result.push(...records(directory, key, visited));
    else if (info.isFile())
      result.push({ path: key, size: info.size, sha256: sha256(path) });
    else throw new Error("发布目录包含特殊文件：" + key);
  }
  return result;
}

export function validateBinary(data, platform, architecture) {
  let valid = false;
  if (
    platform === "windows" &&
    data.length >= 64 &&
    data.subarray(0, 2).toString() === "MZ"
  ) {
    const offset = data.readUInt32LE(60);
    valid =
      offset >= 64 &&
      offset < 1024 * 1024 &&
      offset + 6 <= data.length &&
      data.subarray(offset, offset + 4).equals(Buffer.from([80, 69, 0, 0])) &&
      data.readUInt16LE(offset + 4) ===
        { amd64: 0x8664, arm64: 0xaa64 }[architecture];
  } else if (platform === "linux" && data.length >= 20) {
    valid =
      data.subarray(0, 6).equals(Buffer.from([127, 69, 76, 70, 2, 1])) &&
      data.readUInt16LE(18) === { amd64: 62, arm64: 183 }[architecture];
  } else if (platform === "macos" && data.length >= 8) {
    valid =
      data.readUInt32LE(0) === 0xfeedfacf &&
      data.readUInt32LE(4) ===
        { amd64: 0x01000007, arm64: 0x0100000c }[architecture];
  }
  if (!valid)
    throw new Error(
      "产物文件头或 CPU 架构不符合 " + platform + "/" + architecture,
    );
}

export function releaseNotes(text, version) {
  const lines = text.replaceAll("\r\n", "\n").split("\n");
  const start = lines.findIndex(
    (line) =>
      line === "## " + version || line.startsWith("## " + version + " - "),
  );
  if (start < 0)
    throw new Error("CHANGELOG.md 缺少版本 " + version + " 的公开说明。");
  let end = lines.findIndex(
    (line, index) => index > start && line.startsWith("## "),
  );
  if (end < 0) end = lines.length;
  const notes = lines
    .slice(start + 1, end)
    .join("\n")
    .trim();
  if (!notes) throw new Error("版本更新说明不能为空。");
  return notes + "\n";
}

export function writeActionsMetadata(values) {
  if (process.env.GITHUB_OUTPUT)
    appendFileSync(
      process.env.GITHUB_OUTPUT,
      Object.entries(values)
        .map(([key, value]) => key + "=" + value)
        .join("\n") + "\n",
    );
  console.log(JSON.stringify(values));
}
