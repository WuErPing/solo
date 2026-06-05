import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { getHostRuntimeStore, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { useAppVisible } from "@/hooks/use-app-visible";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

export interface TmuxCapturePaneResult {
  content: string;
  isLoading: boolean;
  error: string | null;
  refetch: () => void;
}

export function tmuxCapturePaneQueryKey(
  serverId: string,
  paneId: string,
): readonly string[] {
  return ["tmux-capture-pane", serverId, paneId];
}

export function useTmuxCapturePane(
  serverId: string,
  paneId: string,
  enabled: boolean,
): TmuxCapturePaneResult {
  const store = getHostRuntimeStore();
  const client = store.getClient(serverId);
  const snapshot = store.getSnapshot(serverId);
  const isConnected = isHostRuntimeConnected(snapshot);
  const isAppVisible = useAppVisible();

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: tmuxCapturePaneQueryKey(serverId, paneId),
    enabled: enabled && Boolean(client) && isConnected,
    staleTime: 5_000,
    refetchInterval: enabled && isAppVisible ? 5_000 : false,
    placeholderData: keepPreviousData,
    retry: 1,
    queryFn: async () => {
      const payload = await withLiveTmuxClient(serverId, (c) => c.tmuxCapturePane(paneId));
      return {
        content: payload.content ?? "",
        error: payload.error ?? null,
      };
    },
  });

  return useMemo(
    () => ({
      content: data?.content ?? "",
      isLoading,
      error:
        data?.error ??
        (error instanceof Error && error.message !== "Daemon client not available"
          ? error.message
          : null),
      refetch: () => {
        void refetch();
      },
    }),
    [data, isLoading, error, refetch],
  );
}
