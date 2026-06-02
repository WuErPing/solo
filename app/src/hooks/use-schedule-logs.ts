import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import type { ScheduleRun } from "@server/server/schedule/types";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";

export interface ScheduleLogsOptions {
  serverId?: string | null;
  scheduleId?: string | null;
  enabled?: boolean;
}

export interface ScheduleLogsResult {
  runs: ScheduleRun[];
  isLoading: boolean;
  isRevalidating: boolean;
  error: string | null;
  refresh: () => void;
}

export function scheduleLogsQueryKey(
  serverId: string | null,
  scheduleId: string | null,
): readonly string[] {
  return ["schedule-logs", serverId ?? "", scheduleId ?? ""];
}

export function useScheduleLogs(options: ScheduleLogsOptions): ScheduleLogsResult {
  const serverId = useMemo(() => {
    const value = options.serverId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.serverId]);

  const scheduleId = useMemo(() => {
    const value = options.scheduleId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.scheduleId]);

  const enabled = options.enabled ?? true;
  const client = useHostRuntimeClient(serverId ?? "");
  const isConnected = useHostRuntimeIsConnected(serverId ?? "");
  const queryKey = useMemo(
    () => scheduleLogsQueryKey(serverId, scheduleId),
    [serverId, scheduleId],
  );

  const query = useQuery<{ runs: ScheduleRun[]; error: string | null }, Error>({
    queryKey,
    enabled: Boolean(enabled && serverId && scheduleId && client && isConnected),
    staleTime: 30_000,
    queryFn: async () => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.scheduleLogs({ id: scheduleId! });
      return {
        runs: payload.runs ?? [],
        error: payload.error ?? null,
      };
    },
  });

  const { data, isLoading, isFetching, refetch } = query;

  const refresh = useCallback(() => {
    if (!serverId || !scheduleId || !client || !isConnected) {
      return;
    }
    void refetch();
  }, [client, isConnected, refetch, serverId, scheduleId]);

  const runs = data?.runs ?? [];
  const rpcError = data?.error ?? null;
  const isRevalidating = isFetching && !isLoading && runs.length > 0;

  return {
    runs,
    isLoading,
    isRevalidating,
    error: rpcError ?? (query.error ? query.error.message : null),
    refresh,
  };
}
