import type { StreamItem } from "@/types/stream";
import type {
  ProcessTimelineResponseOutput,
  TimelineReducerSideEffect,
} from "@/contexts/session-stream-reducers";
import type { Agent, MessageEntry } from "@/stores/session-store";
import { useSessionStore } from "@/stores/session-store";
import type { SessionOutboundMessage } from "@server/shared/messages";
import { resolveInitDeferred, rejectInitDeferred } from "@/utils/agent-initialization";

// ---------------------------------------------------------------------------
// Type aliases
// ---------------------------------------------------------------------------

export type AgentUpdatePayload = Extract<
  SessionOutboundMessage,
  { type: "agent_update" }
>["payload"];

export type WorkspaceSetupProgressPayload = Extract<
  SessionOutboundMessage,
  { type: "workspace_setup_progress" }
>["payload"];

type SessionStoreActions = ReturnType<typeof useSessionStore.getState>;
export type SetInitializingAgents = SessionStoreActions["setInitializingAgents"];
export type SetAgentStreamTail = SessionStoreActions["setAgentStreamTail"];
export type SetAgentStreamHead = SessionStoreActions["setAgentStreamHead"];
export type ClearAgentStreamHead = SessionStoreActions["clearAgentStreamHead"];
export type SetAgentTimelineCursor = SessionStoreActions["setAgentTimelineCursor"];
export type MarkAgentHistorySynchronized = SessionStoreActions["markAgentHistorySynchronized"];
export type SetAgentAuthoritativeHistoryApplied =
  SessionStoreActions["setAgentAuthoritativeHistoryApplied"];

// ---------------------------------------------------------------------------
// Module-level pending agent updates buffer (scoped by serverId)
// ---------------------------------------------------------------------------

const pendingAgentUpdates = new Map<string, AgentUpdatePayload>();

function pendingKey(serverId: string, agentId: string): string {
  return `${serverId}:${agentId}`;
}

export function bufferPendingAgentUpdate(
  serverId: string,
  agentId: string,
  update: AgentUpdatePayload,
): void {
  pendingAgentUpdates.set(pendingKey(serverId, agentId), update);
}

export function flushPendingAgentUpdate(
  serverId: string,
  agentId: string,
): AgentUpdatePayload | undefined {
  const key = pendingKey(serverId, agentId);
  const update = pendingAgentUpdates.get(key);
  pendingAgentUpdates.delete(key);
  return update;
}

export function deletePendingAgentUpdate(serverId: string, agentId: string): void {
  pendingAgentUpdates.delete(pendingKey(serverId, agentId));
}

export function clearPendingAgentUpdates(serverId: string): void {
  for (const key of Array.from(pendingAgentUpdates.keys())) {
    if (key.startsWith(`${serverId}:`)) {
      pendingAgentUpdates.delete(key);
    }
  }
}

// ---------------------------------------------------------------------------
// Pure helper functions
// ---------------------------------------------------------------------------

export function hasAgentUsageChanged(
  incomingUsage: Agent["lastUsage"] | undefined,
  currentUsage: Agent["lastUsage"] | undefined,
): boolean {
  const keys: (keyof NonNullable<Agent["lastUsage"]>)[] = [
    "inputTokens",
    "outputTokens",
    "cachedInputTokens",
    "totalCostUsd",
    "contextWindowMaxTokens",
    "contextWindowUsedTokens",
  ];

  return keys.some((key) => incomingUsage?.[key] !== currentUsage?.[key]);
}

export const getAgentIdFromUpdate = (update: AgentUpdatePayload): string =>
  update.kind === "remove" ? update.agentId : update.agent.id;

export function clearAgentInitializingFlag(
  setInitializingAgents: SetInitializingAgents,
  serverId: string,
  agentId: string,
): void {
  setInitializingAgents(serverId, (prev) => {
    if (prev.get(agentId) !== true) {
      return prev;
    }
    const next = new Map(prev);
    next.set(agentId, false);
    return next;
  });
}

export function handleTimelineError(input: {
  result: ProcessTimelineResponseOutput;
  agentId: string;
  initKey: string;
  serverId: string;
  setInitializingAgents: SetInitializingAgents;
}): void {
  const { result, agentId, initKey, serverId, setInitializingAgents } = input;
  if (result.clearInitializing) {
    clearAgentInitializingFlag(setInitializingAgents, serverId, agentId);
  }
  if (result.initResolution === "reject" && result.error) {
    rejectInitDeferred(initKey, new Error(result.error));
  }
}

export function applyTimelineStreamPatches(input: {
  result: ProcessTimelineResponseOutput;
  agentId: string;
  serverId: string;
  currentTail: StreamItem[];
  currentHead: StreamItem[];
  setAgentStreamTail: SetAgentStreamTail;
  setAgentStreamHead: SetAgentStreamHead;
  clearAgentStreamHead: ClearAgentStreamHead;
  setAgentTimelineCursor: SetAgentTimelineCursor;
}): void {
  const {
    result,
    agentId,
    serverId,
    currentTail,
    currentHead,
    setAgentStreamTail,
    setAgentStreamHead,
    clearAgentStreamHead,
    setAgentTimelineCursor,
  } = input;

  if (result.tail !== currentTail) {
    setAgentStreamTail(serverId, (prev) => {
      const next = new Map(prev);
      next.set(agentId, result.tail);
      return next;
    });
  }

  if (result.head !== currentHead) {
    if (result.head.length === 0) {
      clearAgentStreamHead(serverId, agentId);
    } else {
      setAgentStreamHead(serverId, (prev) => {
        const next = new Map(prev);
        next.set(agentId, result.head);
        return next;
      });
    }
  }

  if (result.cursorChanged) {
    setAgentTimelineCursor(serverId, (prev) => {
      const current = prev.get(agentId);
      if (!result.cursor) {
        if (!current) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      }
      if (
        current &&
        current.epoch === result.cursor.epoch &&
        current.startSeq === result.cursor.startSeq &&
        current.endSeq === result.cursor.endSeq
      ) {
        return prev;
      }
      const next = new Map(prev);
      next.set(agentId, result.cursor);
      return next;
    });
  }
}

export function executeTimelineSideEffects(input: {
  sideEffects: TimelineReducerSideEffect[];
  agentId: string;
  serverId: string;
  requestCanonicalCatchUp: (agentId: string, cursor: { epoch: string; endSeq: number }) => void;
  applyAgentUpdatePayload: (payload: AgentUpdatePayload) => void;
}): void {
  const { sideEffects, agentId, serverId, requestCanonicalCatchUp, applyAgentUpdatePayload } =
    input;
  for (const effect of sideEffects) {
    if (effect.type === "catch_up") {
      requestCanonicalCatchUp(agentId, effect.cursor);
    } else if (effect.type === "flush_pending_updates") {
      const deferredUpdate = flushPendingAgentUpdate(serverId, agentId);
      if (deferredUpdate) {
        applyAgentUpdatePayload(deferredUpdate);
      }
    }
  }
}

export function finalizeTimelineApplication(input: {
  result: ProcessTimelineResponseOutput;
  agentId: string;
  initKey: string;
  serverId: string;
  shouldMarkAuthoritativeHistoryApplied: boolean;
  setInitializingAgents: SetInitializingAgents;
  setAgentAuthoritativeHistoryApplied: SetAgentAuthoritativeHistoryApplied;
  markAgentHistorySynchronized: MarkAgentHistorySynchronized;
}): void {
  const {
    result,
    agentId,
    initKey,
    serverId,
    shouldMarkAuthoritativeHistoryApplied,
    setInitializingAgents,
    setAgentAuthoritativeHistoryApplied,
    markAgentHistorySynchronized,
  } = input;

  if (result.clearInitializing) {
    clearAgentInitializingFlag(setInitializingAgents, serverId, agentId);
  }
  if (shouldMarkAuthoritativeHistoryApplied) {
    setAgentAuthoritativeHistoryApplied(serverId, agentId, true);
  }
  if (result.initResolution === "resolve") {
    resolveInitDeferred(initKey);
  }
  if (result.clearInitializing) {
    markAgentHistorySynchronized(serverId, agentId);
  }
}

export function applyToolResultToMessages(
  toolCallId: string,
  result: unknown,
): (prev: MessageEntry[]) => MessageEntry[] {
  return (prev) =>
    prev.map((msg) =>
      msg.type === "tool_call" && msg.id === toolCallId
        ? { ...msg, result, status: "completed" as const }
        : msg,
    );
}

export function applyToolErrorToMessages(
  toolCallId: string,
  error: unknown,
): (prev: MessageEntry[]) => MessageEntry[] {
  return (prev) =>
    prev.map((msg) =>
      msg.type === "tool_call" && msg.id === toolCallId
        ? { ...msg, error, status: "failed" as const }
        : msg,
    );
}
