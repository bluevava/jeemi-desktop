import {
  zhAuthorization,
  zhAuthorizationHelp,
  zhAuthorizationCleanupHelp,
} from "./authorization";
import { zhMacNetwork, zhMacNetworkHelp } from "./mac-network";
import trayLabels from "../tray-labels.json";
import recoveryLabels from "../recovery-labels.json";
import startupLabels from "../startup-labels.json";
import { zhLocalConfigFieldHelp } from "./local-config-field-help";
import { zhRuleSetEntry } from "./rule-set-entry";
import { zhSubscriptionNormalization } from "./subscription-normalization";
import { zhDNSQuery, zhDNSQueryHelp } from "./dns-query";
import { zhAppUpdate } from "./app-update";

export const zhCN = {
  appUpdate: zhAppUpdate,
  dnsQuery: zhDNSQuery,
  ruleSetEntry: zhRuleSetEntry,
  app: {
    name: "Jeemi",
    tagline: "跨平台 mihomo 桌面客户端",
  },
  tray: trayLabels["zh-CN"],
  recovery: recoveryLabels["zh-CN"],
  startup: startupLabels["zh-CN"],
  nav: {
    home: "主页",
    subscriptions: "订阅",
    config: "配置",
    connections: "链接",
    logs: "日志",
    tools: "工具",
  },
  action: {
    start: "启动代理",
    restart: "重启代理",
    stop: "停止代理",
    minimise: "最小化",
    maximise: "最大化或还原",
    close: "隐藏到托盘",
    unavailable: "将在对应后端能力完成后启用",
  },
  topBar: {
    selectorExits: "当前规则使用的代理选择器出口国家",
    selectorExit: "{{selector}}：{{node}}",
    traffic: "实时上传 {{up}}，下载 {{down}}",
  },
  macNetwork: zhMacNetwork,
  authorization: zhAuthorization,
  runtime: {
    authorization: "Linux 代理统一授权",
    authorizationPending: "正在准备授权助手和代理核心…",
    authorizationCancel: "取消授权",
    authorizationCancelFailed: "取消请求未送达，请关闭系统授权提示。",
    core: "核心",
    systemProxy: "系统代理",
    tun: "TUN",
    tunDevice: "TUN 网卡",
    outboundMode: "实时出站方式",
    proxyMode: "代理模式",
    processId: "mihomo 进程 ID",
    restartAttempt: "连续自动恢复次数",
    platformErrors: {
      corePrivilegesBlocked:
        "当前进程的 NoNewPrivs 或容器权限边界禁止获取网络权限。请在正常 Linux 桌面会话运行 Jeemi，或调整容器的权限限制后重试。",
      systemProxyUnavailable:
        "Linux 系统代理需要当前普通用户的 Ubuntu/GNOME 桌面会话、会话 D-Bus 和可写的 GSettings（libglib2.0-bin）。请勿用 sudo 启动 Jeemi；其它桌面暂未适配。",
      corePermissionRequired:
        "当前 Linux 核心尚未完成持久授权。请点击启动或完整重启，在系统密码框中授权；系统代理和虚拟网卡模式使用同一权限流程。",
      authorizationUnavailable:
        "系统授权不可用。请安装提供 /usr/bin/pkexec 的 Polkit 软件包，并在普通用户桌面会话运行 Jeemi，确认桌面认证代理可用。",
      authorizerMissing:
        "缺少匹配的授权工具，或工具校验失败。请重新解压同一版本、同一架构的完整发布包，将 Jeemi 和 jeemi-authorizer 放在同一目录，并确保助手文件可读。",
      authenticationFailed:
        "系统未批准授权。请确认密码、管理员账户和桌面认证代理，然后重新点击启动。",
      authorizationTimeout: "等待系统授权超时，请重新点击启动。",
      authorizationFailed: "代理授权未完成，请重试并检查系统授权环境。",
      resolverAuthorizationUnavailable:
        "当前系统无法提供按 JeemiTun 限定的 DNS 持久授权。需要 systemd 257 或更新版本、系统 D-Bus、Polkit 和可读取的系统规则目录。",
      resolverAuthorizationConflict:
        "Jeemi 的 DNS 授权规则被修改、权限异常或与已有文件冲突。请由管理员检查 Jeemi 专用规则，再重新启动；程序不会覆盖已有规则。",
      resolverAuthorizationFailed:
        "DNS 持久授权尚未实际生效，可能被管理员策略限制。已停止本次启动，避免使用过程中反复弹出密码框；请检查策略后重试。",
      coreIntegrityFailed:
        "所选核心的文件或摘要发生变化，已停止授权。请重新安装并选择经过校验的核心。",
      authorizationUnsafePath:
        "核心路径不适合持久授权。请使用普通用户拥有的本地 jeemi_data 目录，避免符号链接、硬链接及其他用户可写的目录或核心文件。",
      authorizationBusy:
        "核心文件正在被写入，或文件系统不支持安全锁定。请等待核心安装完成后重试，并使用本地 Linux 文件系统。",
      authorizationFilesystem:
        "核心所在文件系统无法保存或使用网络权限。请将 jeemi_data 放在可写且支持文件能力的本地 Linux 文件系统，避免 nosuid/noexec 挂载。",
      tunDeviceUnavailable:
        "系统没有可用的 /dev/net/tun。请检查 Linux TUN 模块以及虚拟机或容器的设备配置。",
      coreNotExecutable:
        "Linux 核心所在文件系统禁止执行。请将受管数据放在允许执行的本地 Linux 文件系统，并检查 mihomo 的执行权限。",
    },
    on: "已开启",
    off: "未开启",
    proxyModes: {
      off: "未启用",
      system_proxy: "系统代理",
      tun: "虚拟网卡",
    },
    status: {
      frameworkReady: "应用框架已就绪，mihomo 核心尚未安装。",
      coreReady: "所选 mihomo 核心已校验并可供后续启动。",
      coreDownloading: "正在下载并校验官方 mihomo 核心。",
      coreInstalledNotSelected:
        "已有 mihomo 核心，请在主页设置中选择使用版本并确认。",
      coreSelectionMissing:
        "所选 mihomo 版本缺失或校验失败，请重新选择或下载。",
      dataError: "无法读取 Jeemi 数据目录或设置，请检查目录权限与文件完整性。",
      systemProxyRecoveryFailed:
        "检测到上次运行的系统代理恢复状态，但自动恢复失败；请检查系统代理设置。",
      not_installed: "未安装",
      downloading: "下载中",
      ready: "已就绪",
      starting: "启动中",
      running: "运行中",
      stopping: "停止中",
      stopped: "已停止",
      failed: "异常",
    },
  },
  common: {
    planned: "待接入",
    unavailable: "尚未接入",
    learnMore: "查看功能说明",
    cancel: "取消",
    save: "保存",
    edit: "编辑",
    delete: "删除",
    close: "关闭",
    unknownError: "操作失败，未收到具体错误信息。",
  },
  home: {
    stateTitle: "当前状态",
    statusControl: {
      healthy: "运行正常",
      mihomoMissing: "mihomo未安装，去安装",
      geoMissing: "GEO 未安装，去安装",
      geoInvalid: "GEO 配置异常，去处理",
      pending: "代理进程正常，但最新代理参数或配置尚未应用。",
      noSubscription: "没有可用订阅配置，请先在订阅页面选择一份订阅配置。",
      failure: "运行异常（{{phase}} / {{code}}）：{{message}}",
      memory: {
        client: "客户端进程",
        webView: "WebView2",
        webKitGTK: "WebKitGTK",
        webKit: "WebKit",
        mihomo: "mihomo",
        total: "总内存",
      },
      uptime: {
        client: "客户端",
        mihomo: "Mihomo",
        units: {
          second: "秒",
          minute: "分",
          hour: "时",
          day: "天",
        },
      },
    },
    version: "Jeemi",
    platform: "当前平台",
    coreVersion: "Mihomo",
    notAvailable: "尚不可用",
    about: {
      title: "关于Jeemi",
      repository: "开源仓库",
      repositoryError: "无法打开开源仓库，请稍后重试。",
      description: "一个Mihomo内核管理客户端",
      manageCore: "管理版本",
      helper: "授权助手",
      removeHelper: "卸载助手",
      helperStates: { ready: "正常", failed: "异常", update: "待更新", not_installed: "未安装" },
    },
    runtimeActionError:
      "代理操作失败。请确认已选择并校验 mihomo 版本、已选择订阅配置，并检查当前状态中的错误；失败不会替换上一份可用 generation。",
    runtimeConfiguration: {
      title: "最终运行配置",
      open: "查看运行配置",
      loading: "正在读取最终运行配置…",
      error: "无法读取最终运行配置。请先选择订阅并等待配置生成完成。",
      sources: {
        active: "当前运行 generation",
        resolved: "待启动配置快照",
      },
      core: "mihomo {{version}}",
    },
    traffic: {
      title: "网络速度",
      chartLabel: "最近一分钟的实时上传与下载速度",
      requiresCore: "启动 mihomo 后显示真实网络速度",
      upload: "上传",
      download: "下载",
      total: "累计下载 {{down}} · 上传 {{up}}",
      states: {
        offline: "核心未运行",
        connecting: "正在连接流量数据",
        live: "实时流量",
        reconnecting: "正在重连，当前数据已过期",
        error: "流量连接中断，当前数据已过期",
      },
    },
    runtimePreferences: {
      title: "代理运行参数",
      loading: "正在读取代理运行参数…",
      sections: {
        baseline: "基础与监听",
        dns: "DNS 基础",
        dnsResources: "DNS 解析与映射",
      },
      outboundMode: "出站方式",
      outboundModes: {
        rule: "规则",
        global: "全局",
        direct: "直连",
      },
      proxyMode: "代理方式",
      proxyModes: {
        system_proxy: "系统代理",
        tun: "虚拟网卡",
      },
      listener: "外部代理监听",
      listenerTypes: {
        http: "HTTP",
        socks: "SOCKS",
        mixed: "Mixed",
      },
      listenPort: "外部代理监听端口",
      allowLan: "允许局域网连接",
      logLevel: "mihomo 日志级别",
      findProcessMode: "查找进程",
      logLevels: {
        silent: "静默",
        error: "错误",
        warning: "警告",
        info: "信息",
        debug: "调试",
      },
      ipv6: "启用 IPv6",
      dnsEnabled: "启用 DNS",
      dnsListen: "DNS 监听地址",
      dnsIpv6: "DNS 返回 IPv6",
      dnsEnhancedMode: "DNS 增强模式",
      dnsEnhancedModes: {
        "fake-ip": "Fake IP",
        "redir-host": "Redir Host",
      },
      dnsFakeIpRange: "Fake IP IPv4 网段",
      dnsFakeIpRange6: "Fake IP IPv6 网段",
      tunAutoRoute: "TUN 自动路由",
      tunAutoDetectInterface: "TUN 自动检测接口",
      tunRouteExcludeAddress: "Tun排除网段",
      tunRouteExcludeAddressPlaceholder: "- 192.168.0.0/16\n- fc00::/7",
      dnsNameservers: "nameserver",
      dnsFakeIpFilter: "fake-ip-filter",
      dnsProxyServerNameservers: "proxy-server-nameserver",
      dnsNameserverPolicy: "nameserver-policy",
      dnsProxyServerNameserverPolicy: "proxy-server-nameserver-policy",
      hosts: "hosts",
      direct: "DIRECT",
      resourceEnabledAria: "是否注入 {{resource}}",
      quickAdd: "快速添加",
      mergeMode: "组合方式",
      mergeModes: {
        append: "追加去重",
        override: "完全覆盖",
      },
      resolverPlaceholder: "- 223.5.5.5\n- https://dns.alidns.com/dns-query",
      filterPlaceholder: "- +.lan\n- +.local",
      policyPlaceholder: "'geosite:cn':\n  - https://dns.alidns.com/dns-query",
      hostsPlaceholder:
        "'example.test': 127.0.0.1\n'*.example.test':\n  - 127.0.0.1",
      lanBypass: "局域网直连保护",
      lanBypassValue: "{{count}} 条 DIRECT 规则置于 rules 首部",
      lanEditor: {
        edit: "修改",
        title: "修改局域网直连规则",
        restoreDefaults: "恢复系统默认",
        restoreError: "无法读取系统默认规则。",
      },
      yamlError: {
        title: "{{resource}} YAML 解析失败",
        location: "第 {{line}} 行，第 {{column}} 列",
        unknown: "配置格式无效",
        unavailable: "YAML 校验服务当前不可用。",
        revert: "还原本次修改",
        continue: "继续编辑",
      },
      tunStack: "TUN 内核方式",
      tunStacks: {
        system: "system",
        gvisor: "gVisor",
        mixed: "mixed",
      },
      errors: {
        load: "无法读取代理运行参数，请检查 jeemi_data/settings.json 权限与内容。",
        save: "保存代理运行参数失败，现有设置没有被替换。请检查端口、CIDR、列表或 YAML 映射格式。",
      },
    },
  },
  pages: {
    logs: {
      title: "mihomo 实时日志",
      summary: "显示 {{visible}} / {{total}} 条 · 最多保留 {{limit}} 条",
      search: "搜索日志内容",
      coreLevel: "mihomo 日志记录级别",
      displayLevel: "筛选显示级别",
      levels: {
        all: "全部级别",
        error: "错误",
        warning: "警告",
        info: "信息",
        debug: "调试",
      },
      states: {
        offline: "未连接",
        paused: "已暂停",
        connecting: "连接中",
        live: "实时",
        reconnecting: "重连中",
        error: "连接异常",
      },
      columns: {
        time: "时间",
        level: "级别",
        content: "日志内容",
      },
      actions: {
        pause: "暂停接收",
        resume: "继续接收",
        copy: "复制当前结果",
        copied: "已复制",
        clear: "清空日志",
        follow: "回到底部",
      },
      requiresCore: "启动 mihomo 且控制 API 健康后，才能查看实时日志。",
      connecting: "正在连接 mihomo 日志流…",
      empty: "已连接，等待新的 mihomo 日志…",
      pausedEmpty: "日志接收已暂停",
      noMatch: "没有匹配的日志",
      errors: {
        level: "切换 mihomo 日志级别失败，现有配置没有被替换。",
        copy: "复制日志失败，请检查剪贴板权限。",
      },
    },
    tools: {
      cards: {
        ip: "IP 查询",
        unlock: "解锁测试",
        dns: "DNS 查询",
      },
    },
  },
  connections: {
    title: "实时链接",
    summary: "{{count}} 条链接 · 累计上传 {{upload}} · 下载 {{download}}",
    summaryFiltered:
      "显示 {{count}} / {{total}} 条链接 · 累计上传 {{upload}} · 下载 {{download}}",
    search: "搜索目标、进程、规则或代理链",
    loading: "正在连接 mihomo 链接流…",
    requiresCore: "启动 mihomo 且控制 API 健康后，才能查看和关闭实时链接。",
    stale:
      "链接流已中断；以下是 {{time}} 的过期快照，恢复实时连接前不能执行关闭操作。",
    empty: "当前没有活动链接",
    emptySearch: "没有匹配的活动链接",
    process: "进程",
    rule: "命中规则",
    outbound: "实际出口",
    openDetails: "查看链接 {{target}} 的详情",
    viewProcessPath: "查看 {{process}} 的进程路径与链接详情",
    unknown: "未知",
    details: {
      title: "链接详情",
      target: "链接目标",
      processPath: "进程完整路径",
      pathUnavailable: "mihomo 未返回进程路径",
      source: "源地址",
      destination: "目标 IP 与端口",
      network: "网络协议",
      inbound: "入站类型",
      ruleType: "规则类型",
      rulePayload: "规则参数",
      chain: "完整代理链",
      upload: "累计上传",
      download: "累计下载",
      startedAt: "开始时间",
      id: "链接 ID",
      states: {
        active: "活动链接",
        ended: "链接已结束 · 保留最后快照",
        stale: "实时数据已中断 · 保留最后快照",
      },
    },
    closeOne: "关闭此链接",
    closeOneNamed: "关闭链接 {{target}}",
    filters: {
      route: "按出站方式筛选链接",
      all: "全部",
      direct: "直连",
      proxy: "代理",
      selectors: "筛选代理选择器",
      selectorsPlaceholder: "选择代理选择器",
    },
    closeAll: {
      button: "关闭当前结果",
      title: "关闭当前筛选结果？",
      description:
        "将逐条关闭当前筛选出的 {{count}} 条活动链接；未显示的链接不受影响，应用也可能自动重新建立连接。",
      confirm: "关闭当前结果",
    },
    states: {
      offline: "核心未运行",
      connecting: "正在连接",
      live: "实时",
      reconnecting: "正在重连",
      error: "链接流中断",
    },
    errors: {
      close: "关闭链接失败；请确认 mihomo 控制 API 仍然健康后重试。",
    },
  },
  subscription: {
    normalization: zhSubscriptionNormalization,
    fallback: {
      title: "漏网之鱼",
      preserve: "不覆写（{{target}}）",
      direct: "直连",
      undefined: "未定义",
      unavailable: "暂不可用",
      reset: "选择器“{{selector}}”已不存在，漏网之鱼已切换为不覆写。",
      saveFailed: "漏网之鱼保存失败，请刷新配置后重试。",
      connectionResetFailed:
        "漏网之鱼已生效，但部分旧链接未能断开，可能继续使用旧出口。",
    },
    loading: "正在读取订阅配置…",
    import: {
      title: "导入订阅",
      description: "保存完整订阅原文，并生成关联本地配置后的离线选择器预览。",
      add: "添加订阅",
      url: "URL 地址拉取",
      file: "选择配置文件",
      qr: "二维码导入",
      qrScreen: "全屏识别二维码",
      qrImage: "选择二维码图片",
      urlTitle: "从 URL 拉取订阅",
      confirmURL: "拉取并保存",
      fileDialogTitle: "选择订阅配置文件",
      fileFilter: "YAML、JSON、TXT 或 Surge CONF 订阅",
      qrImageDialogTitle: "选择二维码图片",
      qrImageFilter: "PNG、JPEG 或 GIF 图片",
    },
    list: {
      emptyDescription: "还没有订阅配置，请通过 URL、配置文件或二维码导入。",
      noDescription: "没有说明",
    },
    card: {
      size: "原文",
      revisions: "修订",
      updated: "更新时间",
      trafficUnavailable: "未提供流量信息",
      expiryUnavailable: "未提供到期时间",
      ruleProviders: "规则提供者 {{count}}",
      associated: "已关联 {{name}} [{{type}}]",
      handlerConfig: "配置",
      handlerScript: "脚本",
    },
    shelf: {
      title: "订阅配置",
      collapse: "收起订阅配置面板",
      collapseHint: "点击收起订阅面板",
      expand: "展开订阅配置面板",
      refreshCurrent: "重新拉取当前订阅",
      refreshUnavailable: "文件订阅没有可重新拉取的远程地址",
      openRuleProviders: "查看规则提供者",
      subscriptionTools: "当前订阅快捷工具",
      selectorTools: "代理选择器快捷工具",
      speedTest: "测速全部节点",
      speedTestMatched: "测速搜索结果中的节点",
      speedTestBusy: "已有测速队列正在执行，请等待完成",
      speedTestRequiresCore: "当前订阅必须由运行中的 mihomo 加载后才能测速",
      speedTestEmpty: "当前范围没有可测速节点",
      showHiddenSelectors: "显示标记为 hidden 的选择器",
      hideHiddenSelectors: "隐藏标记为 hidden 的选择器",
    },
    selector: {
      title: "代理选择器",
      tools: "代理选择器快捷工具栏",
      search: "快速搜索选择器或节点",
      outboundMode: "切换出站方式",
      outboundModes: {
        rule: "规则模式",
        global: "全局模式",
        direct: "直连模式",
      },
      density: {
        label: "节点卡片密度",
        large: "大",
        medium: "中",
        small: "小",
      },
      globalProxy: "全局代理",
      globalEmpty: "当前配置没有可用于全局代理的真实节点。",
      directMode: "直连模式不会使用代理选择器；所有新链接均直接连接。",
      selectSubscription: "选择一份订阅后查看代理选择器",
      empty: "最终配置中没有代理选择器",
      hiddenOnly: "当前没有可见选择器，可在订阅栏开启显示 hidden 选择器",
      noSearchResults: "没有匹配的选择器或节点",
      unknownType: "未知类型",
      nodeCount: "{{count}}",
      defaultSelection: "配置默认项",
      currentSelection: "当前选择",
      providersPending:
        "{{names}} 的节点由外部代理提供者提供，需要 mihomo 加载后才能显示完整列表。",
      dynamicFiltersPending:
        "配置包含动态 filter/exclude-filter/exclude-type。离线预览不重实现 mihomo 的过滤结果，完整成员列表以运行核心为准。",
      sort: {
        label: "节点排序",
        default: "默认",
        delay: "延迟",
        name: "名称",
      },
      view: {
        label: "选择器风格",
        panel: "面板",
        tabs: "标签",
      },
    },
    delay: {
      test: "测试此节点延迟",
      retest: "重新测试此节点延迟",
      retestCached: "这是已缓存的 Jeemi 测速结果，点击后重新测试",
      retestAutomatic:
        "这是 mihomo 最近记录的测速或健康检查结果，点击后重新测试",
      testNode: "测试节点 {{name}} 的延迟",
      retestNode: "重新测试节点 {{name}} 的延迟",
      requiresCore: "需要当前订阅正在 mihomo 中运行",
      queueBusy: "已有测速队列正在执行",
      failed: "失败",
    },
    projection: {
      failed: "无法生成最终配置预览",
      status: {
        source_unavailable: "当前订阅原文不可用，请检查受管修订文件。",
        normalization_failed: "订阅格式转换失败，请检查转换详情或所选核心版本。",
        local_config_unavailable:
          "关联的本地配置不存在或不可读取，请重新关联。",
        local_script_unavailable:
          "关联的本地脚本不存在或不可读取，请重新关联。",
        script_execution_failed:
          "本地脚本执行失败，订阅原文与当前运行配置均未被改写。",
        composition_failed:
          "订阅配置经过本地处理或运行参数注入后无法完成组合，请检查对应配置。",
        inspection_failed: "组合结果无法解析为可展示的 mihomo 选择器结构。",
      },
    },
    ruleProvider: {
      title: "规则提供者",
      count: "已启用 {{enabled}} / 共 {{total}} 个",
      empty: "最终配置中没有规则提供者",
      refresh: "更新规则提供者 {{name}}",
      requiresCore: "需要 mihomo 已启动且控制 API 健康后才能更新",
      enable: "启用规则提供者 {{name}}",
      disable: "停用规则提供者 {{name}}",
      disabledDescription: "已停用，不会输出到最终运行配置",
      disabledRefresh: "规则提供者已停用，启用并成功加载到 mihomo 后才能更新",
      updated: "{{time}}更新成功",
      updateUnknown: "mihomo 尚未返回有效更新时间",
      updateRequiresCore: "启动当前订阅后显示核心中的更新时间",
      loadingStatus: "正在读取 mihomo 提供者状态…",
      ruleCount: "{{count}} 条规则",
    },
    menu: {
      label: "{{name}} 的订阅操作菜单",
      refresh: "重新拉取",
      edit: "编辑订阅",
      rawText: "查看原文配置",
      associate: "关联本地配置",
      runtimeConfiguration: "查看运行配置",
      associateConfig: "关联本地配置",
      associateScript: "关联本地脚本",
      delete: "删除",
    },
    form: {
      name: "订阅名称",
      namePlaceholder: "例如：日常使用",
      nameOptional: "订阅名称（可选）",
      nameAutoPlaceholder: "留空时从响应头或域名补全",
      url: "订阅 URL",
      file: "原始导入文件",
      description: "说明",
      descriptionPlaceholder: "说明订阅来源或适用场景。",
      icon: "图标",
      iconEdit: "配置订阅图标",
      iconMode: "订阅图标类型",
      iconEmoji: "Emoji",
      iconUrl: "图标 URL",
      iconEmojiPlaceholder: "选择 Emoji",
      iconDetect: "自动检测",
      iconClear: "清除",
      iconDetection: {
        found: "已找到并设置站点图标。",
        not_found:
          "站点根路径没有可用的 favicon.ico、favicon.png、sub.ico 或 sub.png。",
        error: "图标检测失败，请检查订阅地址或网络后重试。",
      },
    },
    edit: {
      title: "编辑订阅",
      save: "保存",
    },
    raw: {
      title: "{{name}} 的原文配置",
    },
    association: {
      title: "关联本地配置",
      none: "不关联本地配置",
      missing: "关联的本地配置不可用",
      empty: "当前还没有本地配置；可以先保持不关联，再到配置页面创建。",
      save: "保存关联",
      conflictTitle: "不能同时关联两种本地处理方式",
      unlinkScriptFirst:
        "这份订阅已关联本地脚本，请先在“关联本地脚本”中取消关联。",
    },
    scriptAssociation: {
      title: "关联本地脚本",
      none: "不关联本地脚本",
      empty: "当前还没有本地脚本；可以先到配置页面创建并运行测试。",
      save: "校验并保存关联",
      conflictTitle: "不能同时关联两种本地处理方式",
      unlinkConfigFirst:
        "这份订阅已关联本地配置，请先在“关联本地配置”中取消关联。",
      validationFailed: "关联脚本后的最终运行配置无效：{{error}}",
    },
    delete: {
      title: "删除订阅？",
      description:
        "将删除“{{name}}”的全部已保存原文修订和关联元数据。此操作不会删除关联的本地配置或脚本。",
    },
    errors: {
      backendUnavailable:
        "订阅管理需要在 Jeemi Wails 桌面运行环境中使用，浏览器独立开发态不会创建模拟订阅。",
      load: "无法读取订阅或本地处理资源，请检查 jeemi_data 目录权限与文件完整性。",
      operation: "订阅操作失败，已保存的当前修订没有被替换。",
      required: "请输入订阅名称和有效的 http/https URL。",
      urlRequired:
        "请输入有效的 http/https 订阅 URL；订阅名称可以留空自动补全。",
      urlImport:
        "URL 拉取失败；请检查地址、网络、文件大小和配置格式。未保存任何不完整内容。",
      fileImport:
        "文件导入失败；支持 .yaml、.json、.txt 和 .conf，内容可以是 mihomo、Surge 或受支持的 URI 订阅。",
      qrImport:
        "二维码图片导入失败；请确认图片清晰，并且二维码内容是 http/https 订阅 URL。",
      screenImport: "二维码订阅导入失败；请检查二维码中的订阅地址及网络连接。",
      screenTimeout:
        "等待系统截图超时，请重新点击识别并完成系统截图确认，也可选择二维码图片导入。",
      screenUnavailable:
        "当前桌面截图服务不可用。Linux Wayland 需要正常运行的桌面截图服务；可先使用系统截图后导入二维码图片。",
      screenDenied:
        "系统未允许本次截图。请在桌面截图确认中允许访问；macOS 请检查屏幕录制权限。也可选择二维码图片导入。",
      screenCapture: "系统截图未完成，请重试或使用系统截图后导入二维码图片。",
      screenImage: "无法读取或清理系统返回的截图，请重试或选择二维码图片导入。",
      screenTooLarge:
        "截图超过大小或像素限制，请用系统截图截取二维码区域后导入图片。",
      screenBusy: "上一次屏幕二维码识别仍在处理中，请稍后再试。",
      screenNoQR:
        "画面中未找到包含有效 http/https 订阅地址的二维码，请完整显示并适当放大二维码后重试。",
      refresh: "重新拉取失败；当前 URL 和当前可用修订保持不变。",
      detail: "无法读取订阅详情，请检查订阅是否仍存在。",
      edit: "保存订阅失败；名称、来源 URL 和当前修订保持不变。",
      rawText: "无法读取当前订阅原文，请检查受管修订文件完整性。",
      association: "保存本地配置关联失败，请确认订阅和本地配置仍然存在。",
      ruleProviderRefresh:
        "规则提供者更新失败，请确认当前运行 generation 使用这份订阅且 mihomo 控制 API 健康。",
      ruleProviderToggle:
        "保存规则提供者开关失败；订阅原文和当前生效配置均未被直接改写。",
      proxySelection:
        "切换代理节点失败；当前选择没有改变，请检查 mihomo 控制 API 和节点名称。",
      proxySelectionSave:
        "节点已切换，但未能保存选择；重启后可能恢复旧选择，请稍后重试。",
      connectionReset:
        "节点已切换，但重置关联链接失败；新链接会使用新节点，现有链接可能继续走旧链路。",
      outboundMode:
        "切换出站方式失败；已保存的运行参数和当前 mihomo 模式保持不变。",
      selectorDisplayPreferences:
        "无法保存节点卡片密度、节点排序或选择器风格；已恢复上一次配置，请检查 settings.json 权限。",
      delayCacheLoad:
        "无法读取节点测速缓存；当前配置仍可使用，但旧测速结果不会显示。",
      delayCacheSave:
        "节点测速结果已显示，但无法写入 jeemi_data 缓存，重启客户端后可能丢失。",
      select: "无法保存当前订阅选择，请检查 settings.json 与订阅目录权限。",
      delete: "删除订阅失败，请检查数据目录权限后重试。",
    },
  },
  localPackage: {
    export: "导出",
    import: "导入并覆盖",
    exportTitle: "导出 Jeemi 配置文件",
    importTitle: "导入 Jeemi 配置文件",
    filter: "Jeemi 配置文件 (*.json)",
    confirmTitle: "确认导入覆盖",
    overwrite: "确认覆盖",
    replace:
      "将使用文件中的“{{name}}”完整覆盖当前“{{target}}”，并保留当前订阅关联。",
    overwrittenGroups: "以下 {{count}} 个同名策略组将被覆盖",
    overwrittenRuleSets: "以下 {{count}} 个同名规则集将被覆盖",
    addedGroups: "将新增 {{count}} 个策略组",
    addedRuleSets: "将新增 {{count}} 个规则集",
    affectedConfigs: "受共享资源变更影响的 {{count}} 份本地配置",
    affectedSubscriptions: "已检查 {{count}} 份关联订阅",
    noConflicts: "没有需要覆盖的同名策略组或规则集。",
    exported: "已导出 .json 文件",
    imported: "已导入并覆盖",
    failed: "导入或导出未完成",
    unchanged: "校验失败的导入不会覆盖现有内容。请处理以下问题后重新选择文件。",
  },
  localScript: {
    tabs: {
      scripts: "本地脚本 {{count}}",
    },
    list: {
      create: "新增本地脚本",
      edit: "编辑",
      delete: "删除",
      menu: "{{name}} 的脚本操作菜单",
      deleteTitle: "删除本地脚本？",
      deleteDescription: "将删除“{{name}}”。仍被订阅关联时会阻止删除。",
      noDescription: "没有说明",
      lines: "行",
      size: "大小",
      updatedAt: "更新时间",
    },
    editor: {
      loading: "正在读取本地脚本与当前订阅…",
      name: "脚本名称",
      namePlaceholder: "例如：移除不可用节点",
      description: "说明",
      descriptionPlaceholder: "说明脚本修改内容和适用订阅。",
      source: "JavaScript 源码",
      testTarget: "测试订阅：{{name}}",
      noTestSubscription:
        "当前没有可用于运行测试的订阅。脚本仍可保存，但关联前必须完成最终配置校验。",
      actionBar: "本地脚本编辑操作",
      test: "运行测试",
      save: "保存脚本",
      unsavedPrompt: "这份本地脚本还有未保存的修改，确定离开并丢弃吗？",
    },
    test: {
      title: "脚本最终运行配置预览",
      staticValidated: "结构校验通过",
      coreValidated: "mihomo 校验通过",
      corePending: "等待 mihomo 校验",
    },
    errors: {
      nameRequired: "请输入脚本名称。",
    },
  },
  localConfig: {
    categories: {
      general: "全局配置",
      controller: "运行时控制",
      inbound: "代理端口与监听器",
      dns: "DNS",
      sniffer: "域名嗅探",
      tun: "TUN",
      outbound: "代理与策略组",
      routing: "规则与规则集合",
      hosts: "Hosts",
      advanced: "隧道与高级功能",
    },
    list: {
      loading: "正在读取本地配置…",
      create: "新增本地配置",
      edit: "编辑",
      delete: "删除",
      menu: "{{name}} 的操作菜单",
      deleteTitle: "删除本地配置？",
      deleteDescription:
        "将删除“{{name}}”的本地配置文件。此操作不会修改订阅配置。",
      noDescription: "没有说明",
      enabledFields: "输出字段",
      ruleProviders: "规则提供者",
      strategyGroups: "本地策略组",
      updatedAt: "更新时间",
    },
    resources: {
      tabs: {
        configs: "本地配置 {{count}}",
        groups: "本地策略组 {{count}}",
        ruleSets: "本地规则集 {{count}}",
      },
      actions: "{{name}} 的资源操作",
      name: "名称",
      description: "说明",
      groups: {
        create: "新增本地策略组",
        edit: "编辑本地策略组",
        deleteTitle: "删除本地策略组？",
        deleteDescription: "将删除“{{name}}”。仍被本地配置引用时会阻止删除。",
        kind: "策略组类别",
        kinds: {
          rule: "规则组",
          selector: "选择器",
        },
        inline: "inline 模式",
        ruleSets: "引用本地规则集",
        ruleSetOwned: "{{name}} · 已由 {{owner}} 引用",
        addRuleSet: "添加规则集",
        removeRuleSet: "移除规则集",
        noRuleSets: "尚未引用本地规则集",
        ruleSetCount: "{{count}} 个规则集",
        policy: "规则目标",
        policyProxy: "代理",
        policyDirect: "直连",
        policyReject: "屏蔽",
        selectorPolicyFixed: "当前选择器（代理）",
        type: "策略组类型",
        emoji: "Emoji 图标",
        emojiPlaceholder: "—",
        icon: "远程图标 URL",
        testUrl: "测速 URL",
        interval: "间隔（秒）",
        tolerance: "容差（毫秒）",
        strategy: "负载均衡策略",
        lazy: "惰性测速",
        proxyTypes: "节点协议筛选",
        namePatterns: "节点名称特征",
        nameRules: "{{count}} 条名称条件",
        help: {
          inline: {
            title: "inline 规则输出",
            description:
              "控制本策略组引用的规则集是以 RULE-SET 方式引用，还是把本地 payload 直接展开到顶层 rules。",
            purpose:
              "明确选择可独立更新的规则提供者，或固定写入最终规则顺序的本地规则。",
            scenarios:
              "默认保持关闭；只有希望把 Jeemi 内编辑的本地规则直接写入最终规则时再开启。",
            cautions:
              "引用 HTTP 规则集时不能开启。切换会改变最终运行配置，仍需完成组合与目标核心校验。",
          },
          policy: {
            title: "规则目标方向",
            description:
              "规则组只声明命中后走代理、直连或屏蔽，不在共享资源中绑定具体选择器。",
            purpose:
              "让同一个规则组在不同本地配置中复用，并由各配置统一决定代理出口。",
            scenarios:
              "代理规则使用本地配置的代理规则出口；直连和屏蔽分别输出 DIRECT 与 REJECT。",
            cautions:
              "选择代理时，引用该规则组的本地配置必须在“规则配置”中设置代理规则出口。",
          },
          ruleSets: {
            title: "策略组规则集",
            description:
              "按当前顺序引用本地规则集，该顺序就是本策略组内规则的输出顺序。",
            purpose:
              "集中维护规则内容与顺序，并让规则目标始终由所属策略组统一决定。",
            scenarios:
              "添加、移除或拖动规则集，以调整同一策略组内规则的匹配先后。",
            cautions:
              "每个规则集最多归属一个策略组；inline 只接受本地 payload，HTTP 规则集必须使用 RULE-SET。",
          },
          lazy: {
            title: "惰性测速",
            description:
              "让非 select 选择器在实际被使用时再执行健康检查，而不是始终主动检查。",
            purpose: "减少暂未使用的选择器产生的周期性探测请求。",
            scenarios: "选择器数量较多，且不希望未使用的组持续测速时开启。",
            cautions:
              "这是 mihomo 的选择器配置语义，不代表节点当前一定可用；实际行为仍受测速 URL 和间隔影响。",
          },
          namePatterns: {
            title: "节点名称条件",
            description:
              "每行填写一个名称片段；普通条件用于包含匹配，以 ! 开头的条件用于排除。",
            purpose:
              "从全部订阅节点开始，按协议和名称筛选；可用国旗工具插入名称中包含的 Emoji。",
            scenarios: "例如用“AI”保留相关节点，再用“!test”排除测试节点。",
            cautions:
              "多个正向条件取并集，所有排除条件最后应用；协议与名称之间取交集，全部留空表示使用全部订阅直接代理节点，按名称去重。",
          },
        },
      },
      ruleSets: {
        create: "新增本地规则集",
        edit: "编辑本地规则集",
        deleteTitle: "删除本地规则集？",
        deleteDescription: "将删除“{{name}}”。仍被本地策略组引用时会阻止删除。",
        sourceType: "来源类型",
        behavior: "规则行为",
        format: "远程格式",
        url: "远程 URL",
        interval: "更新间隔（秒）",
        noResolve: "跳过目标 IP 的 DNS 解析",
        payload: "Mihomo 规则集 YAML",
        payloadPlaceholders: {
          domain: "payload:\n  - +.example.com\n  - example.org",
          ipcidr: "payload:\n  - 192.0.2.0/24\n  - 2001:db8::/32",
          classical:
            "payload:\n  - DOMAIN-SUFFIX,example.com\n  - IP-CIDR,203.0.113.0/24,no-resolve",
        },
        help: {
          noResolve: {
            title: "ipcidr 的 no-resolve",
            description:
              "控制生成 ipcidr 规则时是否追加 no-resolve，跳过对目标 IP 的额外 DNS 解析。",
            purpose: "避免已经是 IP/CIDR 的匹配目标进入不必要的域名解析流程。",
            scenarios:
              "仅在 behavior 为 ipcidr 且确认规则不需要解析目标域名时开启。",
            cautions:
              "domain 不支持该参数；classical 必须在适用的 IP-CIDR 等具体 payload 条目中逐条声明。",
          },
          payload: {
            title: "本地规则集 YAML",
            description:
              "编辑只含顶层 payload 的单文档 mihomo YAML，payload 必须是纯文本规则列表。",
            purpose:
              "保留规则顺序与编辑注释，并让 Jeemi 在保存时执行结构和基础语义校验。",
            scenarios:
              "用于 Jeemi 内维护 domain、ipcidr 或 classical 本地规则内容。",
            cautions:
              "不接受锚点、别名、额外顶层字段或裸列表；classical 的 no-resolve 要写在需要它的具体规则条目中。",
          },
        },
        sourceTypes: {
          inline: "本地 YAML",
          http: "远程 HTTP",
        },
        behaviors: {
          domain: "域名（domain）",
          ipcidr: "IP 网段（ipcidr）",
          classical: "标准规则（classical）",
        },
      },
    },
    editorResources: {
      virtualField: "Jeemi 结构化组合字段",
      groupsTitle: "有序引用本地策略组",
      groupsEmpty: "请先在“本地策略组”页创建规则组或选择器",
      addGroup: "添加本地策略组",
      removeGroup: "移除本地策略组引用",
      noGroups: "尚未引用本地策略组",
      ruleStrategy: "规则覆写方案",
      rulesPrepend: "规则追加到头部",
      rulesBeforeTerminal: "规则追加到终结规则前",
      rulesRebuild: "完全覆写并重构",
      localMatch: "MATCH 覆写",
      matchPreserve: "不覆写",
      matchPlaceholder: "选择 MATCH 出口",
      rebuildSelectorRequired: "完全重构需要引用并启用至少一个本地选择器。",
      matchRequired:
        "完全重构需要选择 MATCH 出口：直连或已引用且启用的本地选择器。",
      matchUnavailable: "MATCH 选择器未引用或已停用，请重新选择。",
      rulesEnabled: "启用本地规则",
      groupEnabled: "启用策略组：{{name}}",
      proxySelector: "代理规则出口",
      proxySelectorRequired:
        "已启用的代理规则需要可用出口，请启用对应选择器或选择其他出口。",
      matchLocked:
        "在上方“规则配置”的 MATCH 覆写中设置；订阅页“漏网之鱼”在本地处理之后还可再次覆写。",
      proxySelectorPlaceholder: "选择代理规则共用的选择器",
      help: {
        localMatch: {
          title: "本地 MATCH 覆写",
          description:
            "设置本地重构结果中未命中前置规则的流量出口，可沿用订阅、直连或选择已引用且启用的本地选择器。",
          purpose: "为本地配置定义兜底出口，与各条已命中规则的目标独立。",
          scenarios: "合并订阅时修改兜底，或完全重构规则后指定新的兜底出口。",
          cautions:
            "完全重构不能选择不覆写，且需要至少一个已启用的本地选择器。关闭本地规则时跳过本地 MATCH。订阅页漏网之鱼随后可再次覆写；其不覆写表示沿用本地处理结果。",
        },
        proxySelector: {
          title: "代理规则出口",
          description:
            "为本地配置中所有目标为“代理”的规则组提供同一个具体选择器。",
          purpose: "把可复用规则组的方向与每份本地配置实际使用的代理出口分开。",
          scenarios:
            "引用代理方向规则组时选择；仅使用选择器策略组时，规则直接指向各自选择器。",
          cautions:
            "这里只控制已命中代理规则的出口，与本地 MATCH 兜底独立。关闭已被代理规则使用的选择器前，需要更换出口或关闭相关规则组。订阅页漏网之鱼最后还能覆盖 MATCH。",
        },
        activation: {
          title: "本地规则开关",
          description:
            "总开关控制是否生成本地选择器、规则和规则提供者；每个引用的开关只控制当前本地配置中的该组。",
          purpose: "暂时忽略全部或部分策略组，保留引用、顺序和组合方式。",
          scenarios: "对比订阅原始规则与本地规则，逐组启用以排查分流效果。",
          cautions:
            "关闭总开关仍会注入自定义字段；重新启用和保存前会检查最终配置。禁用状态随本地配置导入导出，不影响其他配置中相同策略组的开关。",
        },
      },
    },
    editor: {
      loading: "正在加载配置目录与本地配置…",
      name: "配置名称",
      namePlaceholder: "例如：办公网络规则",
      description: "说明",
      descriptionPlaceholder: "说明这份本地配置的用途和适用网络。",
      search: "搜索字段路径或分组",
      onlyEnabled: "仅看已输出",
      showLocked: "显示锁定",
      allProxyNodes: "所有代理节点",
      noSearchResults: "没有匹配的配置字段",
      selectField: "从左侧配置树选择一个字段",
      locked: "Jeemi 管理",
      lockedTitle: "运行时保留字段",
      lockedDescription:
        "该字段由 Jeemi 在生成最终运行配置时安全注入，本地配置不能取得控制权。",
      officialDocs: "官方文档",
      output: "输出到最终运行配置",
      proxyFieldOverride: "代理节点批量覆盖",
      mergeStrategy: "组合方式",
      conflictPolicy: "同名冲突处理",
      localValue: "本地值",
      revealSensitive: "显示敏感值",
      hideSensitive: "隐藏敏感值",
      sensitiveHidden: "该字段可能包含认证信息，默认隐藏。",
      enableToEdit: "启用“输出到最终运行配置”后编辑本地值。",
      actionBar: "本地配置编辑操作",
      preview: "预览本地 YAML",
      save: "保存本地配置",
      unsavedPrompt: "这份本地配置还有未保存的修改，确定离开并丢弃吗？",
    },
    preview: {
      title: "本地 YAML 预览",
      fieldCount: "共 {{count}} 个输出字段",
      proxyOverrideCount: "{{count}} 个代理字段覆盖",
      proxyOverrides: "代理节点批量覆盖（组合时展开）",
    },
    fieldHelp: {
      ...zhLocalConfigFieldHelp,
      generic: {
        title: "{{field}} 字段",
        description:
          "{{field}} 是 mihomo 配置字段；启用后由这份本地配置参与最终运行配置组合。",
        purpose: "为订阅配置补充或覆盖该字段，同时保留订阅原文不变。",
        scenarios: "订阅缺少该能力，或不同订阅需要复用相同客户端配置时使用。",
        cautions:
          "字段含义和可选值以当前 mihomo 官方文档为准；保存前会校验 YAML 类型，运行前还会用真实最终配置校验。",
      },
      "log-level": {
        title: "log-level 日志级别",
        description:
          "控制 mihomo 输出日志的最低级别，可选 silent、error、warning、info 或 debug。",
        purpose: "平衡运行信息完整度、排错能力与日志噪声。",
        scenarios:
          "日常使用通常选择 info；排查核心或规则问题时临时选择 debug。",
        cautions:
          "debug 会产生更多日志，并可能包含目标域名等运行信息，不建议长期启用或未经检查直接分享日志。",
      },
      ipv6: {
        title: "ipv6 总开关",
        description: "控制 mihomo 是否解析和处理 IPv6 流量。",
        purpose: "使支持 IPv6 的网络和节点可以正常使用 IPv6 地址。",
        scenarios: "本地网络、DNS 和代理节点均具备稳定 IPv6 能力时启用。",
        cautions:
          "网络没有可用 IPv6 路由时启用可能增加超时；它与 dns.ipv6 的职责不同。",
      },
      "dns-ipv6": {
        title: "dns.ipv6",
        description: "控制 mihomo DNS 模块是否返回 AAAA（IPv6）结果。",
        purpose: "决定经 mihomo DNS 查询时是否向应用提供 IPv6 地址。",
        scenarios:
          "本地和代理链路具备 IPv6 连通性，并希望应用优先或同时使用 IPv6 时启用。",
        cautions:
          "顶层 ipv6 或实际网络不支持 IPv6 时应谨慎启用，否则应用可能拿到不可达地址。",
      },
      "sniffer-enable": {
        title: "sniffer.enable",
        description:
          "启用域名嗅探，通过 HTTP、TLS 或 QUIC 握手还原连接目标域名。",
        purpose: "让仅提供 IP 目标的连接仍可匹配域名规则。",
        scenarios: "TUN、透明代理或应用绕过系统 DNS 时常用。",
        cautions:
          "嗅探只读取协议握手元数据，不等于解密内容；仍应按隐私需求决定是否开启。",
      },
      "sniffer-force-dns-mapping": {
        title: "sniffer.force-dns-mapping",
        description: "强制对 DNS 映射得到的连接尝试域名嗅探。",
        purpose: "在目标 IP 已存在 DNS 映射时提高域名识别一致性。",
        scenarios: "Fake-IP 或透明代理场景中需要稳定域名规则匹配时使用。",
        cautions:
          "会增加嗅探范围；若特定应用不兼容，可配合 skip-domain 或地址跳过列表。",
      },
      "sniffer-parse-pure-ip": {
        title: "sniffer.parse-pure-ip",
        description: "允许对没有 DNS 映射的纯 IP 连接尝试嗅探域名。",
        purpose: "识别直接连接 IP、但握手中仍携带主机名的流量。",
        scenarios: "应用使用硬编码 IP 或自带 DNS 时使用。",
        cautions: "无法保证所有协议都能识别；不应把嗅探失败视为连接失败。",
      },
      "sniffer-override-destination": {
        title: "sniffer.override-destination",
        description: "使用嗅探得到的域名替换连接目标，供后续解析和路由使用。",
        purpose: "使路由和出站连接以还原后的域名为准。",
        scenarios: "需要域名规则和代理端 DNS 解析一致生效时使用。",
        cautions: "部分使用证书固定、特殊 SNI 或 IP 直连语义的应用可能不兼容。",
      },
      "sniffer-http-ports": {
        title: "sniffer.sniff.HTTP.ports",
        description: "限定进行 HTTP Host 嗅探的端口或端口范围。",
        purpose: "只在预期承载 HTTP 的端口上解析 Host。",
        scenarios: "除 80 外还使用 8080、8880 等 HTTP 端口时补充。",
        cautions: "范围过大会增加无效探测；范围过小会遗漏非标准端口。",
      },
      "sniffer-http-override-destination": {
        title: "sniffer.sniff.HTTP.override-destination",
        description: "仅控制 HTTP 嗅探结果是否覆盖连接目标。",
        purpose: "为 HTTP 单独细化全局 override-destination 行为。",
        scenarios: "希望 HTTP 使用还原域名，但 TLS 或 QUIC 保持原目标时使用。",
        cautions: "覆盖目标可能改变 DNS 解析位置，修改后应验证目标站点连通性。",
      },
      "sniffer-tls-ports": {
        title: "sniffer.sniff.TLS.ports",
        description: "限定从 TLS ClientHello 的 SNI 嗅探域名的端口范围。",
        purpose: "识别 HTTPS 和其他 TLS 连接的目标域名。",
        scenarios: "443、8443 或自定义 TLS 服务端口需要域名规则时使用。",
        cautions: "Encrypted ClientHello 等场景可能无法取得域名。",
      },
      "sniffer-tls-override-destination": {
        title: "sniffer.sniff.TLS.override-destination",
        description: "仅控制 TLS 嗅探结果是否覆盖连接目标。",
        purpose: "为 TLS 流量单独细化目标替换行为。",
        scenarios: "希望 SNI 参与最终连接解析时启用。",
        cautions: "证书固定或依赖原始 IP 的应用可能需要关闭或加入跳过列表。",
      },
      "sniffer-quic-ports": {
        title: "sniffer.sniff.QUIC.ports",
        description: "限定从 QUIC 初始握手嗅探域名的 UDP 端口范围。",
        purpose: "识别 HTTP/3 等 QUIC 流量的目标域名。",
        scenarios: "需要对 443/UDP 或自定义 QUIC 端口应用域名规则时使用。",
        cautions: "协议版本和加密变化可能导致无法识别，范围也不应无谓扩大。",
      },
      "sniffer-quic-override-destination": {
        title: "sniffer.sniff.QUIC.override-destination",
        description: "仅控制 QUIC 嗅探结果是否覆盖连接目标。",
        purpose: "为 QUIC 流量单独决定是否使用还原域名连接。",
        scenarios: "HTTP/3 需要域名路由且目标替换兼容时启用。",
        cautions: "出现 UDP/QUIC 连接异常时可先关闭此项定位问题。",
      },
      "sniffer-force-domain": {
        title: "sniffer.force-domain",
        description: "指定必须尝试嗅探的域名匹配项。",
        purpose: "为容易被默认策略跳过的目标强制启用嗅探。",
        scenarios: "某些域名必须依赖嗅探才能正确命中规则时使用。",
        cautions: "匹配语法应遵循 mihomo 域名通配规则，避免过宽条目。",
      },
      "sniffer-skip-domain": {
        title: "sniffer.skip-domain",
        description: "指定不进行域名嗅探的域名匹配项。",
        purpose: "绕过不兼容或不希望检查握手元数据的目标。",
        scenarios: "智能家居、局域网服务或特殊协议因嗅探异常时使用。",
        cautions: "跳过后相关流量可能只能按 IP 规则匹配。",
      },
      "sniffer-skip-src-address": {
        title: "sniffer.skip-src-address",
        description: "按源 IP/CIDR 跳过域名嗅探。",
        purpose: "为指定局域网设备或来源流量禁用嗅探。",
        scenarios: "某台设备协议不兼容或有独立隐私策略时使用。",
        cautions: "地址变化会使规则失效，CIDR 过宽会跳过大量流量。",
      },
      "sniffer-skip-dst-address": {
        title: "sniffer.skip-dst-address",
        description: "按目标 IP/CIDR 跳过域名嗅探。",
        purpose: "避免对特定服务器或地址段做协议探测。",
        scenarios: "局域网地址、专用服务或已知不兼容目标时使用。",
        cautions: "跳过后只能依赖已有 DNS 映射或 IP 规则。",
      },
      "proxies-ip-version": {
        title: "proxies.ip-version 批量覆盖",
        description: "为订阅中所有代理节点设置域名解析所用的 IP 版本偏好。",
        purpose: "统一节点服务器地址的 IPv4/IPv6 解析策略。",
        scenarios: "订阅节点配置不一致，或当前网络只适合某一 IP 版本时使用。",
        cautions:
          "这是 Jeemi 组合指令，不会写入通配 YAML；最终会展开到真实代理节点。",
      },
      "proxies-udp": {
        title: "proxies.udp 批量覆盖",
        description: "统一设置代理节点是否启用 UDP 转发能力。",
        purpose: "为游戏、QUIC、DNS 等 UDP 流量统一节点行为。",
        scenarios: "订阅未声明 UDP，且所用协议和服务器确认支持 UDP 时启用。",
        cautions: "字段不能让本身不支持 UDP 的协议或服务器凭空获得能力。",
      },
      "proxies-interface-name": {
        title: "proxies.interface-name 批量覆盖",
        description: "将代理节点的拨号流量绑定到指定系统网络接口。",
        purpose: "控制节点连接从哪张物理或虚拟网卡发出。",
        scenarios: "多网卡、策略路由或需要避免 TUN 回环时使用。",
        cautions:
          "接口名必须在当前平台存在；跨设备复用本地配置时通常需要调整。",
      },
      "proxies-routing-mark": {
        title: "proxies.routing-mark 批量覆盖",
        description: "为代理节点的底层连接设置 Linux 路由标记。",
        purpose: "配合 Linux 策略路由表区分 mihomo 发出的连接。",
        scenarios: "高级 Linux 路由、透明代理防回环时使用。",
        cautions: "主要适用于 Linux；标记必须与系统路由规则匹配。",
      },
      "proxies-tfo": {
        title: "proxies.tfo 批量覆盖",
        description: "统一控制代理节点拨号是否使用 TCP Fast Open。",
        purpose: "在系统和服务端支持时减少 TCP 建连等待。",
        scenarios: "已确认网络路径支持 TFO，并希望优化短连接时使用。",
        cautions: "部分网络或系统实现不兼容，遇到连接异常应关闭。",
      },
      "proxies-mptcp": {
        title: "proxies.mptcp 批量覆盖",
        description: "统一控制代理节点拨号是否使用 Multipath TCP。",
        purpose: "在支持的系统和网络中利用多路径传输。",
        scenarios: "具备 MPTCP 内核、路由和服务端条件时使用。",
        cautions: "平台和网络支持有限，不兼容时可能无法连接。",
      },
      "proxies-dialer-proxy": {
        title: "proxies.dialer-proxy 批量覆盖",
        description:
          "让所有代理节点通过指定代理或策略组继续拨号，形成链式代理。",
        purpose: "统一构建前置代理或中转链路。",
        scenarios: "所有订阅节点都需要经过固定入口节点时使用。",
        cautions:
          "目标名称必须存在，且不得形成循环依赖；错误设置会使全部节点不可用。",
      },
      "proxies-client-fingerprint": {
        title: "proxies.client-fingerprint 批量覆盖",
        description:
          "为兼容的 VMess、VLESS、Trojan 和 AnyTLS 节点统一设置 TLS 客户端指纹。",
        purpose: "使 TLS ClientHello 更接近常见浏览器实现。",
        scenarios: "服务端或网络环境要求特定 uTLS 指纹时使用。",
        cautions:
          "Jeemi 只应用到官方支持该字段的协议，不会改写 Shadowsocks 等不适用节点。",
      },
      "proxy-groups": {
        title: "proxy-groups 策略组",
        description: "定义选择器、自动测速、故障转移或负载均衡等代理策略组。",
        purpose: "把代理节点和其他策略组组织成可切换的出站策略。",
        scenarios: "需要按名称或协议筛选节点，并提供手动或自动选择时使用。",
        cautions:
          "本地配置应优先引用配置页统一维护的策略组；同名覆盖和循环引用会阻止最终配置生效。",
      },
      "rule-providers": {
        title: "rule-providers 规则集",
        description:
          "定义可由 RULE-SET 路由规则引用的远程、本地或内联规则集合。",
        purpose: "复用和独立更新大量 domain、ipcidr 或 classical 规则。",
        scenarios: "维护广告拦截、地区分流或业务专用规则集时使用。",
        cautions:
          "behavior、format 和内容必须匹配；远程地址、缓存路径和更新由 mihomo 处理。",
      },
      "local-strategy-groups": {
        title: "本地策略组",
        description:
          "本地策略组分为规则组和选择器。规则组组织规则集与代理、直连、屏蔽方向；选择器筛选代理节点并生成 mihomo proxy-group。",
        purpose:
          "让多份本地配置复用同一套路由规则和代理选择逻辑，资源改名时仍通过稳定 ID 保持引用。",
        scenarios:
          "规则需要走代理规则出口、直连或屏蔽时创建规则组；需要按名称或协议组织节点时创建选择器，国旗可插入名称条件。",
        cautions:
          "规则组不是 mihomo 出站，也不绑定具体选择器；代理方向由本地配置“规则配置”中的代理规则出口解析。选择器规则固定指向自身。",
      },
      "local-rule-sets": {
        title: "本地规则集",
        description:
          "本地规则集保存可复用的 domain、ipcidr 或 classical 规则内容，可来自 Jeemi 内编辑的 payload 或真实 HTTP 地址。",
        purpose:
          "把规则内容与策略组的目标、输出方式和排序分开管理，减少多份本地配置中的重复。",
        scenarios: "维护广告拦截、直连域名、地区分流或业务专用规则列表时使用。",
        cautions:
          "规则集必须由本地策略组引用；behavior 要与内容一致。no-resolve 在 ipcidr 规则集或 classical 的具体目标 IP 条目上设置，不属于策略组引用。远程 HTTP 正文不能在未下载锁定时展开为 inline。",
      },
      rules: {
        title: "规则配置与订阅关联",
        description:
          "启用本地规则后，有序引用策略组并选择加入头部、加入尾部或完全覆写；每个引用均可单独停用。保存后，在订阅卡片菜单的“关联本地配置”中选择此配置。",
        purpose:
          "通过界面重构订阅的分流与选择器；自定义字段配置负责其它 YAML 注入，本地脚本是同一级的另一种处理方式。",
        scenarios:
          "需要把共享规则组或选择器规则追加到订阅头部、终结规则前，或替换订阅普通规则时使用。",
        cautions:
          "完全重构统一替换订阅规则、选择器、规则提供者及原有子规则，要求已引用且启用的本地选择器和明确的 MATCH。本地 MATCH 先生成，订阅页漏网之鱼随后可再覆盖。关闭总开关仅注入自定义字段，保留引用和组合方式。保存前检查全部关联订阅，有可用核心时执行核心校验，失败保留旧数据和草稿。",
      },
    },
    kinds: {
      scalar: "标量字段",
      mapping: "映射字段",
      sequence: "有序数组",
      named_mapping: "命名对象映射",
      named_sequence: "命名对象数组",
      rules: "有序路由规则",
    },
    strategies: {
      replace: "完全覆盖",
      merge: "递归合并，本地冲突优先",
      prepend: "追加到头部",
      append: "追加到尾部",
      merge_by_name: "按名称合并",
      append_before_terminal: "追加到终结规则前",
    },
    conflicts: {
      error: "发现同名项时阻止组合",
      use_local: "明确使用本地项替换同名项",
    },
    errors: {
      backendUnavailable:
        "本地配置需要在 Jeemi Wails 桌面运行环境中使用，浏览器独立开发态不会创建模拟配置。",
      load: "无法读取配置目录或本地配置，请检查 jeemi_data 目录权限与文件完整性。",
      operation:
        "本地配置操作失败，请检查配置是否仍存在、是否仍被订阅关联以及数据目录权限。",
      nameRequired: "请输入本地配置名称。",
      preview: "无法生成本地 YAML 预览，请检查字段值、YAML 类型和合并方式。",
      save: "保存失败；现有本地配置没有被替换，请检查字段值、YAML 类型和数据目录权限。",
    },
    redesign: {
      repairReferences: "资源引用失效，可编辑修复",
      rulesConfig: "规则配置",
      customFields: "自定义字段配置",
      groupInfo: "策略组信息",
      ruleSetConfig: "规则集配置",
      nodeFilter: "节点过滤",
      insertFlag: "插入国旗",
      searchFlag: "搜索英文地区代码，如 US、JP",
      rulesLocked: "此关联字段由上方“规则配置”管理，不写入 mihomo YAML。",
      missingGroup: "缺失的策略组（{{id}}）",
      validationFailed: "检查未通过，本次修改尚未保存",
      keepEditing: "继续编辑",
      revertDraft: "撤回本次修改",
      refreshConflict: "新订阅与本地处理冲突",
      refreshConflictDescription:
        "拉取的订阅配置单独检查通过，但关联的本地配置或脚本处理后检查失败。旧订阅与关联均已保留。可解除关联并保存本次订阅，或拒绝本次更新后继续编辑本地处理。",
      rejectRefresh: "保留旧订阅",
      editHandler: "返回编辑本地处理",
      detachRefresh: "解除关联并更新",
    },
  },
  settings: {
    openPage: "打开{{page}}设置",
    currentPage: "当前正在编辑页面设置",
    pageTitle: "{{page}}设置",
    actionBarLabel: "{{page}}设置操作",
    confirm: "确认",
    cancel: "取消",
    emptyTitle: "{{page}}设置",
    emptyDescription: "当前没有已定义的页面专属设置。",
    managedStorage: {
      loading: "正在读取受管存储目录…",
      error: "无法读取受管存储目录，请检查 jeemi_data 目录权限。",
    },
    home: {
      appearanceTitle: "外观与语言",
      theme: "颜色主题",
      language: "界面语言",
      light: "浅色",
      dark: "深色",
      chinese: "简体中文",
      english: "English",
      chineseShort: "🇨🇳 CN",
      englishShort: "🇺🇸 EN",
      updatesTitle: "应用与核心更新",
      appUpdate: "Jeemi 更新检查",
      coreVersion: "mihomo 版本",
      coreVersionDescription: "检查官方核心更新并选择 Jeemi 使用的版本。",
      core: {
        managerTitle: "mihomo 版本管理",
        loading: "正在读取本地 mihomo 版本…",
        target: "当前平台资产",
        lastChecked: "上次检查",
        latest: "最新稳定版",
        notChecked: "尚未检查",
        check: "检查更新",
        openDirectory: "打开核心目录",
        selectedVersion: "使用版本",
        selectPlaceholder: "选择已安装版本",
        availableTitle: "官方可用版本",
        availableEmpty: "点击“检查更新”获取官方稳定版列表。",
        hideAvailable: "隐藏官方可用版本",
        latestTag: "最新",
        installedTag: "已下载",
        installed: "已下载",
        download: "下载并校验",
        cancelDownload: "取消下载",
        cancelling: "正在取消",
        filesTitle: "本地核心文件",
        inUseTag: "当前使用",
        manualTag: "手动导入",
        noInstalled: "尚未安装 mihomo 核心",
        remove: "删除",
        removeConfirmTitle: "删除本地 mihomo 核心？",
        removeConfirmDescription:
          "将删除 {{version}} 在当前平台下的核心文件，此操作不会删除其他版本。",
        dataDirectory: "Jeemi 数据目录",
        coreDirectory: "mihomo 核心目录",
        manual: {
          open: "手动安装",
          title: "手动安装 mihomo 内核",
          warning:
            "仅导入从 mihomo 官方发布页下载且与你当前平台匹配的文件。手动导入会校验文件边界和可执行格式，但无法提供在线安装具备的发布方 SHA-256 身份校验。",
          stepRepository: "打开 mihomo 官方发布页：",
          openRepository: "打开发布页",
          stepDownload:
            "下载与当前平台资产 {{target}} 对应的 ZIP、GZ 或可执行文件；Windows 官方稳定版通常是 ZIP。",
          stepImport:
            "下载完成后点击下方“导入 mihomo 内核”。Jeemi 会解包并部署到受管核心目录；识别不到版本时会使用 unknown-* 目录。",
          import: "导入 mihomo 内核",
          dialogTitle: "选择 mihomo 内核文件",
          fileFilter: "mihomo 压缩包或可执行文件",
          imported:
            "已导入 {{version}}。如需使用该版本，请在关闭教程后确认“使用版本”，并点击页面底部“确认”。",
        },
        errors: {
          load: "无法读取 mihomo 版本状态，请检查 jeemi_data 目录权限。",
          check: "检查官方 mihomo 版本失败，请检查网络后重试。",
          download: "下载或校验 mihomo 失败，现有可用核心未被替换。",
          cancel: "取消下载失败，请等待当前操作结束后重试。",
          remove:
            "删除本地 mihomo 版本失败；当前使用版本必须先切换后才能删除。",
          open: "无法打开 mihomo 核心目录，请检查系统文件管理器。",
          openReleases: "无法打开 mihomo 官方发布页。",
          import:
            "导入 mihomo 内核失败；请选择当前平台的官方 ZIP、GZ 或可执行文件，现有核心未被替换。",
          select:
            "保存 mihomo 使用版本失败；请确认所选版本仍然存在且校验有效。",
        },
      },
      geoData: {
        managerTitle: "GEO 数据管理",
        loading: "正在读取 GEO 数据状态…",
        geoIpMode: "GeoIP 格式",
        loader: "加载方式",
        source: "更新来源",
        loaders: {
          memconservative: "节省内存",
          standard: "标准",
        },
        sources: {
          official: "官方推荐源",
          custom: "自定义源",
          import: "本地导入",
          existing: "已有活动文件",
        },
        confirmBeforeOperations:
          "当前来源或格式仍是设置草稿。请先点击页面底部“确认”，再检查或更新 GEO 文件。",
        coreRequired:
          "请先安装并确认使用一个 mihomo 核心；下载和导入的 GEO 文件必须由当前核心校验后才能保存。",
        pendingRunning:
          "已校验的新 GEO 文件正在等待重启 mihomo；运行中的核心继续使用上一份活动文件。",
        pendingStopped: "新 GEO 文件已就绪，将在 mihomo 下次启动前安全切换。",
        restartNow: "立即完整重启",
        lastChecked: "上次检查",
        notChecked: "尚未检查",
        activeMode: "活动 GeoIP 格式",
        activeLoader: "活动加载方式",
        check: "检查更新",
        updateAll: "更新当前三项",
        openDirectory: "打开 GEO 目录",
        clean: "清理旧修订",
        cleanConfirmTitle: "清理未使用的 GEO 修订？",
        cleanConfirmDescription:
          "只删除既非当前活动、也非待应用版本的历史文件。",
        cancel: "取消下载",
        cancelling: "正在取消",
        assetsTitle: "当前格式所需规则库",
        assets: {
          "geoip-mmdb": "GeoIP MetaDB",
          "geoip-dat": "GeoIP DAT",
          geosite: "GeoSite DAT",
          asn: "ASN MMDB",
        },
        active: "活动",
        pending: "待重启",
        updateAvailable: "有更新",
        publisherVerified: "发布者摘要已校验",
        notInstalled: "未安装",
        update: "更新",
        download: "下载",
        import: "导入文件",
        importDialogTitle: "导入 {{asset}}",
        importDialogFilter: "mihomo GEO 数据文件",
        manual: {
          open: "手动安装",
          title: "手动安装 GEO 规则库",
          warning:
            "请只从可信发布页下载 GEO 文件。Jeemi 会限制文件大小并使用当前所选 mihomo 核心校验内容，但本地导入文件不具备发布方摘要认证。",
          stepRepository: "打开推荐 GEO 规则库发布页：",
          openRepository: "打开发布页",
          stepDownload:
            "按当前格式下载 GeoIP（geoip.metadb 或 GeoIP.dat）、GeoSite.dat 和 ASN.mmdb。",
          stepImport:
            "使用下方对应按钮逐项导入。全部安装后，停止状态会在下次启动前应用；运行状态需要完整重启。",
          importAsset: "导入 {{asset}}",
        },
        dataDirectory: "修订与清单目录",
        runtimeDirectory: "mihomo 活动文件目录",
        errors: {
          load: "无法读取 GEO 数据状态，请检查 jeemi_data 目录及清单完整性。",
          check: "检查 GEO 更新失败；官方源要求有效的 SHA-256 校验文件。",
          download:
            "GEO 下载、摘要校验或 mihomo 文件校验失败，现有活动文件没有被替换。",
          cancel: "无法取消当前 GEO 下载，请等待操作结束后重试。",
          import: "导入的 GEO 文件未通过大小、格式或当前 mihomo 核心校验。",
          apply:
            "重启并应用 GEO 文件失败；Jeemi 已尝试回滚文件并恢复之前的 mihomo 运行状态。",
          clean: "清理旧 GEO 修订失败，当前活动与待应用文件不会被删除。",
          open: "无法打开 GEO 数据目录，请检查系统文件管理器。",
          openReleases: "无法打开 GEO 规则库发布页。",
        },
      },
      errors: {
        save: "无法保存主页设置；请检查所选核心、GEO 自定义 HTTPS 地址和 jeemi_data/settings.json 权限。",
      },
    },
    subscriptions: {
      fetchTitle: "订阅拉取",
      fetchMode: "拉取方式",
      manual: "手动",
      onLaunch: "启动时",
      displayTitle: "订阅显示",
      displayMode: "节点卡片密度",
      density: {
        large: "大",
        medium: "中",
        small: "小",
      },
      speedTestTitle: "节点测速",
      speedTestConcurrency: "最大并发数量",
      connectionResetTitle: "节点与漏网之鱼切换",
      connectionResetMode: "切换重置链接",
      connectionResetModes: {
        off: "关闭",
        selector: "选择器",
        all: "全部",
      },
      storageTitle: "订阅配置存储目录",
      errors: {
        save: "无法读取或保存订阅页面设置，请检查 jeemi_data/settings.json 权限。",
      },
    },
    config: {
      storageTitle: "受管存储目录",
      scriptStorageTitle: "本地脚本存储目录",
    },
  },
  help: {
    section: {
      description: "功能介绍",
      purpose: "作用",
      scenarios: "使用场景",
      cautions: "注意事项",
    },
    topics: {
      macNetworkAuthorization: zhMacNetworkHelp,
      proxyAuthorization: zhAuthorizationHelp,
      authorizationCleanup: zhAuthorizationCleanupHelp,
      coreAuthorization: {
        title: "Linux 代理统一授权",
        description:
          "首次启动通过系统密码框安装授权助手，为核心的网络操作、低端口监听，以及当前账户对 JeemiTun 的 DNS 设置和恢复统一授权。两种代理模式共用助手，日常启动、停止和完整重启直接复用。",
        purpose:
          "为已校验的 mihomo 授予 CAP_NET_ADMIN、CAP_NET_RAW、CAP_NET_BIND_SERVICE，并安装仅限当前账户与 JeemiTun 的四项 DNS 授权规则。Jeemi 保持普通用户运行，直接管理核心进程。",
        scenarios:
          "首次使用、升级旧版授权方案或修复助手时通过系统确认；新安装的已校验核心由健康助手补齐权限。正常重启系统或 Jeemi 不需要再次输入密码。",
        cautions:
          "助手服务开机待命，不自动启动代理，也不保存密码。同账户其他程序也能修改 JeemiTun 的 DNS；其它网卡不在授权范围。核心被替换后需要重新校验，取消保留原有核心。需要桌面 Polkit 认证代理和本地文件能力支持；使用 systemd-resolved 时需要 systemd 257 或更新版本。系统密码框可能显示已封存授权工具的 /proc/…/fd/… 路径。",
      },
      runtimePreferences: {
        title: "代理运行参数",
        description:
          "自动保存出站、代理、监听、TUN 与安全运行基线，并在订阅和本地配置组合后注入最终运行配置。",
        purpose:
          "即使订阅配置缺少基础字段，也让 mihomo 使用一组明确、可校验且由 Jeemi 统一管理的运行参数。",
        scenarios:
          "在主页调整常用网络参数、DNS 解析器、Fake IP、策略映射和 hosts，无需直接修改订阅原文。",
        cautions:
          "修改先校验再提交；运行中应用失败会保留旧的健康 generation。Linux 系统代理目前适配 Ubuntu/GNOME，只影响遵循桌面代理的应用；终端工具未必跟随。Linux 两种代理模式均在首次启动时通过系统密码框为所选 mihomo 持久授权，界面保持普通用户权限。这里接管的字段在本地配置树中锁定。",
      },
      runtimeBaseline: {
        title: "安全运行基线",
        description:
          "统一控制日志、进程查找、IPv6、代理监听、局域网访问，以及 TUN 自动路由、接口检测和路由排除网段。",
        purpose:
          "补齐不完整订阅中的必要值，并保证系统代理或 TUN 始终由 Jeemi 的单一设置来源管理。",
        scenarios:
          "遇到监听端口冲突、需要局域网共享，或当前平台需要调整 TUN 路由行为时使用。",
        cautions:
          "允许局域网会扩大监听范围；关闭自动路由或接口检测可能需要自行维护路由。排除的网段会绕过 TUN，范围过大会造成流量绕过代理。",
      },
      runtimeFindProcess: {
        title: "查找进程",
        description:
          "控制 mihomo 是否查找连接所属的进程，默认 strict。always 始终查找，strict 由核心判断是否需要，off 关闭查找。",
        purpose:
          "为进程规则和连接信息提供来源进程；主页的选择会注入最终运行配置。",
        scenarios:
          "使用 PROCESS-NAME、PROCESS-PATH 等规则或排查应用连接时调整；通常保留 strict 即可。",
        cautions:
          "查找结果受平台和权限影响；off 会影响依赖进程信息的规则。该字段在本地配置中锁定，订阅或脚本中的同名值会被主页设置覆盖。",
      },
      runtimeTunRouteExclude: {
        title: "Tun排除网段",
        description:
          "以 YAML 字符串数组管理 tun.route-exclude-address；开启注入后可选择追加去重或完全覆盖，启用 auto-route 时匹配网段不会写入 TUN 路由。",
        purpose:
          "让局域网、管理网或其它必须使用系统原生路由的目标绕过 TUN 接管。",
        scenarios:
          "订阅已有排除网段时可追加本地网段，或使用完全覆盖建立独立列表；访问路由器、NAS、企业内网及其它 VPN 网段时常用。",
        cautions:
          "关闭时完整保留订阅配置；只接受 CIDR，例如 192.168.0.0/16 或 fc00::/7。排除公网网段会让流量完全绕过 mihomo，应保持范围最小。",
      },
      runtimeDns: {
        title: "DNS 基础参数",
        description:
          "控制 mihomo DNS 服务、IPv6 响应、增强模式和 Fake IP 地址池。",
        purpose:
          "为缺少 DNS 段的订阅提供可运行默认值，并为 IPv4/IPv6 Fake IP 使用明确、可校验的地址池。",
        scenarios:
          "需要本地 DNS 监听、切换 Fake IP/Redir Host，或处理只支持 IPv4 的本地网络时调整。",
        cautions:
          "默认仅监听 127.0.0.1:1053；改为 0.0.0.0 会向局域网暴露 DNS 服务。Fake IP 网段必须保持正确的 IPv4/IPv6 CIDR 类型。",
      },
      runtimeDnsMerge: {
        title: "DNS 资源组合",
        description:
          "每个 DNS 列表或映射都可独立启用，并选择追加去重或完全覆盖订阅中的同名字段。",
        purpose:
          "既能保留订阅已有解析策略，也能在需要确定结果时由 Jeemi 完整接管。",
        scenarios:
          "追加公共解析器、补充域名策略，或用一套固定列表替换订阅默认值时使用；关闭某项即可完全保留订阅配置。",
        cautions:
          "追加时订阅项保留在前、主页新增项随后加入并按完整值去重；映射发生同名键冲突时以主页值为准。YAML 只在失焦时校验，通过后才保存。",
      },
      runtimeDnsResolvers: {
        title: "DNS 解析器列表",
        description:
          "以 YAML 字符串数组配置 nameserver 或 proxy-server-nameserver，并可通过加号快速加入常用解析器。",
        purpose: "分别为常规域名和代理服务器域名提供明确的上游 DNS。",
        scenarios:
          "需要补充 IP、DoH、DoT 或 DoQ 上游，或避免代理节点域名解析依赖代理链时使用。",
        cautions:
          "nameserver 启用时至少保留一项；关闭资源表示不注入该字段。代理服务器策略只有在 proxy-server-nameserver 有实际值时才会生效。",
      },
      runtimeFakeIpFilter: {
        title: "Fake IP 过滤列表",
        description: "以 YAML 字符串数组管理不应返回 Fake IP 的域名匹配项。",
        purpose:
          "让局域网、发现协议或与 Fake IP 不兼容的域名继续获得真实解析结果。",
        scenarios:
          "局域网域名、设备发现、游戏或特定应用在 Fake IP 模式下解析异常时调整。",
        cautions:
          "关闭资源会完整保留订阅的 fake-ip-filter 及其模式；过宽的规则会降低 Fake IP 模式的覆盖范围。",
      },
      runtimeDnsPolicies: {
        title: "DNS 解析策略映射",
        description: "以 YAML 映射为域名或规则集合指定一个或多个解析器。",
        purpose:
          "让不同域名使用不同上游，并分别控制普通解析与代理服务器解析策略。",
        scenarios:
          "需要分流国内外 DNS、指定内网域名解析器，或单独控制代理节点域名解析时使用。",
        cautions:
          "只接受无锚点、无别名的字符串或字符串数组映射。关闭资源时不会注入或清空订阅字段。",
      },
      runtimeHosts: {
        title: "hosts 映射",
        description:
          "标题栏开关开启后显示编辑器，由主页管理顶层 hosts 映射并同时向 DNS 注入 use-hosts: true。",
        purpose:
          "把特定域名固定解析到一个或多个地址，并按所选方式与订阅 hosts 组合。",
        scenarios: "本地开发、内网服务或临时覆盖域名解析时使用。",
        cautions:
          "只接受无锚点、无别名的 YAML 映射，值必须是字符串或字符串列表。关闭开关会保留订阅原有 use-hosts 与 hosts，而不是强制关闭它们。",
      },
      runtimeLanBypass: {
        title: "局域网直连保护",
        description:
          "Jeemi 默认把常见本机、私网、链路本地和组播地址的 DIRECT 规则放在最终 rules 最前面，并允许修改。",
        purpose: "避免不完整或顺序不当的订阅把局域网与本机访问错误送入代理。",
        scenarios:
          "默认适用于路由器、NAS、局域网服务、回环地址和本地发现协议；开发者也可为调试网络调整或清空列表。",
        cautions:
          "仅允许目标为 DIRECT 的 DOMAIN、DOMAIN-SUFFIX、IP-CIDR 和 IP-CIDR6 规则。空列表表示不追加保护规则；恢复系统默认不会立即保存，仍需点击保存。",
      },
      runtimeListener: {
        title: "外部代理监听",
        description:
          "选择 Jeemi 对外提供的 HTTP、SOCKS 或 Mixed 代理监听及端口。",
        purpose:
          "确保系统代理和需要手动配置代理的应用连接到唯一、明确的本地监听。",
        scenarios:
          "需要兼容特定代理协议，或要调整与其他本机程序冲突的监听端口时使用。",
        cautions:
          "端口必须在 1–65535 范围内且未被占用；修改会重新生成最终运行配置，运行中仅在校验和热更新成功后生效。",
      },
      runtimeLanAccess: {
        title: "允许局域网连接",
        description: "控制代理监听只绑定本机回环地址，还是允许局域网设备连接。",
        purpose: "在明确需要时让同一局域网中的其他设备使用 Jeemi 的代理端口。",
        scenarios:
          "为手机、平板或其他电脑临时提供代理，并已配置网络访问边界时开启。",
        cautions:
          "开启会把代理端口暴露到局域网；应同时配置认证、防火墙和可信网络，公共网络中不要随意开启。",
      },
      trafficChart: {
        title: "实时流量图",
        description:
          "通过 mihomo /traffic WebSocket 每秒接收真实上传、下载速度与累计流量，并绘制最近一分钟趋势。",
        purpose: "快速判断当前是否有流量、上下行变化和代理链路是否持续传输。",
        scenarios: "下载、视频、测速或排查代理启动后没有流量时在主页查看。",
        cautions:
          "只有控制 API 健康时才显示为实时；断线后的曲线会标记为过期并自动重连，不会用模拟数据补齐。",
      },
      home: {
        title: "主页",
        description:
          "集中查看核心状态、内存、运行时长和实时流量，并管理代理启停、出站方式、监听及 DNS 参数。",
        purpose: "让日常检查和问题排查都能快速确认客户端、核心与网络状态。",
        scenarios: "应用启动后、启停核心前后或排查网络问题时优先查看。",
        cautions:
          "打开页面不会修改系统网络。有效运行参数会自动保存；网络操作只有核心健康后才生效。应用失败时保留旧活动配置，并显示原因。",
      },
      processMemory: {
        title: "进程内存",
        description:
          "分别显示客户端、网页运行时、mihomo 和三项合计，使用 MiB 等二进制单位。客户端包含 Jeemi、授权助手和相关辅助进程；授权助手负责提供自身与受管 mihomo 的内存。",
        purpose:
          "单独查看界面与代理核心的内存变化。网页运行时只统计当前 Jeemi 的 WebView2、WebKitGTK 或 WebKit 进程，不混入其他应用；客户端无法读取时由助手协助采集。",
        scenarios: "比较打开页面、加载订阅、隐藏窗口和启停代理前后的内存占用。",
        cautions:
          "Windows 优先统计私有工作集；Linux 和 macOS 统计驻留内存 RSS，共享页可能重复计入，因此合计不是去重后的物理内存占用。助手失联、版本不支持或进程归属无法确认时显示“—”；只有完整取得三项数据才显示合计，不将缺失项当作零。",
      },
      subscriptions: {
        title: "订阅",
        description:
          "通过 URL、配置文件或二维码添加和拉取订阅，管理原文修订，以及互斥的本地配置或本地脚本关联。",
        purpose:
          "把不同来源的订阅以可校验、可追踪、失败可保留的方式纳入 Jeemi。",
        scenarios:
          "首次导入订阅、手动刷新、核对原文或为不同订阅指定本地覆盖时使用。",
        cautions:
          "订阅 URL 和原文可能包含凭据；更新失败不得覆盖当前修订。离线选择器只是组合预览，不能冒充 mihomo 当前运行状态。",
      },
      config: {
        title: "配置",
        description:
          "分区管理本地脚本、本地配置、本地策略组和本地规则集，为订阅提供可复用的处理方式。",
        purpose:
          "用结构化字段或同步 main(config) 脚本调整订阅，并集中维护策略组与规则内容。",
        scenarios:
          "订阅内容不能满足本地网络需求，或不同订阅需要关联不同覆盖策略时使用。",
        cautions:
          "每份订阅最多关联一份本地配置或脚本，两者互斥。保存不等于已激活；被引用资源不能直接删除，最终运行配置仍须校验。",
      },
      connections: {
        title: "链接",
        description:
          "展示实时链接的目标、进程、命中规则、实际出口和流量；点击目标或进程名称查看完整路径、地址和代理链。",
        purpose: "理解当前流量去向并定位规则命中、异常访问或性能问题。",
        scenarios:
          "排查访问失败、异常流量，或需要关闭单条及当前筛选出的活动链接时使用。",
        cautions:
          "规则集显示提供者名称，MATCH 直接显示 MATCH；普通规则显示核心返回的类型和参数，参数缺失时显示 inline，无法还原未返回的原始标记。进程路径可能不可用；链接结束或断流后详情保留最后快照。详情含域名、IP 和路径等隐私，批量关闭需确认且只影响当前筛选结果。",
      },
      logs: {
        title: "日志",
        description: "通过 mihomo 控制 API 实时显示当前核心输出的原始日志。",
        purpose:
          "快速观察规则命中、DNS、连接和核心内部运行信息，并在页面内即时搜索。",
        scenarios:
          "排查访问、解析或规则问题，或临时提高日志级别观察运行过程时使用。",
        cautions:
          "页面只接收打开期间的新日志，不保存历史记录；日志内容按 mihomo 原样显示，可能包含域名、IP 等运行信息。debug 输出量较大，不建议长期启用。",
      },
      tools: {
        title: "工具",
        description: "集中提供 IP 查询、流媒体解锁测试、DNS 查询等独立小工具。",
        purpose: "在不进入复杂配置流程的情况下快速检查当前出口与解析结果。",
        scenarios: "验证代理出口、区域可用性或排查 DNS 解析问题时使用。",
        cautions:
          "查询与解锁测试会向外部服务发起请求并可能暴露出口 IP 或查询域名；必须展示数据来源、超时和失败状态。",
      },
      dnsQuery: zhDNSQueryHelp,
      homeSettings: {
        title: "主页设置",
        description:
          "管理界面主题、语言、Jeemi 更新检查和 mihomo 核心版本选择。",
        purpose: "集中维护影响整个客户端的系统级与界面级偏好。",
        scenarios: "调整显示偏好、检查客户端更新或切换官方核心版本时使用。",
        cautions:
          "主题与语言在确认后由前端保存；核心下载和删除会立即执行，使用版本只有点击确认后才由 Go 控制层持久化。",
      },
      subscriptionSettings: {
        title: "订阅设置",
        description: "管理订阅拉取策略、节点卡片密度和全量测速并发上限。",
        purpose: "让订阅更新、信息密度和测速压力适配不同设备与使用习惯。",
        scenarios:
          "需要手动控制网络请求、切换节点卡片密度或限制大量节点同时测速时使用。",
        cautions:
          "节点密度与测速并发由 Go 原子保存；启动时自动拉取仍保持禁用，直到具备调度与失败回滚。",
      },
      emptySettings: {
        title: "页面设置",
        description: "这是当前导航页面的独立设置入口，但暂时没有已定义选项。",
        purpose: "保持每个一级功能拥有稳定且一致的设置入口。",
        scenarios: "未来出现确实属于该页面的显示或行为偏好时在此增加。",
        cautions:
          "不要为了填满页面创建无实际作用的开关；新增业务设置必须明确默认值、持久化位置和回滚行为。",
      },
      appUpdate: {
        title: "Jeemi 更新检查",
        description: "与 bluevava/jeemi-desktop 开源仓库的最新正式发布版本比较，有更新时询问是否自动下载并更新。",
        purpose: "帮助用户获知安全修复和新功能。",
        scenarios: "在关于卡片或主页设置中手动检查；没有新版本时提示当前已经是最新版本。",
        cautions:
          "确认后会下载匹配系统与架构的发布包，校验摘要，停止代理并自动替换、重启客户端。下载和校验阶段可取消，替换失败会尝试恢复旧文件。安装位置须可写；配套授权助手更新仍通过既有授权流程完成。",
      },
      coreVersion: {
        title: "mihomo 版本",
        description: "检查官方 mihomo 发布并选择 Jeemi 使用的已校验版本。",
        purpose: "在兼容性、功能和回滚需求之间提供明确选择。",
        scenarios: "安装核心、升级新版本或遇到兼容问题需要回退时使用。",
        cautions:
          "在线安装只接受当前平台精确匹配且带 SHA-256 的官方稳定版资产；手动导入没有发布方身份校验，只应选择官方可信文件。安装或导入不会自动切换，选择版本后还需点击页面底部确认。",
      },
      coreFiles: {
        title: "mihomo 核心文件",
        description:
          "管理 jeemi_data/core/mihomo 下由 Jeemi 下载并校验的版本文件。",
        purpose: "允许多个版本并存，以便升级验证、兼容性回退和释放磁盘空间。",
        scenarios:
          "查看核心实际路径、清理不再使用的旧版本或人工备份诊断信息时使用。",
        cautions:
          "当前使用版本禁止删除；识别不到手动导入版本时会使用内容摘要派生的 unknown-* 目录。不要直接替换可执行文件或 metadata.json，否则完整性检查会把该版本视为不可用。",
      },
      geoData: {
        title: "GEO 规则库管理",
        description:
          "独立管理 GeoIP MetaDB/DAT、GeoSite DAT 与 ASN MMDB，并把格式与加载器作为 Jeemi 保留运行参数注入配置。",
        purpose:
          "让来源不完整的订阅也能在需要 GEOIP、GEOSITE 或 IP-ASN 规则时使用经过摘要与当前 mihomo 核心校验的数据库。",
        scenarios:
          "切换 GeoIP 文件格式、更新规则库、使用可信自定义 HTTPS 镜像，或在离线设备上导入本地数据库时使用。",
        cautions:
          "文件不会在运行中的 mihomo 下方直接替换；更新会等待完整重启并支持回滚。自定义源可能没有发布者摘要，只能证明下载完整性与核心可读性，不能证明发布者身份。",
      },
      subscriptionFetch: {
        title: "订阅拉取方式",
        description: "定义订阅由手动操作、应用启动或未来的计划任务触发更新。",
        purpose: "平衡订阅新鲜度、启动速度和网络请求可控性。",
        scenarios: "订阅更新频率不同或需要避免启动时访问订阅服务时调整。",
        cautions:
          "拉取必须可取消并设置超时与大小上限；当前选项仅展示规划，尚未接入持久化。",
      },
      subscriptionDelayTest: {
        title: "节点测速并发",
        description:
          "控制批量测速同时向 mihomo 提交的 delay 请求数量。有快速搜索时只测试搜索结果中的节点，搜索为空时测试全部节点；剩余节点保留在同一队列中。",
        purpose:
          "在测速速度与本机、网络及代理服务商压力之间取得平衡，并阻止重复点击创建重叠队列。",
        scenarios:
          "节点数量很多、设备性能较弱或服务商限制并发时调低；希望更快完成时适度调高。",
        cautions:
          "范围固定为 1–50，默认 8。命中选择器名称会测试该组内全部真实节点，重复节点只测试一次，没有匹配节点时不可启动。测速会产生真实外部请求；队列按点击时的搜索结果执行，修改搜索不改变已开始的队列，执行期间批量和单节点入口暂时锁定。",
      },
      subscriptionConnectionReset: {
        title: "切换重置链接",
        description:
          "决定在订阅页面切换代理节点或漏网之鱼生效后，是否关闭已经建立的链接。",
        purpose: "让长连接及时改走新出口，同时按需限制被中断的链接范围。",
        scenarios:
          "关闭保留现有会话；选择器按代理链精确匹配节点所属选择器，漏网之鱼则匹配切换前实际使用的终结规则目标；全部会重置所有已有链接。",
        cautions:
          "默认是“选择器”。漏网之鱼只在规则模式热更新成功且实际出口改变后，读取当前链接并逐条关闭匹配项；不覆写按原配置目标匹配，直连按 DIRECT 匹配。重启、热更新失败或尚未生效时不额外断开。应用可能自动重连，“全部”会中断其它出口的会话。",
      },
      subscriptionImport: {
        title: "订阅导入",
        description:
          "通过 http/https URL、配置文件或二维码导入 mihomo、Surge 或通用 URI 订阅，保存原始文本。",
        purpose:
          "把不同来源统一保存为可追踪、不可变的订阅修订，为后续组合与运行校验提供可靠输入。",
        scenarios:
          "首次添加订阅、导入 YAML/JSON/TXT/CONF 文件，或扫描服务商提供的订阅二维码时使用。",
        cautions:
          "Surge/URI 会先转换节点和 DNS，再经过本地配置或脚本及运行参数。来源规则不转换；不支持的节点会显示原因。原文不可变，转换或校验失败不会覆盖当前修订。",
      },
      subscriptionQRCode: {
        title: "二维码订阅导入",
        description:
          "从选定图片或桌面即时截图中识别二维码，并把其中的 http/https 内容作为订阅 URL 拉取。Linux Wayland 使用系统截图服务提供的画面。",
        purpose:
          "导入不便手工输入的长订阅地址，同时避免把截图或二维码图片长期保存到数据目录。",
        scenarios:
          "订阅服务只提供二维码，或二维码显示在当前电脑的浏览器、聊天工具及另一块显示器时使用。",
        cautions:
          "识别前会临时隐藏 Jeemi，截图结束或取消后恢复窗口。Linux Wayland 可能显示系统截图确认，是否记住允许由桌面系统决定，不需要管理员密码，也不属于 mihomo 网络授权；macOS 可能要求屏幕录制权限。截图只在本地识别，系统返回的临时截图读取后清理，不保存到 Jeemi 数据目录或上传；只有有效订阅 URL 才会发起拉取。选择已有图片只需文件读取权限。",
      },
      subscriptionRawText: {
        title: "订阅原文",
        description: "只读展示当前订阅修订保存的原始 YAML、JSON 或 TXT 文本。",
        purpose: "便于核对服务商实际返回内容、字段结构和后续配置组合输入。",
        scenarios: "拉取后核查配置、排查格式问题或比较服务商内容时使用。",
        cautions:
          "原文可能包含密码、令牌、节点地址和私有规则 URL；Jeemi 不自动复制或脱敏此视图，分享前必须人工检查。",
      },
      subscriptionAssociation: {
        title: "订阅与本地处理方式关联",
        description:
          "每份订阅最多关联一份本地配置或一份本地脚本，两种处理方式互斥且都可被多份订阅复用。",
        purpose:
          "让结构化字段组合或 JavaScript 对象转换按订阅生效，同时始终保留订阅原文。",
        scenarios:
          "需要复用本地规则和参数时关联配置；需要按程序逻辑转换节点、策略组或其它字段时关联脚本。",
        cautions:
          "脚本关联与已关联脚本的保存都会先生成并校验最终运行配置；校验失败不会写入关联或脚本修改。已被引用的本地处理资源不能删除。",
      },
      subscriptionIcon: {
        title: "订阅图标",
        description:
          "为订阅选择受控 Emoji 或 HTTP(S) 图标 URL；未配置时可从订阅来源站点自动探测常见图标文件。",
        purpose: "在订阅卡片和收起信息栏中快速区分不同订阅。",
        scenarios:
          "可使用固定 Emoji、服务商提供的图标 URL，或让 Jeemi 检测 favicon.ico、favicon.png、sub.ico 与 sub.png。",
        cautions:
          "远程图标在界面显示时会发起网络请求。自动检测只访问订阅来源的同源根路径，不携带订阅路径、查询参数或 Referer，也不会修改订阅原文。",
      },
      localPackage: {
        title: "配置文件导入与导出",
        description:
          "卡片菜单可导出或导入 .json 文件。本地配置包含全部已保存 YAML 树字段、组合方式、引用策略组及其规则集；脚本包含名称、说明和完整源码。",
        purpose: "备份或迁移整套本地配置，也可在当前卡片恢复已有备份。",
        scenarios:
          "在来源客户端导出，到目标客户端的本地配置或脚本卡片选择导入，查看同名覆盖清单后确认。",
        cautions:
          "导入完整覆盖当前卡片并保留其订阅关联；同名资源会影响所有引用它的配置。受影响订阅校验失败时不写入；规则集仍只能属于一个策略组。文件保留原始字段与源码，可能包含自填凭据或 URL，请妥善保存。",
      },
      localConfig: {
        title: "本地配置",
        description:
          "用结构化字段维护订阅配置的稀疏覆盖层，可引用本地策略组并指定字段组合方式。",
        purpose: "让多份订阅复用同一套配置调整，同时保留各自的订阅原文。",
        scenarios:
          "统一节点属性、规则顺序或选择器资源时，新建本地配置，再到订阅页关联。",
        cautions:
          "新建和保存不会自动关联订阅或启动核心；同一订阅的本地配置与本地脚本互斥。Jeemi 保留字段不能由本地配置接管，运行中变更需校验后才应用。",
      },
      localScript: {
        title: "本地脚本转换",
        description:
          "把当前订阅 YAML 解析为普通 JavaScript 对象，调用 main(config)，并把返回对象重新编码为后续组合链路的配置。",
        purpose:
          "为结构化本地配置不便表达的条件式节点、策略组或规则变换提供可复用入口。",
        scenarios:
          "先用当前订阅运行测试并查看最终配置预览，通过后再关联；编辑已被引用的脚本时会逐份验证关联订阅。",
        cautions:
          "脚本在 Go 隔离运行时中执行，不提供文件、网络、进程、环境变量或 Wails API；时钟与随机源固定，单次执行限时且结果必须是有效的顶层配置对象。脚本仍是可执行逻辑，只应使用自己信任的内容。",
      },
      subscriptionShelf: {
        title: "订阅配置面板",
        description:
          "面板位于标题栏下方的应用壳覆盖层；展开时悬浮管理全部订阅卡片，收起后显示当前订阅摘要、带用量背景的流量标签、选择器搜索和快捷操作。",
        purpose:
          "在保留当前订阅流量、到期和更新时间的同时集中节点搜索，并让展开和收起都不移动下方代理选择器。",
        scenarios:
          "切换或编辑订阅时展开；浏览节点时收起，直接搜索选择器或使用刷新、规则提供者、测速和 hidden 选择器按钮。测速只处理当前搜索结果，搜索为空时测试全部节点。",
        cautions:
          "流量和名称可能缺失或过期；测速只在当前投影对应的 mihomo 控制会话健康时启用，显示 hidden 只改变当前界面过滤，不改变订阅配置或运行核心。",
      },
      fallbackTraffic: {
        title: "漏网之鱼",
        description:
          "为当前订阅单独设置未命中前置规则的流量去向，可不覆写、直连或使用组合后存在的选择器。",
        purpose:
          "在本地配置或脚本处理后注入终结规则目标，无需改写订阅原文，也不改变本地代理规则组的默认选择器。",
        scenarios:
          "不同订阅需要不同兜底出口，或希望漏网流量直连时使用。设置按订阅保存，仅在规则模式下参与路由；手动切换生效后，按订阅设置的“切换重置链接”处理旧出口链接。",
        cautions:
          "不覆写括号内显示注入前首条 MATCH / FINAL 的目标，不是当前节点。刷新、切换或编辑配置后，所选选择器须按完整名称匹配，失效会自动保存为不覆写；拉取或组合失败不会清除选择。覆写只原位修改首条顶层终结规则，无终结规则时追加 MATCH；不覆写不会修复来源中已有的无效目标。",
      },
      selectorPreview: {
        title: "代理选择器预览",
        description:
          "从当前订阅原文与关联本地配置的组合结果中读取代理组和可静态解析的节点；标题旁的图标控件与主页共享规则、全局、直连出站方式。",
        purpose:
          "无需先启动 mihomo 就能检查选择器结构；运行后可在规则组、包含全部真实节点的 GLOBAL 组和直连视图之间快速切换。",
        scenarios:
          "导入、刷新、切换订阅或修改本地配置关联后检查组合结果；搜索和临时显示 hidden 组从收起订阅栏操作，出站方式直接在选择器标题旁切换。",
        cautions:
          "全局模式隐藏普通选择器，直连模式不显示选择器。搜索不匹配协议类型或规则/代理提供者名称；外部 provider 节点、延迟和真实当前选择需要运行中的 mihomo。远程组图标仅允许 HTTP(S) 且不发送 Referer。",
      },
      ruleProviderRefresh: {
        title: "规则提供者更新",
        description:
          "显示订阅与本地配置组合后可用的规则提供者；开关按订阅保存，下载按钮通过健康的 mihomo 控制会话更新单个提供者。",
        purpose:
          "让每份订阅独立决定哪些规则提供者进入最终运行配置，并核对核心最后一次成功更新时间。",
        scenarios:
          "临时停用某套分流规则，或规则内容过期、需要让运行核心重新下载时使用。",
        cautions:
          "停用会在组合后移除提供者及其已知引用，不改写订阅原文；开关变动须重新生成、校验并应用配置。更新时间只采用 mihomo 实际返回值；React 不能直接下载规则文件。",
      },
      localConfigField: {
        title: "本地配置字段",
        description:
          "控制一个 mihomo 字段是否输出、本地值以及与订阅值的类型化组合方式。",
        purpose: "避免完整默认配置无意覆盖订阅，并让顺序、冲突和来源可以审查。",
        scenarios: "订阅缺少所需 DNS、规则、TUN 参数或其他高级字段时启用。",
        cautions:
          "只使用当前字段类型允许的策略；同名对象默认阻止组合，运行时保留字段不能由本地值控制。",
      },
      managedStorage: {
        title: "受管存储目录",
        description:
          "只读显示订阅配置、本地配置或本地脚本对应的 Jeemi 数据目录。",
        purpose: "方便备份、迁移和排查文件权限问题。",
        scenarios: "确认数据存放位置，或在关闭客户端后备份相关目录。",
        cautions:
          "目录可能包含订阅凭据或私密配置。运行时不要直接替换受管文件；分享备份前检查内容。此处不修改目录。",
      },
      subscriptionSource: {
        title: "订阅 URL 与名称",
        description:
          "从 HTTP(S) URL 拉取完整订阅配置。新建时名称可留空，依次采用 Profile-Title、Subscription-Userinfo 中的名称或 URL 主机。",
        purpose: "保存可重新拉取的来源，并用可识别的名称区分订阅。",
        scenarios: "首次添加 URL 订阅，或在服务商变更链接后更新来源。",
        cautions:
          "URL 可能含有访问凭据，请勿随意分享。修改 URL 会先拉取并校验新内容，失败时保留旧 URL 和修订；卡片与 Jeemi 普通日志只显示脱敏地址。",
      },
      localConfigPreview: {
        title: "本地 YAML 预览",
        description: "展示当前本地配置草稿输出的 YAML 和批量节点转换信息。",
        purpose: "在保存前核对输出字段、组合方式和敏感内容。",
        scenarios: "检查新增覆盖项，或排查本地配置对订阅的影响。",
        cautions:
          "这里的内容尚未激活，也不是经过订阅组合后的最终运行配置。敏感字段默认隐藏，显示后请勿直接分享。",
      },
      geoDataFormat: {
        title: "GeoIP 格式",
        description:
          "选择 GeoIP 使用的 MMDB/MetaDB 或 DAT 数据库；GeoSite DAT 与 ASN MMDB 单独维护。",
        purpose: "让 GeoIP 数据格式与最终运行配置保持一致。",
        scenarios: "使用默认 MMDB，或在配置需要 DAT 时切换并准备对应数据库。",
        cautions:
          "选择属于设置草稿，确认后才保存。切换格式需要该格式的有效文件；运行中的核心需要完整重启后才能使用新文件。",
      },
      geoDataLoader: {
        title: "GEO 加载器",
        description:
          "选择 DAT 数据的 memconservative（节省内存）或 standard（标准）加载方式。",
        purpose: "按设备资源选择数据加载策略。",
        scenarios:
          "通常保持默认的节省内存模式；在排查 DAT 加载兼容性时尝试标准模式。",
        cautions:
          "加载器不会把 MMDB 转换为 DAT。偏好需要页面确认；运行期间涉及 GEO 变更时等待完整重启。",
      },
      geoDataSource: {
        title: "GEO 下载来源",
        description: "使用推荐来源，或为各类数据库填写自定义 HTTPS 地址。",
        purpose:
          "在默认来源不可达时使用可信镜像，也可通过本地导入准备离线文件。",
        scenarios: "手动检查更新、使用自建镜像，或安装本地下载的数据库。",
        cautions:
          "普通打开设置页不会联网。确认来源后才能按新来源下载；无有效发布者摘要或本地导入只证明本地完整性，不代表发布者认证。",
      },
      runtimeLogLevel: {
        title: "mihomo 日志级别",
        description:
          "控制 mihomo 生成哪些等级的日志，并决定是否显示日志页面入口。",
        purpose: "在日常使用与故障排查之间控制日志量。",
        scenarios: "默认使用静默；排查问题时临时选择错误、警告、信息或调试。",
        cautions:
          "静默会隐藏日志导航并关闭日志页面。调试日志可能含有目标域名、IP 等运行信息；日志页只保存当前内存中的最近 500 条。",
      },
    },
  },
} as const;
