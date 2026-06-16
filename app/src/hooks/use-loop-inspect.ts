import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import type { LoopRecord } from "@server/server/loop/rpc-schemas";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";

export interface LoopInspectOptions {
  serverId?: string | null;
  loopId?: string | null;
  enabled?: boolean;
}

export interface LoopInspectResult {
  loop: LoopRecord | null;
  isLoading: boolean;
  isRevalidating: boolean;
  error: string | null;
  refresh: () => void;
}

export function loopInspectQueryKey(
  serverId: string | null,
  loopId: string | null,
): readonly string[] {
  return ["loop-inspect", serverId ?? "", loopId ?? ""];
}

export function useLoopInspect(options: LoopInspectOptions): LoopInspectResult {
  const serverId = useMemo(() => {
    const value = options.serverId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.serverId]);

  const loopId = useMemo(() => {
    const value = options.loopId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.loopId]);

  const enabled = options.enabled ?? true;
  const client = useHostRuntimeClient(serverId ?? "");
  const isConnected = useHostRuntimeIsConnected(serverId ?? "");
  const queryKey = useMemo(() => loopInspectQueryKey(serverId, loopId), [serverId, loopId]);

  const query = useQuery<{ loop: LoopRecord | null; error: string | null }, Error>({
    queryKey,
    enabled: Boolean(enabled && serverId && loopId && client && isConnected),
    staleTime: 30_000,
    refetchInterval: 2_000,
    queryFn: async () => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopInspect({ id: loopId! });
      return {
        loop: payload.loop ?? null,
        error: payload.error ?? null,
      };
    },
  });

  const { data, isLoading, isFetching, refetch } = query;

  const refresh = useCallback(() => {
    if (!serverId || !loopId || !client || !isConnected) {
      return;
    }
    void refetch();
  }, [client, isConnected, refetch, serverId, loopId]);

  const loop = data?.loop ?? null;
  const rpcError = data?.error ?? null;
  const isRevalidating = isFetching && !isLoading && loop !== null;

  return {
    loop,
    isLoading,
    isRevalidating,
    error: rpcError ?? (query.error ? query.error.message : null),
    refresh,
  };
}
