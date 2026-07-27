import { useQueries, useQueryClient } from "@tanstack/react-query";
import { useMemo, useCallback, useRef, useSyncExternalStore } from "react";
import type { UsageQuotaSnapshot } from "@server/client/usage-rpc";
import { getHostRuntimeStore, useHosts, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { usageQueryKey } from "./use-usage";

export interface AggregatedUsageGroup {
  serverId: string;
  serverLabel: string;
  snapshots: UsageQuotaSnapshot[];
  errors: Record<string, string>;
  cachedAt: string | null;
}

export interface AggregatedUsageResult {
  groups: AggregatedUsageGroup[];
  configuredHosts: { serverId: string; serverLabel: string; isConnected: boolean }[];
  connectedHosts: { serverId: string; serverLabel: string }[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: (options?: { forceRefresh?: boolean }) => void;
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

export function useAggregatedUsage(): AggregatedUsageResult {
  const hosts = useHosts();
  const queryClient = useQueryClient();
  const forceRefreshRef = useRef(false);

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
        queryKey: usageQueryKey(host.serverId),
        enabled: Boolean(client && isConnected),
        staleTime: 30_000,
        queryFn: async () => {
          if (!client) {
            throw new Error("Daemon client not available");
          }
          const payload = await client.usageQuotaList(
            forceRefreshRef.current ? { forceRefresh: true } : undefined,
          );
          return {
            snapshots: payload.snapshots ?? [],
            errors: payload.errors ?? {},
            cachedAt: payload.cachedAt ?? null,
            error: payload.error ?? null,
            serverId: host.serverId,
            serverLabel: host.label,
          };
        },
      };
    }),
  });

  const result = useMemo(() => {
    const groups: AggregatedUsageGroup[] = [];
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
      if (query.data) {
        groups.push({
          serverId: host.serverId,
          serverLabel: host.label,
          snapshots: query.data.snapshots ?? [],
          errors: query.data.errors ?? {},
          cachedAt: query.data.cachedAt ?? null,
        });
      }
    }

    const hasAnyData = groups.some((group) => group.snapshots.length > 0);
    const isInitialLoad = isLoading && !hasAnyData;
    const isRevalidating = isFetching && !isLoading && hasAnyData;

    return {
      groups,
      isLoading,
      isInitialLoad,
      isRevalidating,
      error: anyError,
    };
  }, [queries, hosts]);

  const refreshAll = useCallback(
    (options?: { forceRefresh?: boolean }) => {
      if (options?.forceRefresh) {
        forceRefreshRef.current = true;
      }
      const invalidations = connectedHosts.map((host) =>
        queryClient.invalidateQueries({ queryKey: usageQueryKey(host.serverId) }),
      );
      void Promise.all(invalidations).finally(() => {
        forceRefreshRef.current = false;
      });
    },
    [connectedHosts, queryClient],
  );

  return {
    ...result,
    configuredHosts,
    connectedHosts,
    refreshAll,
  };
}
