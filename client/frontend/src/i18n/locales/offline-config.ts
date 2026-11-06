export const zhOfflineConfig = {
  emptyBindings:
    "尚无本地配置，可在上方选择新建内置通用或进程配置并绑定，也可前往配置页自行编辑。",
  localRules: "本地规则 · 在配置页编辑",
  general: "🌐 内置 · 常规通用",
  process: "💻 内置 · 进程规则",
  generalDescription: "纯IP/域名规则，默认走代理，国内GEO库走直连",
  processDescription:
    "使用进程规则为主设置国内应用直连，不走GEO库。域名/IP规则为辅控制代理分流",
  create: "创建可编辑配置",
  createGeneral: "🌐 新建常规通用配置并绑定",
  createProcess: "💻 新建进程规则配置并绑定",
  createFailed: "创建内置配置失败，请检查本地资源库。",
  auto: "配套 Auto（共用节点筛选）",
  defaultBrowser: "包含系统内置浏览器进程",
  visual: "可视化编辑",
  source: "YAML 源码",
  search: "搜索规则",
  addRule: "添加规则",
  editRule: "编辑规则",
  deleteRule: "删除规则",
  moveUp: "上移",
  moveDown: "下移",
  rule: "匹配规则",
  ruleCount: "共 {{count}} 条",
  actions: "操作",
  ruleType: "规则类型",
  matchValue: "匹配内容",
  invalidPayload:
    "无法编辑当前规则，请在 YAML 源码中检查 payload 结构、条数和单行文本；原文已保留。",
  hints: {
    domain: "例如 +.example.com 匹配主域名和子域名。",
    ipcidr: "输入 IPv4 / IPv6 CIDR，例如 192.0.2.0/24。",
    classical:
      "只填写匹配内容，不含出站目标。目标由所属策略组决定；IP 规则可在内容末尾加 ,no-resolve。",
  },
  help: {
    presets: {
      title: "内置离线配置",
      purpose:
        "创建普通可编辑本地配置，绑定订阅后复用其节点。Jeemi、AI、Telegram、Video 各带 Auto，AI 主组与 Auto 共用 HK 排除条件。",
      scenarios:
        "两套方案都先匹配专用站点，再让系统更新直连。通用方案使用 CN 地域直连；进程方案改用国内应用进程与系统内置浏览器直连，其余走 Jeemi。",
      cautions:
        "本地规则可编辑且不提供仓库更新。通用方案引用本地 GeoSite/GeoIP；进程方案的路由规则不引用 GEO，未列出的应用走 Jeemi。主页默认 Fake IP 过滤仍含 geosite 条目，两套方案都需要 GeoSite。缺库时先离线导入或下载。两套配置共享业务组与系统更新规则，重复创建保留现有编辑。创建后需绑定订阅。",
    },
    auto: {
      title: "配套自动选择",
      purpose:
        "为手动选择器生成同名 Auto 自动测速组，并放在候选首项。手动候选和 Auto 使用同一筛选结果。",
      scenarios: "保留手动选节点，同时可选 Auto 使用 mihomo 的 url-test 选择。",
      cautions:
        "Auto 隐藏独立卡片，但可在主组内选择。测速不代表站点解锁；没有符合条件的节点时拒绝连接。当前筛选范围是订阅顶层节点。",
    },
    browser: {
      title: "系统内置浏览器",
      purpose:
        "固定匹配当前平台的系统内置浏览器：Windows Edge、Ubuntu/Linux Firefox、macOS Safari，不读取系统默认浏览器设置。",
      scenarios:
        "进程方案把本规则放在 AI、Telegram 和 Video 之后，系统内置浏览器的其余流量直连；其它浏览器按现有规则分流。可关闭此开关后手动填写进程规则。",
      cautions:
        "需要 mihomo 能识别进程，通常使用 TUN 并开启主页进程查询。规则按进程名匹配，浏览器未安装不影响配置组合；修改系统默认浏览器不会改变这些规则。仅支持本地 classical 规则集。",
    },
  },
};

export const enOfflineConfig = {
  emptyBindings:
    "No local configurations yet. Choose a built-in general or process configuration above to create and bind it, or create one on the configuration page.",
  localRules: "Local rules · edit on the configuration page",
  general: "🌐 Built-in · General routing",
  process: "💻 Built-in · Process routing",
  generalDescription:
    "IP/domain rules: proxy by default, direct for CN GEO matches.",
  processDescription:
    "Chinese apps connect directly by process, without GEO. Domain/IP rules control proxy routing.",
  create: "Create editable configuration",
  createGeneral: "🌐 Create and bind general configuration",
  createProcess: "💻 Create and bind process configuration",
  createFailed:
    "Could not create the built-in configuration. Check the local resource library.",
  auto: "Include Auto (shared node filter)",
  defaultBrowser: "Include the system built-in browser processes",
  visual: "Visual editor",
  source: "YAML source",
  search: "Search rules",
  addRule: "Add rule",
  editRule: "Edit rule",
  deleteRule: "Delete rule",
  moveUp: "Move up",
  moveDown: "Move down",
  rule: "Match rule",
  ruleCount: "{{count}} rules",
  actions: "Actions",
  ruleType: "Rule type",
  matchValue: "Match value",
  invalidPayload:
    "Cannot edit these rules. Check the payload structure, size and single-line strings in YAML source. The original text is retained.",
  hints: {
    domain: "For example, +.example.com matches the domain and its subdomains.",
    ipcidr: "Enter an IPv4 / IPv6 CIDR, such as 192.0.2.0/24.",
    classical:
      "Enter the match value without an outbound target. The owning group supplies the target. IP rules can append ,no-resolve.",
  },
  help: {
    presets: {
      title: "Built-in offline configurations",
      purpose:
        "Create an editable local configuration that uses nodes from the bound subscription. Jeemi, AI, Telegram and Video each include Auto. AI and its Auto share the HK exclusion filter.",
      scenarios:
        "Both presets prioritize dedicated sites and send system updates directly. General routing uses CN GEO rules; process routing instead sends common Chinese apps and the system built-in browser directly. Remaining traffic uses Jeemi.",
      cautions:
        "Local rules are editable with no repository updater. General routing uses local GeoSite/GeoIP; process routing rules use no GEO and unlisted apps use Jeemi. Home's default Fake IP filter contains geosite entries, so both presets still require GeoSite. Import or download missing databases first. Both presets share business groups and system update rules; creating again preserves edits. Bind a preset to apply it.",
    },
    auto: {
      title: "Paired automatic selection",
      purpose:
        "Generate a matching Auto url-test group as the first choice in a manual selector. Both share the same filtered nodes.",
      scenarios:
        "Choose a node manually or select Auto to let mihomo choose using url-test.",
      cautions:
        "Auto has no separate card but remains selectable inside its parent. Latency does not guarantee site access. An empty candidate set rejects traffic. Filters currently cover top-level subscription nodes.",
    },
    browser: {
      title: "System built-in browser",
      purpose:
        "Use fixed process matchers for Windows Edge, Ubuntu/Linux Firefox, or macOS Safari. System default-browser preferences are not read.",
      scenarios:
        "The process preset places this rule after AI, Telegram, and Video; remaining built-in browser traffic goes direct. Other browsers follow existing routing rules. Disable it to enter process rules manually.",
      cautions:
        "Mihomo must identify processes, usually with TUN and process lookup enabled on Home. Rules match process names; an uninstalled browser does not block composition, and changing the default browser does not change these rules. Only local classical rule sets support this option.",
    },
  },
};
