import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { getHostRuntimeStore, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { useAppVisible } from "@/hooks/use-app-visible";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

const DEFAULT_SCROLLBACK_LINES = 200;
const SCROLLBACK_INCREMENT = 200;
const MAX_SCROLLBACK_LINES = 5000;

// Adaptive polling phases — the next-refetch delay depends on how long
// ago content actually changed (tracked via React Query's dataUpdatedAt):
//   active:  changed within last 2s   → 200ms (5 fps for ASCII animations)
//   warm:    changed 2-10s ago        → 1000ms (ramp-down)
//   idle:    stable >10s              → 5000ms (battery saver)
const ACTIVE_POLL_INTERVAL = 200;
const WARM_POLL_INTERVAL = 1000;
const IDLE_POLL_INTERVAL = 5000;
const ACTIVE_PHASE_MS = 2_000;
const WARM_PHASE_MS = 10_000;

export function computeAdaptiveInterval(dataUpdatedAt: number, now: number): number {
  if (dataUpdatedAt === 0) return ACTIVE_POLL_INTERVAL;
  const elapsed = now - dataUpdatedAt;
  if (elapsed <= ACTIVE_PHASE_MS) return ACTIVE_POLL_INTERVAL;
  if (elapsed <= WARM_PHASE_MS) return WARM_POLL_INTERVAL;
  return IDLE_POLL_INTERVAL;
}

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

  const prevResultRef = useRef<{ content: string; error: string | null } | null>(null);

  // Reset scrollback and dedup cache when pane changes
  useEffect(() => {
    setScrollbackLines(DEFAULT_SCROLLBACK_LINES);
    prevResultRef.current = null;
  }, [paneId]);

  const { data, isLoading, isFetching, error, refetch } = useQuery({
    queryKey: tmuxCapturePaneQueryKey(serverId, paneId, scrollbackLines),
    enabled: enabled && Boolean(client) && isConnected,
    staleTime: 5_000,
    refetchInterval:
      enabled && isAppVisible && autoRefresh
        ? (query) => computeAdaptiveInterval(query.state.dataUpdatedAt, Date.now())
        : false,
    placeholderData: keepPreviousData,
    retry: 1,
    queryFn: async () => {
      const payload = await withLiveTmuxClient(serverId, (c) =>
        c.tmuxCapturePane(paneId, -scrollbackLines),
      );
      const newContent = payload.content ?? "";
      if (prevResultRef.current && prevResultRef.current.content === newContent) {
        return prevResultRef.current;
      }
      const result = { content: newContent, error: payload.error ?? null };
      prevResultRef.current = result;
      return result;
    },
  });

  const loadMoreHistory = useCallback(() => {
    if (scrollbackLines >= MAX_SCROLLBACK_LINES) return;
    setScrollbackLines((prev) => Math.min(prev + SCROLLBACK_INCREMENT, MAX_SCROLLBACK_LINES));
  }, [scrollbackLines]);

  // Pagination-only loading flag: true between a loadMoreHistory() call and the
  // completion of the resulting scrollback-driven fetch. Must NOT flicker on
  // the 5s poll refetch — otherwise the "Loading more history..." row toggles
  // in and out of the ScrollView, causing a content-height pulse every 5s.
  const isPaginatingRef = useRef(false);
  const prevScrollbackRef = useRef(scrollbackLines);
  const [isPaginating, setIsPaginating] = useState(false);
  if (scrollbackLines !== prevScrollbackRef.current) {
    prevScrollbackRef.current = scrollbackLines;
    isPaginatingRef.current = true;
    setIsPaginating(true);
  }
  useEffect(() => {
    if (isPaginatingRef.current && !isFetching) {
      isPaginatingRef.current = false;
      setIsPaginating(false);
    }
  }, [isFetching]);

  const isLoadingMore = isPaginating;
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
