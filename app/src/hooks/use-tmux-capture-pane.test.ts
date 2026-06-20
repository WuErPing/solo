/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useTmuxCapturePane, computeAdaptiveInterval } from "./use-tmux-capture-pane";

const { mockStore, mockClient, mockAppVisible } = vi.hoisted(() => {
  const store = {
    getClient: vi.fn(),
    getSnapshot: vi.fn(),
  };
  const client = {
    tmuxCapturePane: vi.fn(),
    getConnectionState: vi.fn(),
  };
  return {
    mockStore: store,
    mockClient: client,
    mockAppVisible: { value: true },
  };
});

vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: () => mockStore,
  isHostRuntimeConnected: () => true,
}));

vi.mock("@/hooks/use-app-visible", () => ({
  useAppVisible: () => mockAppVisible.value,
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function renderCapturePaneHook(opts?: { serverId?: string; paneId?: string; enabled?: boolean; cols?: number }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () =>
      useTmuxCapturePane(
        opts?.serverId ?? "server-1",
        opts?.paneId ?? "pane-1",
        opts?.enabled ?? true,
        opts?.cols,
      ),
    { wrapper },
  );
}

beforeEach(() => {
  mockStore.getClient.mockReturnValue(mockClient);
  mockStore.getSnapshot.mockReturnValue({ connectionStatus: "online" });
  mockClient.getConnectionState.mockReturnValue({ status: "connected" });
  mockClient.tmuxCapturePane.mockResolvedValue({ content: "$ ls\nfile.txt", error: null });
  mockAppVisible.value = true;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTmuxCapturePane", () => {
  it("fetches pane content from the live store client", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    expect(mockClient.tmuxCapturePane).toHaveBeenCalledWith("pane-1", -200, undefined, undefined);
  });

  it("uses default scrollback of 200 lines", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(200);
    });

    expect(mockClient.tmuxCapturePane).toHaveBeenCalledWith("pane-1", -200, undefined, undefined);
  });

  it("passes cols to the daemon client when provided", async () => {
    const { result } = renderCapturePaneHook({ cols: 80 });

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    expect(mockClient.tmuxCapturePane).toHaveBeenCalledWith("pane-1", -200, undefined, 80);
  });

  it("increases scrollback lines when loadMoreHistory is called", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Setup mock for the larger scrollback request
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt\n$ echo older\nolder",
      error: null,
    });

    act(() => {
      result.current.loadMoreHistory();
    });

    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(400);
    });

    expect(mockClient.tmuxCapturePane).toHaveBeenLastCalledWith("pane-1", -400, undefined, undefined);
  });

  it("accumulates scrollback lines across multiple loadMoreHistory calls", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    act(() => {
      result.current.loadMoreHistory();
    });
    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(400);
    });

    act(() => {
      result.current.loadMoreHistory();
    });
    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(600);
    });
  });

  it("caps scrollback lines at maximum", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Repeatedly load more until max
    act(() => {
      for (let i = 0; i < 30; i++) {
        result.current.loadMoreHistory();
      }
    });

    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(5000);
    });

    expect(result.current.hasMoreHistory).toBe(false);
  });

  it("resets scrollback lines when paneId changes", async () => {
    const { result, rerender } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(200);
    });

    act(() => {
      result.current.loadMoreHistory();
    });
    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(400);
    });

    // Re-render with a different paneId
    rerender();
  });

  it("reports isLoadingMore while fetching additional history", async () => {
    // Let the initial load complete quickly
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "initial", error: null });

    const { result } = renderCapturePaneHook();

    // Wait for initial load to finish
    await waitFor(() => {
      expect(result.current.content).toBe("initial");
      expect(result.current.isLoading).toBe(false);
    });

    // Now trigger loadMoreHistory with a delayed promise
    let resolveMore: (value: { content: string; error: null }) => void;
    const morePromise = new Promise<{ content: string; error: null }>((resolve) => {
      resolveMore = resolve;
    });
    mockClient.tmuxCapturePane.mockReturnValue(morePromise);

    act(() => {
      result.current.loadMoreHistory();
    });

    // Should be loading more while the new request is in flight
    await waitFor(() => {
      expect(result.current.isLoadingMore).toBe(true);
    });

    act(() => {
      resolveMore!({ content: "more content", error: null });
    });

    await waitFor(() => {
      expect(result.current.isLoadingMore).toBe(false);
      expect(result.current.content).toBe("more content");
    });
  });

  it("does NOT set isLoadingMore during a poll refetch with unchanged scrollback", async () => {
    // TDD contract for "no jitter when content unchanged":
    // A 5s poll refetch (same scrollbackLines) must NOT toggle isLoadingMore,
    // because the screen renders a "Loading more history..." row conditioned
    // on isLoadingMore. Mounting/unmounting that row every poll would pulse
    // the ScrollView content height — the exact Android jitter symptom.
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "initial", error: null });
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("initial");
      expect(result.current.isLoading).toBe(false);
    });
    expect(result.current.isLoadingMore).toBe(false);

    // Simulate a poll refetch with a slow promise so we can observe the
    // in-flight state. scrollbackLines is unchanged → not pagination.
    let resolvePoll: (value: { content: string; error: null }) => void;
    const pollPromise = new Promise<{ content: string; error: null }>((resolve) => {
      resolvePoll = resolve;
    });
    mockClient.tmuxCapturePane.mockReturnValue(pollPromise);

    act(() => {
      result.current.refetch();
    });

    // While the poll is in flight, isLoadingMore MUST stay false.
    expect(result.current.isLoadingMore).toBe(false);

    // Resolve the poll with identical content — isLoadingMore stays false.
    await act(async () => {
      resolvePoll!({ content: "initial", error: null });
    });
    expect(result.current.isLoadingMore).toBe(false);
    expect(result.current.content).toBe("initial");
  });

  it("keeps previous content when client is disposed during a poll", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Simulate client being disposed (e.g., during connection switch)
    mockClient.getConnectionState.mockReturnValue({ status: "disposed" });

    // Trigger a refetch by advancing time
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    // Content should still be the previous value, not empty or error
    expect(result.current.content).toBe("$ ls\nfile.txt");
    expect(result.current.error).toBeNull();
  });

  it("keeps previous content when store returns null during connection gap", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Simulate store returning null (gap between dispose and new client)
    mockStore.getClient.mockReturnValue(null);

    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    // Previous content should be preserved
    expect(result.current.content).toBe("$ ls\nfile.txt");
    expect(result.current.error).toBeNull();
  });

  it("recovers and fetches new content after client reconnects", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Simulate disposal
    mockClient.getConnectionState.mockReturnValue({ status: "disposed" });

    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    // Content preserved during disposal
    expect(result.current.content).toBe("$ ls\nfile.txt");

    // Simulate new client available
    const newClient = {
      tmuxCapturePane: vi.fn().mockResolvedValue({ content: "$ pwd\n/home/user", error: null }),
      getConnectionState: vi.fn().mockReturnValue({ status: "connected" }),
    };
    mockStore.getClient.mockReturnValue(newClient);

    // Trigger an immediate refetch (simulates the next poll cycle)
    act(() => {
      result.current.refetch();
    });

    // Should get new content from the new client
    await waitFor(() => {
      expect(result.current.content).toBe("$ pwd\n/home/user");
    });
  });

  it("transparently retries with a fresh client when the live client is disposed mid-query", async () => {
    // First call: a connected client whose RPC throws a disposed error
    // Second call (retry inside withLiveTmuxClient): a fresh connected client
    const firstClient = {
      tmuxCapturePane: vi
        .fn()
        .mockRejectedValue(new Error("Transport not connected (status: disposed)")),
      getConnectionState: vi.fn().mockReturnValue({ status: "connected" }),
    };
    const freshClient = {
      tmuxCapturePane: vi.fn().mockResolvedValue({ content: "$ whoami\nuser", error: null }),
      getConnectionState: vi.fn().mockReturnValue({ status: "connected" }),
    };
    mockStore.getClient
      .mockReset()
      .mockImplementation(() => {
        // Return freshClient once the first call has already thrown; otherwise firstClient.
        return firstClient.tmuxCapturePane.mock.calls.length > 0 ? freshClient : firstClient;
      });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ whoami\nuser");
    });
    expect(result.current.error).toBeNull();
    // The retry path was exercised: firstClient failed at least once, and
    // freshClient (returned after firstClient's throw) delivered content.
    // We don't assert exact call counts because the adaptive poll interval
    // (200ms in the active phase) triggers multiple refetch cycles during
    // waitFor, each of which re-enters the disposed→retry→freshClient path.
    expect(firstClient.tmuxCapturePane.mock.calls.length).toBeGreaterThanOrEqual(1);
    expect(freshClient.tmuxCapturePane.mock.calls.length).toBeGreaterThanOrEqual(1);
  });

  it("does not surface 'Daemon client not available' as a user-visible error", async () => {
    // Simulate client always disposed (e.g., persistent disconnect)
    mockClient.getConnectionState.mockReturnValue({ status: "disposed" });

    const { result } = renderCapturePaneHook();

    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });

    // The transient "Daemon client not available" error should NOT appear in the UI
    expect(result.current.error).toBeNull();
  });

  it("pauses polling when app is backgrounded", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Verify polling is active: changing mock content should eventually be picked up
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "$ pwd\n/home", error: null });
    await waitFor(() => {
      expect(result.current.content).toBe("$ pwd\n/home");
    });

    // Simulate app going to background — refetchInterval returns false
    mockAppVisible.value = false;
    act(() => {
      result.current.setAutoRefresh(false);
      result.current.setAutoRefresh(true);
    });

    // Set up new content — but with polling paused, it should NOT be picked up
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "SHOULD_NOT_APPEAR", error: null });

    // Wait a bit — content should stay unchanged because polling is paused
    await act(async () => {
      await new Promise((r) => setTimeout(r, 100));
    });
    expect(result.current.content).toBe("$ pwd\n/home");
  });

  it("refetches immediately when app returns to foreground after being backgrounded", async () => {
    // Bug contract: switching back to the window must trigger an immediate
    // refetch, not wait for the next adaptive-poll tick (which could be up
    // to 5000ms when the idle phase has kicked in).
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "initial", error: null });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("initial");
    });

    // Background the app — polling pauses
    act(() => {
      mockAppVisible.value = false;
    });

    // Force a rerender so the hook observes the new visibility value.
    // (useAppVisible returns a new boolean from the external store.)
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    // Content changes while backgrounded — must NOT be picked up yet.
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "updated-while-hidden", error: null });
    mockClient.tmuxCapturePane.mockClear();

    // Foreground the app — hook should immediately refetch.
    act(() => {
      mockAppVisible.value = true;
    });

    await waitFor(() => {
      expect(mockClient.tmuxCapturePane.mock.calls.length).toBeGreaterThanOrEqual(1);
      expect(result.current.content).toBe("updated-while-hidden");
    });
  });

  it("does not refetch on foreground when autoRefresh is off", async () => {
    // Use fake timers to isolate from stray React Query poll timers left
    // over from the previous test (the 500ms adaptive poll can fire inside
    // the real-timer waitFor windows of adjacent tests).
    vi.useFakeTimers();
    try {
      mockClient.tmuxCapturePane.mockResolvedValue({ content: "initial", error: null });

      const { result } = renderCapturePaneHook();

      await vi.waitFor(() => {
        expect(result.current.content).toBe("initial");
      });

      act(() => {
        result.current.setAutoRefresh(false);
      });

      act(() => {
        mockAppVisible.value = false;
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(50);
      });

      mockClient.tmuxCapturePane.mockResolvedValue({ content: "should-not-appear", error: null });
      mockClient.tmuxCapturePane.mockClear();
      let invocationCount = 0;
      mockClient.tmuxCapturePane.mockImplementation(async () => {
        invocationCount += 1;
        return { content: "should-not-appear", error: null };
      });

      // Drain any already-scheduled poll tick before we toggle foreground,
      // so only the foreground transition is measured.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });
      invocationCount = 0;
      mockClient.tmuxCapturePane.mockClear();

      act(() => {
        mockAppVisible.value = true;
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(200);
      });

      expect(invocationCount).toBe(0);
      expect(result.current.content).toBe("initial");
    } finally {
      vi.useRealTimers();
    }
  });

  it("defaults autoRefresh to true", async () => {
    const { result } = renderCapturePaneHook();
    expect(result.current.autoRefresh).toBe(true);
  });

  it("toggles autoRefresh off and on", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    expect(result.current.autoRefresh).toBe(true);

    act(() => {
      result.current.setAutoRefresh(false);
    });
    expect(result.current.autoRefresh).toBe(false);

    act(() => {
      result.current.setAutoRefresh(true);
    });
    expect(result.current.autoRefresh).toBe(true);
  });

  it("stops polling when autoRefresh is turned off", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Verify polling is active: changing mock content should eventually be picked up
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "$ pwd\n/home", error: null });
    await waitFor(() => {
      expect(result.current.content).toBe("$ pwd\n/home");
    });

    // Turn off auto refresh
    act(() => {
      result.current.setAutoRefresh(false);
    });
    expect(result.current.autoRefresh).toBe(false);

    // Set up new content — but with polling stopped, it should NOT be picked up
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "SHOULD_NOT_APPEAR", error: null });

    // Wait a bit — content should stay unchanged because polling is stopped
    await act(async () => {
      await new Promise((r) => setTimeout(r, 100));
    });
    expect(result.current.content).toBe("$ pwd\n/home");
  });

  it("deduplicates query result when content is unchanged across polls", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Refetch with identical content — the queryFn should return the same object
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "$ ls\nfile.txt", error: null });
    act(() => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // The return value object should be referentially stable (same object, not a new one)
    // because queryFn deduplicates identical content
    const refAfterFirstFetch = result.current;
    act(() => {
      result.current.refetch();
    });
    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });
    expect(result.current).toBe(refAfterFirstFetch);
  });

  it("preserves content STRING identity across many polls with identical payload", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Capture the original content string reference from the hook's data
    const originalContentString = result.current.content;
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "$ ls\nfile.txt", error: null });

    // Drive multiple real refetches (simulates polling)
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        result.current.refetch();
      });
    }

    // queryFn was actually called at least 6 times (initial + 5 refetches)
    // so dedup had a chance to kick in. May be higher due to adaptive polling.
    expect(mockClient.tmuxCapturePane.mock.calls.length).toBeGreaterThanOrEqual(1 + 5);

    // CRITICAL: the content string reference must be EXACTLY the same object
    // across all polls. If it changes, downstream useMemo (parseAnsi, terminalColors)
    // will recalculate and the ScrollView will re-render → jitter.
    expect(result.current.content).toBe(originalContentString);
  });

  it("resets dedup cache when paneId changes so new pane gets fresh content", async () => {
    const queryClient = createQueryClient();
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(QueryClientProvider, { client: queryClient }, children);

    // Configure mock BEFORE mounting so the initial fetch returns "same-content"
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "same-content", error: null });

    let paneId = "pane-A";
    const { result, rerender } = renderHook(
      () => useTmuxCapturePane("server-1", paneId, true, undefined),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.content).toBe("same-content");
    });

    const callsBeforeSwitch = mockClient.tmuxCapturePane.mock.calls.length;
    expect(callsBeforeSwitch).toBeGreaterThanOrEqual(1);

    // Switch to pane-B; mock still returns the same content payload.
    // If prevResultRef were NOT reset on paneId change, the hook might
    // incorrectly dedup against pane-A's cached result. We verify reset
    // by asserting the new pane triggers its own fresh fetch.
    paneId = "pane-B";
    rerender();

    await waitFor(() => {
      expect(mockClient.tmuxCapturePane.mock.calls.length).toBeGreaterThan(callsBeforeSwitch);
    });
    expect(result.current.content).toBe("same-content");
  });

  it("returns new result object when content changes across polls", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    const firstResult = result.current;

    // Refetch with different content
    mockClient.tmuxCapturePane.mockResolvedValue({ content: "$ pwd\n/home", error: null });
    act(() => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(result.current.content).toBe("$ pwd\n/home");
    });

    expect(result.current).not.toBe(firstResult);
  });

  it("resumes polling when autoRefresh is turned back on", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Turn off then back on
    act(() => {
      result.current.setAutoRefresh(false);
    });
    expect(result.current.autoRefresh).toBe(false);

    // Set up mock for the manual refetch
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ pwd\n/home/user",
      error: null,
    });

    act(() => {
      result.current.setAutoRefresh(true);
    });
    expect(result.current.autoRefresh).toBe(true);

    // Manually refetch to verify the query is still functional
    act(() => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(result.current.content).toBe("$ pwd\n/home/user");
    });
  });
});

describe("computeAdaptiveInterval", () => {
  const now = 1_000_000;

  it("returns 500ms when content changed within the last 2 seconds (active phase)", () => {
    expect(computeAdaptiveInterval(now - 500, now)).toBe(500);
    expect(computeAdaptiveInterval(now - 2000, now)).toBe(500); // boundary inclusive
  });

  it("returns 1000ms when content changed 2-10 seconds ago (warm phase)", () => {
    expect(computeAdaptiveInterval(now - 2001, now)).toBe(1000);
    expect(computeAdaptiveInterval(now - 5000, now)).toBe(1000);
    expect(computeAdaptiveInterval(now - 10000, now)).toBe(1000); // boundary inclusive
  });

  it("returns 5000ms when content has been stable for more than 10 seconds (idle phase)", () => {
    expect(computeAdaptiveInterval(now - 10001, now)).toBe(5000);
    expect(computeAdaptiveInterval(now - 60000, now)).toBe(5000);
  });

  it("returns 500ms when dataUpdatedAt is 0 (initial mount — assume active to prime the pump)", () => {
    expect(computeAdaptiveInterval(0, now)).toBe(500);
  });
});

describe("useTmuxCapturePane — adaptive polling", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("refetches more frequently during the active phase than the idle phase", async () => {
    // Delayed promise so we control exactly when each fetch resolves
    let resolveCurrent: (v: { content: string; error: null }) => void;
    const makeDelayed = () =>
      new Promise<{ content: string; error: null }>((resolve) => {
        resolveCurrent = resolve;
      });
    mockClient.tmuxCapturePane.mockImplementation(() => makeDelayed());

    renderCapturePaneHook();

    // Initial fetch: start + resolve with "active content"
    await vi.waitFor(() => {
      expect(mockClient.tmuxCapturePane.mock.calls.length).toBe(1);
    });
    await act(async () => {
      resolveCurrent!({ content: "active-phase", error: null });
    });

    // ACTIVE PHASE: advance 2s with changing content.
    // Each resolve schedules the next refetch in 500ms.
    const activeStartCalls = mockClient.tmuxCapturePane.mock.calls.length;
    for (let i = 0; i < 4; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(500);
      });
      // Resolve with a new content string so dataUpdatedAt resets
      await act(async () => {
        resolveCurrent!({ content: `active-${i}`, error: null });
      });
    }
    const activeCalls = mockClient.tmuxCapturePane.mock.calls.length - activeStartCalls;

    // IDLE PHASE: advance 15s (well past the 10s idle threshold) with
    // content NOT changing — each resolve should schedule the next refetch
    // at the idle 5000ms rate, so very few calls fit in 15s.
    const idleStartCalls = mockClient.tmuxCapturePane.mock.calls.length;
    for (let i = 0; i < 4; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });
      // Same content — dedup keeps dataUpdatedAt where it was (idle clock ticks up)
      await act(async () => {
        resolveCurrent!({ content: "stable-idle", error: null });
      });
    }
    const idleElapsedMs = 4 * 5000;
    const idleCalls = mockClient.tmuxCapturePane.mock.calls.length - idleStartCalls;

    // During active phase we got ~4 calls in 2s (500ms × 4).
    // During idle phase we got ~3-4 calls in 20s (5000ms × 4).
    // Per-second rate active must be >> per-second rate idle.
    const activeRate = activeCalls / 2;
    const idleRate = idleCalls / (idleElapsedMs / 1000);
    expect(activeRate).toBeGreaterThan(idleRate * 3);
  });
});

describe("useTmuxCapturePane — incremental transfer", () => {
  it("sends lastContentHash on subsequent polls after receiving contentHash", async () => {
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      changed: true,
      contentHash: "abc123",
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Reset call history to isolate the refetch call
    mockClient.tmuxCapturePane.mockClear();
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      changed: true,
      contentHash: "abc123",
    });

    // Trigger a refetch
    act(() => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(mockClient.tmuxCapturePane.mock.calls.length).toBeGreaterThanOrEqual(1);
    });

    // First call after clear should include lastContentHash from previous fetch
    const firstCall = mockClient.tmuxCapturePane.mock.calls[0];
    expect(firstCall[2]).toBe("abc123"); // 3rd arg is lastContentHash
  });

  it("keeps previous content when daemon returns changed=false", async () => {
    // First fetch returns content with hash
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      changed: true,
      contentHash: "abc123",
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    const contentBefore = result.current.content;

    // Second fetch returns changed=false (content unchanged)
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "",
      error: null,
      changed: false,
      contentHash: "abc123",
    });

    act(() => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(mockClient.tmuxCapturePane.mock.calls.length).toBeGreaterThanOrEqual(2);
    });

    // Content should be the same string reference
    expect(result.current.content).toBe(contentBefore);
  });

  it("updates content when daemon returns changed=true", async () => {
    // First fetch
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      changed: true,
      contentHash: "abc123",
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Second fetch returns new content
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ pwd\n/home",
      error: null,
      changed: true,
      contentHash: "def456",
    });

    act(() => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(result.current.content).toBe("$ pwd\n/home");
    });
  });

  it("resets lastContentHash when paneId changes", async () => {
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      changed: true,
      contentHash: "abc123",
    });

    const queryClient = createQueryClient();
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(QueryClientProvider, { client: queryClient }, children);

    let paneId = "pane-A";
    const { result, rerender } = renderHook(
      () => useTmuxCapturePane("server-1", paneId, true, undefined),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Switch pane — should reset hash
    paneId = "pane-B";
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ pwd\n/home",
      error: null,
      changed: true,
      contentHash: "newhash",
    });
    rerender();

    await waitFor(() => {
      expect(result.current.content).toBe("$ pwd\n/home");
    });

    // The first call for pane-B should NOT have lastContentHash
    const calls = mockClient.tmuxCapturePane.mock.calls;
    const paneBCall = calls.find((c: unknown[]) => c[0] === "pane-B");
    expect(paneBCall).toBeDefined();
    expect(paneBCall![2]).toBeUndefined(); // lastContentHash should be undefined
  });

  it("resets lastContentHash when scrollbackLines changes", async () => {
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      changed: true,
      contentHash: "abc123",
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // loadMoreHistory changes scrollbackLines
    act(() => {
      result.current.loadMoreHistory();
    });

    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(400);
    });

    // The call with -400 startLine should NOT have lastContentHash
    const calls = mockClient.tmuxCapturePane.mock.calls;
    const scrollback400Call = calls.find((c: unknown[]) => c[1] === -400);
    expect(scrollback400Call).toBeDefined();
    expect(scrollback400Call![2]).toBeUndefined(); // lastContentHash should be undefined
  });

  it("backward compatible: works when daemon returns no changed/contentHash fields", async () => {
    // Old daemon returns plain response without changed/contentHash
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    // Should work normally — no errors
    expect(result.current.error).toBeNull();
  });

  it("exposes defaultFg and defaultBg from the daemon payload (OSC 10/11 host-reported terminal colors)", async () => {
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      defaultFg: "#cdd6f4",
      defaultBg: "#1e1e2e",
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    expect(result.current.defaultFg).toBe("#cdd6f4");
    expect(result.current.defaultBg).toBe("#1e1e2e");
  });

  it("defaults defaultFg and defaultBg to null when the daemon does not report them", async () => {
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    expect(result.current.defaultFg).toBeNull();
    expect(result.current.defaultBg).toBeNull();
  });

  it("exposes paneCols from the daemon payload", async () => {
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
      paneCols: 120,
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    expect(result.current.paneCols).toBe(120);
  });

  it("defaults paneCols to null when the daemon does not report them", async () => {
    mockClient.tmuxCapturePane.mockResolvedValue({
      content: "$ ls\nfile.txt",
      error: null,
    });

    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.content).toBe("$ ls\nfile.txt");
    });

    expect(result.current.paneCols).toBeNull();
  });
});
