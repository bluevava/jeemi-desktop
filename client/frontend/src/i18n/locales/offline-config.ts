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
  platform: "适用平台",
  platforms: { all: "全部平台", windows: "Windows", darwin: "macOS", linux: "Linux" },
  enabled: "启用",
  disabled: "关闭",
  sharedDisabled: "共享开关已关闭",
  toggleResource: "启用 {{name}}",
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
        "创建普通可编辑本地配置，绑定订阅后复用其节点。Jeemi、AI+Google、IM、Video 各带 Auto，AI+Google 主组与 Auto 共用 HK 排除条件。Instagram 独立归 Video；IM 包含 Telegram、Discord、WhatsApp、LINE。",
      scenarios:
        "两套方案都先匹配专用站点，IM、Video 优先于 Google 宽泛域名，再让系统更新直连。通用方案使用 CN 地域直连；进程方案让国内应用及指定浏览器直连：Windows Edge、Linux Firefox、macOS Chrome，其余走 Jeemi。",
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
    platform: {
      title: "规则集适用平台",
      purpose: "只在所选平台生成该规则集及其规则；全部平台表示不限制。资源内容和引用保持保存。",
      scenarios: "进程模板按 Windows、macOS、Linux 分开；浏览器分别为 Edge、Chrome、Firefox。macOS 匹配 Chrome 主进程及其 Framework 路径，不匹配 Safari 或共享 WebKit 进程。均可逐条编辑或删除。",
      cautions: "导出保留所有平台的定义，导入后按目标客户端平台生成。进程匹配仍需要核心识别进程，通常使用 TUN 并开启主页进程查询。其它平台的规则不会生成空提供者。",
    },
    activation: {
      title: "共享资源开关",
      purpose: "关闭规则集后不生成其提供者和规则；关闭策略组后同时移除该组规则、选择器及配套 Auto。内容和引用仍保留。",
      scenarios: "临时停用某类分流后，流量继续匹配后续规则；重新开启即可恢复原有内容。开关影响所有引用它的本地配置。",
      cautions: "保存前验证全部关联订阅，失败保留原状态。仍用作本地 MATCH 或默认代理目标的选择器须先更换目标才能关闭。规则集还需所属组、本地配置开关及平台条件同时允许才会生成。",
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
  platform: "Target platform",
  platforms: { all: "All platforms", windows: "Windows", darwin: "macOS", linux: "Linux" },
  enabled: "On",
  disabled: "Off",
  sharedDisabled: "Disabled in shared library",
  toggleResource: "Enable {{name}}",
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
        "Create an editable local configuration that uses nodes from the bound subscription. Jeemi, AI+Google, IM and Video each include Auto. AI+Google and its Auto exclude HK. Instagram belongs to Video; IM covers Telegram, Discord, WhatsApp and LINE.",
      scenarios:
        "Both presets prioritize dedicated sites, with IM and Video before broad Google domains, and send system updates directly. General routing uses CN GEO rules; process routing sends common Chinese apps and designated browsers directly: Edge on Windows, Firefox on Linux and Chrome on macOS. Remaining traffic uses Jeemi.",
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
    platform: {
      title: "Rule-set target platform",
      purpose: "Emit this rule set and its rules only on the selected platform. All platforms removes the restriction. Contents and references remain saved.",
      scenarios: "The process preset has separate Windows, macOS and Linux sets for Edge, Chrome and Firefox respectively. macOS matches Chrome's main process and Framework paths, excluding Safari and shared WebKit processes. Each entry is editable.",
      cautions: "Exports retain every platform definition; the target client selects its own platform. Process matching requires core process lookup, usually with TUN. Other platforms do not emit empty providers.",
    },
    activation: {
      title: "Shared resource switch",
      purpose: "Disabling a rule set omits its provider and rules. Disabling a policy group also omits its selector and paired Auto. Contents and references remain saved.",
      scenarios: "Pause a routing category so traffic reaches later rules, then enable it to restore its contents. The switch affects all local configurations using this resource.",
      cautions: "All associated subscriptions are validated before saving; failures keep the previous state. Choose another local MATCH or default proxy target before disabling its selector. Group, configuration and platform conditions must also allow a rule set to be emitted.",
    },
  },
};
