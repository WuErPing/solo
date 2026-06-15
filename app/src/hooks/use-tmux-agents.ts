import { keepPreviousData, useQueries, useQueryClient } from "@tanstack/react-query";
import { useMemo, useCallback, useRef } from "react";
import { getHostRuntimeStore, useHosts, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

export interface TmuxAgent {
  sessionName: string;
  windowName: string;
  paneId: string;
  paneIndex: number;
  panePid: number;
  agentName: string;
  currentCmd: string;
  workingDir: string;
  title?: string;
  status?: string;
  activity?: string;
  serverId: string;
  serverLabel: string;
}

export interface TmuxPane {
  sessionName: string;
  windowName: string;
  paneId: string;
  paneIndex: number;
  panePid: number;
  currentCmd: string;
  workingDir: string;
  title?: string;
  serverId: string;
  serverLabel: string;
}

export type TmuxAgentOrPane = TmuxAgent | TmuxPane;

export interface AgentCommandEntry {
  agentName: string;
  launchCmd: string;
  lastSeen: string;
}

export interface AggregatedTmuxAgentsResult {
  agents: TmuxAgent[];
  otherPanes: TmuxPane[];
  commandHistory: AgentCommandEntry[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

export function tmuxAgentsQueryKey(serverId: string): readonly string[] {
  return ["tmux-agents", serverId];
}

export function useAggregatedTmuxAgents(): AggregatedTmuxAgentsResult {
  const hosts = useHosts();
  const queryClient = useQueryClient();
  const hasFetched = useRef(false);

  const queries = useQueries({
    queries: hosts.map((host) => {
      const store = getHostRuntimeStore();
      const client = store.getClient(host.serverId);
      const snapshot = store.getSnapshot(host.serverId);
      const isConnected = isHostRuntimeConnected(snapshot);

      return {
        queryKey: tmuxAgentsQueryKey(host.serverId),
        enabled: Boolean(client && isConnected),
        placeholderData: keepPreviousData,
        refetchInterval: 5000,
        retry: 1,
        queryFn: async () => {
          const payload = await withLiveTmuxClient(host.serverId, (c) => c.tmuxListAgents());
          return {
            agents: payload.agents ?? [],
            otherPanes: payload.otherPanes ?? [],
            commandHistory: payload.commandHistory ?? [],
            error: payload.error ?? null,
            serverId: host.serverId,
            serverLabel: host.label,
          };
        },
      };
    }),
  });

  const result = useMemo(() => {
    const allAgents: TmuxAgent[] = [];
    const allOtherPanes: TmuxPane[] = [];
    const allCommandHistory: AgentCommandEntry[] = [];
    let anyError: string | null = null;
    let isLoading = false;
    let isFetching = false;

    for (let i = 0; i < queries.length; i++) {
      const query = queries[i];
      if (!query) continue;

      const host = hosts[i];
      if (!host) continue;

      if (query.isLoading) {
        isLoading = true;
      }
      if (query.isFetching) {
        isFetching = true;
      }
      if (query.data?.error && !anyError) {
        anyError = query.data.error;
      }
      if (
        !anyError &&
        query.error instanceof Error &&
        query.error.message !== "Daemon client not available"
      ) {
        anyError = query.error.message;
      }
      if (query.data?.agents) {
        for (const agent of query.data.agents) {
          allAgents.push({
            ...agent,
            serverId: host.serverId,
            serverLabel: host.label,
          });
        }
      }
      if (query.data?.otherPanes) {
        for (const pane of query.data.otherPanes) {
          allOtherPanes.push({
            ...pane,
            serverId: host.serverId,
            serverLabel: host.label,
          });
        }
      }
      if (query.data?.commandHistory) {
        for (const entry of query.data.commandHistory) {
          allCommandHistory.push(entry);
        }
      }
    }

    // Sort by agentName, then by sessionName
    allAgents.sort((left, right) => {
      const nameCmp = left.agentName.localeCompare(right.agentName);
      if (nameCmp !== 0) return nameCmp;
      return left.sessionName.localeCompare(right.sessionName);
    });

    // Sort other panes by sessionName, then windowName
    allOtherPanes.sort((left, right) => {
      const sessionCmp = left.sessionName.localeCompare(right.sessionName);
      if (sessionCmp !== 0) return sessionCmp;
      return left.windowName.localeCompare(right.windowName);
    });

    // Sort command history by lastSeen descending.
    allCommandHistory.sort((a, b) => b.lastSeen.localeCompare(a.lastSeen));

    const hasAnyData = allAgents.length > 0 || allOtherPanes.length > 0;

    // Track whether any query has completed (success or error).
    // Once true, stays true — we only need to know the first fetch happened.
    if (!hasFetched.current) {
      for (const query of queries) {
        if (query && (query.isSuccess || query.isError)) {
          hasFetched.current = true;
          break;
        }
      }
    }

    const isInitialLoad = !hasAnyData && !hasFetched.current;
    const isRevalidating = isFetching && !isLoading && hasAnyData;

    return {
      agents: allAgents,
      otherPanes: allOtherPanes,
      commandHistory: allCommandHistory,
      isLoading,
      isInitialLoad,
      isRevalidating,
      error: anyError,
    };
  }, [queries, hosts]);

  const refreshAll = useCallback(() => {
    for (const host of hosts) {
      void queryClient.invalidateQueries({ queryKey: tmuxAgentsQueryKey(host.serverId) });
    }
  }, [hosts, queryClient]);

  return {
    ...result,
    refreshAll,
  };
}
