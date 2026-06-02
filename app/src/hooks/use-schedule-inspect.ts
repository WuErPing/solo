import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import type { StoredSchedule } from "@server/server/schedule/types";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";

export interface ScheduleInspectOptions {
  serverId?: string | null;
  scheduleId?: string | null;
  enabled?: boolean;
}

export interface ScheduleInspectResult {
  schedule: StoredSchedule | null;
  isLoading: boolean;
  isRevalidating: boolean;
  error: string | null;
  refresh: () => void;
}

export function scheduleInspectQueryKey(
  serverId: string | null,
  scheduleId: string | null,
): readonly string[] {
  return ["schedule-inspect", serverId ?? "", scheduleId ?? ""];
}

export function useScheduleInspect(options: ScheduleInspectOptions): ScheduleInspectResult {
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
    () => scheduleInspectQueryKey(serverId, scheduleId),
    [serverId, scheduleId],
  );

  const query = useQuery<{ schedule: StoredSchedule | null; error: string | null }, Error>({
    queryKey,
    enabled: Boolean(enabled && serverId && scheduleId && client && isConnected),
    staleTime: 30_000,
    queryFn: async () => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.scheduleInspect({ id: scheduleId! });
      return {
        schedule: payload.schedule ?? null,
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

  const schedule = data?.schedule ?? null;
  const rpcError = data?.error ?? null;
  const isRevalidating = isFetching && !isLoading && schedule !== null;

  return {
    schedule,
    isLoading,
    isRevalidating,
    error: rpcError ?? (query.error ? query.error.message : null),
    refresh,
  };
}
