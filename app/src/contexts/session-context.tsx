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
import { useToast } from "@/contexts/toast-context";
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
  const toast = useToast();

  // Zustand store actions
  const initializeSession = useSessionStore((state) => state.initializeSession);
  const clearSession = useSessionStore((state) => state.clearSession);
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
  const setFileExplorer = useSessionStore((state) => state.setFileExplorer);
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
    (...args: Parameters<ReturnType<typeof createHydrateWorkspaces>>) =>
      createHydrateWorkspaces({
        client,
        isConnected,
        serverId,
        setWorkspaces,
        setHasHydratedWorkspaces,
      })(...args),
    [client, isConnected, serverId, setHasHydratedWorkspaces, setWorkspaces],
  );

  const applyAuthoritativeAgentSnapshot = useCallback(
    (...args: Parameters<ReturnType<typeof createApplyAuthoritativeAgentSnapshot>>) =>
      createApplyAuthoritativeAgentSnapshot({
        serverId,
        queryClient,
        setAgents,
        setAgentLastActivity,
        setPendingPermissions,
        setQueuedMessages,
        sendAgentMessageRef,
        previousAgentStatusRef,
      })(...args),
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
    (...args: Parameters<ReturnType<typeof createRunAuthoritativeRevalidation>>) =>
      createRunAuthoritativeRevalidation({
        serverId,
        hydrateWorkspaces,
      })(...args),
    [hydrateWorkspaces, serverId],
  );

  const flushAuthoritativeRevalidation = useCallback(
    (...args: Parameters<ReturnType<typeof createFlushAuthoritativeRevalidation>>) =>
      createFlushAuthoritativeRevalidation({
        client,
        isConnected,
        serverId,
        runAuthoritativeRevalidation,
        revalidationInFlightRef,
        revalidationQueuedRef,
        revalidationTimerRef,
      })(...args),
    [client, isConnected, runAuthoritativeRevalidation, serverId],
  );

  const scheduleAuthoritativeRevalidation = useCallback(
    (...args: Parameters<ReturnType<typeof createScheduleAuthoritativeRevalidation>>) =>
      createScheduleAuthoritativeRevalidation({
        client,
        isConnected,
        flushAuthoritativeRevalidation,
        revalidationTimerRef,
        revalidationQueuedRef,
      })(...args),
    [client, flushAuthoritativeRevalidation, isConnected],
  );

  const handleAppResumed = useCallback(
    (...args: Parameters<ReturnType<typeof createHandleAppResumed>>) =>
      createHandleAppResumed({
        serverId,
        client,
        scheduleAuthoritativeRevalidation,
        bumpHistorySyncGeneration,
      })(...args),
    [bumpHistorySyncGeneration, client, scheduleAuthoritativeRevalidation, serverId],
  );

  // Client activity tracking (heartbeat, push token registration)
  useClientActivity({ client, focusedAgentId, onAppResumed: handleAppResumed });
  usePushTokenRegistration({ client, serverId });

  const notifyAgentAttention = useCallback(
    (...args: Parameters<ReturnType<typeof createNotifyAgentAttention>>) =>
      createNotifyAgentAttention({
        serverId,
        appStateRef,
        attentionNotifiedRef,
        getSessionState: () => useSessionStore.getState().sessions[serverId],
        isAppActivelyVisible: getIsAppActivelyVisible,
        sendNotification: sendOsNotification,
      })(...args),
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
    (...args: Parameters<ReturnType<typeof createApplyAgentUpdatePayload>>) =>
      createApplyAgentUpdatePayload({
        serverId,
        queryClient,
        setAgents,
        setPendingPermissions,
        setQueuedMessages,
        setAgentTimelineCursor,
        setAgentAuthoritativeHistoryApplied,
        setAgentStreamTail,
        setAgentStreamHead,
        setInitializingAgents,
        setFileExplorer,
        applyAuthoritativeAgentSnapshot,
        previousAgentStatusRef,
      })(...args),
    [
      applyAuthoritativeAgentSnapshot,
      queryClient,
      serverId,
      setAgentAuthoritativeHistoryApplied,
      setAgents,
      setAgentStreamHead,
      setAgentStreamTail,
      setAgentTimelineCursor,
      setFileExplorer,
      setInitializingAgents,
      setPendingPermissions,
      setQueuedMessages,
    ],
  );

  const applyWorkspaceSetupProgress = useCallback(
    (...args: Parameters<ReturnType<typeof createApplyWorkspaceSetupProgress>>) =>
      createApplyWorkspaceSetupProgress({
        serverId,
        upsertWorkspaceSetupProgress,
      })(...args),
    [serverId, upsertWorkspaceSetupProgress],
  );

  const requestCanonicalCatchUp = useCallback(
    (...args: Parameters<ReturnType<typeof createRequestCanonicalCatchUp>>) =>
      createRequestCanonicalCatchUp({ client })(...args),
    [client],
  );

  const applyTimelineResponse = useCallback(
    (...args: Parameters<ReturnType<typeof createApplyTimelineResponse>>) =>
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
      })(...args),
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
      // revalidationTimerRef holds a setTimeout id, not a React node.
      // eslint-disable-next-line react-hooks/exhaustive-deps
      const timer = revalidationTimerRef.current;
      if (timer) {
        clearTimeout(timer);
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
      setFileExplorer,
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
    setFileExplorer,
    clearDraftInput,
    updateSessionServerInfo,
  ]);

  const sendAgentMessage = useCallback(
    (...args: Parameters<ReturnType<typeof createSendAgentMessage>>) =>
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
        onError: (error) => {
          toast.error(error instanceof Error ? error.message : "Failed to send message");
        },
      })(...args),
    [serverId, client, setAgentStreamHead, setAgentStreamTail, toast],
  );

  // Keep the ref updated so the agent_update handler can call it
  useEffect(() => {
    sendAgentMessageRef.current = sendAgentMessage;
  });

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      clearWorkspaceSetupServer(serverId);
      clearSession(serverId);
    };
  }, [clearSession, clearWorkspaceSetupServer, serverId]);

  return children;
}
