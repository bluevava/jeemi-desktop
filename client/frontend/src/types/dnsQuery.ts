export interface DNSQueryPreferences {
  customDNS: string;
}

export interface DNSQueryRequest {
  id: string;
  domain: string;
  proxyDNS: string;
  directDNS: string;
  customDNS: string;
  customProxy: boolean;
}

export interface DNSFamilyResult {
  type: "A" | "AAAA";
  addresses: string[];
  error: string;
}

export interface DNSQueryResult {
  server: string;
  route: "proxy" | "direct";
  selector: string;
  node: string;
  status: "success" | "partial" | "error";
  error: string;
  durationMS: number;
  records: DNSFamilyResult[];
}

export interface DNSQueryResponse {
  domain: string;
  queriedAt: string;
  proxy: DNSQueryResult;
  direct: DNSQueryResult;
  custom: DNSQueryResult;
}
