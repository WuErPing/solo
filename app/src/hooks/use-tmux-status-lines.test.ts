/**
 * @vitest-environment jsdom
 */
import React from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useTmuxStatusLines } from "./use-tmux-status-lines";
import type { TmuxAgent } from "./use-tmux-agents";

const { mockStore, mockClient, mockHosts } = vi.hoisted(() => {
  const store = {
    getClient: vi.fn(),
    getSnapshot: vi.fn(),
  };
  const client = {
    tmuxStatusLine: vi.fn(),
    getConnectionState: vi.fn(),
  };
  return {
    mockStore: store,
    mockClient: client,
    mockHosts: { value: [{ serverId: "s1", label: "local" }] as Array<{ serverId: string; label: string }> },
  };
});

let isConnectedOverride = true;

vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: () => mockStore,
  useHosts: () => mockHosts.value,
  isHostRuntimeConnected: () => isConnectedOverride,
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function renderStatusLinesHook(agents: TmuxAgent[]) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useTmuxStatusLines(agents), { wrapper });
}

const baseAgent: TmuxAgent = {
  sessionName: "dev",
  windowName: "main",
  paneId: "%0",
  paneIndex: 0,
  panePid: 100,
  agentName: "claude",
  currentCmd: "claude",
  workingDir: "/home",
  serverId: "s1",
  serverLabel: "local",
};

beforeEach(() => {
  mockStore.getClient.mockReturnValue(mockClient);
  mockStore.getSnapshot.mockReturnValue({ connectionStatus: "online" });
  mockClient.getConnectionState.mockReturnValue({ status: "connected" });
  mockClient.tmuxStatusLine.mockResolvedValue({
    statusLeft: "[#S]",
    statusCenter: "0:claude*",
    statusRight: "%H:%M",
    paneBackground: "#1e1e2e",
    paneForeground: "#cdd6f4",
    error: null,
  });
  mockHosts.value = [{ serverId: "s1", label: "local" }];
  isConnectedOverride = true;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTmuxStatusLines", () => {
  it("fetches status line for each unique session", async () => {
    const agents: TmuxAgent[] = [
      { ...baseAgent, paneId: "%0" },
      { ...baseAgent, paneId: "%1" },
    ];

    const { result } = renderStatusLinesHook(agents);

    await waitFor(() => {
      expect(result.current.length).toBeGreaterThan(0);
    });

    // Only one tmuxStatusLine call despite two agents in same session
    expect(mockClient.tmuxStatusLine).toHaveBeenCalledTimes(1);
    expect(mockClient.tmuxStatusLine).toHaveBeenCalledWith("dev");
    expect(result.current[0].sessionName).toBe("dev");
  });

  it("fetches status lines for multiple sessions", async () => {
    const agents: TmuxAgent[] = [
      { ...baseAgent, sessionName: "dev", paneId: "%0" },
      { ...baseAgent, sessionName: "prod", paneId: "%1" },
    ];

    mockClient.tmuxStatusLine.mockImplementation((session: string) => {
      if (session === "dev") {
        return Promise.resolve({ statusLeft: "[dev]", statusCenter: "0:claude*", statusRight: "10:00", paneBackground: "#1e1e2e", paneForeground: "#cdd6f4", error: null });
      }
      return Promise.resolve({ statusLeft: "[prod]", statusCenter: "0:pi*", statusRight: "11:00", paneBackground: "#2e1e1e", paneForeground: "#f4cdd6", error: null });
    });

    const { result } = renderStatusLinesHook(agents);

    await waitFor(() => {
      expect(result.current.length).toBe(2);
    });

    expect(mockClient.tmuxStatusLine).toHaveBeenCalledTimes(2);
    expect(result.current).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ sessionName: "dev", statusLeft: "[dev]" }),
        expect.objectContaining({ sessionName: "prod", statusLeft: "[prod]" }),
      ]),
    );
  });

  it("returns empty array when agents list is empty", () => {
    const { result } = renderStatusLinesHook([]);

    expect(result.current).toEqual([]);
    expect(mockClient.tmuxStatusLine).not.toHaveBeenCalled();
  });

  it("handles fetch errors gracefully", async () => {
    mockClient.tmuxStatusLine.mockRejectedValue(new Error("tmux not available"));

    const agents: TmuxAgent[] = [baseAgent];
    const { result } = renderStatusLinesHook(agents);

    // Should not crash, just return empty
    await waitFor(() => {
      // The query will error, but the hook should still work
      expect(result.current).toEqual([]);
    });
  });
});
