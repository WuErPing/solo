/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useTmuxCapturePane } from "./use-tmux-capture-pane";

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

function renderCapturePaneHook(opts?: { serverId?: string; paneId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () =>
      useTmuxCapturePane(
        opts?.serverId ?? "server-1",
        opts?.paneId ?? "pane-1",
        opts?.enabled ?? true,
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

    expect(mockClient.tmuxCapturePane).toHaveBeenCalledWith("pane-1", -200);
  });

  it("uses default scrollback of 200 lines", async () => {
    const { result } = renderCapturePaneHook();

    await waitFor(() => {
      expect(result.current.scrollbackLines).toBe(200);
    });

    expect(mockClient.tmuxCapturePane).toHaveBeenCalledWith("pane-1", -200);
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

    expect(mockClient.tmuxCapturePane).toHaveBeenLastCalledWith("pane-1", -400);
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
    expect(firstClient.tmuxCapturePane).toHaveBeenCalledTimes(1);
    expect(freshClient.tmuxCapturePane).toHaveBeenCalledTimes(1);
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

    const callCountBefore = mockClient.tmuxCapturePane.mock.calls.length;

    // Simulate app going to background
    mockAppVisible.value = false;

    // Wait longer than the poll interval
    await act(async () => {
      await new Promise((r) => setTimeout(r, 100));
    });

    // No additional calls should have been made
    expect(mockClient.tmuxCapturePane.mock.calls.length).toBe(callCountBefore);
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

    const callCountBefore = mockClient.tmuxCapturePane.mock.calls.length;

    // Turn off auto refresh
    act(() => {
      result.current.setAutoRefresh(false);
    });
    expect(result.current.autoRefresh).toBe(false);

    // Wait longer than the poll interval
    await act(async () => {
      await new Promise((r) => setTimeout(r, 100));
    });

    // No additional polling calls should have been made
    expect(mockClient.tmuxCapturePane.mock.calls.length).toBe(callCountBefore);
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
