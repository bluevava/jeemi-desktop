import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import {
  checkReleaseTag,
  githubRequest,
  releaseSource,
} from "./release-workflow.mjs";

const sha = "a".repeat(40);
const otherSha = "b".repeat(40);
const source = { tag: "v1.2.3", sha, tagged: false };
const reference = (commit = sha) => ({
  object: { type: "commit", sha: commit },
});

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), "jeemi-release-workflow-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, "client/frontend"), { recursive: true });
  for (const [path, text] of Object.entries({
    "client/wails.json": '{"info":{"productVersion":"1.2.3"}}',
    "client/frontend/package.json": '{"packageManager":"pnpm@11.5.0"}',
    "client/go.mod":
      "module jeemi\ngo 1.26.0\nrequire (\n github.com/wailsapp/wails/v2 v2.14.0\n)\n",
    "client/.node-version": "24.0.0\n",
    "CHANGELOG.md": "## 1.2.3\n\n本次发布说明。\n",
  }))
    writeFileSync(join(root, path), text);
  return root;
}

function fakeAPI({ release = null, ref = null, tags = {} } = {}) {
  const calls = [];
  return {
    calls,
    async request(method, path, body) {
      calls.push({ method, path, body });
      if (method === "GET" && path === "releases/tags/v1.2.3") return release;
      if (method === "GET" && path === "git/ref/tags/v1.2.3") return ref;
      if (method === "GET" && path.startsWith("git/tags/"))
        return tags[path.slice("git/tags/".length)] ?? null;
      if (method === "POST" && path === "git/refs") {
        assert.equal(ref, null, "must not rewrite a tag");
        assert.deepEqual(body, { ref: "refs/tags/v1.2.3", sha });
        ref = reference(body.sha);
        return ref;
      }
      assert.fail("unexpected release operation: " + method + " " + path);
    },
  };
}

test("manual releases derive the version from the selected source and validate notes", (t) => {
  const root = fixture(t);
  const context = { event: "workflow_dispatch", ref: "refs/heads/main", sha };
  assert.deepEqual(releaseSource(root, context), source);
  assert.deepEqual(
    releaseSource(root, { ...context, event: "push", ref: "refs/tags/v1.2.3" }),
    { ...source, tagged: true },
  );
  for (const invalid of [
    { sha: "main" },
    { event: "pull_request" },
    { event: "push" },
    { ref: "refs/tags/v1.2.4" },
  ])
    assert.throws(() => releaseSource(root, { ...context, ...invalid }));
  writeFileSync(join(root, "CHANGELOG.md"), "## 1.2.4\nother version\n");
  assert.throws(() => releaseSource(root, context), /缺少版本/);
});

test("preflight is read only; publishing creates a pinned tag exactly once", async () => {
  const api = fakeAPI();
  assert.deepEqual(await checkReleaseTag(source, api.request), {
    tag: source.tag,
    sha,
    ready: true,
  });
  assert.ok(api.calls.every(({ method }) => method === "GET"));
  await checkReleaseTag(source, api.request, { create: true });
  await checkReleaseTag(source, api.request, { create: true });
  assert.equal(api.calls.filter(({ method }) => method === "POST").length, 1);
});

test("a published version skips builds and writes even when the branch has advanced", async () => {
  for (const create of [false, true]) {
    const api = fakeAPI({
      release: { draft: false },
      ref: reference(otherSha),
    });
    assert.deepEqual(await checkReleaseTag(source, api.request, { create }), {
      tag: source.tag,
      sha,
      ready: false,
    });
    assert.deepEqual(
      api.calls.map(({ path }) => path),
      ["releases/tags/v1.2.3"],
    );
  }
});

test("draft retries allow matching lightweight and annotated tags without rewriting", async () => {
  const annotated = { object: { type: "tag", sha: otherSha } };
  for (const ref of [reference(), annotated]) {
    const api = fakeAPI({
      release: { draft: true },
      ref,
      tags: { [otherSha]: reference() },
    });
    assert.equal(
      (await checkReleaseTag(source, api.request, { create: true })).ready,
      true,
    );
    assert.ok(api.calls.every(({ method }) => method === "GET"));
  }
  const conflict = fakeAPI({
    release: { draft: true },
    ref: reference(otherSha),
  });
  await assert.rejects(
    checkReleaseTag(source, conflict.request, { create: true }),
    /指向其他提交/,
  );
  assert.ok(conflict.calls.every(({ method }) => method === "GET"));
});

test("deleted trigger tags and invalid tag objects cannot be recreated or published", async () => {
  const deleted = fakeAPI();
  await assert.rejects(
    checkReleaseTag({ ...source, tagged: true }, deleted.request, {
      create: true,
    }),
    /标签已不存在/,
  );
  assert.ok(deleted.calls.every(({ method }) => method === "GET"));
  const loop = { object: { type: "tag", sha: otherSha } };
  const invalid = fakeAPI({ ref: loop, tags: { [otherSha]: loop } });
  await assert.rejects(checkReleaseTag(source, invalid.request), /有效提交/);
  assert.ok(invalid.calls.length <= 10);
});

test("only a missing API resource means absent; permission and transport failures stop", async () => {
  const repo = "bluevava/jeemi-desktop";
  const token = "test-token-must-not-appear";
  for (const status of [401, 403, 429, 500]) {
    const request = githubRequest(
      repo,
      token,
      async () => new Response("{}", { status }),
    );
    await assert.rejects(
      checkReleaseTag(source, request),
      new RegExp("HTTP " + status),
    );
  }
  const missing = githubRequest(
    repo,
    token,
    async () => new Response("{}", { status: 404 }),
  );
  assert.equal(await missing("GET", "git/ref/tags/v1.2.3"), null);
  await assert.rejects(missing("POST", "git/refs", {}), /HTTP 404/);
  const failing = githubRequest(repo, token, async () => {
    throw new Error(token);
  });
  await assert.rejects(failing("GET", "git/ref/tags/v1.2.3"), (error) => {
    assert.doesNotMatch(error.message, new RegExp(token));
    return /请求失败或超时/.test(error.message);
  });
  assert.throws(() => githubRequest("example/unrelated-repository", token));
});
