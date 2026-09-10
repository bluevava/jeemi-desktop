# 通用 URI 订阅结构解析说明

**优先推荐 mihomo 原生 YAML 订阅**，完整结构参考 [mihomo 官方文档](https://wiki.metacubex.one/config/)。YAML 能直接表达节点、DNS、规则和选择器。需要 URI 时，推荐按本文把 **mihomo 代理节点字段转换为 URI**，避免依赖不同客户端的私有命名。

本文是 Jeemi 的 URI 生成约定，字段目录核对于 2026-09-10；不是 mihomo 官方定义的分享链接标准。普通订阅、链式代理的文本导入和 URL 导入共用此解析器。其他客户端的已支持别名和编码方式继续兼容，新订阅推荐使用标准字段。

**通用 URI 订阅支持自定义节点 DNS**：独立 `host://` 行可以为代理服务器域名指定 DNS。`host://` 和 `cert://` 是订阅元数据扩展，不代表所有客户端都能识别。见 [自定义节点 DNS](#node-dns)。

## 推荐结构：直接使用 mihomo 字段

每行一个节点，支持两种等价形式：

```text
协议://[凭据@]服务器:端口?mihomo字段=值&其他字段=值#节点名称
协议://?name=名称&server=服务器&port=端口&其他mihomo字段=值
```

- `协议` 对应节点的 `type`，例如 `anytls`、`vmess`、`wireguard`。查询中可重复 `type=anytls`，但必须与协议一致；推荐省略。
- 服务器、端口、凭据、名称可以完全放入查询参数。WireGuard 多 peer、Tailscale 等不要求顶层服务器的节点，适合第二种形式。
- 查询键沿用 mihomo 字段名及大小写，例如 `client-fingerprint`、`idle-session-timeout`、`alterId`、`ws-opts`。不把下划线或驼峰自动改成连字符。
- `name` 与 `#名称`、查询字段与地址/凭据部分、标准字段与兼容别名若重复，规范化后的值必须一致；冲突只跳过当前节点，不能依赖某一种写法覆盖另一种。
- `network` 是传输方式，`type` 是节点协议。`type=ws` 仅作为旧 URI 的兼容写法，推荐 `network=ws`。

以下是完整订阅示例。域名、地址、密码和 UUID 都是演示值，请替换为自己的配置。

```text
# 自定义节点 DNS
host://*.entry.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2Fdns-query

# 标准字段节点
anytls://example-password@at.entry.example.invalid:443?sni=tls.example.invalid&client-fingerprint=firefox&idle-session-timeout=30&tfo=false#AnyTLS
vmess://11111111-2222-4333-8444-555555555555@vm.entry.example.invalid:443?alterId=0&cipher=auto&tls=true&servername=tls.example.invalid&network=ws&ws-opts=%7B%22path%22%3A%22%2Fws%22%7D#VMess
hysteria2://example-password@hy.entry.example.invalid:443?sni=tls.example.invalid&obfs=salamander&obfs-password=example-obfs#Hysteria2
```

保存为 `.txt` 导入，或者由 HTTP(S) 订阅地址返回正文。普通订阅的 URL/二维码入口接收订阅地址；节点 URI 放在订阅正文中。配置页的链式代理手动组可以直接粘贴多个节点 URI。

### 字段值与编码

| mihomo 字段类型 | URI 查询值规则 | 编码前示例 |
| --- | --- | --- |
| 字符串 | 保留字符串语义，按查询值编码 | `password=00123`、`cipher=auto` |
| 布尔值 | 推荐 `true` / `false`，兼容 `1` / `0` | `tfo=false` |
| 整数 | 十进制整数，单位沿用该字段 | `port=443`、`heartbeat-interval=10000` |
| 字符串数组 | JSON 数组，整体作为一个查询值编码 | `alpn=["h2","http/1.1"]` |
| 对象 | JSON 对象，保留原字段名与嵌套结构 | `ws-opts={"path":"/ws","headers":{"Host":"cdn.example.invalid"}}` |
| 对象数组 | JSON 数组，每一项为对象 | `peers=[{"server":"wg.example.invalid","port":51820,"public-key":"..."}]` |
| 多行证书/私钥 | PEM 原文，换行编码为 `%0A` | `certificate`、SSH `private-key`、OpenVPN `ca/cert/key` |

表中对象例子是**编码前的值**。实际 URI 例如：

```text
alpn=%5B%22h2%22%2C%22http%2F1.1%22%5D
ws-opts=%7B%22path%22%3A%22%2Fws%22%2C%22headers%22%3A%7B%22Host%22%3A%22cdn.example.invalid%22%7D%7D
```

对象内部仍按 mihomo 类型填写：布尔值不要加引号，字符串数组不要改成一个逗号字符串。不支持把 `ws-opts.path` 当顶层字段名；需要传整个 `ws-opts` 对象。JSON 必须有效，不接受重复键、YAML 标签、锚点或别名。嵌套对象会保留 JSON 结构，具体子字段和协议组合由所选 mihomo 核心继续校验。

- 按 URI 组件分别编码，不编码整条 URI。密码 `p@ss:word+1` 可写成 `p%40ss%3Aword%2B1`；名称 `HK 01` 写成 `HK%2001`。查询值中的字面量 `+` 必须编码为 `%2B`。
- IPv6 地址部分使用 `[2001:db8::1]:443`；纯查询的 `server` 值填写 IPv6 本身并编码。端口范围为 `1–65535`，SSH 默认 22、OpenVPN 默认 1194；其他例外见协议表。
- 支持 UTF-8、BOM、LF/CRLF、空行与 `#` 整行注释。节点行的 `#` 表示名称，不能再写行尾注释。
- 整份列表可编码一层 Base64，接受标准/URL-safe 字母表及带或不带填充；不递归解码多层订阅。协议自身的 SS/VMess/SSR 编码另算。
- 正文最多 4 MiB、20,000 行，节点 URI 单行最多 256 KiB，嵌套 JSON 另受结构限制。未命名节点自动命名；名称冲突加编号；完全相同的节点去重。

## 协议与标准字段目录

下表涵盖本次核对的 mihomo 协议目录。协议名和字段依据各行官方链接；示例中的配置项不是认证数据。字段是否可用还取决于所选核心版本，解析成功不代表实际连通。

所有协议可填写通用字段 `name`、`type`、`server`、`port`、`ip-version`、`udp`、`interface-name`、`routing-mark`、`tfo`、`mptcp`、`dialer-proxy`、对象 `smux`，但必须满足具体协议的限制；例如 TUIC 不支持 `udp=false`。参见 [通用字段](https://wiki.metacubex.one/config/proxies/)。

| 协议 / scheme | 凭据和端点 | 协议标准字段（另可使用适用的通用/TLS/传输字段） |
| --- | --- | --- |
| [HTTP](https://wiki.metacubex.one/config/proxies/http/) `http` | 可选 `username/password`；无认证可省略凭据 | `username`、`password`、对象 `headers` |
| [SOCKS5](https://wiki.metacubex.one/config/proxies/socks/) `socks5` | 同 HTTP | `username`、`password` |
| [Shadowsocks](https://wiki.metacubex.one/config/proxies/ss/) `ss` | `cipher:password@` 或查询字段 | `cipher`、`password`、`plugin`、对象 `plugin-opts`、`client-fingerprint`、`udp-over-tcp`、`udp-over-tcp-version` |
| [ShadowsocksR](https://wiki.metacubex.one/config/proxies/ssr/) `ssr` | `cipher:password@`，另填 `protocol/obfs` | `cipher`、`password`、`protocol`、`protocol-param`、`obfs`、`obfs-param` |
| [Snell](https://wiki.metacubex.one/config/proxies/snell/) `snell` | `psk@` 或查询 `psk` | `psk`、`version`、对象 `obfs-opts`；版本 1–5，默认 1，版本 1/2 不接受 `udp=true` |
| [VMess](https://wiki.metacubex.one/config/proxies/vmess/) `vmess` | `uuid@` 或查询 `uuid` | `uuid`、`alterId`、`cipher`、`global-padding`、`authenticated-length`、`packet-encoding`；默认 `alterId=0`、`cipher=auto` |
| [VLESS](https://wiki.metacubex.one/config/proxies/vless/) `vless` | `uuid@` 或查询 `uuid` | `uuid`、`flow`、`encryption`、`packet-encoding` |
| [Trojan](https://wiki.metacubex.one/config/proxies/trojan/) `trojan` | `password@` 或查询 `password` | `password`、对象 `ss-opts` |
| [AnyTLS](https://wiki.metacubex.one/config/proxies/anytls/) `anytls` | `password@` 或查询 `password` | `password`、`client-metadata`、`idle-session-check-interval`、`idle-session-timeout`、`min-idle-session`；两个超时为整数秒，范围 1–86400 |
| [Mieru](https://wiki.metacubex.one/config/proxies/mieru/) `mieru` | `username/password`，`port` 与 `port-range` 二选一 | `username`、`password`、`port-range`、`transport`、`multiplexing`、`handshake-mode`、`traffic-pattern` |
| [Sudoku](https://wiki.metacubex.one/config/proxies/sudoku/) `sudoku` | `key@` 或查询 `key` | `key`、`aead-method`、`table-type`、`custom-table`、数组 `custom-tables`、`multiplex`、`padding-min`、`padding-max`、`enable-pure-downlink`、对象 `httpmask` |
| [Hysteria](https://wiki.metacubex.one/config/proxies/hysteria/) `hysteria` | 可选 `auth-str@`；也可在查询使用 `auth/auth-str` | `auth`、`auth-str`、`obfs`、`protocol`、`up`、`down`、`ports`、`recv-window-conn`、`recv-window`、`disable_mtu_discovery`、`fast-open` |
| [Hysteria2](https://wiki.metacubex.one/config/proxies/hysteria2/) `hysteria2` | `password@` 或查询 `password` | `password`、`ports`、`hop-interval`、`up`、`down`、`bbr-profile`、`obfs`、`obfs-password`、对象 `realm-opts`、`obfs-min-packet-size`、`obfs-max-packet-size`、`handshake-timeout`、`initial-stream-receive-window`、`max-stream-receive-window`、`initial-connection-receive-window`、`max-connection-receive-window` |
| [TUIC](https://wiki.metacubex.one/config/proxies/tuic/) `tuic` | v4 使用 `token@`；v5 使用 `uuid:password@`，不能混用 | `token`、`uuid`、`password`、`ip`、`udp-relay-mode`、`congestion-controller`、`bbr-profile`、`heartbeat-interval`、`request-timeout`、`max-udp-relay-packet-size`、`max-open-streams`、`disable-sni`、`reduce-rtt`、`fast-open` |
| [ShadowQUIC](https://wiki.metacubex.one/config/proxies/shadowquic/) `shadowquic` | `username/password` | `username`、`password`、`congestion-controller`、`up`、`down`、`bbr-profile`、数组 `quic-versions`、`keep-alive-interval`、`cwnd`、`max-datagram-frame-size`、`max-open-streams`、`recv-window-conn`、`recv-window`、`udp-over-stream`、`zero-rtt`、`disable-mtu-discovery` |
| [WireGuard](https://wiki.metacubex.one/config/proxies/wg/) `wireguard` | 查询 `private-key`；单 peer 还需 `server/port/public-key`，多 peer 使用 `peers` | `private-key`、`public-key`、`pre-shared-key`、数组 `allowed-ips`、`reserved`、`persistent-keepalive`、`workers`、`refresh-server-ip-interval`、对象数组 `peers`、对象 `amnezia-wg-option`；支持下述 IP 栈字段 |
| [Tailscale](https://wiki.metacubex.one/config/proxies/tailscale/) `tailscale` | 无须 `server/port` | `hostname`、`auth-key`、`control-url`、`state-dir`、`exit-node`、`ephemeral`、`accept-routes`、`exit-node-allow-lan-access` |
| [SSH](https://wiki.metacubex.one/config/proxies/ssh/) `ssh` | `username`；密码或私钥认证 | `username`、`password`、`private-key`、`private-key-passphrase`、数组 `host-key/host-key-algorithms` |
| [MASQUE](https://wiki.metacubex.one/config/proxies/masque/) `masque` | `private-key/public-key` | `public-key`、`network`、`congestion-controller`、`bbr-profile`、`handshake-timeout`；支持 TLS 与 IP 栈字段 |
| [TrustTunnel](https://wiki.metacubex.one/config/proxies/trusttunnel/) `trusttunnel` | `username/password` | `username`、`password`、`congestion-controller`、`bbr-profile`、`health-check`、`quic`、`max-connections`、`min-streams`、`max-streams` |
| [ZeroTier](https://wiki.metacubex.one/config/proxies/zerotier/) `zerotier` | 查询 `network`，无须 `server/port` | `network`、`state-dir`、`planet`、`tcp-fallback-mode`、`tcp-fallback-relay`、`remote-trace-target`、`physical-mtu`、`primary-port`、`secondary-port`、`remote-trace-level`、`low-bandwidth`、`encrypted-hello`、对象数组 `orbit`；支持 IP 栈字段 |
| [OpenVPN](https://wiki.metacubex.one/config/proxies/openvpn/) `openvpn` | `ca`，以及 `username/password` 或 `cert/key` | `proto`、`username`、`password`、`ca`、`cert`、`key`、`tls-auth`、`tls-crypt`、`tls-crypt-v2`、`key-direction`、`dev`、`cipher`、数组 `data-ciphers`、`data-ciphers-fallback`、`auth`、`comp-lzo`、`ping`、`ping-restart`、`tran-window`、`handshake-timeout`、对象 `peer-info`；支持 IP 栈字段 |

另外支持 [Direct](https://wiki.metacubex.one/config/proxies/direct/) `direct`、[DNS](https://wiki.metacubex.one/config/proxies/dns/) `dns`、[Rematch](https://wiki.metacubex.one/config/proxies/rematch/) `rematch` 原生出站对象；不要求 `server/port`，Rematch 可填 `target-rematch-name`、`target-sub-rule`。它们可出现在普通 URI 订阅中，**链式代理导入会跳过这些本地出站**，因为它们不是远程落地节点。URI 只生成一个基础选择器，不会凭 Rematch 的引用名创建子规则或依赖目标；复杂路由推荐 YAML。

### TLS、传输与 IP 栈

TLS 字段使用 [mihomo TLS 文档](https://wiki.metacubex.one/config/proxies/tls/) 的语义：`tls`、`sni` / `servername`、`client-fingerprint`、`fingerprint`、`name-cert-verify`、`skip-cert-verify`、数组 `alpn`、PEM `certificate/private-key`，以及对象 `ech-opts`、`shadow-tls-opts`、`restls-opts`、`jls-opts`、`tlsmirror-opts`。

- HTTP、SOCKS5、VMess、VLESS 按需显式设置 `tls=true`；VMess/VLESS 标准服务器名称字段为 `servername`，其他 TLS 协议为 `sni`，两种拼写也可互作兼容别名。
- AnyTLS、Trojan、Hysteria/Hysteria2、TUIC、ShadowQUIC、MASQUE、TrustTunnel 固有 TLS，无须添加 `tls=true`，不能使用 `tls=false`。TLS ClientHello 指纹只适用于 VMess/VLESS/Trojan/AnyTLS（SS 的插件另有对应选项）；不要为其他协议机械复制不适用的字段。`tlsmirror-opts` 仅允许 VMess。
- `client-fingerprint` 是 TLS ClientHello 指纹；`fingerprint` 是**服务端叶证书 DER 的 SHA-256**，推荐 64 位十六进制。二者不同，也不能把 SPKI PIN 当作 `fingerprint`。
- VMess/VLESS/Trojan 可使用原生 `reality-opts` 对象，例如 `{"public-key":"...","short-id":"abcd"}`。公钥须解码为 32 字节，Short ID 为最多 8 字节的十六进制；需配置 TLS 名称，不能同时使用叶证书 PIN 或显式 `skip-cert-verify`。
- 不会为导入成功而自动开启跳过证书验证。客户端证书身份应使用标准 `certificate/private-key`；下面的 `cert://` 扩展另有用途。

`network` 和传输对象使用 [mihomo 传输配置](https://wiki.metacubex.one/config/proxies/transport/) 的名称：

| 协议 | `network` | 按选定传输填写的对象 |
| --- | --- | --- |
| VMess | `tcp/ws/http/h2/grpc/mkcp/mekya` | `ws-opts/http-opts/h2-opts/grpc-opts/mkcp-opts/mekya-opts` |
| VLESS | `tcp/ws/http/h2/grpc/xhttp` | `ws-opts/http-opts/h2-opts/grpc-opts/xhttp-opts` |
| Trojan | `tcp/ws/grpc` | `ws-opts/grpc-opts` |
| MASQUE | `h3/h2/h3-l4proxy` | 参照对应协议字段 |

VMess/VLESS/Trojan 默认 `network=tcp`。不要同时填另一个传输的对象。XHTTP 或非空且非 `none` 的 VLESS `encryption` 要求可识别的 mihomo **v1.19.30 或更新稳定版**；未知版本的手动导入核心不作为支持依据。

WireGuard、MASQUE、ZeroTier、OpenVPN 的 IP 栈字段为：`ip`、`ipv6`、`mtu`、`remote-dns-resolve`、数组 `dns`、对象 `ip-stack`。`reserved` 为三个 `0–255` 整数组成的 JSON 数组，也接受解码后三字节的 Base64 字符串。WireGuard 多 peer 的服务器、端口、公钥、路由与 reserved 放在各 peer 对象中，不能展开成重名顶层参数。

WireGuard 的 `workers` 与 `refresh-server-ip-interval` 也按[官方字段定义](https://github.com/MetaCubeX/mihomo/blob/Meta/adapter/outbound/wireguard.go)接受整数。下面的 JavaScript 展示如何从节点对象生成标准 URI，避免手工转义复杂字段：

```js
function toJeemiURI(node) {
  const { type, ...fields } = node;
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(fields)) {
    if (value === undefined) continue;
    query.set(key, typeof value === "object" ? JSON.stringify(value) : String(value));
  }
  return `${type}://?${query}`;
}

const uri = toJeemiURI({
  type: "vless",
  name: "HK Example",
  server: "node.entry.example.invalid",
  port: 443,
  uuid: "11111111-2222-4333-8444-555555555555",
  tls: true,
  servername: "tls.example.invalid",
  network: "ws",
  "ws-opts": { path: "/ws", headers: { Host: "cdn.example.invalid" } },
  udp: true,
});
```

此函数只负责编码；输入对象仍须满足对应协议的字段与核心要求。它不会生成选择器、规则或 DNS 元数据。

<a id="node-dns"></a>

## 自定义节点 DNS

`host://` 指定“使用哪个 DNS 服务器解析这个域名”。它是一项订阅元数据，不计入代理节点数量，也不是把域名固定映射到某个 IP 的系统 hosts 记录。

推荐格式：

```text
host://<域名或通配模式>?server=<经过百分号编码的DNS服务器地址>
```

例如，下面两行让 `node.entry.example.invalid` 通过指定的 DoH 服务器解析，再使用解析结果连接 AnyTLS 节点：

```text
host://node.entry.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2Fdns-query
anytls://example-password@node.entry.example.invalid:443?sni=tls.example.invalid#Custom-DNS
```

这里需要匹配的是节点 URI 中 `@` 后面的服务器域名 `node.entry.example.invalid`。`sni=tls.example.invalid` 用于 TLS 握手，不是本例要解析的节点地址。节点地址已经是 IP 时，无需对它进行域名解析。

### 域名匹配范围

| 写法 | 匹配范围 |
| --- | --- |
| `entry.example.invalid` | 只匹配这个域名 |
| `*.entry.example.invalid` | 只匹配一级子域名，如 `a.entry.example.invalid`；不含根域名或 `a.b.entry.example.invalid` |
| `.entry.example.invalid` | 匹配各级子域名，不含根域名 |
| `+.entry.example.invalid` | 匹配根域名及各级子域名 |

域名会规范化大小写及国际化域名形式。这些匹配方式对应 [mihomo 域名通配语法](https://wiki.metacubex.one/handbook/syntax/)；推荐需要精确指定单个节点时填写完整域名。

### DNS 地址与多个解析器

`server` 支持以下形式。表中地址是编码前的值，放入 `host://` 查询参数时需按前述规则编码。

| DNS 类型 | 编码前示例 |
| --- | --- |
| UDP，默认端口 | `192.0.2.53`、`2001:db8::53` |
| UDP，指定端口 | `192.0.2.53:5353`、`[2001:db8::53]:5353`、`udp://192.0.2.53:5353` |
| TCP | `tcp://192.0.2.53:53` |
| DoH | `https://dns.example.invalid/dns-query` |
| DoT | `tls://dns.example.invalid:853` |
| DoQ | `quic://dns.example.invalid:853` |
| HTTP/3 DoH | `h3://dns.example.invalid/dns-query` |
| 系统 DNS | `system`，兼容 `system://` |

同一域名可以重复 `server` 参数，也可以写成多条 `host://` 行。Jeemi 按出现顺序合并并去重，不把它们解释成主备优先级；具体查询由 mihomo 执行。例如：

```text
host://node.entry.example.invalid?server=192.0.2.53&server=https%3A%2F%2Fdns.example.invalid%2Fdns-query
```

DoH 自身的路径和查询参数应整体放入编码后的 `server` 值。例如 `https://dns.example.invalid/Resolve?profile=AbC&region=HK` 写成：

```text
host://node.entry.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2FResolve%3Fprofile%3DAbC%26region%3DHK
```

`host://` 只接受 `server` 查询参数，不接受用户名、密码、路径或片段。DNS 地址本身也不接受用户名、密码及 `#` 附加参数；HTTP/3 使用上表的 `h3://` 写法，由 Jeemi 转换。裸域名不能单独作为 DNS 地址，应填写 IP 或带协议的地址。

### 在 mihomo 中的含义

上述单个 DoH 示例会生成以下 DNS 配置片段：

```yaml
dns:
  nameserver-policy:
    node.entry.example.invalid:
      - https://dns.example.invalid/dns-query
  proxy-server-nameserver:
    - system
  proxy-server-nameserver-policy:
    node.entry.example.invalid:
      - https://dns.example.invalid/dns-query
```

Jeemi 同时生成普通域名策略和节点域名策略，并为 URI 订阅补充 `proxy-server-nameserver: [system]`，使节点策略能够生效，也为不匹配策略的节点提供默认解析器。这与 [mihomo 的节点 DNS 配置要求](https://wiki.metacubex.one/config/dns/#proxy-server-nameserver-policy) 一致。该片段是订阅转换结果；主页 DNS 设置和关联的本地脚本等后续处理仍可能改变最终值。

私有节点 DoH 不会自动成为所有网站的默认 DNS。若 DoH 地址本身使用域名，该域名也需要能够通过已有的引导 DNS 配置解析；`host://` 不会额外完成 DNS 服务器域名的引导配置。需要设置 `default-nameserver` 等完整 DNS 选项时，优先使用 mihomo YAML 订阅或主页 DNS 设置。

## 可选证书元数据

```text
cert://<域名或通配模式>#<PEM证书内容的无填充Base64URL>
```

第一张必须是服务器叶证书，后面可附 PEM 证书链；不接受以 CA 证书为首的材料或私钥。元数据按节点 TLS 名称匹配，未填名称时使用服务器域名，并检查证书覆盖范围。Jeemi 计算叶证书 DER 的 SHA-256 写入 `fingerprint`，与已有 PIN 冲突时拒绝本次订阅。REALITY 不使用这项元数据。

此扩展不安装系统根证书，也不把服务器证书当作 mTLS 客户端身份。单项解码后的证书材料最多 256 KiB。

## 已支持的兼容写法

兼容字段按语义转换，只适用于支持它们的协议。推荐左列的标准写法；不要在新订阅中混用多个别名。

| 标准写法 | 已支持的兼容写法 |
| --- | --- |
| `ss` / `ssr` / `socks5` / `hysteria2` / `wireguard` | `shadowsocks` / `shadowsocksr` / `socks` 或 `s5` / `hy2` / `wg` |
| `http` + `tls=true` | `https://` |
| `sni` 或 VMess/VLESS 的 `servername` | `sni`、`servername`、`peer` |
| `client-fingerprint` | `fp` |
| `fingerprint` | `server-cert-fingerprint-sha256`、`pcs`；兼容冒号分隔摘要与 `sha256:` 前缀 |
| `skip-cert-verify` | `allowInsecure`、`insecure`、`allow_insecure` |
| `udp` | `udp-relay` |
| `alpn` JSON 数组 | 逗号列表，如 `h2,http/1.1`，放入 URI 时仍需编码 |
| `idle-session-check-interval` / `idle-session-timeout` / `min-idle-session` | AnyTLS 的 `idle_session_check_interval` / `idle_session_timeout` / `min_idle_session`；两种时间字段拼写也兼容 `30s` |
| `congestion-controller` / `udp-relay-mode` / `reduce-rtt` | TUIC 的 `congestion_control` / `udp_relay_mode` / `zero_rtt_handshake` |
| `heartbeat-interval=10000` | TUIC 的 `heartbeat=10s`，按毫秒转换；正值且不超过 24 小时 |
| `tls=true/false`、`reality-opts` | `security=tls/none/reality`；REALITY 可配 `pbk` / `public-key`、`sid` / `short-id` |
| `network` | 旧 `type=tcp/ws/grpc/xhttp/...` |
| 对象 `ws-opts/xhttp-opts/http-opts/h2-opts` | 按传输映射 `path`、`host`；XHTTP 的 `mode` |
| `grpc-opts.grpc-service-name` 所在的完整 JSON 对象 | `serviceName`、`service-name` |
| `packet-encoding` | `packetEncoding` |
| Snell 的 `obfs-opts` 对象 | `obfs` / `obfs-mode`、`obfs_host` / `obfs-host` |

AnyTLS 兼容其他客户端导出的 `type=tcp` / `network=tcp` 和 `security=tls`，不将它们写成虚假的 AnyTLS 传输字段；`type=ws`、`security=none/reality` 会跳过该节点。`tfo=0` 会保留为 `tfo: false`。例如：

```text
anytls://example-password@192.0.2.10:443?type=tcp&fp=firefox&security=tls&sni=tls.example.invalid&udp=1&tfo=0#HK%20Example%2004
```

还支持以下协议内部编码：

```text
ss://<Base64(cipher:password)>@<server>:<port>#<name>
ss://<Base64(cipher:password@server:port)>#<name>
vmess://<Base64(JSON对象)>
ssr://<Base64(server:port:protocol:cipher:obfs:Base64(password)/?参数)>
```

- SS 密码中的冒号（含 SS2022 密码）作为完整密码保留，按 URI 组件编码。插件推荐用标准 `plugin` 和 JSON `plugin-opts`；其他客户端的分号打包插件串不在兼容范围内。
- VMess JSON 兼容 `v/ps/add/port/id/aid/scy/net/type/tls/sni/fp/alpn/path/host`，`type` 只接受空值或 `none`；无法等价转换的私有字段会跳过该节点。
- SSR 兼容 Base64 查询值 `obfsparam/protoparam/remarks/group`，其中 `group` 是来源展示信息，不生成 mihomo 选择器。
- 未定义的 `pinSHA256`、Snell 独立 `userkey` / v6 等不猜测、不降级。未知字段不能依靠忽略后强行导入。
- 部分 Surge 节点和可识别 DNS 继续兼容；Surge 的原选择器、路由、脚本、MITM、系统设置不迁移为同等语义。完整配置请使用 YAML。

## 混合订阅、跳过规则与更新行为

URI 列表按行解析。未知协议、未知节点顶层字段、错误端口/凭据/值/转义、冲突别名、不能等价转换的节点组合，会**跳过该节点并记录行号与原因**；其他有效节点继续转换。首行是坏节点、或整份列表使用 Base64 时也适用。

转换结果包含可用节点、一个 `Proxy` 手动选择器、`MATCH,Proxy` 和有效 DNS 元数据，再经过关联的本地配置/脚本及主页运行参数。原始订阅的全部字节仍保持不变，后续解析器升级可以重新解析原来被跳过的节点。

以下仍拒绝本次导入/更新：整份文本编码或总大小不合法、共享 `host://` / `cert://` 元数据无效或冲突、没有任何可用节点，或最终组合配置未通过校验。失败保留上一份有效订阅和运行配置。不能通过忽略公共 DNS/证书错误来生成行为不同的订阅。

字段目录描述 Jeemi 能转换的协议配置；所选 mihomo 版本、凭据有效性、节点服务以及链式代理中继的传输能力仍影响运行结果。未知顶层字段会被诊断，已识别嵌套对象的具体子字段需按官方文档提供并由所选核心继续校验，不能把解析成功等同于节点连通或所有字段已由核心执行。

当前实现见 [订阅格式解析器](client/internal/subscriptionformat/)、[标准字段测试](client/internal/subscriptionformat/native_uri_test.go)。返回 [README](README.md#订阅格式)。
