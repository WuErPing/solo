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
//   active:  changed within last 2s   → 500ms (2 fps)
//   warm:    changed 2-10s ago        → 1000ms (ramp-down)
//   idle:    stable >10s              → 5000ms (battery saver)
const ACTIVE_POLL_INTERVAL = 500;
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
  defaultFg: string | null;
  defaultBg: string | null;
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

  const prevResultRef = useRef<{
    content: string;
    error: string | null;
    defaultFg: string | null;
    defaultBg: string | null;
  } | null>(null);
  const lastContentHashRef = useRef<string | null>(null);

  // Reset scrollback, dedup cache, and hash when pane changes
  useEffect(() => {
    setScrollbackLines(DEFAULT_SCROLLBACK_LINES);
    prevResultRef.current = null;
    lastContentHashRef.current = null;
  }, [paneId]);

  // Reset hash when scrollback depth changes (different range = different content)
  useEffect(() => {
    lastContentHashRef.current = null;
  }, [scrollbackLines]);

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
        c.tmuxCapturePane(paneId, -scrollbackLines, lastContentHashRef.current ?? undefined),
      );

      // Daemon says content unchanged — return cached result (same object ref)
      if (payload.changed === false) {
        if (payload.contentHash) {
          lastContentHashRef.current = payload.contentHash;
        }
        if (prevResultRef.current) return prevResultRef.current;
        return { content: "", error: null, defaultFg: null, defaultBg: null };
      }

      // Update hash from daemon response
      if (payload.contentHash) {
        lastContentHashRef.current = payload.contentHash;
      }

      const newContent = payload.content ?? "";
      const payloadWithColors = payload as typeof payload & { defaultFg?: string | null; defaultBg?: string | null };
      const newDefaultFg = payloadWithColors.defaultFg ?? null;
      const newDefaultBg = payloadWithColors.defaultBg ?? null;
      if (
        prevResultRef.current &&
        prevResultRef.current.content === newContent &&
        prevResultRef.current.defaultFg === newDefaultFg &&
        prevResultRef.current.defaultBg === newDefaultBg
      ) {
        return prevResultRef.current;
      }
      const result = {
        content: newContent,
        error: payload.error ?? null,
        defaultFg: newDefaultFg,
        defaultBg: newDefaultBg,
      };
      prevResultRef.current = result;
      return result;
    },
  });

  // Foreground edge refetch: when the app transitions from hidden to visible
  // while autoRefresh is on, refetch immediately instead of waiting for the
  // next adaptive-poll tick (up to IDLE_POLL_INTERVAL = 5000ms).
  const prevVisibleRef = useRef(isAppVisible);
  useEffect(() => {
    const wasVisible = prevVisibleRef.current;
    prevVisibleRef.current = isAppVisible;
    if (!wasVisible && isAppVisible && enabled && autoRefresh) {
      void refetch();
    }
  }, [isAppVisible, enabled, autoRefresh, refetch]);

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
      defaultFg: data?.defaultFg ?? null,
      defaultBg: data?.defaultBg ?? null,
    }),
    [data, isLoading, isLoadingMore, error, refetch, scrollbackLines, loadMoreHistory, hasMoreHistory, autoRefresh],
  );
}
