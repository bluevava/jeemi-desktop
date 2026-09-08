# 通用 URI 订阅结构解析说明

**优先推荐 mihomo 原生 YAML 订阅**，完整配置结构参考 [mihomo 官方文档](https://wiki.metacubex.one/config/)。需要提供 URI 订阅时，建议使用本文的统一写法。

本文描述 Jeemi 当前解析器支持的格式。“推荐结构”是 Jeemi 的订阅生成约定；其中 `host://`、`cert://` 是 Jeemi 支持的扩展，不代表所有客户端都会识别。其他客户端的已支持方言仍可导入，但不推荐新订阅依赖这些兼容写法。

**通用 URI 订阅支持自定义节点 DNS**，可用独立的 `host://` 行指定代理节点域名的解析服务器，详见下方 [自定义节点 DNS](#node-dns)。

## 推荐的订阅整体结构

订阅正文使用 UTF-8 文本，每行一项：先列出可选的 DNS、证书元数据，再列出节点。元数据也可以放在节点之后；解析结果与它们在节点前后的顺序无关。

下面是一份包含自定义节点 DNS 和三种节点的完整结构示例。所有域名、密码和 UUID 均为演示值，使用时替换为自己的服务信息。

```text
# 为 entry.example.invalid 下的一级子域名指定节点 DNS
host://*.entry.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2Fdns-query

# 节点列表
anytls://example-password@at.entry.example.invalid:443?sni=at.entry.example.invalid&udp=true#AnyTLS
tuic://11111111-2222-4333-8444-555555555555:example-password@tuic.entry.example.invalid:443?sni=tuic.entry.example.invalid&congestion_control=bbr&udp_relay_mode=native&heartbeat=10s#TUIC
vless://11111111-2222-4333-8444-555555555555@vl.entry.example.invalid:443?security=tls&type=ws&path=%2Fws&sni=vl.entry.example.invalid&encryption=none#VLESS
```

将正文保存为 `.txt` 文件后使用文件导入，或通过 HTTP(S) 订阅地址返回该正文。客户端的 URL 和二维码导入入口接收 HTTP(S) 订阅地址；单条节点 URI 应放入订阅正文或文件。

### 文本与编码

- 推荐明文、LF 换行；也接受 CRLF、空行和以 `#` 开头的整行注释。节点 URI 中的 `#` 表示节点名称，不要在行尾再添加注释。
- 支持将**整份 URI 列表编码一次 Base64**；标准 Base64、Base64URL 及各自带或不带 `=` 填充的形式均可识别。不支持对整份订阅递归套多层 Base64。
- 节点的一般形式为 `协议://凭据@服务器:端口?参数=值#名称`。端口必须显式填写，范围为 `1–65535`；IPv6 服务器使用方括号，例如 `[2001:db8::1]:443`。
- 按 URI 组件分别进行百分号编码：凭据、查询参数值和名称中的特殊字符应编码，结构分隔符保持原样。例如密码 `p@ss:word+1` 写成 `p%40ss%3Aword%2B1`，名称 `HK 01` 写成 `HK%2001`。不要把整条节点 URI 一起编码。
- 查询参数只解码一层。查询值中的字面量 `+` 必须写成 `%2B`；DoH 地址、WS 路径等嵌套值要完整编码一次，保留原有大小写。
- 参数名区分大小写，使用本文给出的拼写；布尔值推荐 `true` / `false`，兼容 `1` / `0`。可选参数不用时直接省略，避免附加其他客户端的私有参数。
- 正文最多 4 MiB、20,000 行，普通节点 URI 单行最多 32 KiB。推荐为每个节点填写不同名称；未命名节点会自动命名，名称冲突会添加编号，名称与配置均相同的重复节点会去重。

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

## 当前支持的节点协议

以下示例均使用虚构凭据。URI 中只填写对应协议支持的参数；mihomo 原生 YAML 支持的字段和协议范围更广，不能据此推断 URI 解析器也全部支持。

### Shadowsocks（SS）

```text
ss://aes-128-gcm:example-password@ss.entry.example.invalid:8388?udp=true#SS
```

凭据格式为 `加密方法:密码`，分别编码这两部分；`udp` 可选，默认 `true`。SS2022 的密码即使含冒号，也会作为完整密码保留，写入 URI 时将密码中的冒号编码为 `%3A`。加密方法和密钥是否合法，最终由所选 mihomo 核心校验。

兼容 Base64 编码的 `加密方法:密码` 用户信息，以及整体 Base64 编码的旧式 SS 链接，详见后文兼容说明。当前 URI 解析不转换 SS 插件参数。

### SOCKS5

```text
socks5://example-user:example-password@socks.entry.example.invalid:1080?udp=true#SOCKS5
```

当前 URI 形式要求用户名和密码均非空，支持可选的 `udp` 参数。无认证或需要更多 SOCKS 配置时使用 mihomo YAML。

### AnyTLS

```text
anytls://example-password@at.entry.example.invalid:443?sni=at.entry.example.invalid&fp=chrome&alpn=h2%2Chttp%2F1.1&idle_session_check_interval=30s&idle_session_timeout=30s&min_idle_session=0&udp=true#AnyTLS
```

凭据部分只有密码；TLS 固有启用，`udp` 默认 `true`。支持下表中的 TLS 公共参数，以及：

| 推荐参数 | 含义 |
| --- | --- |
| `idle_session_check_interval` | 空闲会话检查间隔，如 `30s`；也接受整数秒 |
| `idle_session_timeout` | 空闲会话超时，如 `30s`；也接受整数秒 |
| `min_idle_session` | 最少空闲会话数，非负整数 |

两个时间参数必须为正整数秒，最多 24 小时。

### TUIC

```text
tuic://11111111-2222-4333-8444-555555555555:example-password@tuic.entry.example.invalid:443?sni=tuic.entry.example.invalid&alpn=h3&congestion_control=bbr&udp_relay_mode=native&zero_rtt_handshake=false&heartbeat=10s#TUIC
```

当前接受 UUID 与密码形式的 TUIC v5 节点，TLS 固有启用。支持 TLS 公共参数中的 `sni`、`alpn`、证书 PIN 和 `skip-cert-verify`，**不支持 `fp` / `client-fingerprint`**。

| 推荐参数 | 可选值或单位 |
| --- | --- |
| `congestion_control` | `cubic`、`new_reno`、`bbr` |
| `udp_relay_mode` | `native`、`quic` |
| `zero_rtt_handshake` | `true` / `false` |
| `heartbeat` | 带单位的时间，如 `10s`、`500ms`，转换为整数毫秒；范围为大于零且不超过 24 小时 |

TUIC 的 UDP 能力不通过 `udp=false` 关闭；省略该参数即可，兼容显式 `udp=true`。如填写 `security`，只接受 `tls`。

### VLESS

```text
vless://11111111-2222-4333-8444-555555555555@vl.entry.example.invalid:443?security=tls&type=ws&path=%2Fws&host=cdn.example.invalid&sni=tls.example.invalid&fp=chrome&encryption=none&packetEncoding=xudp#VLESS-WS
```

凭据部分只有 UUID，`udp` 默认 `true`。`type` 指传输方式，节点协议始终是 VLESS。

| 推荐参数 | 含义与限制 |
| --- | --- |
| `security` | `none`、`tls`、`reality`；省略时不启用 TLS |
| `type` | `tcp`、`ws`、`grpc`、`xhttp`；默认 `tcp` |
| `path`、`host` | 用于 WS / XHTTP；非空 `path` 必须以 `/` 开头并作为查询值编码 |
| `serviceName` | gRPC 必填；gRPC 不接受 `path`、`host`、`mode` |
| `mode` | 仅 XHTTP 支持：`auto`、`packet-up`、`stream-up`、`stream-one` |
| `flow` | 可选 `xtls-rprx-vision`，能否与其他参数组合由所选核心校验 |
| `encryption` | 普通节点推荐 `none`；非空且非 `none` 的值需要匹配的核心能力 |
| `packetEncoding` | `xudp`、`packetaddr`、`none` |
| `pbk`、`sid` | REALITY 公钥和 Short ID；公钥须解码为 32 字节，Short ID 为最多 8 字节的十六进制值 |

TLS 公共参数用于启用 TLS 的节点。REALITY 需要 `sni` 和 `pbk`；`sid` 可留空或省略，是否接受空值取决于服务器。REALITY 不能同时使用证书 PIN 或 `skip-cert-verify`，即使后者为 `false` 也应省略。TCP 不接受 WS / gRPC / XHTTP 的路径、Host、服务名和模式参数。

**XHTTP 或非空且非 `none` 的 `encryption` 要求 mihomo v1.19.30 或更新的可识别正式版本**；无法识别版本的手动导入核心不作为支持这些字段的依据。具体协议组合仍需通过目标核心校验。

### Snell

```text
snell://example-psk@snell.entry.example.invalid:443?version=4&udp=true#Snell
```

凭据部分为 PSK，推荐显式填写 `version`。当前 URI 转换接受版本 `1–5`，省略时为 `1`；实际版本支持由所选 mihomo 核心决定。可选参数为 `udp`、`obfs`（`none` / `http` / `tls`）、`obfs_host`。版本 1 / 2 不接受 `udp=true`。

兼容在查询参数中再次填写 `psk`，但必须与凭据部分一致；推荐只在凭据部分填写。当前不支持独立 `userkey`、非空 `mode` 或 Snell v6，不会把这些配置降级为其他版本。

## TLS 公共参数

仅适用于上文明确支持 TLS 参数的协议。

| 推荐参数 | 作用 |
| --- | --- |
| `sni` | TLS 服务器名称；不替换节点 URI 的连接地址 |
| `alpn` | 逗号分隔的协议列表，如编码后的 `h2%2Chttp%2F1.1` |
| `fp` | TLS ClientHello 指纹，如 `chrome`；AnyTLS / VLESS 支持，TUIC 不支持 |
| `fingerprint` | **服务端叶证书 DER 的 SHA-256**，推荐 64 位十六进制；不是 ClientHello 指纹或公钥 SPKI PIN |
| `skip-cert-verify` | 是否跳过证书验证；通常省略或填写 `false`，不要为了导入而自动设为 `true` |

`fingerprint` 兼容冒号分隔的十六进制和 `sha256:` 前缀。未知语义的 `pinSHA256` 等字段不会被当作证书摘要转换。

### 可选证书元数据

如需随订阅携带用于叶证书 PIN 的证书材料，支持：

```text
cert://<域名或通配模式>#<PEM证书内容的无填充Base64URL>
```

第一张证书必须为服务器叶证书，后面可以附带 PEM 证书链；不接受以 CA 证书为首的材料或私钥。元数据按 TLS `sni` 匹配节点，未填写 `sni` 时使用节点服务器名，并检查叶证书是否覆盖该名称。Jeemi 将叶证书 DER 的 SHA-256 写入节点 `fingerprint`；与节点已有 PIN 冲突时拒绝该订阅。REALITY 不应用这项证书元数据。

这项扩展不会安装系统根证书，也不表示支持客户端证书认证（mTLS）。单项解码后的证书材料最多 256 KiB。

## 已支持的兼容写法

这些形式用于兼容已有订阅。新订阅推荐统一使用上文参数名，同一个含义只提供一个参数。别名只在支持该字段的协议中有效，不是全局通用参数。

| 推荐写法 | 已支持的别名 |
| --- | --- |
| `ss://` | `shadowsocks://` |
| `socks5://` | `socks://`、`s5://` |
| `sni` | `servername`、`peer` |
| `fingerprint` | `server-cert-fingerprint-sha256`、`pcs`，均按叶证书 SHA-256 解释 |
| `fp` | `client-fingerprint` |
| `skip-cert-verify` | `allowInsecure`、`insecure`、`allow_insecure` |
| `udp` | `udp-relay` |
| `idle_session_check_interval` | `idle-session-check-interval` |
| `idle_session_timeout` | `idle-session-timeout` |
| `min_idle_session` | `min-idle-session` |
| `congestion_control` | `congestion-controller` |
| `udp_relay_mode` | `udp-relay-mode` |
| `zero_rtt_handshake` | `reduce-rtt` |
| `type` | `network` |
| `serviceName` | `service-name` |
| `packetEncoding` | `packet-encoding` |
| `pbk`、`sid` | `public-key`、`short-id` |
| `obfs`、`obfs_host` | `obfs-mode`、`obfs-host` |

SS 额外兼容以下两种形式，其中 Base64 支持标准与 URL 安全字母表、带或不带填充：

```text
ss://<Base64(cipher:password)>@<server>:<port>#<name>
ss://<Base64(cipher:password@server:port)>#<name>
```

如果同一字段的多个别名同时存在，规范化后的值必须一致；冲突会导致订阅解析失败。证书 PIN 与 ClientHello 指纹属于不同字段，不能互相替代。

此外，Jeemi 兼容解析部分 Surge 配置中的节点和可识别 DNS，原有策略组、路由规则、脚本、MITM 与系统设置不会按 Surge 语义迁移。需要完整配置时推荐 mihomo YAML，不建议为 Jeemi 专门生成其他客户端格式。

## 转换结果与支持边界

URI 订阅会转换为 mihomo 节点列表、一个包含可用节点的 `Proxy` 手动选择器和 `MATCH,Proxy` 兜底规则，并合入可识别的 DNS 元数据。后续再应用关联的本地配置或脚本及主页运行参数，原始订阅正文保持不变。

- 当前 URI 协议范围为 **SS、SOCKS5、AnyTLS、TUIC、VLESS、Snell**。例如 `vmess://`、`trojan://`、`hysteria2://` 当前不在此解析范围内；需要这些节点时使用目标 mihomo 核心支持的原生 YAML。
- 不支持的协议、节点私有字段或协议组合会跳过对应节点并显示诊断，其他有效节点可以保留。不能依靠附加一个未知参数后仍按原节点导入。
- 已知协议的凭据、端口、参数值无效，别名冲突，或 DNS / 证书元数据无效时，整份订阅解析失败；没有可用节点也会失败。更新失败会保留上一份有效订阅。
- URI 转换成功不等于节点已经连通。最终配置仍须满足所选 mihomo 版本的协议与配置要求。
- 部分客户端的导出格式会省略 DNS 或证书元数据；Jeemi 不能从缺失信息的正文恢复这些设置。需要自定义节点 DNS 时，应保留本文的 `host://` 行，或直接提供完整 mihomo YAML。

当前实现可参阅公开源码中的 [订阅格式解析器](client/internal/subscriptionformat/) 和 [解析测试](client/internal/subscriptionformat/normalize_test.go)。返回 [README](README.md#订阅格式)。
