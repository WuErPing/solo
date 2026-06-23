import type { QueryClient } from "@tanstack/react-query";
import type { DaemonClient } from "@server/client/daemon-client";
import type { AgentAttachment, SessionOutboundMessage } from "@server/shared/messages";
import type { AgentLifecycleStatus } from "@server/shared/agent-lifecycle";
import type { AttachmentMetadata } from "@/attachments/types";
import { getHostRuntimeStore } from "@/runtime/host-runtime";
import {
  useSessionStore,
  type Agent,
  type WorkspaceDescriptor,
  normalizeWorkspaceDescriptor,
} from "@/stores/session-store";
import { clearArchiveAgentPending } from "@/hooks/use-archive-agent";
import { derivePendingPermissionKey, normalizeAgentSnapshot } from "@/utils/agent-snapshots";
import { resolveProjectPlacement } from "@/utils/project-placement";
import { getInitKey, getInitDeferred } from "@/utils/agent-initialization";
import { splitComposerAttachmentsForSubmit } from "@/components/composer-attachments";
import { isNative } from "@/constants/platform";
import { processTimelineResponse } from "@/contexts/session-stream-reducers";
import {
  hasAgentUsageChanged,
  handleTimelineError,
  applyTimelineStreamPatches,
  executeTimelineSideEffects,
  finalizeTimelineApplication,
  deletePendingAgentUpdate,
  type AgentUpdatePayload,
  type WorkspaceSetupProgressPayload,
  type SetInitializingAgents,
  type SetAgentStreamTail,
  type SetAgentStreamHead,
  type ClearAgentStreamHead,
  type SetAgentTimelineCursor,
  type MarkAgentHistorySynchronized,
  type SetAgentAuthoritativeHistoryApplied,
} from "./session-helpers";

const HISTORY_STALE_AFTER_MS = 60_000;
const AUTHORITATIVE_REVALIDATION_DEBOUNCE_MS = 300;

type SessionStoreActions = ReturnType<typeof useSessionStore.getState>;

type SendAgentMessageFn = (
  agentId: string,
  message: string,
  images?: AttachmentMetadata[],
  attachments?: AgentAttachment[],
) => Promise<void>;

// ---------------------------------------------------------------------------
// createHydrateWorkspaces
// ---------------------------------------------------------------------------

export interface HydrateWorkspacesDeps {
  client: DaemonClient;
  isConnected: boolean;
  serverId: string;
  setWorkspaces: SessionStoreActions["setWorkspaces"];
  setHasHydratedWorkspaces: SessionStoreActions["setHasHydratedWorkspaces"];
}

export function createHydrateWorkspaces(deps: HydrateWorkspacesDeps) {
  return async (options?: { subscribe?: boolean; isCancelled?: () => boolean }) => {
    if (!deps.client || !deps.isConnected) {
      return;
    }

    const workspaces = new Map<string, WorkspaceDescriptor>();
    let cursor: string | null = null;
    let includeSubscribe = options?.subscribe ?? false;

    while (true) {
      const payload = await deps.client.fetchWorkspaces({
        sort: [{ key: "activity_at", direction: "desc" }],
        ...(includeSubscribe ? { subscribe: {} } : {}),
        page: cursor ? { limit: 200, cursor } : { limit: 200 },
      });
      if (options?.isCancelled?.()) {
        return;
      }

      for (const entry of payload.entries) {
        const workspace = normalizeWorkspaceDescriptor(entry);
        workspaces.set(workspace.id, workspace);
      }

      if (!payload.pageInfo.hasMore || !payload.pageInfo.nextCursor) {
        break;
      }
      cursor = payload.pageInfo.nextCursor;
      includeSubscribe = false;
    }

    if (options?.isCancelled?.()) {
      return;
    }

    deps.setWorkspaces(deps.serverId, workspaces);
    deps.setHasHydratedWorkspaces(deps.serverId, true);
  };
}

// ---------------------------------------------------------------------------
// createApplyAuthoritativeAgentSnapshot
// ---------------------------------------------------------------------------

export interface ApplyAuthoritativeAgentSnapshotDeps {
  serverId: string;
  queryClient: QueryClient;
  setAgents: SessionStoreActions["setAgents"];
  setAgentLastActivity: SessionStoreActions["setAgentLastActivity"];
  setPendingPermissions: SessionStoreActions["setPendingPermissions"];
  setQueuedMessages: SessionStoreActions["setQueuedMessages"];
  sendAgentMessageRef: { current: SendAgentMessageFn | null };
  previousAgentStatusRef: { current: Map<string, AgentLifecycleStatus> };
}

export function createApplyAuthoritativeAgentSnapshot(
  deps: ApplyAuthoritativeAgentSnapshotDeps,
) {
  return (agent: Agent) => {
    deps.setAgents(deps.serverId, (prev) => {
      const current = prev.get(agent.id);
      if (current && agent.updatedAt.getTime() < current.updatedAt.getTime()) {
        const hasUsageUpdate = hasAgentUsageChanged(agent.lastUsage, current.lastUsage);
        if (hasUsageUpdate) {
          const next = new Map(prev);
          next.set(agent.id, {
            ...current,
            lastUsage: agent.lastUsage,
          });
          return next;
        }
        return prev;
      }
      const next = new Map(prev);
      next.set(agent.id, agent);
      return next;
    });

    if (agent.archivedAt) {
      clearArchiveAgentPending({
        queryClient: deps.queryClient,
        serverId: deps.serverId,
        agentId: agent.id,
      });
    }

    deps.setAgentLastActivity(agent.id, agent.lastActivityAt);

    deps.setPendingPermissions(deps.serverId, (prev) => {
      const existingKeysForAgent: string[] = [];
      for (const [key, pending] of prev.entries()) {
        if (pending.agentId === agent.id) {
          existingKeysForAgent.push(key);
        }
      }

      const nextEntries = agent.pendingPermissions.map((request) => ({
        key: derivePendingPermissionKey(agent.id, request),
        agentId: agent.id,
        request,
      }));

      let changed = existingKeysForAgent.length !== nextEntries.length;
      if (!changed) {
        const existingKeySet = new Set(existingKeysForAgent);
        for (const entry of nextEntries) {
          const existing = prev.get(entry.key);
          if (!existingKeySet.has(entry.key) || !existing) {
            changed = true;
            break;
          }

          const currentRequest = existing.request;
          if (
            currentRequest.id !== entry.request.id ||
            currentRequest.kind !== entry.request.kind ||
            currentRequest.name !== entry.request.name ||
            currentRequest.title !== entry.request.title ||
            currentRequest.description !== entry.request.description
          ) {
            changed = true;
            break;
          }
        }
      }

      if (!changed) {
        return prev;
      }

      const next = new Map(prev);
      for (const key of existingKeysForAgent) {
        next.delete(key);
      }
      for (const entry of nextEntries) {
        next.set(entry.key, entry);
      }
      return next;
    });

    const prevStatus = deps.previousAgentStatusRef.current.get(agent.id);
    if (prevStatus === "running" && agent.status !== "running") {
      const session = useSessionStore.getState().sessions[deps.serverId];
      const queue = session?.queuedMessages.get(agent.id);
      if (queue && queue.length > 0) {
        const [next, ...rest] = queue;
        if (deps.sendAgentMessageRef.current) {
          const wirePayload = splitComposerAttachmentsForSubmit(next.attachments);
          void deps.sendAgentMessageRef.current(
            agent.id,
            next.text,
            wirePayload.images,
            wirePayload.attachments,
          );
        }
        deps.setQueuedMessages(deps.serverId, (prev) => {
          const updated = new Map(prev);
          updated.set(agent.id, rest);
          return updated;
        });
      }
    }

    deps.previousAgentStatusRef.current.set(agent.id, agent.status);
  };
}

// ---------------------------------------------------------------------------
// createRunAuthoritativeRevalidation
// ---------------------------------------------------------------------------

export interface RunAuthoritativeRevalidationDeps {
  serverId: string;
  hydrateWorkspaces: (options?: { subscribe?: boolean; isCancelled?: () => boolean }) => Promise<void>;
}

export function createRunAuthoritativeRevalidation(deps: RunAuthoritativeRevalidationDeps) {
  return async () => {
    await Promise.all([
      getHostRuntimeStore().refreshAgentDirectory({ serverId: deps.serverId }),
      deps.hydrateWorkspaces(),
    ]);
  };
}

// ---------------------------------------------------------------------------
// createFlushAuthoritativeRevalidation
// ---------------------------------------------------------------------------

export interface FlushAuthoritativeRevalidationDeps {
  client: DaemonClient;
  isConnected: boolean;
  serverId: string;
  runAuthoritativeRevalidation: () => Promise<void>;
  revalidationInFlightRef: { current: Promise<void> | null };
  revalidationQueuedRef: { current: boolean };
  revalidationTimerRef: { current: ReturnType<typeof setTimeout> | null };
}

export function createFlushAuthoritativeRevalidation(
  deps: FlushAuthoritativeRevalidationDeps,
) {
  const flushAuthoritativeRevalidation = () => {
    if (!deps.client || !deps.isConnected) {
      return;
    }
    if (deps.revalidationInFlightRef.current) {
      deps.revalidationQueuedRef.current = true;
      return;
    }

    const run = deps.runAuthoritativeRevalidation()
      .catch((error) => {
        console.error("[Session] authoritative revalidation failed", {
          serverId: deps.serverId,
          error,
        });
      })
      .finally(() => {
        if (deps.revalidationInFlightRef.current === run) {
          deps.revalidationInFlightRef.current = null;
        }
        if (!deps.revalidationQueuedRef.current) {
          return;
        }
        deps.revalidationQueuedRef.current = false;
        if (deps.revalidationTimerRef.current) {
          clearTimeout(deps.revalidationTimerRef.current);
        }
        deps.revalidationTimerRef.current = setTimeout(() => {
          deps.revalidationTimerRef.current = null;
          flushAuthoritativeRevalidation();
        }, AUTHORITATIVE_REVALIDATION_DEBOUNCE_MS);
      });

    deps.revalidationInFlightRef.current = run;
  };
  return flushAuthoritativeRevalidation;
}

// ---------------------------------------------------------------------------
// createScheduleAuthoritativeRevalidation
// ---------------------------------------------------------------------------

export interface ScheduleAuthoritativeRevalidationDeps {
  client: DaemonClient;
  isConnected: boolean;
  flushAuthoritativeRevalidation: () => void;
  revalidationTimerRef: { current: ReturnType<typeof setTimeout> | null };
  revalidationQueuedRef: { current: boolean };
}

export function createScheduleAuthoritativeRevalidation(
  deps: ScheduleAuthoritativeRevalidationDeps,
) {
  return () => {
    if (!deps.client || !deps.isConnected) {
      return;
    }

    deps.revalidationQueuedRef.current = true;
    if (deps.revalidationTimerRef.current) {
      return;
    }
    deps.revalidationTimerRef.current = setTimeout(() => {
      deps.revalidationTimerRef.current = null;
      if (!deps.revalidationQueuedRef.current) {
        return;
      }
      deps.revalidationQueuedRef.current = false;
      deps.flushAuthoritativeRevalidation();
    }, AUTHORITATIVE_REVALIDATION_DEBOUNCE_MS);
  };
}

// ---------------------------------------------------------------------------
// createHandleAppResumed
// ---------------------------------------------------------------------------

export interface HandleAppResumedDeps {
  serverId: string;
  client: DaemonClient;
  scheduleAuthoritativeRevalidation: () => void;
  bumpHistorySyncGeneration: SessionStoreActions["bumpHistorySyncGeneration"];
}

export function createHandleAppResumed(deps: HandleAppResumedDeps) {
  return (awayMs: number) => {
    deps.scheduleAuthoritativeRevalidation();

    if (isNative) {
      const session = useSessionStore.getState().sessions[deps.serverId];
      const agentId = session?.focusedAgentId;
      const cursor = agentId ? session?.agentTimelineCursor.get(agentId) : undefined;
      if (agentId && cursor) {
        void deps.client
          .fetchAgentTimeline(agentId, {
            direction: "after",
            cursor: { epoch: cursor.epoch, seq: cursor.endSeq },
            limit: 0,
            projection: "canonical",
          })
          .catch((error) => {
            console.warn("[Session] failed to fetch catch-up timeline on resume", agentId, error);
          });
      }
    }

    if (awayMs < HISTORY_STALE_AFTER_MS) {
      return;
    }
    deps.bumpHistorySyncGeneration(deps.serverId);
  };
}

// ---------------------------------------------------------------------------
// createRequestCanonicalCatchUp
// ---------------------------------------------------------------------------

export interface RequestCanonicalCatchUpDeps {
  client: DaemonClient;
}

export function createRequestCanonicalCatchUp(deps: RequestCanonicalCatchUpDeps) {
  return (agentId: string, cursor: { epoch: string; endSeq: number }) => {
    void deps.client
      .fetchAgentTimeline(agentId, {
        direction: "after",
        cursor: { epoch: cursor.epoch, seq: cursor.endSeq },
        limit: 0,
        projection: "canonical",
      })
      .catch((error) => {
        console.warn("[Session] failed to fetch canonical catch-up timeline", agentId, error);
      });
  };
}

// ---------------------------------------------------------------------------
// createApplyTimelineResponse
// ---------------------------------------------------------------------------

export interface ApplyTimelineResponseDeps {
  serverId: string;
  applyAuthoritativeAgentSnapshot: (agent: Agent) => void;
  applyAgentUpdatePayload: (update: AgentUpdatePayload) => void;
  requestCanonicalCatchUp: (agentId: string, cursor: { epoch: string; endSeq: number }) => void;
  setInitializingAgents: SetInitializingAgents;
  setAgentStreamTail: SetAgentStreamTail;
  setAgentStreamHead: SetAgentStreamHead;
  clearAgentStreamHead: ClearAgentStreamHead;
  setAgentTimelineCursor: SetAgentTimelineCursor;
  setAgentAuthoritativeHistoryApplied: SetAgentAuthoritativeHistoryApplied;
  markAgentHistorySynchronized: MarkAgentHistorySynchronized;
}

export function createApplyTimelineResponse(deps: ApplyTimelineResponseDeps) {
  return (
    payload: Extract<
      SessionOutboundMessage,
      { type: "fetch_agent_timeline_response" }
    >["payload"],
  ) => {
    const agentId = payload.agentId;
    const initKey = getInitKey(deps.serverId, agentId);
    const shouldMarkAuthoritativeHistoryApplied =
      payload.direction === "tail" || payload.direction === "after";

    // Read current store state
    const session = useSessionStore.getState().sessions[deps.serverId];
    const isInitializing = session?.initializingAgents.get(agentId) === true;
    const activeInitDeferred = getInitDeferred(initKey);
    const hasActiveInitDeferred = Boolean(activeInitDeferred);
    const currentCursor = session?.agentTimelineCursor.get(agentId);
    const currentTail = session?.agentStreamTail.get(agentId) ?? [];
    const currentHead = session?.agentStreamHead.get(agentId) ?? [];

    if (payload.agent) {
      const normalized = normalizeAgentSnapshot(payload.agent, deps.serverId);
      deps.applyAuthoritativeAgentSnapshot({
        ...normalized,
        projectPlacement: session?.agents.get(agentId)?.projectPlacement ?? null,
      });
    }

    // Call pure reducer
    const result = processTimelineResponse({
      payload,
      currentTail,
      currentHead,
      currentCursor,
      isInitializing,
      hasActiveInitDeferred,
      initRequestDirection: activeInitDeferred?.requestDirection ?? "tail",
    });

    if (result.error) {
      handleTimelineError({
        result,
        agentId,
        initKey,
        serverId: deps.serverId,
        setInitializingAgents: deps.setInitializingAgents,
      });
      return;
    }

    applyTimelineStreamPatches({
      result,
      agentId,
      serverId: deps.serverId,
      currentTail,
      currentHead,
      setAgentStreamTail: deps.setAgentStreamTail,
      setAgentStreamHead: deps.setAgentStreamHead,
      clearAgentStreamHead: deps.clearAgentStreamHead,
      setAgentTimelineCursor: deps.setAgentTimelineCursor,
    });

    executeTimelineSideEffects({
      sideEffects: result.sideEffects,
      agentId,
      serverId: deps.serverId,
      requestCanonicalCatchUp: deps.requestCanonicalCatchUp,
      applyAgentUpdatePayload: deps.applyAgentUpdatePayload,
    });

    finalizeTimelineApplication({
      result,
      agentId,
      initKey,
      serverId: deps.serverId,
      shouldMarkAuthoritativeHistoryApplied,
      setInitializingAgents: deps.setInitializingAgents,
      setAgentAuthoritativeHistoryApplied: deps.setAgentAuthoritativeHistoryApplied,
      markAgentHistorySynchronized: deps.markAgentHistorySynchronized,
    });
  };
}

// ---------------------------------------------------------------------------
// createApplyAgentUpdatePayload
// ---------------------------------------------------------------------------

export interface ApplyAgentUpdatePayloadDeps {
  serverId: string;
  queryClient: QueryClient;
  setAgents: SessionStoreActions["setAgents"];
  setPendingPermissions: SessionStoreActions["setPendingPermissions"];
  setQueuedMessages: SessionStoreActions["setQueuedMessages"];
  setAgentTimelineCursor: SetAgentTimelineCursor;
  setAgentAuthoritativeHistoryApplied: SetAgentAuthoritativeHistoryApplied;
  applyAuthoritativeAgentSnapshot: (agent: Agent) => void;
  previousAgentStatusRef: { current: Map<string, AgentLifecycleStatus> };
}

export function createApplyAgentUpdatePayload(deps: ApplyAgentUpdatePayloadDeps) {
  return (update: AgentUpdatePayload) => {
    if (update.kind === "remove") {
      const agentId = update.agentId;
      deps.previousAgentStatusRef.current.delete(agentId);
      deletePendingAgentUpdate(deps.serverId, agentId);
      clearArchiveAgentPending({ queryClient: deps.queryClient, serverId: deps.serverId, agentId });

      deps.setAgents(deps.serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });

      deps.setPendingPermissions(deps.serverId, (prev) => {
        if (prev.size === 0) {
          return prev;
        }
        let changed = false;
        const next = new Map(prev);
        for (const [key, pending] of Array.from(next.entries())) {
          if (pending.agentId === agentId) {
            next.delete(key);
            changed = true;
          }
        }
        return changed ? next : prev;
      });

      deps.setQueuedMessages(deps.serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });

      deps.setAgentTimelineCursor(deps.serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });
      deps.setAgentAuthoritativeHistoryApplied(deps.serverId, agentId, false);
      return;
    }

    const normalized = normalizeAgentSnapshot(update.agent, deps.serverId);
    const agent = {
      ...normalized,
      projectPlacement: resolveProjectPlacement({
        projectPlacement: update.project,
        cwd: normalized.cwd,
      }),
    };

    deps.applyAuthoritativeAgentSnapshot(agent);
  };
}

// ---------------------------------------------------------------------------
// createApplyWorkspaceSetupProgress
// ---------------------------------------------------------------------------

export interface ApplyWorkspaceSetupProgressDeps {
  serverId: string;
  upsertWorkspaceSetupProgress: (input: {
    serverId: string;
    payload: WorkspaceSetupProgressPayload;
  }) => void;
}

export function createApplyWorkspaceSetupProgress(
  deps: ApplyWorkspaceSetupProgressDeps,
) {
  return (payload: WorkspaceSetupProgressPayload) => {
    deps.upsertWorkspaceSetupProgress({ serverId: deps.serverId, payload });
  };
}
