import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const frontendDirectory = fileURLToPath(
  new URL("../frontend/", import.meta.url),
);
const requireFrontend = createRequire(
  new URL("../frontend/package.json", import.meta.url),
);
// Keep WebAssembly available: Vite's module lexer cannot run with --jitless.
const compatibilityFlags = ["--no-opt", "--no-maglev"];

export function isKnownV8Failure(output) {
  const objectCreateFailure =
    output.includes("Object.defineProperties called on non-object") &&
    output.includes("at Object.create (<anonymous>)") &&
    output.includes("EntityPathTracker");
  const nativeFailure =
    /# Fatal error in /u.test(output) && output.includes("V8_Fatal");
  return objectCreateFailure || nativeFailure;
}

export function runNodeStage(stage, flags) {
  return new Promise((resolveResult, reject) => {
    const child = spawn(
      process.execPath,
      [...flags, stage.entry, ...stage.args],
      {
        cwd: frontendDirectory,
        stdio: ["inherit", "pipe", "pipe"],
        windowsHide: true,
      },
    );
    let output = "";
    for (const [source, destination] of [
      [child.stdout, process.stdout],
      [child.stderr, process.stderr],
    ]) {
      source.pipe(destination, { end: false });
      source.on("data", (chunk) => {
        // Retain only a bounded diagnostic tail; continue streaming the full log.
        output = (output + chunk.toString()).slice(-64 * 1024);
      });
    }
    child.once("error", reject);
    child.once("close", (code, signal) =>
      resolveResult({ code, signal, output }),
    );
  });
}

export async function buildFrontend({
  platform = process.platform,
  run = runNodeStage,
  log = console.log,
} = {}) {
  const stages = [
    {
      name: "TypeScript 类型检查",
      entry: requireFrontend.resolve("typescript/bin/tsc"),
      args: ["--noEmit"],
    },
    {
      name: "Vite 打包",
      entry: join(
        dirname(requireFrontend.resolve("vite/package.json")),
        "bin/vite.js",
      ),
      args: ["build"],
    },
  ];
  let flags = [];
  log(`前端构建：Node ${process.version}（${process.execPath}）`);
  for (const stage of stages) {
    let result = await run(stage, flags);
    if (
      result.code !== 0 &&
      !result.signal &&
      platform === "win32" &&
      flags.length === 0 &&
      isKnownV8Failure(result.output)
    ) {
      log(
        `${stage.name}遇到已识别的 Node/V8 异常，关闭 V8 优化编译后重试一次。`,
      );
      flags = compatibilityFlags;
      result = await run(stage, flags);
    }
    if (result.code !== 0 || result.signal) {
      log(
        `${stage.name}失败（${result.signal || result.code}），未完成前端构建。`,
      );
      return Number.isInteger(result.code) &&
        result.code > 0 &&
        result.code < 256
        ? result.code
        : 1;
    }
  }
  return 0;
}

if (
  process.argv[1] &&
  pathToFileURL(resolve(process.argv[1])).href === import.meta.url
) {
  try {
    process.exitCode = await buildFrontend();
  } catch (error) {
    console.error(`前端构建无法执行：${error.message}`);
    process.exitCode = 1;
  }
}
