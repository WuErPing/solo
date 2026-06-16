/**
 * @vitest-environment jsdom
 */
import React from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAggregatedTmuxAgents } from "./use-tmux-agents";

const { mockStore, mockClient, mockHosts } = vi.hoisted(() => {
  const store = {
    getClient: vi.fn(),
    getSnapshot: vi.fn(),
  };
  const client = {
    tmuxListAgents: vi.fn(),
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

function renderAgentsHook() {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useAggregatedTmuxAgents(), { wrapper });
}

beforeEach(() => {
  mockStore.getClient.mockReturnValue(mockClient);
  mockStore.getSnapshot.mockReturnValue({ connectionStatus: "online" });
  mockClient.getConnectionState.mockReturnValue({ status: "connected" });
  mockClient.tmuxListAgents.mockResolvedValue({
    agents: [],
    error: null,
  });
  mockHosts.value = [{ serverId: "s1", label: "local" }];
  isConnectedOverride = true;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useAggregatedTmuxAgents", () => {
  it("reports isInitialLoad true while queries are disabled (no connected host)", () => {
    isConnectedOverride = false;
    mockStore.getClient.mockReturnValue(null);

    const { result } = renderAgentsHook();

    // BUG: isInitialLoad should be true here because no data exists and
    // no query is running. The dashboard should show "Scanning tmux panes..."
    // not the empty state.
    expect(result.current.isInitialLoad).toBe(true);
    expect(result.current.agents).toEqual([]);
  });

  it("reports isInitialLoad true while first fetch is in flight", async () => {
    let resolveFetch: (value: { agents: Array<{ agentName: string; sessionName: string; windowName: string; paneId: string; paneIndex: number; panePid: number; currentCmd: string; workingDir: string; serverId: string; serverLabel: string }>; error: null }) => void;
    const fetchPromise = new Promise<{
      agents: Array<{ agentName: string; sessionName: string; windowName: string; paneId: string; paneIndex: number; panePid: number; currentCmd: string; workingDir: string; serverId: string; serverLabel: string }>;
      error: null;
    }>((resolve) => {
      resolveFetch = resolve;
    });
    mockClient.tmuxListAgents.mockReturnValue(fetchPromise);

    const { result } = renderAgentsHook();

    expect(result.current.isInitialLoad).toBe(true);

    resolveFetch!({
      agents: [
        {
          agentName: "claude",
          sessionName: "dev",
          windowName: "main",
          paneId: "%0",
          paneIndex: 0,
          panePid: 100,
          currentCmd: "claude",
          workingDir: "/home",
          serverId: "s1",
          serverLabel: "local",
        },
      ],
      error: null,
    });

    await waitFor(() => {
      expect(result.current.isInitialLoad).toBe(false);
      expect(result.current.agents).toHaveLength(1);
    });
  });

  it("reports isInitialLoad false after empty result from connected host", async () => {
    mockClient.tmuxListAgents.mockResolvedValue({ agents: [], error: null });

    const { result } = renderAgentsHook();

    await waitFor(() => {
      expect(result.current.isInitialLoad).toBe(false);
    });

    expect(result.current.agents).toEqual([]);
  });

  it("returns agents from connected hosts", async () => {
    mockClient.tmuxListAgents.mockResolvedValue({
      agents: [
        {
          agentName: "claude",
          sessionName: "dev",
          windowName: "main",
          paneId: "%0",
          paneIndex: 0,
          panePid: 100,
          currentCmd: "claude",
          workingDir: "/home",
          serverId: "s1",
          serverLabel: "local",
        },
      ],
      error: null,
    });

    const { result } = renderAgentsHook();

    await waitFor(() => {
      expect(result.current.agents).toHaveLength(1);
      expect(result.current.agents[0].agentName).toBe("claude");
    });
  });

  it("isInitialLoad stays true until fetch completes with no data", async () => {
    let resolveFetch: (value: { agents: never[]; error: null }) => void;
    const fetchPromise = new Promise<{ agents: never[]; error: null }>((resolve) => {
      resolveFetch = resolve;
    });
    mockClient.tmuxListAgents.mockReturnValue(fetchPromise);

    const { result } = renderAgentsHook();

    // While fetch is in flight, isInitialLoad should be true
    expect(result.current.isInitialLoad).toBe(true);
    expect(result.current.isLoading).toBe(true);

    // Complete the fetch with empty result
    resolveFetch!({ agents: [], error: null });

    await waitFor(() => {
      expect(result.current.isInitialLoad).toBe(false);
      expect(result.current.isLoading).toBe(false);
    });
  });

  describe("sort by lastContentChange descending", () => {
    it("sorts agents by lastContentChange descending (newest first)", async () => {
      mockClient.tmuxListAgents.mockResolvedValue({
        agents: [
          { agentName: "alpha", sessionName: "s1", windowName: "w1", paneId: "%0", paneIndex: 0, panePid: 100, currentCmd: "claude", workingDir: "/tmp", lastContentChange: 1000 },
          { agentName: "beta", sessionName: "s2", windowName: "w1", paneId: "%1", paneIndex: 0, panePid: 200, currentCmd: "claude", workingDir: "/tmp", lastContentChange: 3000 },
          { agentName: "gamma", sessionName: "s3", windowName: "w1", paneId: "%2", paneIndex: 0, panePid: 300, currentCmd: "claude", workingDir: "/tmp", lastContentChange: 2000 },
        ],
        error: null,
      });

      const { result } = renderAgentsHook();

      await waitFor(() => {
        expect(result.current.agents).toHaveLength(3);
      });
      expect(result.current.agents[0].agentName).toBe("beta");
      expect(result.current.agents[1].agentName).toBe("gamma");
      expect(result.current.agents[2].agentName).toBe("alpha");
    });

    it("falls back to agentName when lastContentChange is equal", async () => {
      mockClient.tmuxListAgents.mockResolvedValue({
        agents: [
          { agentName: "zebra", sessionName: "s1", windowName: "w1", paneId: "%0", paneIndex: 0, panePid: 100, currentCmd: "claude", workingDir: "/tmp", lastContentChange: 5000 },
          { agentName: "alpha", sessionName: "s2", windowName: "w1", paneId: "%1", paneIndex: 0, panePid: 200, currentCmd: "claude", workingDir: "/tmp", lastContentChange: 5000 },
        ],
        error: null,
      });

      const { result } = renderAgentsHook();

      await waitFor(() => {
        expect(result.current.agents).toHaveLength(2);
      });
      expect(result.current.agents[0].agentName).toBe("alpha");
      expect(result.current.agents[1].agentName).toBe("zebra");
    });

    it("treats missing lastContentChange as 0 (sorts last)", async () => {
      mockClient.tmuxListAgents.mockResolvedValue({
        agents: [
          { agentName: "no-activity", sessionName: "s1", windowName: "w1", paneId: "%0", paneIndex: 0, panePid: 100, currentCmd: "claude", workingDir: "/tmp" },
          { agentName: "has-activity", sessionName: "s2", windowName: "w1", paneId: "%1", paneIndex: 0, panePid: 200, currentCmd: "claude", workingDir: "/tmp", lastContentChange: 9000 },
        ],
        error: null,
      });

      const { result } = renderAgentsHook();

      await waitFor(() => {
        expect(result.current.agents).toHaveLength(2);
      });
      expect(result.current.agents[0].agentName).toBe("has-activity");
      expect(result.current.agents[1].agentName).toBe("no-activity");
    });

    it("sorts otherPanes by lastContentChange descending", async () => {
      mockClient.tmuxListAgents.mockResolvedValue({
        agents: [],
        otherPanes: [
          { sessionName: "old", windowName: "w1", paneId: "%0", paneIndex: 0, panePid: 100, currentCmd: "bash", workingDir: "/tmp", lastContentChange: 100 },
          { sessionName: "new", windowName: "w1", paneId: "%1", paneIndex: 0, panePid: 200, currentCmd: "bash", workingDir: "/tmp", lastContentChange: 9000 },
          { sessionName: "mid", windowName: "w1", paneId: "%2", paneIndex: 0, panePid: 300, currentCmd: "bash", workingDir: "/tmp", lastContentChange: 5000 },
        ],
        error: null,
      });

      const { result } = renderAgentsHook();

      await waitFor(() => {
        expect(result.current.otherPanes).toHaveLength(3);
      });
      expect(result.current.otherPanes[0].sessionName).toBe("new");
      expect(result.current.otherPanes[1].sessionName).toBe("mid");
      expect(result.current.otherPanes[2].sessionName).toBe("old");
    });

    it("falls back to sessionName for otherPanes when lastContentChange is equal", async () => {
      mockClient.tmuxListAgents.mockResolvedValue({
        agents: [],
        otherPanes: [
          { sessionName: "zebra", windowName: "w1", paneId: "%0", paneIndex: 0, panePid: 100, currentCmd: "bash", workingDir: "/tmp", lastContentChange: 5000 },
          { sessionName: "alpha", windowName: "w1", paneId: "%1", paneIndex: 0, panePid: 200, currentCmd: "bash", workingDir: "/tmp", lastContentChange: 5000 },
        ],
        error: null,
      });

      const { result } = renderAgentsHook();

      await waitFor(() => {
        expect(result.current.otherPanes).toHaveLength(2);
      });
      expect(result.current.otherPanes[0].sessionName).toBe("alpha");
      expect(result.current.otherPanes[1].sessionName).toBe("zebra");
    });
  });
});
