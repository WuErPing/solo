import { useQueries, useQueryClient } from "@tanstack/react-query";
import { useMemo, useCallback } from "react";
import type { ScheduleSummary } from "@server/server/schedule/types";
import { getHostRuntimeStore, useHosts, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { schedulesQueryKey } from "./use-schedules";

export interface AggregatedSchedule extends ScheduleSummary {
  serverId: string;
  serverLabel: string;
}

export interface AggregatedSchedulesResult {
  schedules: AggregatedSchedule[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

export function useAggregatedSchedules(): AggregatedSchedulesResult {
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
        queryKey: schedulesQueryKey(host.serverId),
        enabled: Boolean(client && isConnected),
        staleTime: 30_000,
        queryFn: async () => {
          if (!client) {
            throw new Error("Daemon client not available");
          }
          const payload = await client.scheduleList();
          return {
            schedules: payload.schedules ?? [],
            error: payload.error ?? null,
            serverId: host.serverId,
            serverLabel: host.label,
          };
        },
      };
    }),
  });

  const result = useMemo(() => {
    const allSchedules: AggregatedSchedule[] = [];
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
      if (query.data?.schedules) {
        for (const schedule of query.data.schedules) {
          allSchedules.push({
            ...schedule,
            serverId: host.serverId,
            serverLabel: host.label,
          });
        }
      }
    }

    // Sort by status (active first), then by name
    allSchedules.sort((left, right) => {
      const statusOrder = { active: 0, paused: 1, completed: 2 };
      const leftOrder = statusOrder[left.status] ?? 3;
      const rightOrder = statusOrder[right.status] ?? 3;
      if (leftOrder !== rightOrder) {
        return leftOrder - rightOrder;
      }
      return (left.name ?? "").localeCompare(right.name ?? "");
    });

    const hasAnyData = allSchedules.length > 0;
    const isInitialLoad = isLoading && !hasAnyData;
    const isRevalidating = isFetching && !isLoading && hasAnyData;

    return {
      schedules: allSchedules,
      isLoading,
      isInitialLoad,
      isRevalidating,
      error: anyError,
    };
  }, [queries, connectedHosts]);

  const refreshAll = useCallback(() => {
    for (const host of connectedHosts) {
      void queryClient.invalidateQueries({ queryKey: schedulesQueryKey(host.serverId) });
    }
  }, [connectedHosts, queryClient]);

  return {
    ...result,
    refreshAll,
  };
}
