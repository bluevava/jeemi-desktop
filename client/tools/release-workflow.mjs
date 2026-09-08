import { appendFileSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import {
  metadata,
  releaseNotes,
  repositoryDirectory,
  run,
  writeActionsMetadata,
} from "./release-common.mjs";

const commitPattern = /^[0-9a-f]{40}$/;

export function releaseSource(root, { event, ref, sha }) {
  if (
    !["workflow_dispatch", "push"].includes(event) ||
    !/^refs\/(heads|tags)\/.+/.test(ref) ||
    (event === "push" && !ref.startsWith("refs/tags/")) ||
    !commitPattern.test(sha)
  )
    throw new Error("发布必须来自手动运行或版本标签，并固定到完整提交 SHA。");
  const tagged = ref.startsWith("refs/tags/");
  const info = metadata(root, tagged ? ref.slice("refs/tags/".length) : "");
  releaseNotes(readFileSync(join(root, "CHANGELOG.md"), "utf8"), info.version);
  return { tag: info.tag, sha, tagged };
}

export function githubRequest(repository, token, fetcher = fetch) {
  if (repository !== "bluevava/jeemi-desktop" || !token)
    throw new Error("发布需要公开仓库自身的 GITHUB_TOKEN。");
  return async (method, path, body) => {
    let response;
    try {
      response = await fetcher(
        "https://api.github.com/repos/" + repository + "/" + path,
        {
          method,
          headers: {
            Accept: "application/vnd.github+json",
            Authorization: "Bearer " + token,
            "Content-Type": "application/json",
            "X-GitHub-Api-Version": "2026-03-10",
            "User-Agent": "Jeemi-release-workflow",
          },
          body: body === undefined ? undefined : JSON.stringify(body),
          signal: AbortSignal.timeout(30_000),
          redirect: "error",
        },
      );
    } catch {
      throw new Error("GitHub 发布请求失败或超时：" + method + " " + path);
    }
    if (method === "GET" && response.status === 404) return null;
    if (!response.ok)
      throw new Error(
        "GitHub " +
          method +
          " " +
          path +
          " 失败（HTTP " +
          response.status +
          "）。",
      );
    return response.json();
  };
}

async function tagCommit(reference, request) {
  let object = reference.object;
  // Annotated tags may point at another annotated tag; bound the traversal.
  for (let depth = 0; depth < 8; depth++) {
    if (!commitPattern.test(object?.sha)) break;
    if (object.type === "commit") return object.sha;
    if (object.type !== "tag") break;
    object = (await request("GET", "git/tags/" + object.sha))?.object;
  }
  throw new Error("版本标签没有指向有效提交。");
}

export async function checkReleaseTag(
  source,
  request,
  { create = false } = {},
) {
  const { tag, sha, tagged } = source;
  const release = await request("GET", "releases/tags/" + tag);
  if (release?.draft === false) return { tag, sha, ready: false };
  if (release && release.draft !== true)
    throw new Error("无法确认现有 Release 是否已经发布。");
  let reference = await request("GET", "git/ref/tags/" + tag);
  if (!reference && tagged)
    throw new Error("触发本次发布的版本标签已不存在，请检查标签状态。");
  if (!reference && create) {
    // Continue publishing in this run: GITHUB_TOKEN tag pushes do not start
    // another push workflow. Never update or force an existing reference.
    reference = await request("POST", "git/refs", {
      ref: "refs/tags/" + tag,
      sha,
    });
    if (!reference) throw new Error("创建版本标签后未收到有效结果。");
  }
  if (reference && (await tagCommit(reference, request)) !== sha)
    throw new Error(
      "版本标签 " + tag + " 指向其他提交，请更新版本号或重新运行原发布任务。",
    );
  return { tag, sha, ready: true };
}

if (
  process.argv[1] &&
  pathToFileURL(resolve(process.argv[1])).href === import.meta.url
) {
  try {
    const [command, ...extra] = process.argv.slice(2);
    if (!["check", "ensure-tag"].includes(command) || extra.length)
      throw new Error("用法：release-workflow.mjs check | ensure-tag");
    const source = releaseSource(repositoryDirectory, {
      event: process.env.GITHUB_EVENT_NAME,
      ref: process.env.GITHUB_REF,
      sha: run("git", ["rev-parse", "HEAD"], {
        cwd: repositoryDirectory,
        capture: true,
      }).trim(),
    });
    if (
      command === "ensure-tag" &&
      (source.tag !== process.env.RELEASE_TAG ||
        source.sha !== process.env.RELEASE_COMMIT)
    )
      throw new Error("发布阶段的源码与构建阶段的版本或提交不一致。");
    const result = await checkReleaseTag(
      source,
      githubRequest(process.env.GITHUB_REPOSITORY, process.env.GH_TOKEN),
      { create: command === "ensure-tag" },
    );
    writeActionsMetadata(result);
    if (!result.ready)
      console.log("该版本已发布，跳过构建与发布：" + result.tag);
    if (process.env.GITHUB_STEP_SUMMARY)
      appendFileSync(
        process.env.GITHUB_STEP_SUMMARY,
        "发布版本：`" +
          result.tag +
          "`\n\n源码提交：`" +
          result.sha +
          "`\n\n" +
          (result.ready
            ? "版本校验通过。\n"
            : "该版本已发布，跳过构建与发布。\n"),
      );
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
