import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState } from "react";
import { getHostRuntimeStore, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { useAppVisible } from "@/hooks/use-app-visible";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

const DEFAULT_SCROLLBACK_LINES = 200;
const SCROLLBACK_INCREMENT = 200;
const MAX_SCROLLBACK_LINES = 5000;

export interface TmuxCapturePaneResult {
  content: string;
  isLoading: boolean;
  isLoadingMore: boolean;
  error: string | null;
  refetch: () => void;
  scrollbackLines: number;
  loadMoreHistory: () => void;
  hasMoreHistory: boolean;
  autoRefresh: boolean;
  setAutoRefresh: (value: boolean) => void;
}

export function tmuxCapturePaneQueryKey(
  serverId: string,
  paneId: string,
  scrollbackLines: number,
): readonly (string | number)[] {
  return ["tmux-capture-pane", serverId, paneId, scrollbackLines];
}

export function useTmuxCapturePane(
  serverId: string,
  paneId: string,
  enabled: boolean,
): TmuxCapturePaneResult {
  const [scrollbackLines, setScrollbackLines] = useState(DEFAULT_SCROLLBACK_LINES);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const store = getHostRuntimeStore();
  const client = store.getClient(serverId);
  const snapshot = store.getSnapshot(serverId);
  const isConnected = isHostRuntimeConnected(snapshot);
  const isAppVisible = useAppVisible();

  // Reset scrollback when pane changes
  useEffect(() => {
    setScrollbackLines(DEFAULT_SCROLLBACK_LINES);
  }, [paneId]);

  const { data, isLoading, isFetching, error, refetch } = useQuery({
    queryKey: tmuxCapturePaneQueryKey(serverId, paneId, scrollbackLines),
    enabled: enabled && Boolean(client) && isConnected,
    staleTime: 5_000,
    refetchInterval: enabled && isAppVisible && autoRefresh ? 5_000 : false,
    placeholderData: keepPreviousData,
    retry: 1,
    queryFn: async () => {
      const payload = await withLiveTmuxClient(serverId, (c) =>
        c.tmuxCapturePane(paneId, -scrollbackLines),
      );
      return {
        content: payload.content ?? "",
        error: payload.error ?? null,
      };
    },
  });

  const loadMoreHistory = useCallback(() => {
    if (scrollbackLines >= MAX_SCROLLBACK_LINES) return;
    setScrollbackLines((prev) => Math.min(prev + SCROLLBACK_INCREMENT, MAX_SCROLLBACK_LINES));
  }, [scrollbackLines]);

  const isLoadingMore = useMemo(() => isFetching && !!data, [isFetching, data]);
  const hasMoreHistory = scrollbackLines < MAX_SCROLLBACK_LINES;

  return useMemo(
    () => ({
      content: data?.content ?? "",
      isLoading,
      isLoadingMore,
      error:
        data?.error ??
        (error instanceof Error && error.message !== "Daemon client not available"
          ? error.message
          : null),
      refetch: () => {
        void refetch();
      },
      scrollbackLines,
      loadMoreHistory,
      hasMoreHistory,
      autoRefresh,
      setAutoRefresh,
    }),
    [data, isLoading, isLoadingMore, error, refetch, scrollbackLines, loadMoreHistory, hasMoreHistory, autoRefresh],
  );
}
