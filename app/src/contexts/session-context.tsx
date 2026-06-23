import { useRef, ReactNode, useCallback, useEffect } from "react";
import { AppState } from "react-native";
import { useQueryClient } from "@tanstack/react-query";
import { useClientActivity } from "@/hooks/use-client-activity";
import { usePushTokenRegistration } from "@/hooks/use-push-token-registration";
import { prefetchProvidersSnapshot } from "@/hooks/use-providers-snapshot";
import type { AgentAttachment } from "@server/shared/messages";
import type { AgentLifecycleStatus } from "@server/shared/agent-lifecycle";
import type { DaemonClient } from "@server/client/daemon-client";
import { useHostRuntimeIsConnected } from "@/runtime/host-runtime";
import { useSessionStore } from "@/stores/session-store";
import { useDraftStore } from "@/stores/draft-store";
import { useWorkspaceSetupStore } from "@/stores/workspace-setup-store";
import { sendOsNotification } from "@/utils/os-notifications";
import { getIsAppActivelyVisible } from "@/utils/app-visibility";
import { clearInitDeferredsForServer } from "@/utils/agent-initialization";
import type { AttachmentMetadata } from "@/attachments/types";
import { reconcilePreviousAgentStatuses } from "@/contexts/session-status-tracking";
import { createNotifyAgentAttention } from "./session-notifications";
import { createSendAgentMessage } from "./agent-operations";
import { clearPendingAgentUpdates } from "./session-helpers";
import {
  createHydrateWorkspaces,
  createApplyAuthoritativeAgentSnapshot,
  createRunAuthoritativeRevalidation,
  createFlushAuthoritativeRevalidation,
  createScheduleAuthoritativeRevalidation,
  createHandleAppResumed,
  createRequestCanonicalCatchUp,
  createApplyTimelineResponse,
  createApplyAgentUpdatePayload,
  createApplyWorkspaceSetupProgress,
} from "./session-agent-sync";
import { subscribeToSessionEvents } from "./session-event-handlers";

// Re-export types from session-store and draft-store for backward compatibility
export type { DraftInput } from "@/stores/draft-store";
export type {
  MessageEntry,
  Agent,
  ExplorerEntry,
  ExplorerFile,
  ExplorerEntryKind,
  ExplorerFileKind,
  ExplorerEncoding,
  AgentFileExplorerState,
} from "@/stores/session-store";

// Re-export buffer functions and type aliases from session-helpers
export {
  bufferPendingAgentUpdate,
  flushPendingAgentUpdate,
  deletePendingAgentUpdate,
  clearPendingAgentUpdates,
  type AgentUpdatePayload,
  type WorkspaceSetupProgressPayload,
} from "./session-helpers";

interface SessionProviderSharedProps {
  children: ReactNode;
  serverId: string;
}

interface SessionProviderClientProps extends SessionProviderSharedProps {
  client: DaemonClient;
}

export type SessionProviderProps = SessionProviderClientProps;

function SessionProviderWithClient({ children, serverId, client }: SessionProviderClientProps) {
  return (
    <SessionProviderInternal serverId={serverId} client={client}>
      {children}
    </SessionProviderInternal>
  );
}

// SessionProvider: Daemon client message handler that updates Zustand store
export function SessionProvider(props: SessionProviderProps) {
  return <SessionProviderWithClient {...props} />;
}

function SessionProviderInternal({ children, serverId, client }: SessionProviderClientProps) {
  const queryClient = useQueryClient();
  const isConnected = useHostRuntimeIsConnected(serverId);

  // Zustand store actions
  const initializeSession = useSessionStore((state) => state.initializeSession);
  const clearSession = useSessionStore((state) => state.clearSession);
  const setMessages = useSessionStore((state) => state.setMessages);
  const setCurrentAssistantMessage = useSessionStore((state) => state.setCurrentAssistantMessage);
  const setAgentStreamTail = useSessionStore((state) => state.setAgentStreamTail);
  const setAgentStreamHead = useSessionStore((state) => state.setAgentStreamHead);
  const setAgentStreamState = useSessionStore((state) => state.setAgentStreamState);
  const clearAgentStreamHead = useSessionStore((state) => state.clearAgentStreamHead);
  const setAgentTimelineCursor = useSessionStore((state) => state.setAgentTimelineCursor);
  const setInitializingAgents = useSessionStore((state) => state.setInitializingAgents);
  const bumpHistorySyncGeneration = useSessionStore((state) => state.bumpHistorySyncGeneration);
  const markAgentHistorySynchronized = useSessionStore(
    (state) => state.markAgentHistorySynchronized,
  );
  const setAgentAuthoritativeHistoryApplied = useSessionStore(
    (state) => state.setAgentAuthoritativeHistoryApplied,
  );
  const setHasHydratedWorkspaces = useSessionStore((state) => state.setHasHydratedWorkspaces);
  const setAgents = useSessionStore((state) => state.setAgents);
  const setWorkspaces = useSessionStore((state) => state.setWorkspaces);
  const mergeWorkspaces = useSessionStore((state) => state.mergeWorkspaces);
  const removeWorkspace = useSessionStore((state) => state.removeWorkspace);
  const setAgentLastActivity = useSessionStore((state) => state.setAgentLastActivity);
  const flushAgentLastActivity = useSessionStore((state) => state.flushAgentLastActivity);
  const setPendingPermissions = useSessionStore((state) => state.setPendingPermissions);
  const clearDraftInput = useDraftStore((state) => state.clearDraftInput);
  const setQueuedMessages = useSessionStore((state) => state.setQueuedMessages);
  const updateSessionClient = useSessionStore((state) => state.updateSessionClient);
  const updateSessionServerInfo = useSessionStore((state) => state.updateSessionServerInfo);
  const upsertWorkspaceSetupProgress = useWorkspaceSetupStore((state) => state.upsertProgress);
  const removeWorkspaceSetup = useWorkspaceSetupStore((state) => state.removeWorkspace);
  const clearWorkspaceSetupServer = useWorkspaceSetupStore((state) => state.clearServer);

  // Track focused agent for heartbeat
  const focusedAgentId = useSessionStore(
    (state) => state.sessions[serverId]?.focusedAgentId ?? null,
  );
  const sessionAgents = useSessionStore((state) => state.sessions[serverId]?.agents);

  const previousAgentStatusRef = useRef<Map<string, AgentLifecycleStatus>>(new Map());
  const sendAgentMessageRef = useRef<
    | ((
        agentId: string,
        message: string,
        images?: AttachmentMetadata[],
        attachments?: AgentAttachment[],
      ) => Promise<void>)
    | null
  >(null);
  const attentionNotifiedRef = useRef<Map<string, number>>(new Map());
  const appStateRef = useRef(AppState.currentState);
  const revalidationTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const revalidationInFlightRef = useRef<Promise<void> | null>(null);
  const revalidationQueuedRef = useRef(false);
  const wasConnectedRef = useRef(isConnected);

  useEffect(() => {
    const subscription = AppState.addEventListener("change", (nextState) => {
      appStateRef.current = nextState;
    });

    return () => {
      subscription.remove();
    };
  }, []);

  useEffect(() => {
    previousAgentStatusRef.current = reconcilePreviousAgentStatuses(
      previousAgentStatusRef.current,
      sessionAgents,
    );
  }, [sessionAgents]);

  const hydrateWorkspaces = useCallback(
    createHydrateWorkspaces({
      client,
      isConnected,
      serverId,
      setWorkspaces,
      setHasHydratedWorkspaces,
    }),
    [client, isConnected, serverId, setHasHydratedWorkspaces, setWorkspaces],
  );

  const applyAuthoritativeAgentSnapshot = useCallback(
    createApplyAuthoritativeAgentSnapshot({
      serverId,
      queryClient,
      setAgents,
      setAgentLastActivity,
      setPendingPermissions,
      setQueuedMessages,
      sendAgentMessageRef,
      previousAgentStatusRef,
    }),
    [
      queryClient,
      serverId,
      setAgentLastActivity,
      setAgents,
      setPendingPermissions,
      setQueuedMessages,
    ],
  );

  const runAuthoritativeRevalidation = useCallback(
    createRunAuthoritativeRevalidation({
      serverId,
      hydrateWorkspaces,
    }),
    [hydrateWorkspaces, serverId],
  );

  const flushAuthoritativeRevalidation = useCallback(
    createFlushAuthoritativeRevalidation({
      client,
      isConnected,
      serverId,
      runAuthoritativeRevalidation,
      revalidationInFlightRef,
      revalidationQueuedRef,
      revalidationTimerRef,
    }),
    [client, isConnected, runAuthoritativeRevalidation, serverId],
  );

  const scheduleAuthoritativeRevalidation = useCallback(
    createScheduleAuthoritativeRevalidation({
      client,
      isConnected,
      flushAuthoritativeRevalidation,
      revalidationTimerRef,
      revalidationQueuedRef,
    }),
    [client, flushAuthoritativeRevalidation, isConnected],
  );

  const handleAppResumed = useCallback(
    createHandleAppResumed({
      serverId,
      client,
      scheduleAuthoritativeRevalidation,
      bumpHistorySyncGeneration,
    }),
    [bumpHistorySyncGeneration, client, scheduleAuthoritativeRevalidation, serverId],
  );

  // Client activity tracking (heartbeat, push token registration)
  useClientActivity({ client, focusedAgentId, onAppResumed: handleAppResumed });
  usePushTokenRegistration({ client, serverId });

  const notifyAgentAttention = useCallback(
    createNotifyAgentAttention({
      serverId,
      appStateRef,
      attentionNotifiedRef,
      getSessionState: () => useSessionStore.getState().sessions[serverId],
      isAppActivelyVisible: getIsAppActivelyVisible,
      sendNotification: sendOsNotification,
    }),
    [serverId],
  );

  // Initialize session in store
  useEffect(() => {
    initializeSession(serverId, client);
  }, [serverId, client, initializeSession]);

  useEffect(() => {
    updateSessionClient(serverId, client);
  }, [serverId, client, updateSessionClient]);

  useEffect(() => {
    const serverInfo = client.getLastServerInfoMessage();
    if (!serverInfo) {
      return;
    }

    updateSessionServerInfo(serverId, {
      serverId: serverInfo.serverId,
      hostname: serverInfo.hostname,
      version: serverInfo.version,
      ...(serverInfo.capabilities ? { capabilities: serverInfo.capabilities } : {}),
      ...(serverInfo.features ? { features: serverInfo.features } : {}),
    });
  }, [client, serverId, updateSessionServerInfo]);

  useEffect(() => {
    if (!isConnected) {
      return;
    }

    const serverInfo = client.getLastServerInfoMessage();
    if (!serverInfo?.features?.providersSnapshot) {
      return;
    }

    prefetchProvidersSnapshot(serverId, client);
  }, [client, isConnected, serverId]);

  // If the client dropped mid-initialization, clear pending flags
  useEffect(() => {
    if (!isConnected) {
      flushAgentLastActivity();
      clearPendingAgentUpdates(serverId);
      clearInitDeferredsForServer(serverId, new Error("Disconnected"));
      setInitializingAgents(serverId, new Map());
    }
  }, [flushAgentLastActivity, serverId, isConnected, setInitializingAgents]);

  useEffect(() => {
    if (!client || !isConnected) {
      return;
    }

    let cancelled = false;
    void (async () => {
      try {
        await hydrateWorkspaces({
          subscribe: true,
          isCancelled: () => cancelled,
        });
      } catch (error) {
        console.error("[Session] Failed to hydrate workspaces:", error);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [client, hydrateWorkspaces, isConnected]);

  const applyAgentUpdatePayload = useCallback(
    createApplyAgentUpdatePayload({
      serverId,
      queryClient,
      setAgents,
      setPendingPermissions,
      setQueuedMessages,
      setAgentTimelineCursor,
      setAgentAuthoritativeHistoryApplied,
      applyAuthoritativeAgentSnapshot,
      previousAgentStatusRef,
    }),
    [
      applyAuthoritativeAgentSnapshot,
      queryClient,
      serverId,
      setAgentAuthoritativeHistoryApplied,
      setAgents,
      setAgentTimelineCursor,
      setPendingPermissions,
      setQueuedMessages,
    ],
  );

  const applyWorkspaceSetupProgress = useCallback(
    createApplyWorkspaceSetupProgress({
      serverId,
      upsertWorkspaceSetupProgress,
    }),
    [serverId, upsertWorkspaceSetupProgress],
  );

  const requestCanonicalCatchUp = useCallback(
    createRequestCanonicalCatchUp({ client }),
    [client],
  );

  const applyTimelineResponse = useCallback(
    createApplyTimelineResponse({
      serverId,
      applyAuthoritativeAgentSnapshot,
      applyAgentUpdatePayload,
      requestCanonicalCatchUp,
      setInitializingAgents,
      setAgentStreamTail,
      setAgentStreamHead,
      clearAgentStreamHead,
      setAgentTimelineCursor,
      setAgentAuthoritativeHistoryApplied,
      markAgentHistorySynchronized,
    }),
    [
      applyAuthoritativeAgentSnapshot,
      applyAgentUpdatePayload,
      clearAgentStreamHead,
      markAgentHistorySynchronized,
      requestCanonicalCatchUp,
      serverId,
      setAgentAuthoritativeHistoryApplied,
      setAgentStreamHead,
      setAgentStreamTail,
      setAgentTimelineCursor,
      setInitializingAgents,
    ],
  );

  useEffect(() => {
    if (isConnected) {
      return;
    }
    clearPendingAgentUpdates(serverId);
  }, [isConnected, serverId]);

  useEffect(() => {
    const wasConnected = wasConnectedRef.current;
    wasConnectedRef.current = isConnected;
    if (!wasConnected && isConnected) {
      scheduleAuthoritativeRevalidation();
    }
  }, [isConnected, scheduleAuthoritativeRevalidation]);

  useEffect(() => {
    return () => {
      if (revalidationTimerRef.current) {
        clearTimeout(revalidationTimerRef.current);
      }
    };
  }, []);

  // Daemon message handlers - directly update Zustand store
  useEffect(() => {
    return subscribeToSessionEvents({
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
      setAgentStreamHead,
      setAgentStreamState,
      clearAgentStreamHead,
      setAgentTimelineCursor,
      setInitializingAgents,
      setAgents,
      setWorkspaces,
      mergeWorkspaces,
      removeWorkspace,
      removeWorkspaceSetup,
      setPendingPermissions,
      clearDraftInput,
      updateSessionServerInfo,
    });
  }, [
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
    setAgentStreamHead,
    setAgentStreamState,
    clearAgentStreamHead,
    setAgentTimelineCursor,
    setInitializingAgents,
    setAgents,
    setWorkspaces,
    mergeWorkspaces,
    removeWorkspace,
    removeWorkspaceSetup,
    setPendingPermissions,
    clearDraftInput,
    updateSessionServerInfo,
  ]);

  const sendAgentMessage = useCallback(
    createSendAgentMessage({
      serverId,
      client,
      setAgentStreamHead,
      setAgentStreamTail,
      getCurrentHead: (agentId) =>
        useSessionStore.getState().sessions[serverId]?.agentStreamHead?.get(agentId),
      getAgent: (agentId) => {
        const agent = useSessionStore.getState().sessions[serverId]?.agents?.get(agentId);
        return agent ? { status: agent.status, persistence: agent.persistence ?? null } : undefined;
      },
    }),
    [serverId, client, setAgentStreamHead, setAgentStreamTail],
  );

  // Keep the ref updated so the agent_update handler can call it
  sendAgentMessageRef.current = sendAgentMessage;

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      clearWorkspaceSetupServer(serverId);
      clearSession(serverId);
    };
  }, [clearSession, clearWorkspaceSetupServer, serverId]);

  return children;
}
