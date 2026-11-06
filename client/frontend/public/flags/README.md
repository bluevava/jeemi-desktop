# 内置国家与地区旗帜

这些 SVG 随 Jeemi 前端一起分发，显示时只访问客户端同源资源，不依赖系统国旗字体或外部 CDN。目录包含 259 个国家及地区旗帜，以及英格兰、苏格兰、威尔士的 3 个 subdivision 旗帜。

## 来源与许可

- 除 `cq.svg` 外，资源来自 [flag-icons 7.6.0](https://github.com/lipis/flag-icons/tree/v7.6.0/flags/4x3)，Copyright (c) 2013 Panayiotis Lipiridis，采用 [MIT 许可](LICENSE.flag-icons.txt)。原始 SVG 未修改；`ac.svg` 对应上游 `sh-ac.svg`，`ta.svg` 对应 `sh-ta.svg`，`ea.svg` 对应 `es.svg`。
- `cq.svg`（Sark）来自 [Twemoji 17.0.3](https://github.com/jdecked/twemoji/blob/v17.0.3/assets/svg/1f1e8-1f1f6.svg)，Copyright Twitter, Inc. and other contributors，采用 [CC BY 4.0](LICENSE.twemoji.txt)。仅把 `viewBox` 从 `0 0 36 36` 调整为 `0 4.5 36 27`，以统一显示比例；图形路径未修改。

[sources.json](sources.json) 保存固定来源版本、npm 归档完整性值、上游相对路径与每个 SVG 的 SHA-256。源码和二进制分发均须保留此说明及两份许可文本。目录受公开源码白名单 `frontend/public` 覆盖，Vite 会把资源和许可原样复制到前端产物，并由 Wails 嵌入。

国家及地区使用 Unicode 区域指示符对应的两字母代码，包含 `cn`、`tw`、`hk`、`mo`、`jp`、`us`、`cq` 等。`gb-eng`、`gb-sct`、`gb-wls` 对应 Unicode tag 序列。代码只用于原始 Emoji 到图像的映射，不推测或改写节点名称。
