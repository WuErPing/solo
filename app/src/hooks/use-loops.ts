import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import type { LoopListItem } from "@server/server/loop/rpc-schemas";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";

export interface LoopsResult {
  loops: LoopListItem[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

export function loopsQueryKey(serverId: string | null): readonly string[] {
  return ["loops", serverId ?? ""];
}

export function useLoops(options: {
  serverId?: string | null;
  enabled?: boolean;
}): LoopsResult {
  const serverId = useMemo(() => {
    const value = options.serverId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.serverId]);
  const enabled = options.enabled ?? true;
  const client = useHostRuntimeClient(serverId ?? "");
  const isConnected = useHostRuntimeIsConnected(serverId ?? "");
  const queryKey = useMemo(() => loopsQueryKey(serverId), [serverId]);

  const query = useQuery<{ loops: LoopListItem[]; error: string | null }, Error>({
    queryKey,
    enabled: Boolean(enabled && serverId && client && isConnected),
    staleTime: 30_000,
    queryFn: async () => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopList();
      return {
        loops: payload.loops ?? [],
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

  const loops = data?.loops ?? [];
  const rpcError = data?.error ?? null;
  const isInitialLoad = isLoading && loops.length === 0;
  const isRevalidating = isFetching && !isLoading && loops.length > 0;

  return {
    loops,
    isLoading,
    isInitialLoad,
    isRevalidating,
    error: rpcError ?? (queryError ? queryError.message : null),
    refreshAll,
  };
}
