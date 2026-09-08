# 开发、构建与发布

本文的命令从公开仓库根目录执行。`client/` 是 Wails 工程和唯一 Go 模块。

## 工具版本

| 工具    | 版本来源                                                   |
| ------- | ---------------------------------------------------------- |
| Go      | `client/go.mod` 的 toolchain 或 go 声明                    |
| Wails   | `client/go.mod` 锁定的 `github.com/wailsapp/wails/v2` 版本 |
| Node.js | `client/.node-version`                                     |
| pnpm    | `client/frontend/package.json` 的 packageManager           |

安装对应版本的 Go 和 Node.js 后，可读取工具信息：

```sh
node client/tools/release.mjs metadata
```

按输出版本安装 pnpm 和 Wails CLI。例如，当前版本可执行：

```sh
npm install --global pnpm@11.5.0
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
pnpm --dir client/frontend install --frozen-lockfile
```

将 Go 的 `bin` 目录加入 PATH。Windows 需要 WebView2 Runtime；macOS 构建需要 Xcode Command Line Tools，当前 Node.js 工具链要求 macOS 13.5 或更新系统。完整依赖参见 [Wails 安装说明](https://wails.io/docs/gettingstarted/installation/)。

Ubuntu 22.04 的原生依赖：

```sh
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev binutils
```

Linux 发布使用 WebKitGTK 4.1。运行环境需要 `libgtk-3-0` 与 `libwebkit2gtk-4.1-0` 等对应运行库；构建清单会记录实际系统与 ELF 依赖，不能只根据 glibc 版本判断兼容性。

## 开发运行与检查

```sh
cd client
wails dev
```

Linux 使用 `wails dev -tags webkit2_41`。macOS 无证书开发使用 `wails dev -tags jeemi_local_test`；需要代理授权时，先用下方完整构建入口生成带助手的应用包。

以下命令从仓库根执行：

```sh
pnpm --dir client/frontend typecheck
pnpm --dir client/frontend test --maxWorkers=2
node --test client/tools/*.test.mjs
go -C client test ./...
go -C client vet ./...
```

Linux 的 Go 命令添加 `-tags webkit2_41`，如 `go -C client test -tags webkit2_41 ./...`。macOS 公开构建使用 `jeemi_local_test`，验证该配置时相应添加 `-tags jeemi_local_test`。

需要更新 Wails 绑定时在 `client/` 执行 `wails generate module`。生成绑定使用专门的无数据副作用入口，输出到 `client/frontend/wailsjs/`。开发运行的应用内版本显示开发默认值；发布工具会注入 `wails.json` 中的产品版本。

## 构建完整客户端

先完成依赖安装。下面以 Windows amd64 为例：

```sh
node client/tools/build.mjs --platform windows --arch amd64 --dry-run
node client/tools/build.mjs --platform windows --arch amd64
```

`--platform` 支持 `windows`、`linux`、`macos`；`--arch` 支持 `amd64`、`arm64`。Windows 可以从 amd64 构建 arm64；Linux/macOS 使用相应架构的原生构建环境。`--dry-run` 仅展示计划。

工具依次生成宿主绑定、构建前端、构建授权助手、验证助手、注入版本与助手摘要、构建 GUI、校验架构，再更新 `Bin/jeemi/<平台-架构>/`。失败保留旧发布目录。Windows GUI 保持 `asInvoker`，助手带 `requireAdministrator`；Linux 助手不依赖图形库。

macOS 默认采用 ad-hoc 签名，GUI 与助手同时编译 `jeemi_local_test` 标签。包内助手按固定 CDHash 授权，服务名为 `com.jeemi.Jeemi.NetworkHelper.LocalTest`；首次安装或更新由客户端发起管理员授权。此构建不需要证书、不提交公证，也不代表已经通过目标 Mac 的网络验收。

直接执行 `wails build` 可以生成 GUI，但完整的授权助手和摘要绑定需要上述构建入口。

## GitHub 发布

普通分支与 PR 运行源码检查。推送与 `client/wails.json` 一致的 `vX.Y.Z` 标签会触发六目标构建，全部成功后才发布 Release。每个版本需要在 `CHANGELOG.md` 中有 `## X.Y.Z` 条目。

GitHub Actions 工作流只读取本仓库。发布任务使用仓库自身的 `GITHUB_TOKEN` 和明确声明的 `contents: write` 权限；当前 ad-hoc 签名无需证书 Secret。

分发归档由原生构建宿主生成：

```sh
node client/tools/release.mjs pack windows amd64
```

同样支持其他两个平台。`Bin/jeemi/` 保留常规发布目录；GitHub 专用 ZIP 或 tar.gz、归档摘要及版本清单位于 `Bin/github/`。macOS 使用 `ditto` 归档，Linux 使用 tar.gz 保留执行权限。下载后的包包含自己的 `SHA256SUMS`，Release 另附所有归档的总摘要。

发布先建立草稿并上传全部文件，完成后才公开。失败可以重跑原工作流；已公开版本不覆盖，后续变更需要新版本。工作流运行结果证明构建与测试通过，不替代真实桌面、授权和网络场景验证。
