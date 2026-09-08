// Addresses from wateray2's DNS page, limited to its UDP and HTTPS presets.
// Plain IP entries use the DNS tool's existing TCP transport on port 53.
export const dnsServerPresets = [
  "8.8.8.8",
  "223.5.5.5",
  "223.6.6.6",
  "119.29.29.29",
  "114.114.114.114",
  "180.76.76.76",
  "101.226.4.6",
  "9.9.9.9",
  "94.140.14.14",
  "1.1.1.1",
  "https://dns.google/dns-query",
  "https://dns.alidns.com/dns-query",
  "https://cloudflare-dns.com/dns-query",
  "https://doh.pub/dns-query",
  "https://doh.114dns.com/dns-query",
  "https://mirror2.pcloud.baidu.com/dns-query",
  "https://doh.360.cn/dns-query",
  "https://dns.quad9.net/dns-query",
  "https://dns.adguard.com/dns-query",
  "https://common.dot.dns.yandex.net/dns-query",
].map((value) => ({ value }));
