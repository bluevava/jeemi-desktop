import type { SubscriptionNormalizationReport } from "./subscription";

export interface ChainProxyNode { id: string; name: string; type: string; sourceId: string }
export interface ChainProxySource {
  id: string; label: string; filter: string; updatedAt: string; nodeCount: number;
  report: SubscriptionNormalizationReport;
}
export interface ChainProxyGroup {
  id: string; name: string; kind: "manual" | "subscription";
  selectorFilter: string; nodeFilter: string;
  nodes: ChainProxyNode[]; sources: ChainProxySource[];
}
export interface ChainProxyState { revision: number; groups: ChainProxyGroup[] }
export interface ChainProxyGroupInput {
  revision: number; id: string; name: string; kind: "manual" | "subscription";
  selectorFilter: string; nodeFilter: string;
}
export interface ChainProxyImportInput { revision: number; groupId: string; nodeId: string; contents: string }
export interface ChainProxySourceInput { revision: number; groupId: string; id: string; url: string; filter: string }
export interface ChainProxyFailure { groupId: string; sourceId: string; code: string }
export interface ChainProxyResult {
  state: ChainProxyState; report?: SubscriptionNormalizationReport; failures: ChainProxyFailure[];
}
export interface ChainProxyComposition {
  fingerprint: string; generated: number;
  diagnostics: { groupId: string; selector: string; code: string; count: number }[];
}
