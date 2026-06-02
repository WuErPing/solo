import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import type { ScheduleSummary } from "@server/server/schedule/types";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";

export interface SchedulesResult {
  schedules: ScheduleSummary[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

export function schedulesQueryKey(serverId: string | null): readonly string[] {
  return ["schedules", serverId ?? ""];
}

export function useSchedules(options: {
  serverId?: string | null;
  enabled?: boolean;
}): SchedulesResult {
  const serverId = useMemo(() => {
    const value = options.serverId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.serverId]);
  const enabled = options.enabled ?? true;
  const client = useHostRuntimeClient(serverId ?? "");
  const isConnected = useHostRuntimeIsConnected(serverId ?? "");
  const queryKey = useMemo(() => schedulesQueryKey(serverId), [serverId]);

  const query = useQuery<{ schedules: ScheduleSummary[]; error: string | null }, Error>({
    queryKey,
    enabled: Boolean(enabled && serverId && client && isConnected),
    staleTime: 30_000,
    queryFn: async () => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.scheduleList();
      return {
        schedules: payload.schedules ?? [],
        error: payload.error ?? null,
      };
    },
  });

  const { data, isLoading, isFetching, refetch, error: queryError } = query;

  const refreshAll = useCallback(() => {
    if (!serverId || !client || !isConnected) {
      return;
    }
    void refetch();
  }, [client, isConnected, refetch, serverId]);

  const schedules = data?.schedules ?? [];
  const rpcError = data?.error ?? null;
  const isInitialLoad = isLoading && schedules.length === 0;
  const isRevalidating = isFetching && !isLoading && schedules.length > 0;

  return {
    schedules,
    isLoading,
    isInitialLoad,
    isRevalidating,
    error: rpcError ?? (queryError ? queryError.message : null),
    refreshAll,
  };
}
