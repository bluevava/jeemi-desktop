export const zhRuleSetEntry = {
  title: "添加规则集",
  add: "添加规则集",
  documentation: "官方文档",
  documentationError: "无法打开 mihomo 官方规则文档，请稍后重试。",
  destination: "保存到规则集",
  newRuleSet: "新建规则集（classical）",
  unnamed: "未命名",
  refresh: "刷新规则集",
  remote: "远程来源，无法追加",
  matchType: "匹配方式",
  ignoreCase: "忽略大小写",
  preview: "将追加的规则行",
  saved: "规则集已保存。",
  notSaved: "本次尚未保存，可继续修改。",
  loadError: "无法加载本地规则集，请重试。",
  previewError: "无法检查当前规则，请修改后重试。",
  types: {
    domain: "域名 · DOMAIN-SUFFIX",
    ip: "IP · IP-CIDR",
    processName: "进程名 · PROCESS-NAME",
    processPath: "进程路径 · PROCESS-PATH-REGEX",
  },
  values: {
    domain: "域名后缀",
    ip: "IP 地址或网段",
    processName: "完整进程名",
    processPath: "完整进程路径或要保留的路径片段",
  },
  errors: {
    resourcesChanged:
      "规则集或策略组已发生变化。请刷新规则集后再保存，当前输入会保留。",
    missingRuleSet: "所选规则集已不存在，请刷新并重新选择。",
    invalidName: "规则集名称应为 1–80 个字符。",
    duplicateName: "已存在同名规则集，请修改名称，或选择该规则集追加。",
    remoteRuleSet: "无法向远程 URL 规则集追加，请选择本地规则集。",
    incompatibleBehavior:
      "domain 规则集只支持域名，ipcidr 只支持 IP；进程规则需要 classical 规则集。",
    invalidPayload:
      "规则集 YAML 无效或超出容量限制，请在配置页面检查原规则集。",
    invalidValue:
      "请输入单行匹配内容，不含换行或控制字符，且不超过 4096 字节。",
    invalidDomain: "请输入有效域名，不含协议、端口、路径、通配符或 IP 地址。",
    invalidIP:
      "请输入有效的 IPv4 / IPv6 地址或网段；前缀长度范围分别为 0–32 / 0–128。",
    invalidProcessName: "请输入完整进程名，不含目录分隔符或逗号。",
    invalidMatchType: "不支持当前匹配方式，请重新选择。",
  },
  help: {
    destination: {
      title: "规则集保存方式",
      purpose: "把当前链接的匹配条件保存到新建或已有的本地规则集；追加时保留原有顺序和注释，新建默认使用 classical。",
      scenarios: "例如把工作软件的域名追加到已有规则集末尾，或保存为“未命名1”后再整理名称。",
      cautions: "点击保存才创建规则集。新规则集还需由本地策略组引用并关联订阅才会生效。规则集中的条目不含代理目标，目标由策略组决定。远程 URL 来源无法追加；domain / ipcidr 仅接受对应类型。结构遵循 mihomo 规则集格式，详见官方文档。",
    },
    domain: {
      title: "域名后缀匹配",
      purpose: "按域名后缀匹配站点及其子域名，用一条规则覆盖同一站点。",
      scenarios: "希望同一站点的主域名和 API、图片等子域名统一走一个出口时使用。",
      cautions: "不要输入 https://、端口或页面路径。classical 中保存 DOMAIN-SUFFIX 条目；domain 规则集中转换为 +.anthropic.com。结构与匹配行为以 mihomo 官方文档为准。",
      example: "anthropic.com 可匹配 anthropic.com 和 api.anthropic.com，不匹配 other-anthropic.com。",
    },
    ip: {
      title: "目标 IP 网段匹配",
      purpose: "按目标 IPv4/IPv6 地址或 CIDR 网段匹配链接。",
      scenarios: "目标只有 IP 地址，或需要统一处理已知服务器网段时使用。",
      cautions: "IPv6 为 128 位，/64 表示网段而不是单个地址。共享 IP 或大网段可能包含其他服务。此处不自动添加 no-resolve，现有 ipcidr 规则集的相关设置会保留。classical 保存 IP-CIDR 条目，ipcidr 只保存网段文本；具体结构与解析行为详见 mihomo 官方文档。",
      example: "20.203.144.245 自动补为 /32；2001:db8::1 自动补为 /128。20.203.144.245/24 会规范为 20.203.144.0/24，覆盖整个网段。",
    },
    processName: {
      title: "完整进程名匹配",
      purpose: "按完整进程名匹配，让同名应用进程的链接使用同一条规则。",
      scenarios: "需要按应用分流，且完整进程名稳定时使用。",
      cautions: "这不是名称片段或正则表达式匹配，不同目录下的同名进程可能同时命中。需要 mihomo 能获取进程信息，且规则集行为为 classical。具体支持情况与结构详见 mihomo 官方文档。",
      example: "输入 WeChat.exe，生成 PROCESS-NAME,WeChat.exe；不要输入 C:\\Apps\\WeChat.exe。",
    },
    processPath: {
      title: "进程路径包含匹配",
      purpose: "按可执行文件路径中包含的文本识别应用，保存为 PROCESS-PATH-REGEX 规则。",
      scenarios: "需要区分同名程序的安装位置，或统一处理某个目录下的程序时使用。",
      cautions: "输入原始路径文本，无需手写正则或双反斜杠。保存时自动转义反斜杠、点号和括号等字符；Linux / macOS 通常使用正斜杠 /，它无需转义。Windows 路径默认忽略大小写，可用开关调整。匹配方式为包含，路径越短范围越大；需要核心取得进程信息并使用 classical 规则集，具体结构详见 mihomo 官方文档。",
      example: "可将 C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe 缩短为 \\Microsoft\\Edge。Linux / macOS 可保留 /usr/bin/ 或 /Applications/Example.app/。",
    },
  },
};

export const enRuleSetEntry = {
  title: "Add to rule set",
  add: "Add to rule set",
  documentation: "Official docs",
  documentationError:
    "Could not open the official mihomo rule documentation. Please try again.",
  destination: "Destination rule set",
  newRuleSet: "New rule set (classical)",
  unnamed: "Unnamed",
  refresh: "Refresh rule sets",
  remote: "Remote source; cannot append",
  matchType: "Match type",
  ignoreCase: "Ignore case",
  preview: "Rule line to append",
  saved: "Rule set saved.",
  notSaved: "Changes were not saved. You can continue editing.",
  loadError: "Could not load local rule sets. Please try again.",
  previewError: "Could not check this rule. Edit it and try again.",
  types: {
    domain: "Domain · DOMAIN-SUFFIX",
    ip: "IP · IP-CIDR",
    processName: "Process name · PROCESS-NAME",
    processPath: "Process path · PROCESS-PATH-REGEX",
  },
  values: {
    domain: "Domain suffix",
    ip: "IP address or subnet",
    processName: "Full process name",
    processPath: "Full process path or a path fragment to keep",
  },
  errors: {
    resourcesChanged:
      "Rule sets or strategy groups have changed. Refresh rule sets before saving; your input will be kept.",
    missingRuleSet:
      "The selected rule set no longer exists. Refresh and select another.",
    invalidName: "Use a rule set name containing 1–80 characters.",
    duplicateName:
      "A rule set with this name already exists. Rename the draft or select that rule set to append.",
    remoteRuleSet:
      "Cannot append to a remote URL rule set. Select a local rule set.",
    incompatibleBehavior:
      "domain rule sets accept domains; ipcidr accepts IPs. Process rules require classical.",
    invalidPayload:
      "The rule set YAML is invalid or exceeds its limits. Check the original rule set on the Config page.",
    invalidValue:
      "Enter a single line without control characters, up to 4096 bytes.",
    invalidDomain:
      "Enter a valid domain without a scheme, port, path, wildcard, or IP address.",
    invalidIP:
      "Enter a valid IPv4 / IPv6 address or subnet. Prefix lengths must be 0–32 / 0–128 respectively.",
    invalidProcessName:
      "Enter the full process name without directory separators or commas.",
    invalidMatchType: "This match type is not supported. Select another.",
  },
  help: {
    destination: {
      title: "Saving a rule",
      purpose: "Save matching conditions from this connection to a new or existing local rule set. Appending preserves its order and comments; new rule sets default to classical.",
      scenarios: "Append a work application's domain to an existing rule set, or save it as Unnamed1 and rename it later.",
      cautions: "The rule set is created only when you save. A new set must be referenced by a local strategy group associated with a subscription to take effect. Entries contain no outbound target; the strategy group supplies it. Remote URL sets cannot be appended to; domain / ipcidr accept only their respective types. See the official mihomo documentation for the rule set structure.",
    },
    domain: {
      title: "Domain suffix matching",
      purpose: "Match a domain and its subdomains with one suffix rule.",
      scenarios: "Use when a site and its API, image, or other subdomains should share a route.",
      cautions: "Omit https://, ports, and page paths. A classical set stores a DOMAIN-SUFFIX entry; a domain set uses +.anthropic.com. See the official mihomo documentation for syntax and matching behavior.",
      example: "anthropic.com matches anthropic.com and api.anthropic.com, but not other-anthropic.com.",
    },
    ip: {
      title: "Destination IP matching",
      purpose: "Match connections by destination IPv4/IPv6 address or CIDR range.",
      scenarios: "Use when only an IP address is available, or a known server range needs the same routing.",
      cautions: "IPv6 addresses are 128 bits; /64 is a subnet, not a single address. Shared IPs or large subnets may include other services. This form does not add no-resolve; existing ipcidr settings are preserved. A classical set stores an IP-CIDR entry; an ipcidr set stores only the subnet. See the official mihomo documentation for syntax and resolution behavior.",
      example: "20.203.144.245 gets /32; 2001:db8::1 gets /128. Entering 20.203.144.245/24 produces 20.203.144.0/24 and covers the entire subnet.",
    },
    processName: {
      title: "Full process name matching",
      purpose: "Match an exact process name so connections from processes with that name share one rule.",
      scenarios: "Use to route an application whose exact process name is stable.",
      cautions: "This is not a substring or regular expression match. Identically named processes in different directories may all match. It requires process information from mihomo and a classical rule set. See the official mihomo documentation for platform support and syntax.",
      example: "WeChat.exe produces PROCESS-NAME,WeChat.exe. Do not enter C:\\Apps\\WeChat.exe.",
    },
    processPath: {
      title: "Process path substring matching",
      purpose: "Identify an application by text contained in its executable path, saved as a PROCESS-PATH-REGEX rule.",
      scenarios: "Use to distinguish same-named programs by installation location, or route programs from one directory together.",
      cautions: "Enter a literal path, without writing regex syntax or doubling backslashes. Backslashes, dots, parentheses, and other regex characters are escaped automatically. Linux / macOS normally use forward slashes /, which need no escaping. Windows paths ignore case by default; adjust the switch as needed. Shorter substrings match more paths. Requires process information and a classical rule set; see the official mihomo documentation for syntax.",
      example: "Shorten C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe to \\Microsoft\\Edge. On Linux / macOS, keep /usr/bin/ or /Applications/Example.app/.",
    },
  },
};
