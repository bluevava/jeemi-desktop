import type { ChainProxyGroupInput, ChainProxyImportInput, ChainProxySourceInput, ChainProxyState, ChainProxyResult } from "../types/chainProxy";
import type { SubscriptionState } from "../types/subscription";

export interface ChainProxyAPI {
  GetChainProxyState(): Promise<ChainProxyState>;
  SaveChainProxyGroup(input: ChainProxyGroupInput): Promise<ChainProxyResult>;
  ImportChainProxyNodes(input: ChainProxyImportInput): Promise<ChainProxyResult>;
  SaveChainProxySource(input: ChainProxySourceInput): Promise<ChainProxyResult>;
  RefreshChainProxySources(groupId: string, revision: number): Promise<ChainProxyResult>;
  CancelChainProxyRefresh(): Promise<void>;
  DeleteChainProxyItem(groupId: string, kind: string, id: string, revision: number): Promise<ChainProxyResult>;
  GetChainProxyNodeText(groupId: string, nodeId: string): Promise<string>;
  GetChainProxySource(groupId: string, sourceId: string): Promise<ChainProxySourceInput>;
  SetSubscriptionChainProxyGroups(id: string, groupIds: string[], revision: number, libraryRevision: number): Promise<SubscriptionState>;
}

function bridge(): ChainProxyAPI {
  const api = window.go?.desktop?.App;
  if (!api) throw new Error("chain_proxy:desktop_unavailable");
  return api;
}
export const getChainProxyState = () => bridge().GetChainProxyState();
export const saveChainProxyGroup = (input: ChainProxyGroupInput) => bridge().SaveChainProxyGroup(input);
export const importChainProxyNodes = (input: ChainProxyImportInput) => bridge().ImportChainProxyNodes(input);
export const saveChainProxySource = (input: ChainProxySourceInput) => bridge().SaveChainProxySource(input);
export const refreshChainProxySources = (groupId: string, revision: number) => bridge().RefreshChainProxySources(groupId, revision);
export const cancelChainProxyRefresh = () => bridge().CancelChainProxyRefresh();
export const deleteChainProxyItem = (groupId: string, kind: string, id: string, revision: number) => bridge().DeleteChainProxyItem(groupId, kind, id, revision);
export const getChainProxyNodeText = (groupId: string, nodeId: string) => bridge().GetChainProxyNodeText(groupId, nodeId);
export const getChainProxySource = (groupId: string, sourceId: string) => bridge().GetChainProxySource(groupId, sourceId);
export const setSubscriptionChainProxyGroups = (id: string, groupIds: string[], revision: number, libraryRevision: number) => bridge().SetSubscriptionChainProxyGroups(id, groupIds, revision, libraryRevision);
