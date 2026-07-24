import type { QueryClient } from "@tanstack/react-query";
import type { DaemonClient } from "@server/client/daemon-client";
import type { AgentStreamEventPayload } from "@server/shared/messages";
import { parseServerInfoStatusPayload } from "@server/shared/messages";
import {
  useSessionStore,
  normalizeWorkspaceDescriptor,
} from "@/stores/session-store";
import { useDraftStore } from "@/stores/draft-store";
import { useWorkspaceSetupStore } from "@/stores/workspace-setup-store";
import { clearArchiveAgentPending } from "@/hooks/use-archive-agent";
import { derivePendingPermissionKey } from "@/utils/agent-snapshots";
import { buildDraftStoreKey } from "@/stores/draft-keys";
import { patchWorkspaceScripts } from "@/contexts/session-workspace-scripts";
import { createSessionAgentStreamReducerQueue } from "@/contexts/session-stream-reducers";
import { getInitKey, getInitDeferred } from "@/utils/agent-initialization";
import {
  getAgentIdFromUpdate,
  bufferPendingAgentUpdate,
  deletePendingAgentUpdate,
} from "./session-helpers";
import type { createNotifyAgentAttention } from "./session-notifications";
import type {
  createApplyAgentUpdatePayload,
  createApplyTimelineResponse,
  createApplyWorkspaceSetupProgress,
  createRequestCanonicalCatchUp,
} from "./session-agent-sync";

type SessionStoreActions = ReturnType<typeof useSessionStore.getState>;
type DraftStoreActions = ReturnType<typeof useDraftStore.getState>;
type WorkspaceSetupStoreActions = ReturnType<typeof useWorkspaceSetupStore.getState>;

export interface SubscribeToSessionEventsDeps {
  client: DaemonClient;
  serverId: string;
  queryClient: QueryClient;

  // Factory-created callbacks
  notifyAgentAttention: ReturnType<typeof createNotifyAgentAttention>;
  applyAgentUpdatePayload: ReturnType<typeof createApplyAgentUpdatePayload>;
  applyTimelineResponse: ReturnType<typeof createApplyTimelineResponse>;
  applyWorkspaceSetupProgress: ReturnType<typeof createApplyWorkspaceSetupProgress>;
  requestCanonicalCatchUp: ReturnType<typeof createRequestCanonicalCatchUp>;

  // Store actions
  setAgentStreamTail: SessionStoreActions["setAgentStreamTail"];
  setAgentStreamHead: SessionStoreActions["setAgentStreamHead"];
  setAgentStreamState: SessionStoreActions["setAgentStreamState"];
  clearAgentStreamHead: SessionStoreActions["clearAgentStreamHead"];
  setAgentTimelineCursor: SessionStoreActions["setAgentTimelineCursor"];
  setInitializingAgents: SessionStoreActions["setInitializingAgents"];
  setAgents: SessionStoreActions["setAgents"];
  setWorkspaces: SessionStoreActions["setWorkspaces"];
  mergeWorkspaces: SessionStoreActions["mergeWorkspaces"];
  removeWorkspace: SessionStoreActions["removeWorkspace"];
  setPendingPermissions: SessionStoreActions["setPendingPermissions"];
  setFileExplorer: SessionStoreActions["setFileExplorer"];
  clearDraftInput: DraftStoreActions["clearDraftInput"];
  updateSessionServerInfo: SessionStoreActions["updateSessionServerInfo"];
  removeWorkspaceSetup: WorkspaceSetupStoreActions["removeWorkspace"];
}

export function subscribeToSessionEvents(deps: SubscribeToSessionEventsDeps): () => void {
  const {
    client,
    serverId,
    queryClient,
    notifyAgentAttention,
    applyAgentUpdatePayload,
    applyTimelineResponse,
    applyWorkspaceSetupProgress,
    requestCanonicalCatchUp,
    setAgentStreamTail,
    setAgentStreamState,
    clearAgentStreamHead,
    setAgentTimelineCursor,
    setInitializingAgents,
    setAgents,
    setWorkspaces,
    mergeWorkspaces,
    removeWorkspace,
    setPendingPermissions,
    setFileExplorer,
    clearDraftInput,
    updateSessionServerInfo,
    removeWorkspaceSetup,
  } = deps;

  const unsubs: (() => void)[] = [];

  unsubs.push(
    client.on("agent_update", (message) => {
      if (message.type !== "agent_update") return;
      const update = message.payload;
      const agentId = getAgentIdFromUpdate(update);
      const initKey = getInitKey(serverId, agentId);
      const session = useSessionStore.getState().sessions[serverId];
      const isSyncingHistory =
        session?.initializingAgents.get(agentId) === true && Boolean(getInitDeferred(initKey));

      if (isSyncingHistory) {
        bufferPendingAgentUpdate(serverId, agentId, update);
        return;
      }

      deletePendingAgentUpdate(serverId, agentId);
      applyAgentUpdatePayload(update);
    }),
  );

  const agentStreamReducerQueue = createSessionAgentStreamReducerQueue({
    serverId,
    setAgentStreamState,
    setAgentTimelineCursor,
    setAgents,
    requestCanonicalCatchUp,
  });

  unsubs.push(
    client.on("agent_stream", (message) => {
      if (message.type !== "agent_stream") return;
      const { agentId, event, timestamp, seq, epoch } = message.payload;
      const parsedTimestamp = new Date(timestamp);
      const streamEvent = event as AgentStreamEventPayload;

      // Attention notification stays in React (not extractable to pure reducer)
      if (event.type === "attention_required") {
        if (event.shouldNotify) {
          notifyAgentAttention({
            agentId,
            reason: event.reason,
            timestamp: event.timestamp,
            notification: event.notification,
          });
        }
      }

      agentStreamReducerQueue.enqueue(agentId, {
        event: streamEvent,
        seq,
        epoch,
        timestamp: parsedTimestamp,
      });

      // NOTE: We don't update lastActivityAt on every stream event to prevent
      // cascading rerenders. The agent_update handler updates agent.lastActivityAt
      // on status changes, which is sufficient for sorting and display purposes.
    }),
  );

  unsubs.push(
    client.on("fetch_agent_timeline_response", (message) => {
      if (message.type !== "fetch_agent_timeline_response") return;
      agentStreamReducerQueue.flushAgent(message.payload.agentId);
      applyTimelineResponse(message.payload);
    }),
  );

  unsubs.push(
    client.on("workspace_update", (message) => {
      if (message.type !== "workspace_update") return;
      if (message.payload.kind === "remove") {
        removeWorkspaceSetup({ serverId, workspaceId: String(message.payload.id) });
        removeWorkspace(serverId, String(message.payload.id));
        return;
      }
      const workspace = normalizeWorkspaceDescriptor(message.payload.workspace);
      mergeWorkspaces(serverId, [workspace]);
    }),
  );

  unsubs.push(
    client.on("script_status_update", (message) => {
      if (message.type !== "script_status_update") return;
      setWorkspaces(serverId, (prev) => patchWorkspaceScripts(prev, message.payload));
    }),
  );

  unsubs.push(
    client.on("workspace_setup_progress", (message) => {
      if (message.type !== "workspace_setup_progress") return;
      applyWorkspaceSetupProgress(message.payload);
    }),
  );

  unsubs.push(
    client.on("workspace_setup_status_response", (message) => {
      if (message.type !== "workspace_setup_status_response") return;
      const { workspaceId, snapshot } = message.payload;
      if (snapshot) {
        applyWorkspaceSetupProgress({ workspaceId, ...snapshot });
      }
    }),
  );

  unsubs.push(
    client.on("status", (message) => {
      if (message.type !== "status") return;
      const serverInfo = parseServerInfoStatusPayload(message.payload);
      if (serverInfo) {
        updateSessionServerInfo(serverId, {
          serverId: serverInfo.serverId,
          hostname: serverInfo.hostname,
          version: serverInfo.version,
          ...(serverInfo.capabilities ? { capabilities: serverInfo.capabilities } : {}),
          ...(serverInfo.features ? { features: serverInfo.features } : {}),
        });
        return;
      }
    }),
  );

  unsubs.push(
    client.on("agent_permission_request", (message) => {
      if (message.type !== "agent_permission_request") return;
      const { agentId, request } = message.payload;

      setPendingPermissions(serverId, (prev) => {
        const next = new Map(prev);
        const key = derivePendingPermissionKey(agentId, request);
        next.set(key, { key, agentId, request });
        return next;
      });
    }),
  );

  unsubs.push(
    client.on("agent_permission_resolved", (message) => {
      if (message.type !== "agent_permission_resolved") return;
      const { requestId, agentId } = message.payload;

      setPendingPermissions(serverId, (prev) => {
        const next = new Map(prev);
        const derivedKey = `${agentId}:${requestId}`;
        if (!next.delete(derivedKey)) {
          for (const [key, pending] of next.entries()) {
            if (pending.agentId === agentId && pending.request.id === requestId) {
              next.delete(key);
              break;
            }
          }
        }
        return next;
      });
    }),
  );

  unsubs.push(
    client.on("agent_deleted", (message) => {
      if (message.type !== "agent_deleted") {
        return;
      }
      const { agentId } = message.payload;
      deletePendingAgentUpdate(serverId, agentId);
      clearArchiveAgentPending({ queryClient, serverId, agentId });

      setAgents(serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });

      // Remove from agentLastActivity slice (top-level)
      useSessionStore.setState((state) => {
        if (!state.agentLastActivity.has(agentId)) {
          return state;
        }
        const nextActivity = new Map(state.agentLastActivity);
        nextActivity.delete(agentId);
        return {
          ...state,
          agentLastActivity: nextActivity,
        };
      });

      setAgentStreamTail(serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });
      clearAgentStreamHead(serverId, agentId);
      setAgentTimelineCursor(serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });

      // Remove draft input
      clearDraftInput({
        draftKey: buildDraftStoreKey({ serverId, agentId }),
      });

      setPendingPermissions(serverId, (prev) => {
        let changed = false;
        const next = new Map(prev);
        for (const [key, pending] of prev.entries()) {
          if (pending.agentId === agentId) {
            next.delete(key);
            changed = true;
          }
        }
        return changed ? next : prev;
      });

      setInitializingAgents(serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });

      setFileExplorer(serverId, (prev) => {
        if (!prev.has(agentId)) {
          return prev;
        }
        const next = new Map(prev);
        next.delete(agentId);
        return next;
      });
    }),
  );

  unsubs.push(
    client.on("agent_archived", (message) => {
      if (message.type !== "agent_archived") {
        return;
      }
      const { agentId, archivedAt } = message.payload;
      clearArchiveAgentPending({ queryClient, serverId, agentId });

      setAgents(serverId, (prev) => {
        const existing = prev.get(agentId);
        if (!existing) {
          return prev;
        }
        const next = new Map(prev);
        next.set(agentId, {
          ...existing,
          archivedAt: new Date(archivedAt),
        });
        return next;
      });
    }),
  );

  return () => {
    unsubs.forEach((fn) => fn());
    agentStreamReducerQueue.dispose({ flush: true });
  };
}
