# Jeemi 构建资源

本目录保存 Wails 需要版本控制的应用图标与平台打包资源。

- `appicon.png`：跨平台正式应用图标源，同时用于 macOS/Linux 窗口与托盘；React 标题栏使用同源副本 `frontend/src/assets/appicon.png`。
- `darwin/`：macOS 开发和发布 plist。
- `windows/icon.ico`：由 `appicon.png` 生成的多尺寸 Windows 应用与托盘图标。
- `windows/` 其余文件：Windows 版本信息和 `requireAdministrator` 应用程序清单；发布脚本会校验源 XML 与最终 PE 标记。
- `bin/`：Wails 编译输出，已由 Git 忽略。

最终可分发文件由发布脚本归档到 `Bin/jeemi/<platform>-<arch>/`。
