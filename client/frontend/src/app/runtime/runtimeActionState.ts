import type { CoreState, MihomoRuntimeState } from "../../types/runtime";

export type RuntimeActionKind = "start" | "restart" | "stop";
export type RuntimePrimaryAction = "start" | "restart";

export function runtimePrimaryAction(
  state: MihomoRuntimeState,
  desiredRunning: boolean,
): RuntimePrimaryAction {
  return state === "running" || (state === "failed" && desiredRunning)
    ? "restart"
    : "start";
}

export function runtimeActionDisplayState(
  coreState: CoreState,
  actionKind: RuntimeActionKind | null,
): CoreState {
  if (actionKind === "stop") return "stopping";
  if (actionKind === "start" || actionKind === "restart") return "starting";
  return coreState;
}

export function runtimeActionReachedTarget(
  actionKind: RuntimeActionKind,
  state: MihomoRuntimeState,
): boolean {
  if (actionKind === "stop") return state === "stopped";
  return state === "running";
}
