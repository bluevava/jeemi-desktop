import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  writeFileSync,
} from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import {
  metadata,
  platforms,
  records,
  releaseNotes,
  repositoryDirectory,
  run,
  sha256,
  targetFor,
  writeActionsMetadata,
} from "./release-common.mjs";

export function archiveName(version, target) {
  return (
    "Jeemi-v" +
    version +
    "-" +
    target.directory +
    (target.platform === "linux" ? ".tar.gz" : ".zip")
  );
}

export function pack(platform, architecture, execute = run) {
  const target = targetFor(platform, architecture);
  const info = metadata();
  if (process.platform !== target.host)
    throw new Error("归档必须在构建宿主完成。");
  const source = join(repositoryDirectory, "Bin/jeemi", target.directory);
  const manifest = JSON.parse(
    readFileSync(join(source, "release.json"), "utf8"),
  );
  if (
    manifest.version !== info.version ||
    manifest.platform !== platform ||
    manifest.architecture !== architecture
  )
    throw new Error("产物清单与发布目标不一致。");
  const actual = new Map(records(source).map((file) => [file.path, file]));
  const expectedNames = new Set([
    ...manifest.files.map((file) => file.path),
    "release.json",
    "SHA256SUMS",
  ]);
  if (
    actual.size !== expectedNames.size ||
    [...actual.keys()].some((name) => !expectedNames.has(name))
  )
    throw new Error("产物目录包含清单外文件或缺少发布文件。");
  for (const file of manifest.files)
    if (
      actual.get(file.path)?.sha256 !== file.sha256 ||
      actual.get(file.path)?.size !== file.size
    )
      throw new Error("产物在构建后被修改：" + file.path);
  const checksumText =
    [...actual.values()]
      .filter((file) => file.path !== "SHA256SUMS")
      .map((file) => file.sha256 + "  " + file.path)
      .join("\n") + "\n";
  if (readFileSync(join(source, "SHA256SUMS"), "utf8") !== checksumText)
    throw new Error("包内摘要清单与文件不一致。");
  const output = join(repositoryDirectory, "Bin/github");
  mkdirSync(output, { recursive: true });
  const archive = join(output, archiveName(info.version, target));
  if (existsSync(archive))
    throw new Error("归档文件已存在，请保留或移走后重新生成。");
  if (platform === "macos")
    execute("/usr/bin/ditto", [
      "-c",
      "-k",
      "--sequesterRsrc",
      "--keepParent",
      source,
      archive,
    ]);
  else
    execute(platform === "windows" ? "tar.exe" : "tar", [
      platform === "windows" ? "-acf" : "-czf",
      archive,
      "-C",
      join(source, ".."),
      target.directory,
    ]);
  const name = archiveName(info.version, target);
  writeFileSync(archive + ".sha256", sha256(archive) + "  " + name + "\n");
  copyFileSync(join(source, "release.json"), archive + ".release.json");
  console.log("已归档：" + archive);
}

export function verifyArchives(directory, version) {
  const checksums = [];
  const expected = [];
  for (const platform of Object.keys(platforms))
    for (const arch of ["amd64", "arm64"]) {
      const target = targetFor(platform, arch);
      const name = archiveName(version, target);
      const path = join(directory, name);
      const checksum = sha256(path) + "  " + name;
      if (readFileSync(path + ".sha256", "utf8").trim() !== checksum)
        throw new Error("下载的构建产物摘要不一致：" + name);
      const manifest = JSON.parse(readFileSync(path + ".release.json", "utf8"));
      if (
        manifest.version !== version ||
        manifest.platform !== platform ||
        manifest.architecture !== arch
      )
        throw new Error("构建清单目标不一致：" + name);
      if (
        platform === "macos" &&
        (manifest.macosNetworkHelper?.signing !== "ad-hoc" ||
          manifest.macosNetworkHelper?.notarized !== false)
      )
        throw new Error("macOS 签名声明不符合本次发布策略。");
      expected.push(name, name + ".sha256", name + ".release.json");
      checksums.push(checksum);
    }
  const extra = readdirSync(directory).filter(
    (name) =>
      !expected.includes(name) &&
      !["SHA256SUMS", "release-notes.md"].includes(name),
  );
  if (extra.length)
    throw new Error("发布目录包含非预期文件：" + extra.join(", "));
  return { expected, checksums: checksums.join("\n") + "\n" };
}

export function prepareRelease(directory, tag) {
  const info = metadata(repositoryDirectory, tag);
  const result = verifyArchives(directory, info.version);
  writeFileSync(join(directory, "SHA256SUMS"), result.checksums);
  writeFileSync(
    join(directory, "release-notes.md"),
    releaseNotes(
      readFileSync(join(repositoryDirectory, "CHANGELOG.md"), "utf8"),
      info.version,
    ) +
      "\n### 下载说明\n\nWindows/Linux 压缩包内包含 Jeemi 与配套 jeemi-authorizer，请共同保留。macOS 使用 ad-hoc 签名与固定 CDHash 授权助手，未经 Apple 公证；首次安装或更新助手需要管理员授权。\n",
  );
  return result;
}

if (
  process.argv[1] &&
  pathToFileURL(resolve(process.argv[1])).href === import.meta.url
) {
  try {
    const [command, ...args] = process.argv.slice(2);
    if (command === "metadata")
      writeActionsMetadata(metadata(repositoryDirectory, args[0]));
    else if (command === "pack") pack(args[0], args[1]);
    else if (command === "prepare") prepareRelease(resolve(args[0]), args[1]);
    else
      throw new Error(
        "用法：release.mjs metadata [tag] | pack <platform> <arch> | prepare <directory> <tag>",
      );
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
