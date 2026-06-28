import { useQueries, useQueryClient } from "@tanstack/react-query";
import { useMemo, useCallback, useRef, useSyncExternalStore } from "react";
import type { ScheduleSummary } from "@server/server/schedule/types";
import { getHostRuntimeStore, useHosts, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { schedulesQueryKey } from "./use-schedules";

export interface AggregatedSchedule extends ScheduleSummary {
  serverId: string;
  serverLabel: string;
}

export interface AggregatedSchedulesResult {
  schedules: AggregatedSchedule[];
  configuredHosts: { serverId: string; serverLabel: string; isConnected: boolean }[];
  connectedHosts: { serverId: string; serverLabel: string }[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

function useConnectedHostsSnapshot(
  hosts: { serverId: string; label: string }[],
): { serverId: string; serverLabel: string }[] {
  const lastSnapshot = useRef<{ serverId: string; serverLabel: string }[]>([]);

  const subscribe = useCallback(
    (callback: () => void) => {
      const store = getHostRuntimeStore();
      const unsubscribes = hosts.map((host) => store.subscribe(host.serverId, callback));
      return () => unsubscribes.forEach((unsub) => unsub());
    },
    [hosts],
  );

  return useSyncExternalStore(
    subscribe,
    () => {
      const store = getHostRuntimeStore();
      const connected = hosts
        .filter((host) => isHostRuntimeConnected(store.getSnapshot(host.serverId)))
        .map((host) => ({ serverId: host.serverId, serverLabel: host.label }));

      const previous = lastSnapshot.current;
      if (
        previous.length === connected.length &&
        previous.every(
          (host, index) =>
            host.serverId === connected[index]?.serverId &&
            host.serverLabel === connected[index]?.serverLabel,
        )
      ) {
        return previous;
      }

      lastSnapshot.current = connected;
      return connected;
    },
    () => [],
  );
}

export function useAggregatedSchedules(): AggregatedSchedulesResult {
  const hosts = useHosts();
  const queryClient = useQueryClient();

  const connectedHosts = useConnectedHostsSnapshot(hosts);
  const connectedHostIds = useMemo(
    () => new Set(connectedHosts.map((host) => host.serverId)),
    [connectedHosts],
  );

  const configuredHosts = useMemo(
    () =>
      hosts.map((host) => ({
        serverId: host.serverId,
        serverLabel: host.label,
        isConnected: connectedHostIds.has(host.serverId),
      })),
    [hosts, connectedHostIds],
  );

  const queries = useQueries({
    queries: hosts.map((host) => {
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

      const host = hosts[i];
      if (!host) continue;

      if (query.isLoading) {
        isLoading = true;
      }
      if (query.isFetching) {
        isFetching = true;
      }
      if (query.error instanceof Error && !anyError) {
        anyError = query.error.message;
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
  }, [queries, hosts]);

  const refreshAll = useCallback(() => {
    for (const host of connectedHosts) {
      void queryClient.invalidateQueries({ queryKey: schedulesQueryKey(host.serverId) });
    }
  }, [connectedHosts, queryClient]);

  return {
    ...result,
    configuredHosts,
    connectedHosts,
    refreshAll,
  };
}
