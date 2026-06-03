import { useQueries, useQueryClient } from "@tanstack/react-query";
import { useMemo, useCallback } from "react";
import { getHostRuntimeStore, useHosts, isHostRuntimeConnected } from "@/runtime/host-runtime";

export interface TmuxAgent {
  sessionName: string;
  windowName: string;
  paneId: string;
  paneIndex: number;
  panePid: number;
  agentName: string;
  currentCmd: string;
  workingDir: string;
  serverId: string;
  serverLabel: string;
}

export interface AggregatedTmuxAgentsResult {
  agents: TmuxAgent[];
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

  const connectedHosts = useMemo(() => {
    const store = getHostRuntimeStore();
    return hosts.filter((host) => {
      const snapshot = store.getSnapshot(host.serverId);
      return isHostRuntimeConnected(snapshot);
    });
  }, [hosts]);

  const queries = useQueries({
    queries: connectedHosts.map((host) => {
      const store = getHostRuntimeStore();
      const client = store.getClient(host.serverId);
      const snapshot = store.getSnapshot(host.serverId);
      const isConnected = isHostRuntimeConnected(snapshot);

      return {
        queryKey: tmuxAgentsQueryKey(host.serverId),
        enabled: Boolean(client && isConnected),
        staleTime: 30_000,
        queryFn: async () => {
          if (!client) {
            throw new Error("Daemon client not available");
          }
          const payload = await client.tmuxListAgents();
          return {
            agents: payload.agents ?? [],
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
    let anyError: string | null = null;
    let isLoading = false;
    let isFetching = false;

    for (let i = 0; i < queries.length; i++) {
      const query = queries[i];
      if (!query) continue;

      const host = connectedHosts[i];
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
      if (query.data?.agents) {
        for (const agent of query.data.agents) {
          allAgents.push({
            ...agent,
            serverId: host.serverId,
            serverLabel: host.label,
          });
        }
      }
    }

    // Sort by agentName, then by sessionName
    allAgents.sort((left, right) => {
      const nameCmp = left.agentName.localeCompare(right.agentName);
      if (nameCmp !== 0) return nameCmp;
      return left.sessionName.localeCompare(right.sessionName);
    });

    const hasAnyData = allAgents.length > 0;
    const isInitialLoad = isLoading && !hasAnyData;
    const isRevalidating = isFetching && !isLoading && hasAnyData;

    return {
      agents: allAgents,
      isLoading,
      isInitialLoad,
      isRevalidating,
      error: anyError,
    };
  }, [queries, connectedHosts]);

  const refreshAll = useCallback(() => {
    for (const host of connectedHosts) {
      void queryClient.invalidateQueries({ queryKey: tmuxAgentsQueryKey(host.serverId) });
    }
  }, [connectedHosts, queryClient]);

  return {
    ...result,
    refreshAll,
  };
}
