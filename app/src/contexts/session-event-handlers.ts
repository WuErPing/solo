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
import { generateMessageId } from "@/types/stream";
import { patchWorkspaceScripts } from "@/contexts/session-workspace-scripts";
import { createSessionAgentStreamReducerQueue } from "@/contexts/session-stream-reducers";
import { getInitKey, getInitDeferred } from "@/utils/agent-initialization";
import {
  getAgentIdFromUpdate,
  bufferPendingAgentUpdate,
  deletePendingAgentUpdate,
  applyToolResultToMessages,
  applyToolErrorToMessages,
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
  setMessages: SessionStoreActions["setMessages"];
  setCurrentAssistantMessage: SessionStoreActions["setCurrentAssistantMessage"];
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
    setMessages,
    setCurrentAssistantMessage,
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
    client.on("activity_log", (message) => {
      if (message.type !== "activity_log") return;
      const data = message.payload;

      if (data.type === "tool_call" && data.metadata) {
        const {
          toolCallId,
          toolName,
          arguments: args,
        } = data.metadata as {
          toolCallId: string;
          toolName: string;
          arguments: unknown;
        };

        setMessages(serverId, (prev) => [
          ...prev,
          {
            type: "tool_call",
            id: toolCallId,
            timestamp: Date.now(),
            toolName,
            args,
            status: "executing",
          },
        ]);
        return;
      }

      if (data.type === "tool_result" && data.metadata) {
        const { toolCallId, result } = data.metadata as {
          toolCallId: string;
          result: unknown;
        };

        const applyToolResult = applyToolResultToMessages(toolCallId, result);
        setMessages(serverId, applyToolResult);
        return;
      }

      if (data.type === "error" && data.metadata && "toolCallId" in data.metadata) {
        const { toolCallId, error } = data.metadata as {
          toolCallId: string;
          error: unknown;
        };

        const applyToolError = applyToolErrorToMessages(toolCallId, error);
        setMessages(serverId, applyToolError);
      }

      let activityType: "system" | "info" | "success" | "error" = "info";
      if (data.type === "error") activityType = "error";

      if (data.type === "transcript") {
        setMessages(serverId, (prev) => [
          ...prev,
          {
            type: "user",
            id: generateMessageId(),
            timestamp: Date.now(),
            message: data.content,
          },
        ]);
        return;
      }

      if (data.type === "assistant") {
        setMessages(serverId, (prev) => [
          ...prev,
          {
            type: "assistant",
            id: generateMessageId(),
            timestamp: Date.now(),
            message: data.content,
          },
        ]);
        setCurrentAssistantMessage(serverId, "");
        return;
      }

      setMessages(serverId, (prev) => [
        ...prev,
        {
          type: "activity",
          id: generateMessageId(),
          timestamp: Date.now(),
          activityType,
          message: data.content,
          metadata: data.metadata,
        },
      ]);
    }),
  );

  unsubs.push(
    client.on("assistant_chunk", (message) => {
      if (message.type !== "assistant_chunk") return;
      setCurrentAssistantMessage(serverId, (prev) => prev + message.payload.chunk);
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
