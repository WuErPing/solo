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

    expect(mockClient.tmuxCapturePane).toHaveBeenCalledWith("pane-1");
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
});
