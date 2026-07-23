import type { ListTerminalsResponse } from "@server/shared/messages";
import type { CheckoutStatusPayload } from "@/hooks/use-checkout-status-query";
import {
  resolveWorkspaceHeaderRenderState,
  type WorkspaceHeaderCheckoutState,
} from "@/screens/workspace/workspace-header-source";
import type { WorkspaceDescriptor } from "@/stores/session-store";
import type { WorkspaceExecutionAuthorityResult } from "@/utils/workspace-execution";

export function trimNonEmpty(value: string | null | undefined): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

export function decodeSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export type ListTerminalsPayload = ListTerminalsResponse["payload"];

export type PaneDirection = "left" | "right" | "up" | "down";

export function parsePaneDirection(actionId: string): PaneDirection | null {
  const direction = actionId.split(".").pop();
  if (direction === "left" || direction === "right" || direction === "up" || direction === "down") {
    return direction;
  }
  return null;
}

export interface WorkspaceHeaderFields {
  isWorkspaceHeaderLoading: boolean;
  workspaceHeaderTitle: string;
  workspaceHeaderSubtitle: string;
  shouldShowWorkspaceHeaderSubtitle: boolean;
  isGitCheckout: boolean;
  currentBranchName: string | null;
}

export function buildWorkspaceHeaderCheckoutState(input: {
  isCheckoutStatusLoading: boolean;
  isError: boolean;
  data: CheckoutStatusPayload | undefined;
}): WorkspaceHeaderCheckoutState {
  if (input.isCheckoutStatusLoading) {
    return { kind: "pending" };
  }
  if (input.isError || !input.data) {
    return { kind: "error" };
  }
  return {
    kind: "ready",
    checkout: {
      isGit: input.data.isGit,
      currentBranch: input.data.currentBranch,
    },
  };
}

export function deriveWorkspaceHeaderFields(input: {
  workspace: WorkspaceDescriptor | null;
  checkoutState: WorkspaceHeaderCheckoutState;
}): WorkspaceHeaderFields {
  const renderState = resolveWorkspaceHeaderRenderState(input);
  if (renderState.kind !== "ready") {
    return {
      isWorkspaceHeaderLoading: true,
      workspaceHeaderTitle: "",
      workspaceHeaderSubtitle: "",
      shouldShowWorkspaceHeaderSubtitle: false,
      isGitCheckout: false,
      currentBranchName: null,
    };
  }
  return {
    isWorkspaceHeaderLoading: false,
    workspaceHeaderTitle: renderState.title,
    workspaceHeaderSubtitle: renderState.subtitle,
    shouldShowWorkspaceHeaderSubtitle: renderState.shouldShowSubtitle,
    isGitCheckout: renderState.isGitCheckout,
    currentBranchName: renderState.currentBranchName,
  };
}

export interface WorkspaceAuthorityState {
  workspaceDirectory: string | null;
  isMissingWorkspaceExecutionAuthority: boolean;
}

export function resolveWorkspaceAuthorityState(
  workspaceAuthority: WorkspaceExecutionAuthorityResult,
  workspaceDescriptor: WorkspaceDescriptor | null | undefined,
): WorkspaceAuthorityState {
  const authority = workspaceAuthority.ok ? workspaceAuthority.authority : null;
  return {
    workspaceDirectory: authority?.workspaceDirectory ?? null,
    isMissingWorkspaceExecutionAuthority: Boolean(workspaceDescriptor && !authority),
  };
}

export function reconcilePendingScriptTerminals(liveTerminalIds: string[], dataUpdatedAt: number) {
  return function update(pendingTerminalIds: Map<string, number>): Map<string, number> {
    if (pendingTerminalIds.size === 0) {
      return pendingTerminalIds;
    }
    const liveIds = new Set(liveTerminalIds);
    let changed = false;
    const nextTerminalIds = new Map<string, number>();
    for (const [terminalId, listedAt] of pendingTerminalIds) {
      if (liveIds.has(terminalId) || dataUpdatedAt > listedAt) {
        changed = true;
        continue;
      }
      nextTerminalIds.set(terminalId, listedAt);
    }
    return changed ? nextTerminalIds : pendingTerminalIds;
  };
}

export function removeTerminalFromPayload(terminalId: string) {
  return function updatePayload(
    current: ListTerminalsPayload | undefined,
  ): ListTerminalsPayload | undefined {
    if (!current) {
      return current;
    }
    return {
      ...current,
      terminals: current.terminals.filter((terminal) => terminal.id !== terminalId),
    };
  };
}
