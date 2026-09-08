import {
  enAuthorization,
  enAuthorizationHelp,
  enAuthorizationCleanupHelp,
} from "./authorization";
import { enMacNetwork, enMacNetworkHelp } from "./mac-network";
import trayLabels from "../tray-labels.json";
import recoveryLabels from "../recovery-labels.json";
import { enLocalConfigFieldHelp } from "./local-config-field-help";
import { enRuleSetEntry } from "./rule-set-entry";

export const enUS = {
  ruleSetEntry: enRuleSetEntry,
  app: {
    name: "Jeemi",
    tagline: "Cross-platform mihomo desktop client",
  },
  tray: trayLabels["en-US"],
  recovery: recoveryLabels["en-US"],
  nav: {
    home: "Home",
    subscriptions: "Subscriptions",
    config: "Config",
    connections: "Connections",
    logs: "Logs",
    tools: "Tools",
  },
  action: {
    start: "Start proxy",
    restart: "Restart proxy",
    stop: "Stop proxy",
    minimise: "Minimise",
    maximise: "Maximise or restore",
    close: "Hide to tray",
    unavailable:
      "Available after the matching backend capability is implemented",
  },
  topBar: {
    selectorExits: "Exit countries for proxy selectors used by current rules",
    selectorExit: "{{selector}}: {{node}}",
    traffic: "Live upload {{up}}, download {{down}}",
  },
  macNetwork: enMacNetwork,
  authorization: enAuthorization,
  runtime: {
    authorization: "Linux proxy authorization",
    authorizationPending: "Preparing the authorization helper and proxy core…",
    authorizationCancel: "Cancel authorization",
    authorizationCancelFailed:
      "The cancellation request failed. Dismiss the system authorization prompt.",
    core: "Core",
    systemProxy: "System proxy",
    tun: "TUN",
    tunDevice: "TUN adapter",
    outboundMode: "Live outbound mode",
    proxyMode: "Proxy mode",
    processId: "mihomo process ID",
    restartAttempt: "Consecutive recovery attempts",
    platformErrors: {
      corePrivilegesBlocked:
        "NoNewPrivs or the container capability bounding set prevents acquiring network privileges. Run Jeemi in a normal Linux desktop session or adjust the container restrictions before retrying.",
      systemProxyUnavailable:
        "Linux system proxy requires the current unprivileged user's Ubuntu/GNOME desktop, session D-Bus, and writable GSettings (libglib2.0-bin). Do not launch Jeemi with sudo. Other desktops are not supported yet.",
      corePermissionRequired:
        "The selected Linux core needs persistent authorization. Click Start or Full restart and approve the system password dialog. Both proxy modes use the same authorization flow.",
      authorizationUnavailable:
        "System authorization is unavailable. Install the Polkit package providing /usr/bin/pkexec, run Jeemi in your normal desktop session, and ensure a desktop authentication agent is available.",
      authorizerMissing:
        "The matching authorization helper is missing or failed verification. Extract the complete release for the same version and architecture, keep Jeemi and jeemi-authorizer together, and ensure the helper file is readable.",
      authenticationFailed:
        "System authorization was not approved. Check the password, administrator account, and desktop authentication agent, then click Start again.",
      authorizationTimeout:
        "System authorization timed out. Click Start again.",
      authorizationFailed:
        "Proxy authorization did not complete. Retry and check the system authorization environment.",
      resolverAuthorizationUnavailable:
        "This system cannot provide persistent DNS authorization scoped to JeemiTun. It requires systemd 257 or newer, the system D-Bus, Polkit, and a readable system rules directory.",
      resolverAuthorizationConflict:
        "The Jeemi DNS authorization rule was modified, has unsafe permissions, or conflicts with an existing file. Ask the administrator to check the dedicated Jeemi rule and try again. Existing rules will not be overwritten.",
      resolverAuthorizationFailed:
        "Persistent DNS authorization is not effective, possibly due to an administrator policy. This start was cancelled to avoid repeated password dialogs during use. Check the policy and retry.",
      coreIntegrityFailed:
        "The selected core file or digest changed. Authorization was stopped. Reinstall and select a verified core.",
      authorizationUnsafePath:
        "The core path is unsafe for persistent authorization. Use a locally stored jeemi_data directory owned by your normal user, without symlinks, hard links, or directories and core files writable by other users.",
      authorizationBusy:
        "The core file is being written, or the filesystem cannot safely lock it. Wait for installation to finish and retry on a local Linux filesystem.",
      authorizationFilesystem:
        "The core filesystem cannot persist or use network capabilities. Store jeemi_data on a writable local Linux filesystem supporting file capabilities, without nosuid/noexec mounts.",
      tunDeviceUnavailable:
        "No usable /dev/net/tun device is available. Check Linux TUN support and the device configuration of the VM or container.",
      coreNotExecutable:
        "The Linux core is on a filesystem that disallows execution. Use a local Linux filesystem that permits execution for managed data and check the mihomo executable permission.",
    },
    on: "On",
    off: "Off",
    proxyModes: {
      off: "Disabled",
      system_proxy: "System proxy",
      tun: "Virtual adapter",
    },
    status: {
      frameworkReady:
        "The app framework is ready; the mihomo core is not installed yet.",
      coreReady:
        "The selected mihomo core is verified and ready for a later start.",
      coreDownloading: "Downloading and verifying an official mihomo core.",
      coreInstalledNotSelected:
        "A mihomo core is installed. Select a version in Home settings and confirm it.",
      coreSelectionMissing:
        "The selected mihomo version is missing or failed verification. Download or select another version.",
      dataError:
        "Jeemi could not read its data directory or settings. Check permissions and file integrity.",
      systemProxyRecoveryFailed:
        "Jeemi found system-proxy recovery state from the previous run but could not restore it automatically. Check the operating-system proxy settings.",
      not_installed: "Not installed",
      downloading: "Downloading",
      ready: "Ready",
      starting: "Starting",
      running: "Running",
      stopping: "Stopping",
      stopped: "Stopped",
      failed: "Error",
    },
  },
  common: {
    planned: "Planned",
    unavailable: "Not connected yet",
    learnMore: "View feature help",
    cancel: "Cancel",
    save: "Save",
    edit: "Edit",
    delete: "Delete",
    close: "Close",
    unknownError: "The operation failed without an error description.",
  },
  home: {
    stateTitle: "Current status",
    statusControl: {
      healthy: "Running normally",
      mihomoMissing: "mihomo is not installed — install it",
      geoMissing: "GEO data is not installed — install it",
      geoInvalid: "GEO configuration is invalid — review it",
      pending:
        "The proxy process is healthy, but the latest parameters or configuration have not been applied.",
      noSubscription:
        "No usable subscription configuration is selected. Select one on Subscriptions first.",
      failure: "Runtime error ({{phase}} / {{code}}): {{message}}",
      memory: {
        client: "Client process",
        webView: "WebView2",
        webKitGTK: "WebKitGTK",
        webKit: "WebKit",
        mihomo: "mihomo",
        total: "Total memory",
      },
      uptime: {
        client: "Client",
        mihomo: "Mihomo",
        units: {
          second: "s",
          minute: "min",
          hour: "h",
          day: "d",
        },
      },
    },
    version: "Jeemi version",
    platform: "Platform",
    coreVersion: "mihomo version",
    notAvailable: "Not available yet",
    about: {
      title: "About Jeemi",
      repository: "Source repository",
      repositoryError:
        "Could not open the source repository. Please try again.",
      description:
        "A configuration management client powered by the Mihomo core",
    },
    runtimeActionError:
      "The proxy operation failed. Confirm that a verified mihomo version and subscription are selected, then review the current error state. A failure never replaces the last usable generation.",
    runtimeConfiguration: {
      title: "Final runtime configuration",
      open: "View runtime configuration",
      loading: "Reading the final runtime configuration…",
      error:
        "The final runtime configuration could not be read. Select a subscription and wait for generation to finish.",
      sources: {
        active: "Active runtime generation",
        resolved: "Ready-to-start snapshot",
      },
      core: "mihomo {{version}}",
    },
    traffic: {
      title: "Network speed",
      chartLabel: "Live upload and download speed over the last minute",
      requiresCore: "Start mihomo to display real network speed",
      upload: "Upload",
      download: "Download",
      total: "Total down {{down}} · up {{up}}",
      states: {
        offline: "Core is not running",
        connecting: "Connecting to traffic stream",
        live: "Live traffic",
        reconnecting: "Reconnecting; displayed data is stale",
        error: "Traffic stream disconnected; displayed data is stale",
      },
    },
    runtimePreferences: {
      title: "Proxy runtime parameters",
      loading: "Reading proxy runtime parameters…",
      sections: {
        baseline: "Basics and listeners",
        dns: "DNS basics",
        dnsResources: "DNS resolvers and mappings",
      },
      outboundMode: "Outbound mode",
      outboundModes: {
        rule: "Rule",
        global: "Global",
        direct: "Direct",
      },
      proxyMode: "Proxy method",
      proxyModes: {
        system_proxy: "System proxy",
        tun: "Virtual adapter",
      },
      listener: "External proxy listener",
      listenerTypes: {
        http: "HTTP",
        socks: "SOCKS",
        mixed: "Mixed",
      },
      listenPort: "External proxy listen port",
      allowLan: "Allow LAN connections",
      logLevel: "mihomo log level",
      logLevels: {
        silent: "Silent",
        error: "Error",
        warning: "Warning",
        info: "Info",
        debug: "Debug",
      },
      ipv6: "Enable IPv6",
      dnsEnabled: "Enable DNS",
      dnsListen: "DNS listen address",
      dnsIpv6: "Return IPv6 from DNS",
      dnsEnhancedMode: "DNS enhanced mode",
      dnsEnhancedModes: {
        "fake-ip": "Fake IP",
        "redir-host": "Redir Host",
      },
      dnsFakeIpRange: "Fake IP IPv4 range",
      dnsFakeIpRange6: "Fake IP IPv6 range",
      tunAutoRoute: "TUN automatic routes",
      tunAutoDetectInterface: "TUN interface detection",
      tunRouteExcludeAddress: "TUN excluded subnets",
      tunRouteExcludeAddressPlaceholder: "- 192.168.0.0/16\n- fc00::/7",
      dnsNameservers: "nameserver",
      dnsFakeIpFilter: "fake-ip-filter",
      dnsProxyServerNameservers: "proxy-server-nameserver",
      dnsNameserverPolicy: "nameserver-policy",
      dnsProxyServerNameserverPolicy: "proxy-server-nameserver-policy",
      hosts: "hosts",
      direct: "DIRECT",
      resourceEnabledAria: "Whether to inject {{resource}}",
      quickAdd: "Quick add",
      mergeMode: "Composition mode",
      mergeModes: {
        append: "Append unique",
        override: "Override",
      },
      resolverPlaceholder: "- 223.5.5.5\n- https://dns.alidns.com/dns-query",
      filterPlaceholder: "- +.lan\n- +.local",
      policyPlaceholder: "'geosite:cn':\n  - https://dns.alidns.com/dns-query",
      hostsPlaceholder:
        "'example.test': 127.0.0.1\n'*.example.test':\n  - 127.0.0.1",
      lanBypass: "LAN direct-route protection",
      lanBypassValue: "{{count}} DIRECT rules lead the rules list",
      lanEditor: {
        edit: "Edit",
        title: "Edit LAN direct-route rules",
        restoreDefaults: "Restore defaults",
        restoreError: "Could not read the system default rules.",
      },
      yamlError: {
        title: "Invalid {{resource}} YAML",
        location: "Line {{line}}, column {{column}}",
        unknown: "The configuration format is invalid",
        unavailable: "The YAML validation service is unavailable.",
        revert: "Revert this edit",
        continue: "Continue editing",
      },
      tunStack: "TUN stack",
      tunStacks: {
        system: "system",
        gvisor: "gVisor",
        mixed: "mixed",
      },
      errors: {
        load: "Could not read proxy runtime parameters. Check jeemi_data/settings.json permissions and contents.",
        save: "Could not save proxy runtime parameters. Existing settings were preserved. Check ports, CIDRs, lists, and YAML mapping syntax.",
      },
    },
  },
  pages: {
    logs: {
      title: "Live mihomo logs",
      summary: "Showing {{visible}} of {{total}} · retaining up to {{limit}}",
      search: "Search log content",
      coreLevel: "mihomo log level",
      displayLevel: "Displayed log level",
      levels: {
        all: "All levels",
        error: "Error",
        warning: "Warning",
        info: "Info",
        debug: "Debug",
      },
      states: {
        offline: "Offline",
        paused: "Paused",
        connecting: "Connecting",
        live: "Live",
        reconnecting: "Reconnecting",
        error: "Connection error",
      },
      columns: {
        time: "Time",
        level: "Level",
        content: "Log content",
      },
      actions: {
        pause: "Pause receiving",
        resume: "Resume receiving",
        copy: "Copy visible results",
        copied: "Copied",
        clear: "Clear logs",
        follow: "Jump to bottom",
      },
      requiresCore:
        "Start mihomo with a healthy control API to view live logs.",
      connecting: "Connecting to the mihomo log stream…",
      empty: "Connected and waiting for new mihomo logs…",
      pausedEmpty: "Log receiving is paused",
      noMatch: "No logs match the current filters",
      errors: {
        level:
          "Could not change the mihomo log level. Existing settings were preserved.",
        copy: "Could not copy logs. Check clipboard permissions.",
      },
    },
    tools: {
      cards: {
        ip: "IP lookup",
        unlock: "Unlock tests",
        dns: "DNS lookup",
      },
    },
  },
  connections: {
    title: "Live connections",
    summary:
      "{{count}} connections · {{upload}} uploaded · {{download}} downloaded",
    summaryFiltered:
      "Showing {{count}} of {{total}} connections · {{upload}} uploaded · {{download}} downloaded",
    search: "Search targets, processes, rules, or proxy chains",
    loading: "Connecting to the mihomo connection stream…",
    requiresCore:
      "Start mihomo with a healthy control API to inspect and close live connections.",
    stale:
      "The connection stream stopped. This is a stale snapshot from {{time}}; closing is disabled until the live stream returns.",
    empty: "There are no active connections",
    emptySearch: "No active connection matches",
    process: "Process",
    rule: "Matched rule",
    outbound: "Actual outbound",
    openDetails: "View details for connection {{target}}",
    viewProcessPath:
      "View the process path and connection details for {{process}}",
    unknown: "Unknown",
    details: {
      title: "Connection details",
      target: "Connection target",
      processPath: "Full process path",
      pathUnavailable: "mihomo did not return a process path",
      source: "Source address",
      destination: "Destination IP and port",
      network: "Network protocol",
      inbound: "Inbound type",
      ruleType: "Rule type",
      rulePayload: "Rule payload",
      chain: "Full proxy chain",
      upload: "Total uploaded",
      download: "Total downloaded",
      startedAt: "Started at",
      id: "Connection ID",
      states: {
        active: "Active connection",
        ended: "Connection ended · Last snapshot retained",
        stale: "Live data disconnected · Last snapshot retained",
      },
    },
    closeOne: "Close this connection",
    closeOneNamed: "Close connection {{target}}",
    filters: {
      route: "Filter connections by route",
      all: "All",
      direct: "Direct",
      proxy: "Proxy",
      selectors: "Filter proxy selectors",
      selectorsPlaceholder: "Choose proxy selectors",
    },
    closeAll: {
      button: "Close current results",
      title: "Close the current filtered results?",
      description:
        "This closes the {{count}} currently filtered connections one by one. Hidden connections are unaffected, and applications may reconnect automatically.",
      confirm: "Close current results",
    },
    states: {
      offline: "Core stopped",
      connecting: "Connecting",
      live: "Live",
      reconnecting: "Reconnecting",
      error: "Stream lost",
    },
    errors: {
      close:
        "The connection could not be closed. Confirm that the mihomo control API is still healthy and try again.",
    },
  },
  subscription: {
    fallback: {
      title: "Fallback traffic",
      preserve: "No override ( {{target}})",
      direct: "Direct",
      undefined: "Undefined",
      unavailable: "Unavailable",
      reset:
        "Selector “{{selector}}” is no longer available. Fallback traffic now uses no override.",
      saveFailed:
        "Could not save fallback traffic. Refresh the configuration and try again.",
      connectionResetFailed:
        "Fallback traffic is applied, but some existing connections could not be closed and may still use the previous route.",
    },
    loading: "Reading subscription configurations…",
    import: {
      title: "Import subscription",
      description:
        "Preserves complete source text and builds an offline selector preview after local-config composition.",
      add: "Add subscription",
      url: "Fetch from URL",
      file: "Choose config file",
      qr: "Import QR code",
      qrScreen: "Recognise QR on screens",
      qrImage: "Choose QR image",
      urlTitle: "Fetch subscription from URL",
      confirmURL: "Fetch and save",
      fileDialogTitle: "Choose a subscription configuration",
      fileFilter: "YAML, JSON, or TXT configuration",
      qrImageDialogTitle: "Choose a QR code image",
      qrImageFilter: "PNG, JPEG, or GIF image",
    },
    list: {
      emptyDescription:
        "No subscription configuration exists yet. Import one from a URL, configuration file, or QR code.",
      noDescription: "No description",
    },
    card: {
      size: "Source",
      revisions: "Revisions",
      updated: "Updated",
      trafficUnavailable: "Traffic metadata unavailable",
      expiryUnavailable: "Expiry unavailable",
      ruleProviders: "{{count}} rule providers",
      associated: "Associated with {{name}} [{{type}}]",
      handlerConfig: "Config",
      handlerScript: "Script",
    },
    shelf: {
      title: "Subscription configurations",
      collapse: "Collapse subscription shelf",
      collapseHint: "Click to collapse subscription panel",
      expand: "Expand subscription shelf",
      refreshCurrent: "Fetch the current subscription again",
      refreshUnavailable:
        "A file subscription has no remote address to fetch again",
      openRuleProviders: "View rule providers",
      subscriptionTools: "Current-subscription shortcuts",
      selectorTools: "Proxy-selector shortcuts",
      speedTest: "Test all nodes",
      speedTestBusy:
        "A delay-test queue is already running; wait for it to finish",
      speedTestRequiresCore:
        "The current subscription must be loaded by a running mihomo before testing",
      speedTestEmpty: "The current selectors contain no testable node",
      showHiddenSelectors: "Show selectors marked hidden",
      hideHiddenSelectors: "Hide selectors marked hidden",
    },
    selector: {
      title: "Proxy selectors",
      tools: "Proxy-selector shortcut toolbar",
      search: "Search selectors or nodes",
      outboundMode: "Switch outbound mode",
      outboundModes: {
        rule: "Rule mode",
        global: "Global mode",
        direct: "Direct mode",
      },
      density: {
        label: "Node card density",
        large: "L",
        medium: "M",
        small: "S",
      },
      globalProxy: "Global proxy",
      globalEmpty:
        "The current configuration has no real node available for Global mode.",
      directMode:
        "Direct mode does not use proxy selectors; every new connection goes direct.",
      selectSubscription:
        "Select a subscription to inspect its proxy selectors",
      empty: "The final configuration contains no proxy selector",
      hiddenOnly:
        "No selector is currently visible. Use the subscription bar to show hidden selectors.",
      noSearchResults: "No selector or node matches",
      unknownType: "Unknown type",
      nodeCount: "{{count}}",
      defaultSelection: "Config default",
      currentSelection: "Current selection",
      providersPending:
        "Nodes from {{names}} are supplied by external proxy providers and become complete only after mihomo loads them.",
      dynamicFiltersPending:
        "This configuration uses dynamic filter, exclude-filter, or exclude-type rules. The offline preview does not reimplement mihomo filtering; the running core remains authoritative.",
      sort: {
        label: "Node order",
        default: "Default",
        delay: "Delay",
        name: "Name",
      },
      view: {
        label: "Selector style",
        panel: "Panels",
        tabs: "Tabs",
      },
    },
    delay: {
      test: "Test this node's delay",
      retest: "Test this node again",
      retestCached:
        "This is a cached Jeemi delay result; click to test it again",
      retestAutomatic:
        "This is mihomo's latest recorded delay or health-check result; click to test it again",
      testNode: "Test delay for node {{name}}",
      retestNode: "Test delay for node {{name}} again",
      requiresCore: "The current subscription must be running in mihomo",
      queueBusy: "A delay-test queue is already running",
      failed: "Failed",
    },
    projection: {
      failed: "Could not build the final-configuration preview",
      status: {
        source_unavailable:
          "The saved subscription source is unavailable. Check the managed revision files.",
        local_config_unavailable:
          "The associated local configuration is missing or unreadable. Associate it again.",
        local_script_unavailable:
          "The associated local script is missing or unreadable. Associate it again.",
        script_execution_failed:
          "The local script failed. The subscription source and active runtime configuration were not modified.",
        composition_failed:
          "The subscription could not be composed after local processing or runtime-parameter injection. Check the affected configuration.",
        inspection_failed:
          "The composed result cannot be inspected as a mihomo selector structure.",
      },
    },
    ruleProvider: {
      title: "Rule providers",
      count: "{{enabled}} enabled / {{total}} total",
      empty: "The final configuration contains no rule provider",
      refresh: "Update rule provider {{name}}",
      requiresCore:
        "A running mihomo core with a healthy control API is required",
      enable: "Enable rule provider {{name}}",
      disable: "Disable rule provider {{name}}",
      disabledDescription:
        "Disabled and omitted from the final runtime configuration",
      disabledRefresh:
        "Enable this provider and load it successfully in mihomo before updating it",
      updated: "Updated successfully {{time}}",
      updateUnknown: "mihomo has not reported a valid update time",
      updateRequiresCore:
        "Start this subscription to read the core's update time",
      loadingStatus: "Reading mihomo provider status…",
      ruleCount: "{{count}} rules",
    },
    menu: {
      label: "Subscription actions for {{name}}",
      refresh: "Fetch again",
      edit: "Edit subscription",
      rawText: "View source config",
      associate: "Associate local config",
      runtimeConfiguration: "View runtime config",
      associateConfig: "Associate local config",
      associateScript: "Associate local script",
      delete: "Delete",
    },
    form: {
      name: "Subscription name",
      namePlaceholder: "For example: Daily use",
      nameOptional: "Subscription name (optional)",
      nameAutoPlaceholder: "Leave blank to use the response header or host",
      url: "Subscription URL",
      file: "Original imported file",
      description: "Description",
      descriptionPlaceholder: "Describe the source or intended use.",
      icon: "Icon",
      iconEdit: "Configure subscription icon",
      iconMode: "Subscription icon type",
      iconEmoji: "Emoji",
      iconUrl: "Icon URL",
      iconEmojiPlaceholder: "Choose an Emoji",
      iconDetect: "Auto-detect",
      iconClear: "Clear",
      iconDetection: {
        found: "A site icon was found and selected.",
        not_found:
          "No usable favicon.ico, favicon.png, sub.ico, or sub.png was found at the site root.",
        error:
          "Icon detection failed. Check the subscription address or network and try again.",
      },
    },
    edit: {
      title: "Edit subscription",
      save: "Save",
    },
    raw: {
      title: "Source config for {{name}}",
    },
    association: {
      title: "Associate local configuration",
      none: "No local configuration",
      missing: "Associated local configuration is unavailable",
      empty:
        "No local configuration exists yet. Leave this unassociated and create one from Config when needed.",
      save: "Save association",
      conflictTitle: "Only one local handler can be associated",
      unlinkScriptFirst:
        "This subscription already uses a local script. Unlink it from “Associate local script” first.",
    },
    scriptAssociation: {
      title: "Associate local script",
      none: "No local script",
      empty:
        "No local script exists yet. Create and test one from Config first.",
      save: "Validate and save",
      conflictTitle: "Only one local handler can be associated",
      unlinkConfigFirst:
        "This subscription already uses a local configuration. Unlink it from “Associate local config” first.",
      validationFailed:
        "The final runtime configuration produced by this script association is invalid: {{error}}",
    },
    delete: {
      title: "Delete subscription?",
      description:
        "This deletes every saved source revision and association metadata for “{{name}}”. It does not delete the associated local configuration or script.",
    },
    errors: {
      backendUnavailable:
        "Subscription management requires the Jeemi Wails desktop runtime. Standalone browser development does not create simulated subscriptions.",
      load: "Could not read subscriptions or local processing resources. Check permissions and integrity under jeemi_data.",
      operation:
        "The subscription operation failed. The current saved revision was not replaced.",
      required: "Enter a subscription name and a valid http/https URL.",
      urlRequired:
        "Enter a valid http/https subscription URL. The name may be left blank for automatic completion.",
      urlImport:
        "URL fetching failed. Check the address, network, file size, and configuration format. No incomplete content was saved.",
      fileImport:
        "File import failed. Only .yaml, .json, and .txt are supported, and the content must be a complete mihomo configuration mapping.",
      qrImport:
        "QR image import failed. Make sure the image is clear and the QR content is an http/https subscription URL.",
      screenImport:
        "QR subscription import failed. Check the subscription address in the code and your network connection.",
      screenTimeout:
        "The system screenshot request timed out. Try again and complete the desktop confirmation, or import a QR image.",
      screenUnavailable:
        "The desktop screenshot service is unavailable. Linux Wayland requires a working desktop screenshot service. You can take a system screenshot and import the QR image instead.",
      screenDenied:
        "The system did not allow this screenshot. Allow access in the desktop confirmation; on macOS, check Screen Recording permission. You can also import a QR image.",
      screenCapture:
        "The system screenshot did not complete. Try again, or take a system screenshot and import the QR image.",
      screenImage:
        "The system screenshot could not be read or cleaned up. Try again or import a QR image.",
      screenTooLarge:
        "The screenshot exceeds the size or pixel limit. Capture just the QR area with the system screenshot tool and import that image.",
      screenBusy:
        "The previous screen QR operation is still in progress. Please try again later.",
      screenNoQR:
        "No QR code containing a valid http/https subscription address was found. Show the whole code and enlarge it before trying again.",
      refresh:
        "Fetching again failed. The current URL and usable revision remain unchanged.",
      detail:
        "Could not read subscription details. Confirm that the subscription still exists.",
      edit: "Saving failed. The subscription name, source URL, and current revision remain unchanged.",
      rawText:
        "Could not read the current subscription source. Check the managed revision file integrity.",
      association:
        "Could not save the local configuration association. Confirm that both objects still exist.",
      ruleProviderRefresh:
        "Could not update the rule provider. Confirm that the active generation uses this subscription and that the mihomo control API is healthy.",
      ruleProviderToggle:
        "Could not save the rule-provider switch. The subscription source and active configuration were not directly rewritten.",
      proxySelection:
        "Could not switch the proxy node. The current selection was not changed; check the mihomo control API and node name.",
      connectionReset:
        "The node changed, but its related connections could not be reset. New connections use the new node while existing ones may remain on the old path.",
      outboundMode:
        "Could not switch outbound mode. Saved runtime parameters and the current mihomo mode were left unchanged.",
      selectorDisplayPreferences:
        "Could not save the node-card density, node order, or selector style. The previous setting was restored; check permissions for settings.json.",
      delayCacheLoad:
        "Could not read the node-delay cache. The configuration remains usable, but previous results are not shown.",
      delayCacheSave:
        "The delay result is visible but could not be cached under jeemi_data, so it may be lost after restart.",
      select:
        "Could not save the selected subscription. Check permissions for settings.json and the subscription directory.",
      delete:
        "Could not delete the subscription. Check data-directory permissions and try again.",
    },
  },
  localPackage: {
    export: "Export",
    import: "Import and replace",
    exportTitle: "Export Jeemi package",
    importTitle: "Import Jeemi package",
    filter: "Jeemi package (*.json)",
    confirmTitle: "Confirm import and replacement",
    overwrite: "Replace",
    replace:
      "Replace all of “{{target}}” with “{{name}}” from the file, keeping its subscription associations.",
    overwrittenGroups: "Same-name strategy groups to replace ({{count}})",
    overwrittenRuleSets: "Same-name rule sets to replace ({{count}})",
    addedGroups: "New strategy groups ({{count}})",
    addedRuleSets: "New rule sets ({{count}})",
    affectedConfigs:
      "Local configurations affected by shared resources ({{count}})",
    affectedSubscriptions: "Associated subscriptions checked ({{count}})",
    noConflicts: "No same-name strategy groups or rule sets need replacement.",
    exported: ".json file exported",
    imported: "Import and replacement completed",
    failed: "Import or export did not complete",
    unchanged:
      "Imports that fail validation do not replace existing content. Resolve the issue below and select the file again.",
  },
  localScript: {
    tabs: {
      scripts: "Local scripts {{count}}",
    },
    list: {
      create: "New local script",
      edit: "Edit",
      delete: "Delete",
      menu: "Script actions for {{name}}",
      deleteTitle: "Delete local script?",
      deleteDescription:
        "This deletes “{{name}}”. Deletion is blocked while a subscription still uses it.",
      noDescription: "No description",
      lines: "lines",
      size: "size",
      updatedAt: "Updated",
    },
    editor: {
      loading: "Reading the local script and current subscription…",
      name: "Script name",
      namePlaceholder: "For example: Remove unavailable nodes",
      description: "Description",
      descriptionPlaceholder:
        "Describe what the script changes and where it applies.",
      source: "JavaScript source",
      testTarget: "Test subscription: {{name}}",
      noTestSubscription:
        "No subscription is available for a test run. The script can still be saved, but its final configuration must pass validation before association.",
      actionBar: "Local script editor actions",
      test: "Run test",
      save: "Save script",
      unsavedPrompt:
        "This local script has unsaved changes. Leave and discard them?",
    },
    test: {
      title: "Script final-runtime preview",
      staticValidated: "Structure valid",
      coreValidated: "mihomo validation passed",
      corePending: "mihomo validation pending",
    },
    errors: {
      nameRequired: "Enter a script name.",
    },
  },
  localConfig: {
    categories: {
      general: "General",
      controller: "Runtime control",
      inbound: "Proxy ports and listeners",
      dns: "DNS",
      sniffer: "Domain sniffing",
      tun: "TUN",
      outbound: "Proxies and policy groups",
      routing: "Rules and rule providers",
      hosts: "Hosts",
      advanced: "Tunnels and advanced features",
    },
    list: {
      loading: "Reading local configurations…",
      create: "New local configuration",
      edit: "Edit",
      delete: "Delete",
      menu: "Actions for {{name}}",
      deleteTitle: "Delete this local configuration?",
      deleteDescription:
        "This deletes the managed files for “{{name}}”. Subscription configuration is not changed.",
      noDescription: "No description",
      enabledFields: "Output fields",
      ruleProviders: "Rule providers",
      strategyGroups: "Local policy groups",
      updatedAt: "Updated",
    },
    resources: {
      tabs: {
        configs: "Local configurations {{count}}",
        groups: "Local policy groups {{count}}",
        ruleSets: "Local rule sets {{count}}",
      },
      actions: "Resource actions for {{name}}",
      name: "Name",
      description: "Description",
      groups: {
        create: "New local policy group",
        edit: "Edit local policy group",
        deleteTitle: "Delete this local policy group?",
        deleteDescription:
          "This deletes “{{name}}”. Deletion is blocked while a local configuration still references it.",
        kind: "Policy group kind",
        kinds: {
          rule: "Rule group",
          selector: "Selector",
        },
        inline: "Inline mode",
        ruleSets: "Referenced local rule sets",
        ruleSetOwned: "{{name}} · referenced by {{owner}}",
        addRuleSet: "Add rule set",
        removeRuleSet: "Remove rule set",
        noRuleSets: "No local rule sets are referenced",
        ruleSetCount: "{{count}} rule sets",
        policy: "Rule target",
        policyProxy: "Proxy",
        policyDirect: "Direct",
        policyReject: "Block",
        selectorPolicyFixed: "This selector (proxy)",
        type: "Group type",
        emoji: "Emoji icon",
        emojiPlaceholder: "—",
        icon: "Remote icon URL",
        testUrl: "Test URL",
        interval: "Interval (seconds)",
        tolerance: "Tolerance (milliseconds)",
        strategy: "Load-balancing strategy",
        lazy: "Lazy health test",
        proxyTypes: "Proxy protocol filter",
        namePatterns: "Proxy name patterns",
        nameRules: "{{count}} name conditions",
        help: {
          inline: {
            title: "Inline rule output",
            description:
              "Choose whether this policy group references rule sets through RULE-SET or expands local payload items directly into top-level rules.",
            purpose:
              "Explicitly choose between independently updated rule providers and fixed local rules in the final rule order.",
            scenarios:
              "Leave it off by default; enable it only when rules edited in Jeemi should be written directly into the final rules list.",
            cautions:
              "It cannot be enabled while an HTTP rule set is referenced. Changing it alters the final runtime configuration and still requires composition and core validation.",
          },
          policy: {
            title: "Rule target direction",
            description:
              "A rule group declares proxy, direct, or block routing without binding a concrete selector inside the shared resource.",
            purpose:
              "Reuse one rule group across local configurations while each configuration chooses its own proxy outbound.",
            scenarios:
              "Proxy rules use the local configuration's proxy rule target; direct and block emit DIRECT and REJECT.",
            cautions:
              "When Proxy is selected, every local configuration referencing this group must configure a proxy rule target in the Rules panel.",
          },
          ruleSets: {
            title: "Policy-group rule sets",
            description:
              "Reference local rule sets in order; this order becomes the rule output order inside the policy group.",
            purpose:
              "Keep rule content and ordering together while the owning policy group defines one consistent target.",
            scenarios:
              "Add, remove, or drag rule sets to change their matching priority within this policy group.",
            cautions:
              "Each rule set can belong to only one policy group. Inline accepts local payload only; HTTP rule sets must use RULE-SET.",
          },
          lazy: {
            title: "Lazy health test",
            description:
              "Make a non-select selector run its health check when the selector is actually used instead of checking continuously.",
            purpose:
              "Reduce periodic probe requests from selectors that are not currently in use.",
            scenarios:
              "Enable it when many selectors exist and unused groups should not keep testing nodes.",
            cautions:
              "This is mihomo selector configuration and does not guarantee current node availability; behavior also depends on the test URL and interval.",
          },
          namePatterns: {
            title: "Proxy name conditions",
            description:
              "Enter one name fragment per line. Plain terms include matches; terms beginning with ! exclude matches.",
            purpose:
              "Start with all direct subscription proxies and filter by protocol and name. Use the flag tool to insert an Emoji into a name condition.",
            scenarios:
              "For example, keep names containing “AI”, then exclude test nodes with “!test”.",
            cautions:
              "Positive terms use OR and every exclusion is applied last. Protocol and name conditions use AND; leaving all filters empty includes all direct subscription proxies, deduplicated by name.",
          },
        },
      },
      ruleSets: {
        create: "New local rule set",
        edit: "Edit local rule set",
        deleteTitle: "Delete this local rule set?",
        deleteDescription:
          "This deletes “{{name}}”. Deletion is blocked while a local policy group still references it.",
        sourceType: "Source type",
        behavior: "Rule behavior",
        format: "Remote format",
        url: "Remote URL",
        interval: "Update interval (seconds)",
        noResolve: "Skip DNS resolution for target IP rules",
        payload: "Mihomo rule-set YAML",
        payloadPlaceholders: {
          domain: "payload:\n  - +.example.com\n  - example.org",
          ipcidr: "payload:\n  - 192.0.2.0/24\n  - 2001:db8::/32",
          classical:
            "payload:\n  - DOMAIN-SUFFIX,example.com\n  - IP-CIDR,203.0.113.0/24,no-resolve",
        },
        help: {
          noResolve: {
            title: "no-resolve for ipcidr",
            description:
              "Controls whether generated ipcidr rules append no-resolve and skip extra DNS resolution for an IP target.",
            purpose:
              "Avoid sending targets that are already IP addresses or CIDRs through an unnecessary domain-resolution path.",
            scenarios:
              "Enable it only for ipcidr behavior when the rule does not need target-domain resolution.",
            cautions:
              "Domain behavior does not support this parameter. Classical content declares it on each applicable IP-CIDR or other target-IP payload item.",
          },
          payload: {
            title: "Local rule-set YAML",
            description:
              "Edit one mihomo YAML document containing only a top-level payload whose value is a plain-text rule list.",
            purpose:
              "Preserve rule order and editing comments while Jeemi validates structure and basic semantics when saving.",
            scenarios:
              "Use it to maintain local domain, ipcidr, or classical rule content inside Jeemi.",
            cautions:
              "Anchors, aliases, extra top-level fields, and bare lists are rejected. In classical content, put no-resolve on each rule item that needs it.",
          },
        },
        sourceTypes: {
          inline: "Local YAML",
          http: "Remote HTTP",
        },
        behaviors: {
          domain: "Domains (domain)",
          ipcidr: "IP ranges (ipcidr)",
          classical: "Standard rules (classical)",
        },
      },
    },
    editorResources: {
      virtualField: "Jeemi structured composition field",
      groupsTitle: "Ordered local policy group references",
      groupsEmpty:
        "Create a rule group or selector under Local policy groups first",
      addGroup: "Add local policy group",
      removeGroup: "Remove policy group reference",
      noGroups: "No local policy groups are referenced",
      ruleStrategy: "Rule override mode",
      rulesPrepend: "Prepend rules",
      rulesBeforeTerminal: "Insert before terminal rule",
      rulesRebuild: "Replace and reconstruct",
      localMatch: "MATCH override",
      matchPreserve: "Keep upstream MATCH",
      matchPlaceholder: "Choose a MATCH target",
      rebuildSelectorRequired:
        "Complete reconstruction requires at least one included, enabled local selector.",
      matchRequired:
        "Complete reconstruction needs a MATCH target: Direct or an included, enabled local selector.",
      matchUnavailable:
        "The MATCH selector is not included or is disabled. Choose another target.",
      rulesEnabled: "Enable local rules",
      groupEnabled: "Enable policy group: {{name}}",
      proxySelector: "Proxy rule target",
      proxySelectorRequired:
        "Enabled proxy rules need an available target. Enable the selected selector or choose another target.",
      matchLocked:
        "Set MATCH override in the Rules panel above. Subscription Fallback traffic can override it again after local processing.",
      proxySelectorPlaceholder: "Select the shared target for proxy rules",
      help: {
        localMatch: {
          title: "Local MATCH override",
          description:
            "Choose where unmatched traffic goes in the local result: keep the subscription target, use Direct, or select an included and enabled local selector.",
          purpose:
            "Define a fallback independently of targets assigned to matching rules.",
          scenarios:
            "Adjust fallback while merging a subscription, or choose a new fallback after reconstructing its routing.",
          cautions:
            "Complete reconstruction requires an explicit MATCH target and at least one enabled local selector. Turning off local rules skips local MATCH. Subscription Fallback traffic runs afterwards; Keep means retaining the locally processed result.",
        },
        proxySelector: {
          title: "Proxy rule target",
          description:
            "Provides one concrete selector for every proxy-directed rule group in this local configuration.",
          purpose:
            "Separate a reusable rule group's direction from the proxy outbound chosen by each local configuration.",
          scenarios:
            "Choose it for proxy-directed rule groups. Selector groups already target themselves.",
          cautions:
            "This target handles matching proxy rules independently of local MATCH. Change the target or disable dependent rule groups before disabling their selector. Subscription Fallback traffic can override MATCH afterwards.",
        },
        activation: {
          title: "Local rule switches",
          description:
            "The main switch controls local selectors, rules and providers. Each reference switch controls that group only in this local configuration.",
          purpose:
            "Temporarily skip all or selected groups while keeping references, order and composition modes.",
          scenarios:
            "Compare original subscription routing with local rules, or enable groups one at a time to troubleshoot routing.",
          cautions:
            "Custom fields still apply when the main switch is off. Re-enabling and saving validates the final configuration. Packages preserve these switches; other configurations keep their own choices.",
        },
      },
    },
    editor: {
      loading: "Loading the configuration catalog and local configuration…",
      name: "Configuration name",
      namePlaceholder: "For example: Office network rules",
      description: "Description",
      descriptionPlaceholder:
        "Describe what this local configuration is for and where it applies.",
      search: "Search field paths or groups",
      onlyEnabled: "Output fields only",
      showLocked: "Show locked",
      allProxyNodes: "All proxy nodes",
      noSearchResults: "No configuration fields match",
      selectField: "Select a field from the configuration tree",
      locked: "Managed by Jeemi",
      lockedTitle: "Runtime-owned field",
      lockedDescription:
        "Jeemi safely injects this field while generating the final runtime configuration. A local configuration cannot control it.",
      officialDocs: "Official docs",
      output: "Output to final runtime configuration",
      proxyFieldOverride: "Bulk proxy override",
      mergeStrategy: "Composition strategy",
      conflictPolicy: "Same-name conflict handling",
      localValue: "Local value",
      revealSensitive: "Show sensitive value",
      hideSensitive: "Hide sensitive value",
      sensitiveHidden:
        "This field may contain credentials and is hidden by default.",
      enableToEdit: "Enable output to edit the local value.",
      actionBar: "Local configuration editor actions",
      preview: "Preview local YAML",
      save: "Save local configuration",
      unsavedPrompt:
        "This local configuration has unsaved changes. Leave and discard them?",
    },
    preview: {
      title: "Local YAML preview",
      fieldCount: "{{count}} output fields",
      proxyOverrideCount: "{{count}} proxy field overrides",
      proxyOverrides: "Bulk proxy overrides (expanded during composition)",
    },
    fieldHelp: {
      ...enLocalConfigFieldHelp,
      generic: {
        title: "{{field}} field",
        description:
          "{{field}} is a mihomo configuration field. When enabled, this local configuration contributes it to final runtime composition.",
        purpose:
          "Supplement or override the field without modifying the immutable subscription source.",
        scenarios:
          "Use it when subscriptions omit the capability or several subscriptions should reuse one client-side setting.",
        cautions:
          "Meanings and accepted values follow the current official mihomo documentation. Jeemi validates YAML shape on save and the real final configuration before activation.",
      },
      "log-level": {
        title: "log-level",
        description:
          "Sets the minimum mihomo log level: silent, error, warning, info, or debug.",
        purpose:
          "Balance operational visibility, troubleshooting detail, and log noise.",
        scenarios:
          "Use info for normal operation and temporarily use debug while diagnosing core or rule behavior.",
        cautions:
          "Debug produces more data and may expose destination domains. Avoid leaving it enabled or sharing logs without review.",
      },
      ipv6: {
        title: "ipv6 master switch",
        description:
          "Controls whether mihomo resolves and handles IPv6 traffic.",
        purpose:
          "Allow IPv6-capable networks and proxy nodes to use IPv6 addresses.",
        scenarios:
          "Enable it when the local network, DNS path, and proxies all have reliable IPv6 connectivity.",
        cautions:
          "Enabling it without a working IPv6 route can add timeouts. It is distinct from dns.ipv6.",
      },
      "dns-ipv6": {
        title: "dns.ipv6",
        description:
          "Controls whether the mihomo DNS module returns AAAA (IPv6) answers.",
        purpose:
          "Decide whether applications receive IPv6 addresses from DNS queries handled by mihomo.",
        scenarios:
          "Enable it when local and proxy paths support IPv6 and applications should use it.",
        cautions:
          "Use care when the top-level ipv6 switch or real network lacks IPv6, or applications may receive unreachable addresses.",
      },
      "sniffer-enable": {
        title: "sniffer.enable",
        description:
          "Enables domain sniffing from HTTP, TLS, or QUIC handshake metadata.",
        purpose:
          "Recover a domain for IP-only connections so domain routing rules can still match.",
        scenarios:
          "Common with TUN, transparent proxying, or applications that bypass system DNS.",
        cautions:
          "Sniffing reads protocol handshake metadata; it does not decrypt payloads. Enable it according to privacy requirements.",
      },
      "sniffer-force-dns-mapping": {
        title: "sniffer.force-dns-mapping",
        description:
          "Forces sniffing attempts for connections that already have a DNS mapping.",
        purpose:
          "Improve domain identification consistency when an address came from DNS mapping.",
        scenarios:
          "Useful with Fake-IP or transparent proxying where stable domain-rule matching is required.",
        cautions:
          "It broadens sniffing. Use skip-domain or address exclusions for incompatible applications.",
      },
      "sniffer-parse-pure-ip": {
        title: "sniffer.parse-pure-ip",
        description:
          "Allows domain sniffing on pure-IP connections that have no DNS mapping.",
        purpose:
          "Recognize traffic that connects to a hard-coded IP but still carries a hostname in its handshake.",
        scenarios:
          "Use it for applications with built-in DNS or hard-coded endpoints.",
        cautions:
          "Not every protocol can be recognized, and sniff failure must not be treated as connection failure.",
      },
      "sniffer-override-destination": {
        title: "sniffer.override-destination",
        description:
          "Replaces the connection destination with the sniffed domain for later resolution and routing.",
        purpose:
          "Make routing and outbound resolution use the recovered domain.",
        scenarios:
          "Use it when domain rules and proxy-side DNS should consistently apply.",
        cautions:
          "Certificate pinning, unusual SNI, or intentional IP semantics may be incompatible.",
      },
      "sniffer-http-ports": {
        title: "sniffer.sniff.HTTP.ports",
        description:
          "Limits HTTP Host sniffing to selected ports or port ranges.",
        purpose: "Parse Host only on ports expected to carry HTTP.",
        scenarios:
          "Add ports such as 8080 or 8880 when HTTP is used outside port 80.",
        cautions:
          "Broad ranges add useless probing, while narrow ranges can miss non-standard services.",
      },
      "sniffer-http-override-destination": {
        title: "sniffer.sniff.HTTP.override-destination",
        description:
          "Controls destination replacement specifically for HTTP sniff results.",
        purpose: "Refine the global override behavior for HTTP traffic.",
        scenarios:
          "Use it when HTTP should use recovered domains but TLS or QUIC should keep original targets.",
        cautions:
          "Replacement may change where DNS is resolved; verify destination connectivity after changes.",
      },
      "sniffer-tls-ports": {
        title: "sniffer.sniff.TLS.ports",
        description:
          "Limits SNI sniffing from TLS ClientHello to selected ports.",
        purpose: "Recognize domains for HTTPS and other TLS connections.",
        scenarios:
          "Use it for 443, 8443, or custom TLS ports that need domain routing.",
        cautions:
          "Encrypted ClientHello and similar mechanisms can make the domain unavailable.",
      },
      "sniffer-tls-override-destination": {
        title: "sniffer.sniff.TLS.override-destination",
        description:
          "Controls destination replacement specifically for TLS sniff results.",
        purpose: "Refine target replacement for TLS traffic.",
        scenarios:
          "Enable it when SNI should participate in final connection resolution.",
        cautions:
          "Pinned certificates or applications relying on the original IP may need exclusions.",
      },
      "sniffer-quic-ports": {
        title: "sniffer.sniff.QUIC.ports",
        description:
          "Limits domain sniffing from initial QUIC handshakes to selected UDP ports.",
        purpose: "Recognize domains for HTTP/3 and other QUIC traffic.",
        scenarios:
          "Use it when 443/UDP or custom QUIC ports require domain routing.",
        cautions:
          "Protocol and encryption changes can prevent recognition; avoid unnecessarily broad ranges.",
      },
      "sniffer-quic-override-destination": {
        title: "sniffer.sniff.QUIC.override-destination",
        description:
          "Controls destination replacement specifically for QUIC sniff results.",
        purpose: "Decide whether QUIC connections use the recovered domain.",
        scenarios:
          "Enable it when HTTP/3 needs domain routing and replacement is compatible.",
        cautions:
          "Disable it first when diagnosing UDP or QUIC connectivity regressions.",
      },
      "sniffer-force-domain": {
        title: "sniffer.force-domain",
        description:
          "Lists domain patterns for which sniffing must be attempted.",
        purpose: "Force sniffing for targets that default behavior might skip.",
        scenarios:
          "Use it when a domain depends on sniffing to match the correct rule.",
        cautions:
          "Follow mihomo domain wildcard syntax and avoid overly broad entries.",
      },
      "sniffer-skip-domain": {
        title: "sniffer.skip-domain",
        description: "Lists domain patterns that should not be sniffed.",
        purpose:
          "Bypass incompatible targets or ones whose handshake metadata should not be inspected.",
        scenarios:
          "Useful for smart-home, LAN, or specialized protocols that fail under sniffing.",
        cautions: "Skipped traffic may only match IP-based rules.",
      },
      "sniffer-skip-src-address": {
        title: "sniffer.skip-src-address",
        description: "Skips domain sniffing for selected source IPs or CIDRs.",
        purpose:
          "Disable sniffing for specified LAN devices or traffic origins.",
        scenarios:
          "Use it for an incompatible device or a separate privacy policy.",
        cautions:
          "Address changes can invalidate entries, while broad CIDRs can skip much more traffic than intended.",
      },
      "sniffer-skip-dst-address": {
        title: "sniffer.skip-dst-address",
        description:
          "Skips domain sniffing for selected destination IPs or CIDRs.",
        purpose:
          "Avoid protocol probing for specified servers or address ranges.",
        scenarios:
          "Use it for LAN ranges, private services, or known incompatible targets.",
        cautions:
          "Skipped traffic must rely on existing DNS mappings or IP rules.",
      },
      "proxies-ip-version": {
        title: "Bulk proxies.ip-version override",
        description:
          "Sets the IP-version preference used to resolve every subscription proxy server.",
        purpose: "Normalize IPv4/IPv6 resolution behavior across proxy nodes.",
        scenarios:
          "Use it when subscriptions are inconsistent or the current network favors one IP version.",
        cautions:
          "This is Jeemi composition metadata, not wildcard YAML; it is expanded onto real proxy nodes.",
      },
      "proxies-udp": {
        title: "Bulk proxies.udp override",
        description: "Sets UDP forwarding consistently across proxy nodes.",
        purpose:
          "Normalize node behavior for games, QUIC, DNS, and other UDP traffic.",
        scenarios:
          "Enable it when a subscription omits UDP and the protocol and server are known to support it.",
        cautions:
          "The field cannot add UDP capability to an incompatible protocol or server.",
      },
      "proxies-interface-name": {
        title: "Bulk proxies.interface-name override",
        description:
          "Binds proxy-node dial traffic to a named system network interface.",
        purpose:
          "Control which physical or virtual interface establishes proxy connections.",
        scenarios:
          "Useful for multi-homed systems, policy routing, or avoiding TUN loops.",
        cautions:
          "The interface must exist on the current platform, so portable local configurations may need adjustment.",
      },
      "proxies-routing-mark": {
        title: "Bulk proxies.routing-mark override",
        description:
          "Sets a Linux routing mark on underlying proxy-node connections.",
        purpose:
          "Let Linux policy-routing tables distinguish connections created by mihomo.",
        scenarios:
          "Use it for advanced transparent routing and loop prevention on Linux.",
        cautions:
          "It is primarily Linux-specific and must match actual system routing rules.",
      },
      "proxies-tfo": {
        title: "Bulk proxies.tfo override",
        description:
          "Controls TCP Fast Open consistently for proxy-node dials.",
        purpose:
          "Reduce TCP setup latency where both the system and server support TFO.",
        scenarios:
          "Use it after verifying path support and when optimizing short connections.",
        cautions:
          "Some networks and operating systems are incompatible; disable it when connections regress.",
      },
      "proxies-mptcp": {
        title: "Bulk proxies.mptcp override",
        description:
          "Controls Multipath TCP consistently for proxy-node dials.",
        purpose:
          "Use multiple paths where the operating system, network, and server support MPTCP.",
        scenarios:
          "Enable it only in a deliberately configured MPTCP environment.",
        cautions:
          "Platform and network support is limited, and incompatibility can prevent connections.",
      },
      "proxies-dialer-proxy": {
        title: "Bulk proxies.dialer-proxy override",
        description:
          "Makes every proxy node dial through a named proxy or group, forming a chain.",
        purpose: "Apply a common entry or relay hop to subscription nodes.",
        scenarios:
          "Use it when every node must be reached through one fixed upstream.",
        cautions:
          "The target must exist and must not create a cycle; mistakes can disable all nodes.",
      },
      "proxies-client-fingerprint": {
        title: "Bulk proxies.client-fingerprint override",
        description:
          "Sets a TLS client fingerprint for compatible VMess, VLESS, Trojan, and AnyTLS nodes.",
        purpose:
          "Make TLS ClientHello resemble common browser implementations.",
        scenarios:
          "Use it when a server or network requires a particular uTLS fingerprint.",
        cautions:
          "Jeemi applies it only to protocols documented to support the field, not to unrelated nodes such as Shadowsocks.",
      },
      "proxy-groups": {
        title: "proxy-groups policy groups",
        description:
          "Defines selectors, URL tests, failover, and load-balancing groups.",
        purpose:
          "Organize proxy nodes and other groups into switchable outbound policies.",
        scenarios:
          "Use groups to filter nodes by name or protocol and offer manual or automatic selection.",
        cautions:
          "Prefer references to centrally managed groups. Duplicate names and dependency cycles block final configuration activation.",
      },
      "rule-providers": {
        title: "rule-providers rule sets",
        description:
          "Defines remote, file-backed, or inline rule collections referenced by RULE-SET rules.",
        purpose:
          "Reuse and independently update large domain, ipcidr, or classical rule sets.",
        scenarios:
          "Use them for ad blocking, regional routing, or application-specific routing lists.",
        cautions:
          "Behavior, format, and content must agree. mihomo handles remote downloads, cache paths, and refreshes.",
      },
      "local-strategy-groups": {
        title: "Local policy groups",
        description:
          "Local policy groups are either rule groups or selectors. Rule groups organize rule sets and proxy, direct, or block directions; selectors filter proxies and materialize a mihomo proxy-group.",
        purpose:
          "Reuse routing and proxy-selection logic across local configurations while stable IDs keep references intact after renaming.",
        scenarios:
          "Create a rule group to use the proxy rule target, route directly, or block. Create a selector to organize proxies by name or protocol, including flags inserted into name conditions.",
        cautions:
          "A rule group is not a mihomo outbound and does not bind a selector. The local configuration's rules.rules default selector resolves proxy routing; selector-owned rules target themselves.",
      },
      "local-rule-sets": {
        title: "Local rule sets",
        description:
          "Local rule sets hold reusable domain, ipcidr, or classical content from an editable payload or a real HTTP source.",
        purpose:
          "Keep rule content separate from policy-group targets, output mode, and order to avoid duplicating it across local configurations.",
        scenarios:
          "Use them for ad blocking, direct-domain lists, regional routing, or application-specific routing rules.",
        cautions:
          "A local policy group must reference the rule set and behavior must match its content. Configure no-resolve on an ipcidr rule set or the applicable target-IP entries in classical content, never on a policy-group reference. Mutable HTTP content cannot be expanded inline before it is fetched and pinned.",
      },
      rules: {
        title: "Rules and subscription association",
        description:
          "Enable local rules, add policy groups in order, and prepend, append, or replace rules. Each reference can be disabled separately. After saving, select this configuration from the subscription card’s Associate local configuration menu.",
        purpose:
          "Restructure routing and selectors through the UI. Custom YAML fields handle other injected values; local scripts are an alternative at the same processing stage.",
        scenarios:
          "Use reusable rule groups for proxy, direct, or reject policies and selector groups for filtered subscription nodes.",
        cautions:
          "Complete reconstruction replaces source rules, selectors, rule providers and sub-rules together. It requires an included, enabled local selector and an explicit MATCH. Subscription Fallback traffic may override the local MATCH afterwards. Turning off local rules applies only custom fields and preserves references and composition modes. Every affected subscription is checked before saving, including core validation when available; failure preserves saved data and the draft.",
      },
    },
    kinds: {
      scalar: "Scalar field",
      mapping: "Mapping field",
      sequence: "Ordered array",
      named_mapping: "Named-object mapping",
      named_sequence: "Named-object array",
      rules: "Ordered routing rules",
    },
    strategies: {
      replace: "Replace completely",
      merge: "Recursive merge, local value wins",
      prepend: "Prepend",
      append: "Append",
      merge_by_name: "Merge by name",
      append_before_terminal: "Append before the terminal rule",
    },
    conflicts: {
      error: "Block composition on a same-name item",
      use_local: "Explicitly replace the same-name item with the local item",
    },
    errors: {
      backendUnavailable:
        "Local configurations require the Jeemi Wails desktop runtime. Standalone browser development does not create simulated configuration data.",
      load: "Could not read the configuration catalog or local configuration. Check permissions and integrity under jeemi_data.",
      operation:
        "The local configuration operation failed. Confirm that it still exists, is not referenced by a subscription, and the data directory is writable.",
      nameRequired: "Enter a local configuration name.",
      preview:
        "Could not generate the local YAML preview. Check field values, YAML types, and composition strategies.",
      save: "Save failed. The existing local configuration was not replaced. Check field values, YAML types, and data directory permissions.",
    },
    redesign: {
      repairReferences: "Invalid resource references — edit to repair",
      rulesConfig: "Rule configuration",
      customFields: "Custom YAML fields",
      groupInfo: "Policy group information",
      ruleSetConfig: "Rule set configuration",
      nodeFilter: "Node filtering",
      insertFlag: "Insert a flag",
      searchFlag: "Search region code, e.g. US or JP",
      rulesLocked:
        "Manage this association in Rule configuration above. It is never written to mihomo YAML.",
      missingGroup: "Missing policy group ({{id}})",
      validationFailed: "Validation failed. Changes have not been saved.",
      keepEditing: "Continue editing",
      revertDraft: "Discard these changes",
      refreshConflict: "New subscription conflicts with local processing",
      refreshConflictDescription:
        "The fetched subscription passes validation on its own but fails after the associated local configuration or script runs. The old subscription and association are preserved. Detach and accept this update, or reject it and edit local processing.",
      rejectRefresh: "Keep old subscription",
      editHandler: "Edit local processing",
      detachRefresh: "Detach and update",
    },
  },
  settings: {
    openPage: "Open {{page}} settings",
    currentPage: "Currently editing page settings",
    pageTitle: "{{page}} settings",
    actionBarLabel: "{{page}} settings actions",
    confirm: "Confirm",
    cancel: "Cancel",
    emptyTitle: "{{page}} settings",
    emptyDescription: "No page-specific settings are defined yet.",
    managedStorage: {
      loading: "Reading the managed storage directory…",
      error:
        "Could not read the managed storage directory. Check permissions for jeemi_data.",
    },
    home: {
      appearanceTitle: "Appearance and language",
      theme: "Color theme",
      language: "Interface language",
      light: "Light",
      dark: "Dark",
      chinese: "简体中文",
      english: "English",
      chineseShort: "🇨🇳 CN",
      englishShort: "🇺🇸 EN",
      updatesTitle: "Application and core updates",
      appUpdate: "Jeemi update checks",
      coreVersion: "mihomo version",
      coreVersionDescription:
        "Check official core releases and select the version Jeemi should use.",
      core: {
        managerTitle: "mihomo version management",
        loading: "Reading local mihomo versions…",
        target: "Platform asset",
        lastChecked: "Last checked",
        latest: "Latest stable",
        notChecked: "Not checked yet",
        check: "Check for updates",
        openDirectory: "Open core directory",
        selectedVersion: "Version to use",
        selectPlaceholder: "Select an installed version",
        availableTitle: "Official available versions",
        availableEmpty:
          "Select “Check for updates” to load official stable releases.",
        hideAvailable: "Hide official available versions",
        latestTag: "Latest",
        installedTag: "Downloaded",
        installed: "Downloaded",
        download: "Download and verify",
        cancelDownload: "Cancel download",
        cancelling: "Cancelling",
        filesTitle: "Local core files",
        inUseTag: "In use",
        manualTag: "Manual import",
        noInstalled: "No mihomo core is installed",
        remove: "Remove",
        removeConfirmTitle: "Remove this local mihomo core?",
        removeConfirmDescription:
          "This removes {{version}} for the current platform without deleting other versions.",
        dataDirectory: "Jeemi data directory",
        coreDirectory: "mihomo core directory",
        manual: {
          open: "Manual install",
          title: "Manually install the mihomo core",
          warning:
            "Import only files downloaded from the official mihomo releases page that match this platform. Manual import validates file boundaries and executable format, but it cannot provide the publisher SHA-256 identity verification used by online installation.",
          stepRepository: "Open the official mihomo releases page:",
          openRepository: "Open releases",
          stepDownload:
            "Download the ZIP, GZ, or executable matching platform target {{target}}. Official stable Windows builds are normally ZIP files.",
          stepImport:
            "After downloading, select “Import mihomo core” below. Jeemi extracts it into the managed core directory and uses an unknown-* directory when no version can be recognised.",
          import: "Import mihomo core",
          dialogTitle: "Select a mihomo core file",
          fileFilter: "mihomo archive or executable",
          imported:
            "Imported {{version}}. To use it, close this guide, confirm the version selection, then select Confirm at the bottom of the page.",
        },
        errors: {
          load: "Could not read mihomo version state. Check permissions on the jeemi_data directory.",
          check:
            "Could not check official mihomo releases. Check the network and try again.",
          download:
            "mihomo download or verification failed. Existing usable cores were not replaced.",
          cancel:
            "Could not cancel the download. Wait for the current operation to finish and try again.",
          remove:
            "Could not remove the local mihomo version. Switch away from the active version first.",
          open: "Could not open the mihomo core directory. Check the system file manager.",
          openReleases: "Could not open the official mihomo releases page.",
          import:
            "Could not import the mihomo core. Select an official ZIP, GZ, or executable for this platform. Existing cores were not replaced.",
          select:
            "Could not save the mihomo version. Confirm that the selected version still exists and is verified.",
        },
      },
      geoData: {
        managerTitle: "GEO data management",
        loading: "Reading GEO data state…",
        geoIpMode: "GeoIP format",
        loader: "Loader",
        source: "Update source",
        loaders: {
          memconservative: "Memory conservative",
          standard: "Standard",
        },
        sources: {
          official: "Recommended source",
          custom: "Custom source",
          import: "Local import",
          existing: "Existing active file",
        },
        confirmBeforeOperations:
          "The source or format is still a settings draft. Select Confirm at the bottom before checking or updating GEO files.",
        coreRequired:
          "Install and confirm a mihomo version first. Downloaded and imported GEO files must pass validation by the selected core before they can be saved.",
        pendingRunning:
          "Validated GEO files are waiting for a full mihomo restart. The running core continues using the previous active files.",
        pendingStopped:
          "New GEO files are ready and will be switched safely before the next mihomo start.",
        restartNow: "Restart and apply",
        lastChecked: "Last checked",
        notChecked: "Not checked yet",
        activeMode: "Active GeoIP format",
        activeLoader: "Active loader",
        check: "Check for updates",
        updateAll: "Update selected three",
        openDirectory: "Open GEO directory",
        clean: "Clean old revisions",
        cleanConfirmTitle: "Clean unused GEO revisions?",
        cleanConfirmDescription:
          "Only historical files that are neither active nor pending are removed.",
        cancel: "Cancel download",
        cancelling: "Cancelling",
        assetsTitle: "Databases required by the selected format",
        assets: {
          "geoip-mmdb": "GeoIP MetaDB",
          "geoip-dat": "GeoIP DAT",
          geosite: "GeoSite DAT",
          asn: "ASN MMDB",
        },
        active: "Active",
        pending: "Restart pending",
        updateAvailable: "Update available",
        publisherVerified: "Publisher digest verified",
        notInstalled: "Not installed",
        update: "Update",
        download: "Download",
        import: "Import file",
        importDialogTitle: "Import {{asset}}",
        importDialogFilter: "mihomo GEO data files",
        manual: {
          open: "Manual install",
          title: "Manually install GEO databases",
          warning:
            "Download GEO files only from a trusted release page. Jeemi limits file size and validates content with the selected mihomo core, but locally imported files do not have publisher digest authentication.",
          stepRepository: "Open the recommended GEO database releases page:",
          openRepository: "Open releases",
          stepDownload:
            "Download GeoIP (geoip.metadb or GeoIP.dat), GeoSite.dat, and ASN.mmdb for the selected format.",
          stepImport:
            "Import each file with the matching button below. A stopped core applies them before its next start; a running core requires a full restart.",
          importAsset: "Import {{asset}}",
        },
        dataDirectory: "Revision and manifest directory",
        runtimeDirectory: "mihomo active-file directory",
        errors: {
          load: "Could not read GEO data state. Check the jeemi_data directory and manifest integrity.",
          check:
            "Could not check GEO updates. The recommended source requires a valid SHA-256 sidecar.",
          download:
            "GEO download, digest verification, or mihomo validation failed. Existing active files were not replaced.",
          cancel:
            "Could not cancel the GEO download. Wait for the current operation to finish and try again.",
          import:
            "The imported GEO file failed size, format, or selected-mihomo validation.",
          apply:
            "Restarting with the new GEO files failed. Jeemi attempted to roll back the files and restore the previous mihomo runtime.",
          clean:
            "Could not clean old GEO revisions. Active and pending files were not removed.",
          open: "Could not open the GEO data directory. Check the system file manager.",
          openReleases: "Could not open the GEO database releases page.",
        },
      },
      errors: {
        save: "Could not save Home settings. Check the selected core, custom GEO HTTPS URLs, and permissions for jeemi_data/settings.json.",
      },
    },
    subscriptions: {
      fetchTitle: "Subscription fetching",
      fetchMode: "Fetch method",
      manual: "Manual",
      onLaunch: "On launch",
      displayTitle: "Subscription display",
      displayMode: "Node card density",
      density: {
        large: "Large",
        medium: "Medium",
        small: "Small",
      },
      speedTestTitle: "Node delay testing",
      speedTestConcurrency: "Maximum concurrency",
      connectionResetTitle: "Node and fallback switching",
      connectionResetMode: "Reset connections on switch",
      connectionResetModes: {
        off: "Off",
        selector: "Selector",
        all: "All",
      },
      storageTitle: "Subscription storage directory",
      errors: {
        save: "Could not read or save subscription page settings. Check permissions for jeemi_data/settings.json.",
      },
    },
    config: {
      storageTitle: "Managed storage directory",
      scriptStorageTitle: "Local script storage directory",
    },
  },
  help: {
    section: {
      description: "Description",
      purpose: "Purpose",
      scenarios: "When to use it",
      cautions: "Cautions",
    },
    topics: {
      macNetworkAuthorization: enMacNetworkHelp,
      proxyAuthorization: enAuthorizationHelp,
      authorizationCleanup: enAuthorizationCleanupHelp,
      coreAuthorization: {
        title: "Unified Linux proxy authorization",
        description:
          "The system password dialog installs the authorization helper for core networking, privileged listener ports, and this account's DNS setup and cleanup on JeemiTun. Both proxy modes share the helper; ordinary starts, stops, and full restarts reuse it.",
        purpose:
          "Grant CAP_NET_ADMIN, CAP_NET_RAW, and CAP_NET_BIND_SERVICE to the verified mihomo core, and install four DNS permissions restricted to this account and JeemiTun. Jeemi remains unprivileged and directly manages the core process.",
        scenarios:
          "Confirm installation, an upgrade from an older authorization setup, or helper repair. A healthy helper grants permissions to newly installed, verified cores. Restarting Linux or Jeemi normally requires no password.",
        cautions:
          "The helper service waits at startup without starting the proxy or storing passwords. Other processes of the same account can also change JeemiTun DNS; other interfaces are outside this grant. Replaced cores need verification again; cancellation preserves the existing core. Requires a desktop Polkit agent and file capability support; systemd-resolved integration requires systemd 257 or newer. The password dialog may show the sealed helper's /proc/…/fd/… path.",
      },
      runtimePreferences: {
        title: "Proxy runtime parameters",
        description:
          "Automatically saves outbound, proxy, listener, TUN, and safe runtime baseline settings, then injects them after subscription and local configuration composition.",
        purpose:
          "Gives mihomo an explicit, validated runtime baseline even when a subscription omits essential fields.",
        scenarios:
          "Adjust common network parameters, DNS resolvers, Fake IP, policy mappings, and hosts from Home without editing immutable subscription source text.",
        cautions:
          "Edits are validated before commit; a failed live apply keeps the previous healthy generation. Linux system proxy currently supports Ubuntu/GNOME and only affects apps that follow desktop proxy settings; terminal tools may not. Both Linux proxy modes request persistent authorization for the selected mihomo through the system password dialog on first start. The GUI stays unprivileged. Managed fields stay locked in the local configuration tree.",
      },
      runtimeBaseline: {
        title: "Safe runtime baseline",
        description:
          "Controls logging, IPv6, proxy listening, LAN access, TUN automatic routes, interface detection, and route-exclusion ranges in one place.",
        purpose:
          "Completes missing subscription values and keeps system proxy and TUN behavior under one Jeemi-managed source.",
        scenarios:
          "Use it for listener port conflicts, trusted LAN sharing, or platform-specific TUN routing behavior.",
        cautions:
          "LAN access widens the listener boundary. Disabling automatic routes or interface detection can require manual route maintenance. Excluded ranges bypass TUN, so broad exclusions can bypass the proxy.",
      },
      runtimeTunRouteExclude: {
        title: "TUN excluded subnets",
        description:
          "Manages tun.route-exclude-address as a YAML string array. When injection is enabled, entries can be appended uniquely or fully override the source; with auto-route enabled, matching ranges are omitted from TUN routes.",
        purpose:
          "Keeps LAN, management, or other destinations that require native system routing outside TUN capture.",
        scenarios:
          "Append local ranges to subscription exclusions or override them with an independent list for routers, NAS devices, enterprise networks, or another VPN.",
        cautions:
          "When disabled, the subscription value is preserved. Only CIDRs such as 192.168.0.0/16 or fc00::/7 are accepted. Public exclusions bypass mihomo entirely, so keep them narrow.",
      },
      runtimeDns: {
        title: "DNS baseline",
        description:
          "Controls mihomo DNS service, IPv6 answers, enhanced mode, and Fake IP pools.",
        purpose:
          "Supplies working defaults when a subscription omits DNS and gives IPv4 and IPv6 Fake IP explicit, validated address pools.",
        scenarios:
          "Adjust it for local DNS listening, switching Fake IP or Redir Host, or an IPv4-only local network.",
        cautions:
          "The default binds only to 127.0.0.1:1053. Using 0.0.0.0 exposes DNS to the LAN. Fake IP ranges must remain valid CIDRs of the expected address family.",
      },
      runtimeDnsMerge: {
        title: "DNS resource composition",
        description:
          "Each DNS list or mapping can be enabled independently, then append unique values to or fully override the matching subscription field.",
        purpose:
          "Preserves useful subscription policies while still supporting a deterministic Jeemi-managed result when needed.",
        scenarios:
          "Append public resolvers, add domain policies, replace subscription defaults with a fixed list, or disable a resource to preserve the subscription unchanged.",
        cautions:
          "Append keeps subscription entries first, adds Home values, and removes exact duplicates. Home wins same-key mapping conflicts. YAML is validated on blur and saved only after it passes.",
      },
      runtimeDnsResolvers: {
        title: "DNS resolver lists",
        description:
          "Configures nameserver or proxy-server-nameserver as a YAML string array, with quick-add choices for common resolvers.",
        purpose:
          "Provides explicit upstream DNS services for ordinary domains and proxy-server domains.",
        scenarios:
          "Use it to add IP, DoH, DoT, or DoQ upstreams, or to keep proxy-server name resolution independent from the proxy chain.",
        cautions:
          "An enabled nameserver needs at least one entry. Disabling a resource stops injecting it. Proxy-server policy is effective only when proxy-server-nameserver has a usable value.",
      },
      runtimeFakeIpFilter: {
        title: "Fake IP filter list",
        description:
          "Manages domain matchers that should not receive Fake IP answers as a YAML string array.",
        purpose:
          "Lets LAN, discovery, and Fake-IP-incompatible domains continue to receive real DNS results.",
        scenarios:
          "Adjust it when local domains, discovery, games, or specific applications resolve incorrectly in Fake IP mode.",
        cautions:
          "Disabling the resource preserves the subscription fake-ip-filter and mode. Overly broad matchers reduce Fake IP coverage.",
      },
      runtimeDnsPolicies: {
        title: "DNS resolver policy maps",
        description:
          "Uses a YAML map to assign one or more resolvers to domains or rule collections.",
        purpose:
          "Routes different domains to different upstreams and separately controls normal and proxy-server resolution policies.",
        scenarios:
          "Use it for regional DNS routing, intranet domains, or dedicated proxy-server name resolution.",
        cautions:
          "Only maps of strings or string arrays without anchors or aliases are accepted. Disabling a resource never injects or clears the subscription field.",
      },
      runtimeHosts: {
        title: "hosts mapping",
        description:
          "The header switch reveals the editor; when enabled, Home manages the top-level hosts map and injects use-hosts: true into DNS.",
        purpose:
          "Pins a domain to one or more addresses and composes those entries with subscription hosts using the selected mode.",
        scenarios:
          "Use it for local development, intranet services, or a temporary DNS override.",
        cautions:
          "Only YAML maps without anchors or aliases are accepted, and values must be strings or string lists. Turning it off preserves subscription use-hosts and hosts instead of forcing them off.",
      },
      runtimeLanBypass: {
        title: "LAN direct-route protection",
        description:
          "Jeemi places default DIRECT rules for common loopback, private, link-local, and multicast destinations at the start of final rules and lets you edit them.",
        purpose:
          "Prevents an incomplete or badly ordered subscription from sending local and LAN access through a proxy.",
        scenarios:
          "The defaults cover routers, NAS devices, intranet services, loopback addresses, and discovery protocols; developers can adjust or clear them for debugging networks.",
        cautions:
          "Only DOMAIN, DOMAIN-SUFFIX, IP-CIDR, and IP-CIDR6 rules targeting DIRECT are accepted. An empty list disables the prefix. Restoring defaults still requires Save.",
      },
      runtimeListener: {
        title: "External proxy listener",
        description:
          "Selects the HTTP, SOCKS, or Mixed proxy listener and port exposed by Jeemi.",
        purpose:
          "Keeps the operating-system proxy and manually configured applications pointed at one explicit local listener.",
        scenarios:
          "Use it to support a required proxy protocol or resolve a local port conflict with another application.",
        cautions:
          "The port must be between 1 and 65535 and not already in use. Changes regenerate the final runtime configuration and become active only after validation and hot reload succeed.",
      },
      runtimeLanAccess: {
        title: "Allow LAN connections",
        description:
          "Controls whether the proxy listener binds only to loopback or accepts connections from LAN devices.",
        purpose:
          "Lets other devices on a trusted local network use Jeemi's proxy port when explicitly needed.",
        scenarios:
          "Enable it to provide a temporary proxy to a phone, tablet, or another computer after defining the network boundary.",
        cautions:
          "This exposes the proxy port to the LAN. Pair it with authentication, firewall rules, and a trusted network; avoid enabling it on public networks.",
      },
      trafficChart: {
        title: "Live traffic chart",
        description:
          "Receives real upload, download, and cumulative traffic from mihomo's /traffic WebSocket once per second and plots the last minute.",
        purpose:
          "Quickly confirms whether traffic is flowing and how upstream and downstream rates change over time.",
        scenarios:
          "Use it during downloads, video, speed tests, or when a started proxy appears to carry no traffic.",
        cautions:
          "Data is marked live only while the control API is healthy. A disconnected curve is marked stale and reconnects automatically; Jeemi never fills gaps with simulated traffic.",
      },
      home: {
        title: "Home",
        description:
          "Shows core state, memory, uptime, and live traffic, with proxy start/stop controls and outbound, listener, and DNS parameters.",
        purpose:
          "Provides a quick client, core, and network check for routine use and troubleshooting.",
        scenarios:
          "Start here after launch, around core start or stop actions, or while diagnosing network problems.",
        cautions:
          "Opening Home does not change system networking. Valid parameter edits save automatically; network operations take effect only after core health checks. Failed applies retain the previous active configuration and show the reason.",
      },
      processMemory: {
        title: "Process memory",
        description:
          "Shows the client, web runtime, mihomo, and their combined memory in binary units such as MiB. The client includes Jeemi, its authorization helper, and related support processes. The authorization helper reports its own memory and the managed mihomo memory.",
        purpose:
          "Tracks the UI and proxy core separately. The web runtime includes only this Jeemi instance's WebView2, WebKitGTK, or WebKit processes, excluding other applications. The helper can collect these measurements when the client cannot read them.",
        scenarios:
          "Compare memory before and after opening pages, loading subscriptions, hiding the window, or starting and stopping the proxy.",
        cautions:
          "Windows prefers private working set. Linux and macOS report resident memory (RSS), which can count shared pages more than once, so the total is not unique physical memory usage. An unreachable or incompatible helper, or uncertain process ownership, produces a dash. The total requires all three complete measurements; missing values are never treated as zero.",
      },
      subscriptions: {
        title: "Subscriptions",
        description:
          "Adds and fetches subscriptions from URLs, configuration files, or QR codes, then manages source revisions and mutually exclusive local configuration or local script associations.",
        purpose:
          "Brings different subscription sources into Jeemi with validation, traceability, and failure-safe retention.",
        scenarios:
          "Use it to import a subscription, fetch it again, inspect its source, or assign a local override per subscription.",
        cautions:
          "Subscription URLs and source text can contain credentials, and failed updates must not replace the current revision. The offline selector is a composition preview, never a claimed live mihomo state.",
      },
      config: {
        title: "Config",
        description:
          "Organises local scripts, local configurations, local policy groups, and local rule sets into reusable subscription processing resources.",
        purpose:
          "Adjust subscriptions with structured fields or synchronous main(config) scripts, and maintain policy groups and rule content in one place.",
        scenarios:
          "Use it when subscription content cannot cover local network needs or different subscriptions require different overrides.",
        cautions:
          "Each subscription can associate at most one local configuration or script; the two are mutually exclusive. Saving is not activation, referenced resources cannot be deleted, and final runtime configurations still require validation.",
      },
      connections: {
        title: "Connections",
        description:
          "Shows live destinations, processes, matched rules, actual outbounds, and traffic. Click a target or process name to inspect its full path, addresses, and proxy chain.",
        purpose:
          "Explains current traffic routing and helps diagnose rule matches, unexpected access, or performance issues.",
        scenarios:
          "Use it for failed requests, unexpected traffic, or closing one connection or the currently filtered result set.",
        cautions:
          "Rule sets show their provider name, and MATCH shows MATCH. Other rules show the type and payload returned by the core, falling back to inline if the payload is absent; missing original flags cannot be reconstructed. Process paths may be unavailable. Details retain the last snapshot when a connection ends or its stream disconnects. Domains, IPs, and paths may be private; bulk close requires confirmation and only affects the current results.",
      },
      logs: {
        title: "Logs",
        description:
          "Streams raw logs from the current mihomo core through its control API.",
        purpose:
          "Quickly inspect rule matches, DNS, connections, and internal core activity with instant in-page search.",
        scenarios:
          "Use it to diagnose routing, resolution, or rule issues, or temporarily raise the log level while observing behavior.",
        cautions:
          "Only new entries received while the page is active are shown; no history is stored. Output is displayed exactly as mihomo sends it and may include domains, IPs, and other runtime data. Debug output is high-volume and is not recommended for long-term use.",
      },
      tools: {
        title: "Tools",
        description:
          "Collects focused utilities such as IP lookup, streaming unlock tests, and DNS lookup.",
        purpose:
          "Checks the active egress and DNS result without entering a complex configuration workflow.",
        scenarios:
          "Use it to verify proxy egress, regional availability, or DNS resolution.",
        cautions:
          "Lookups and unlock tests contact external services and may disclose the egress IP or queried domain. Data source, timeout, and failure state must be visible.",
      },
      homeSettings: {
        title: "Home settings",
        description:
          "Manages the interface theme, language, Jeemi update checks, and mihomo core version selection.",
        purpose:
          "Keeps client-wide interface and system preferences in one place.",
        scenarios:
          "Use it to change display preferences, check for client updates, or select an official core version.",
        cautions:
          "Theme and language are stored by the frontend after confirmation. Core downloads and removals run immediately; the active version is persisted by Go only after confirmation.",
      },
      subscriptionSettings: {
        title: "Subscription settings",
        description:
          "Manages subscription fetching, node-card density, and the all-node delay-test concurrency limit.",
        purpose:
          "Adapts subscription freshness, information density, and test load to different devices and workflows.",
        scenarios:
          "Use it to keep fetching manual, change node-card density, or limit simultaneous tests for large node lists.",
        cautions:
          "Go saves density and concurrency atomically. On-launch fetching remains disabled until scheduling and failure rollback are implemented.",
      },
      emptySettings: {
        title: "Page settings",
        description:
          "This is the current navigation page's dedicated settings entry, but no options are defined yet.",
        purpose:
          "Keeps a stable, consistent settings entry for every primary feature.",
        scenarios:
          "Add options here only when a real page-specific display or behavior preference appears.",
        cautions:
          "Do not invent non-functional switches to fill space. New business settings need a default, persistence location, and rollback behavior.",
      },
      appUpdate: {
        title: "Jeemi update checks",
        description:
          "Queries available Jeemi versions, release notes, and platform-matched artifacts.",
        purpose: "Keeps users informed about security fixes and new features.",
        scenarios:
          "Use it for manual checks or a future configured update interval.",
        cautions:
          "Download, signature verification, and installation must remain separate visible stages. The update backend is not connected yet.",
      },
      coreVersion: {
        title: "mihomo version",
        description:
          "Checks official mihomo releases and selects a verified version for Jeemi.",
        purpose: "Makes compatibility, feature, and rollback choices explicit.",
        scenarios:
          "Use it to install a core, upgrade, or roll back after a compatibility issue.",
        cautions:
          "Online installation accepts only exact platform matches from official stable releases with SHA-256. Manual imports have no publisher identity verification and should use trusted official files only. Neither path switches versions automatically; choose one and confirm at the bottom of the page.",
      },
      coreFiles: {
        title: "mihomo core files",
        description:
          "Manages versions downloaded and verified by Jeemi below jeemi_data/core/mihomo.",
        purpose:
          "Keeps multiple versions available for upgrade testing, compatibility rollback, and disk cleanup.",
        scenarios:
          "Use it to inspect the actual core path, remove unused versions, or manually back up diagnostic information.",
        cautions:
          "The active version cannot be removed. When an imported version cannot be recognised, Jeemi uses a content-derived unknown-* directory. Do not replace the executable or metadata.json directly, or integrity checks will mark that version unusable.",
      },
      geoData: {
        title: "GEO database management",
        description:
          "Manages GeoIP MetaDB/DAT, GeoSite DAT, and ASN MMDB independently while injecting the format and loader as Jeemi-reserved runtime parameters.",
        purpose:
          "Lets incomplete subscriptions use GEOIP, GEOSITE, or IP-ASN rules with databases verified by a publisher digest when available and by the selected mihomo core.",
        scenarios:
          "Use it to change GeoIP formats, update databases, use a trusted custom HTTPS mirror, or import local databases on an offline device.",
        cautions:
          "Files are never replaced beneath a running mihomo process. Updates wait for a full restart and can be rolled back. A custom source without a publisher digest proves readability and content integrity only, not publisher identity.",
      },
      subscriptionFetch: {
        title: "Subscription fetch method",
        description:
          "Defines whether updates are triggered manually, at launch, or by a future scheduled job.",
        purpose:
          "Balances subscription freshness, startup speed, and control over network requests.",
        scenarios:
          "Adjust it when subscriptions have different update needs or startup requests should be avoided.",
        cautions:
          "Fetching needs cancellation, timeouts, and size limits. These choices show the plan only and are not persisted yet.",
      },
      subscriptionDelayTest: {
        title: "Node-test concurrency",
        description:
          "Controls how many delay requests “Test all nodes” submits to mihomo at once while the remaining nodes stay in one Jeemi queue.",
        purpose:
          "Balances completion speed against local, network, and provider load while preventing repeated clicks from creating overlapping queues.",
        scenarios:
          "Lower it for large node lists, slower devices, or provider concurrency limits; raise it moderately for faster completion.",
        cautions:
          "The allowed range is 1–50 and the default is 8. Tests make real outbound requests; all-node and per-node entries are temporarily locked while a queue exists.",
      },
      subscriptionConnectionReset: {
        title: "Reset connections on switch",
        description:
          "Chooses whether established connections are closed after a node or fallback traffic change takes effect on the Subscriptions page.",
        purpose:
          "Moves long-lived traffic to the new route promptly while limiting how much active traffic is interrupted.",
        scenarios:
          "Off preserves sessions. Selector matches the node's selector exactly in connection chains; fallback changes match the previously active terminal-rule target. All resets every existing connection.",
        cautions:
          "Selector is the default. After a successful rule-mode reload changes the effective target, Jeemi reads current connections and closes matching IDs. No override matches the original target, and Direct matches DIRECT. Restarts, failed reloads and pending changes do not trigger extra closures. Applications may reconnect; All also interrupts unrelated routes.",
      },
      subscriptionImport: {
        title: "Subscription import",
        description:
          "Imports the complete subscription source from an http/https URL, configuration file, or QR code.",
        purpose:
          "Normalises different sources into traceable, immutable subscription revisions for later composition and runtime validation.",
        scenarios:
          "Use it for a first subscription, migration from an existing YAML/JSON/TXT file, or a provider-supplied subscription QR code.",
        cautions:
          "Each configuration has size and YAML-safety limits. A revision changes only after fetching, parsing, and atomic storage all succeed; failures never replace current content.",
      },
      subscriptionQRCode: {
        title: "QR subscription import",
        description:
          "Recognises QR codes from a selected image or an immediate desktop screenshot, then fetches http/https content as a subscription URL. Linux Wayland uses the image supplied by the desktop screenshot service.",
        purpose:
          "Imports long addresses that are awkward to type without retaining screenshots or QR images in the data directory.",
        scenarios:
          "Use it when a provider offers only a QR code or when the code is visible in a browser, chat app, or another display.",
        cautions:
          "Jeemi temporarily hides and restores its window when capture finishes or is cancelled. Linux Wayland may show a desktop screenshot confirmation; the desktop decides whether permission is remembered. This requires no administrator password and is separate from mihomo network authorization. macOS may require Screen Recording permission. Images are decoded locally; temporary portal screenshots are cleaned up after reading, never stored in Jeemi data or uploaded. Only a valid subscription URL is fetched. Selecting an existing image needs only file read access.",
      },
      subscriptionRawText: {
        title: "Subscription source",
        description:
          "Shows the original YAML, JSON, or TXT text from the current saved subscription revision as read-only content.",
        purpose:
          "Lets you verify what the provider returned, inspect field structure, and understand later composition input.",
        scenarios:
          "Use it after fetching, while diagnosing format problems, or when comparing provider content.",
        cautions:
          "The source may contain passwords, tokens, node addresses, and private rule URLs. Jeemi does not copy or redact this view automatically; inspect it before sharing.",
      },
      subscriptionAssociation: {
        title: "Subscription and local handler association",
        description:
          "Each subscription can use either one local configuration or one local script. The two handlers are mutually exclusive and both may be reused.",
        purpose:
          "Applies structured field composition or a JavaScript object transform per subscription while preserving the original source.",
        scenarios:
          "Associate a configuration for reusable rules and parameters, or a script for programmatic node, group, rule, and field transforms.",
        cautions:
          "Script association and saves to referenced scripts first generate and validate final runtime configurations. Failures preserve the prior association and script. Referenced handlers cannot be deleted.",
      },
      subscriptionIcon: {
        title: "Subscription icon",
        description:
          "Choose a controlled Emoji or HTTP(S) icon URL, or detect common icon files from the subscription source when no icon is configured.",
        purpose:
          "Makes subscriptions easier to distinguish in cards and the collapsed subscription shelf.",
        scenarios:
          "Use a fixed Emoji, a provider-supplied icon URL, or let Jeemi probe favicon.ico, favicon.png, sub.ico, and sub.png.",
        cautions:
          "Displaying a remote icon makes a network request. Automatic detection only probes same-origin root paths and sends no subscription path, query, or Referer; it never changes the saved source.",
      },
      localPackage: {
        title: "Import and export packages",
        description:
          "Card menus export and import .json files. Configuration packages include all saved YAML tree fields, merge settings, referenced strategy groups and their rule sets. Script packages include the name, description and full source.",
        purpose:
          "Back up or transfer complete local configurations, or restore a backup into the current card.",
        scenarios:
          "Export on the source client, import from a configuration or script card on the destination, review same-name replacements, then confirm.",
        cautions:
          "Import replaces the current card while keeping its subscription associations. Same-name resources affect every configuration using them. Failed subscription checks prevent saving; each rule set must still have one strategy-group owner. Files preserve raw fields and source, which may include credentials or URLs you entered; store them carefully.",
      },
      localConfig: {
        title: "Local configuration",
        description:
          "Maintains a sparse subscription overlay with structured fields, reusable local policy groups, and explicit composition modes.",
        purpose:
          "Reuses the same adjustments across subscriptions while preserving each original subscription configuration.",
        scenarios:
          "Create a local configuration to share node properties, rule ordering, or selector resources, then associate it on the Subscriptions page.",
        cautions:
          "Creating or saving does not associate a subscription or start the core. A subscription can use either a local configuration or a local script. Jeemi-owned fields remain reserved, and changes to a running configuration must pass validation before application.",
      },
      localScript: {
        title: "Local script transform",
        description:
          "Parses the current subscription YAML into a plain JavaScript object, calls main(config), and encodes the returned object for the remaining composition pipeline.",
        purpose:
          "Provides reusable conditional transforms for nodes, groups, rules, or fields that structured local configuration cannot express conveniently.",
        scenarios:
          "Run a test against the current subscription and inspect the final preview before association. Editing a referenced script validates every associated subscription.",
        cautions:
          "The Go-isolated runtime exposes no filesystem, network, process, environment, or Wails APIs. Its clock and random source are fixed, execution is time-bounded, and the result must be a valid top-level configuration object. Scripts are still executable logic; use only content you trust.",
      },
      subscriptionShelf: {
        title: "Subscription shelf",
        description:
          "The shelf lives in an app-shell overlay below the title bar. Expanded mode manages all cards; collapsed mode shows the current summary, a usage-backed traffic label, selector search, and quick actions.",
        purpose:
          "Keeps traffic, expiry, update age, and node search together without moving proxy selectors when the shelf expands or collapses.",
        scenarios:
          "Expand it to switch or edit subscriptions; collapse it to search selectors or use refresh, rule-provider, speed-test, and hidden-selector actions.",
        cautions:
          "Traffic and names may be absent or stale. Testing is enabled only for a healthy mihomo session matching the current projection, and showing hidden groups changes only this view, not the subscription or running core.",
      },
      fallbackTraffic: {
        title: "Fallback traffic",
        description:
          "Choose where unmatched traffic goes for this subscription: leave the original rule unchanged, connect directly, or use a selector from the composed configuration.",
        purpose:
          "Inject the terminal target after local configuration or script processing, while keeping the subscription source and local proxy rule-group binding intact.",
        scenarios:
          "Use it when subscriptions need different fallback routes or unmatched traffic should go directly. The preference is saved per subscription and affects routing in rule mode. After a manual change takes effect, existing connections follow Reset connections on switch in subscription settings.",
        cautions:
          "The original target is the first MATCH / FINAL before injection, not the selected node. After refresh, switching or editing, selector names must match exactly; a missing target resets to no override. Fetch or composition failures keep the preference. An override edits the first top-level terminal in place, or appends MATCH if absent. No override does not repair an invalid source target.",
      },
      selectorPreview: {
        title: "Proxy selector preview",
        description:
          "Reads proxy groups and statically available nodes from the current subscription composed with its local configuration. The icon control beside the title shares Rule, Global, and Direct outbound modes with Home.",
        purpose:
          "Lets you inspect selector structure offline, then quickly switch between rule groups, GLOBAL with every real node, and the direct view while mihomo is running.",
        scenarios:
          "Use it after importing, refreshing, switching subscriptions, or changing an association. Search and temporary hidden-group controls live in the collapsed shelf; outbound mode lives beside the selector title.",
        cautions:
          "Global hides ordinary selectors and Direct shows none. Search excludes protocol and provider names; external-provider nodes, delay, and true selections require running mihomo. Remote icons are HTTP(S)-only and omit Referer.",
      },
      ruleProviderRefresh: {
        title: "Rule-provider update",
        description:
          "Lists providers available after subscription/local composition. Switches are saved per subscription, while downloads use a healthy mihomo control session.",
        purpose:
          "Lets each subscription decide which providers enter its final runtime configuration and shows the core's last successful update time.",
        scenarios:
          "Use it to temporarily disable a routing rule set or to download stale provider content again.",
        cautions:
          "Disabling removes the provider and its known references after composition without rewriting source. The change must be regenerated, validated, and applied. Update time is shown only when mihomo reports it; React never downloads provider files directly.",
      },
      localConfigField: {
        title: "Local configuration field",
        description:
          "Controls whether one mihomo field is output, its local value, and its type-aware composition with the subscription value.",
        purpose:
          "Prevents a full default configuration from unintentionally overriding subscriptions and keeps order, conflicts, and sources reviewable.",
        scenarios:
          "Enable it when a subscription lacks required DNS, rules, TUN parameters, or another advanced field.",
        cautions:
          "Use only strategies allowed for the field type. Same-name objects block composition by default, and runtime-owned fields cannot be controlled locally.",
      },
      managedStorage: {
        title: "Managed storage directory",
        description:
          "Shows the read-only Jeemi directory for subscription configurations, local configurations, or local scripts.",
        purpose:
          "Helps with backups, migration, and file-permission diagnostics.",
        scenarios:
          "Locate your data or back up its directory after closing the client.",
        cautions:
          "These files may contain subscription credentials or private configuration. Do not replace managed files while the client is running, and review backups before sharing. This view does not change the directory.",
      },
      subscriptionSource: {
        title: "Subscription URL and name",
        description:
          "Fetches a complete subscription configuration from an HTTP(S) URL. An empty name on creation uses Profile-Title, then the Subscription-Userinfo name, then the URL host.",
        purpose:
          "Keeps a refreshable source with a name that makes subscriptions easy to identify.",
        scenarios:
          "Add a URL subscription or update the source when a provider changes its link.",
        cautions:
          "URLs may contain access credentials; keep them private. A changed URL is fetched and validated before saving, and failure preserves the old URL and revision. Cards and ordinary Jeemi logs show a redacted address.",
      },
      localConfigPreview: {
        title: "Local YAML preview",
        description:
          "Shows YAML and bulk node transforms produced by the current local configuration draft.",
        purpose:
          "Lets you review enabled fields, composition choices, and sensitive content before saving.",
        scenarios:
          "Check a new override or investigate how local settings will affect a subscription.",
        cautions:
          "This preview is not active and is not the final configuration composed with a subscription. Sensitive fields are hidden initially; review them before sharing a revealed preview.",
      },
      geoDataFormat: {
        title: "GeoIP format",
        description:
          "Chooses MMDB/MetaDB or DAT for GeoIP; GeoSite DAT and ASN MMDB are maintained separately.",
        purpose:
          "Keeps GeoIP data consistent with the final runtime configuration.",
        scenarios:
          "Keep the default MMDB, or choose DAT and prepare its database when needed.",
        cautions:
          "This is a settings draft until confirmed. The chosen format needs a valid matching file; a running core uses new files only after a full restart.",
      },
      geoDataLoader: {
        title: "GEO loader",
        description:
          "Chooses the memory-conserving memconservative or standard loader for DAT data.",
        purpose:
          "Adjusts the loading strategy to the available device resources.",
        scenarios:
          "Keep the memory-conserving default, or try the standard loader when investigating DAT compatibility.",
        cautions:
          "The loader does not convert MMDB into DAT. Confirm the preference on this page; GEO changes during a running session wait for a full restart.",
      },
      geoDataSource: {
        title: "GEO download source",
        description:
          "Uses the recommended source or a custom HTTPS address for each database.",
        purpose:
          "Supports trusted mirrors when the default source is unavailable, and local imports for offline use.",
        scenarios:
          "Check for updates manually, use a private mirror, or import a previously downloaded database.",
        cautions:
          "Opening settings normally does not access the network. Confirm source changes before downloading. Imports or custom sources without a valid publisher digest prove local integrity, not publisher identity.",
      },
      runtimeLogLevel: {
        title: "mihomo log level",
        description:
          "Controls the levels emitted by mihomo and whether the Logs page is available.",
        purpose: "Balances everyday logging volume with diagnostic detail.",
        scenarios:
          "Use the Silent default, or temporarily choose Error, Warning, Info, or Debug while investigating a problem.",
        cautions:
          "Silent hides the Logs navigation entry and closes that page. Debug output may include destinations and IP addresses; the page keeps only the latest 500 entries in memory.",
      },
    },
  },
} as const;
