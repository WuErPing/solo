import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useRef } from "react";
import type { UsageQuotaSnapshot } from "@server/client/usage-rpc";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";

export interface UsageResult {
  snapshots: UsageQuotaSnapshot[];
  errors: Record<string, string>;
  cachedAt: string | null;
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: (options?: { forceRefresh?: boolean }) => void;
}

export function usageQueryKey(serverId: string | null): readonly string[] {
  return ["usage", serverId ?? ""];
}

export function useUsage(options: {
  serverId?: string | null;
  enabled?: boolean;
}): UsageResult {
  const serverId = useMemo(() => {
    const value = options.serverId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.serverId]);
  const enabled = options.enabled ?? true;
  const client = useHostRuntimeClient(serverId ?? "");
  const isConnected = useHostRuntimeIsConnected(serverId ?? "");
  const queryKey = useMemo(() => usageQueryKey(serverId), [serverId]);
  const forceRefreshRef = useRef(false);

  const query = useQuery<
    {
      snapshots: UsageQuotaSnapshot[];
      errors: Record<string, string>;
      cachedAt: string | null;
      error: string | null;
    },
    Error
  >({
    queryKey,
    enabled: Boolean(enabled && serverId && client && isConnected),
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
      };
    },
  });

  const { data, isLoading, isFetching, refetch, error: queryError } = query;

  const refreshAll = useCallback(
    (refreshOptions?: { forceRefresh?: boolean }) => {
      if (!serverId || !client || !isConnected) {
        return;
      }
      if (refreshOptions?.forceRefresh) {
        forceRefreshRef.current = true;
      }
      void refetch().finally(() => {
        forceRefreshRef.current = false;
      });
    },
    [client, isConnected, refetch, serverId],
  );

  const snapshots = data?.snapshots ?? [];
  const errors = data?.errors ?? {};
  const cachedAt = data?.cachedAt ?? null;
  const rpcError = data?.error ?? null;
  const isInitialLoad = isLoading && snapshots.length === 0;
  const isRevalidating = isFetching && !isLoading && snapshots.length > 0;

  return {
    snapshots,
    errors,
    cachedAt,
    isLoading,
    isInitialLoad,
    isRevalidating,
    error: rpcError ?? (queryError ? queryError.message : null),
    refreshAll,
  };
}
