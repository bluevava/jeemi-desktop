import assert from "node:assert/strict";
import { test } from "node:test";
import { buildFrontend, isKnownV8Failure } from "./frontend-build.mjs";

const rollupFailure = `Object.defineProperties called on non-object
    at Object.create (<anonymous>)
    at new EntityPathTracker (file:///D:/project/frontend/node_modules/rollup/dist/es/shared/node-entry.js:1796:35)`;
const nativeFailure =
  "# Fatal error in , line 0\n# unreachable code\n  1: V8_Fatal [node.exe]";
const success = { code: 0, signal: null, output: "" };
const failed = (output, code = 1, signal = null) => ({ code, signal, output });

async function scenario(results, platform = "win32") {
  const calls = [];
  const messages = [];
  const code = await buildFrontend({
    platform,
    run: async (stage, flags) => {
      calls.push({ entry: stage.entry, args: stage.args, flags: [...flags] });
      assert.ok(results.length, "unexpected additional attempt");
      return results.shift();
    },
    log: (message) => messages.push(message),
  });
  assert.equal(results.length, 0, "expected stage was skipped");
  return { code, calls, messages };
}

test("healthy build checks types before bundling without compatibility flags", async () => {
  const { code, calls, messages } = await scenario([success, success]);
  assert.equal(code, 0);
  assert.match(calls[0].entry, /typescript[\\/]bin[\\/]tsc$/u);
  assert.deepEqual(calls[0].args, ["--noEmit"]);
  assert.match(calls[1].entry, /vite[\\/]bin[\\/]vite\.js$/u);
  assert.deepEqual(calls[1].args, ["build"]);
  assert.deepEqual(
    calls.map(({ flags }) => flags),
    [[], []],
  );
  assert.equal(messages.length, 1);
});

test("reported Rollup intrinsic failure retries the same bundle in compatibility mode", async () => {
  const { code, calls, messages } = await scenario([
    success,
    failed(rollupFailure),
    success,
  ]);
  assert.equal(code, 0);
  assert.equal(calls[1].entry, calls[2].entry);
  assert.deepEqual(calls[2].flags, ["--no-opt", "--no-maglev"]);
  assert.match(messages[1], /重试一次/u);
});

test("native TypeScript crash switches the remaining build to compatibility mode", async () => {
  const { code, calls } = await scenario([
    failed(nativeFailure, 2147483651),
    success,
    success,
  ]);
  assert.equal(code, 0);
  assert.equal(calls[0].entry, calls[1].entry);
  assert.deepEqual(
    calls.map(({ flags }) => flags),
    [[], ["--no-opt", "--no-maglev"], ["--no-opt", "--no-maglev"]],
  );
});

test("type errors, unresolved imports, and memory exhaustion are never retried", async () => {
  for (const output of [
    "error TS2322: Type 'number' is not assignable to type 'string'.",
    "Could not resolve import",
    "FATAL ERROR: JavaScript heap out of memory",
  ]) {
    assert.equal(isKnownV8Failure(output), false);
    const { code, calls } = await scenario([failed(output, 2)]);
    assert.equal(code, 2);
    assert.equal(calls.length, 1);
  }
  assert.equal(
    isKnownV8Failure(
      "Object.defineProperties called on non-object\n at userFunction",
    ),
    false,
  );
});

test("compatibility failure stops; no third attempt or following bundle", async () => {
  const { code, calls } = await scenario([
    failed(nativeFailure),
    failed(nativeFailure),
  ]);
  assert.equal(code, 1);
  assert.equal(calls.length, 2);
});

test("a normal failure after fallback also stops without another retry", async () => {
  const { code } = await scenario([
    failed(nativeFailure),
    success,
    failed(rollupFailure),
  ]);
  assert.equal(code, 1);
});

test("Linux and macOS keep their original failure behavior", async () => {
  for (const platform of ["linux", "darwin"]) {
    const { code, calls } = await scenario([failed(rollupFailure)], platform);
    assert.equal(code, 1);
    assert.equal(calls.length, 1);
  }
});

test("interrupted builds are not retried even if the log contains the signature", async () => {
  const { code, calls } = await scenario([
    failed(nativeFailure, null, "SIGINT"),
  ]);
  assert.equal(code, 1);
  assert.equal(calls.length, 1);
});
