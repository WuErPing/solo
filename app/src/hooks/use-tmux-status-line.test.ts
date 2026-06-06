/**
 * @vitest-environment jsdom
 */
import React from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useTmuxStatusLine } from "./use-tmux-status-line";

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

function renderStatusLineHook(sessionId: string) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useTmuxStatusLine(sessionId), { wrapper });
}

beforeEach(() => {
  mockStore.getClient.mockReturnValue(mockClient);
  mockStore.getSnapshot.mockReturnValue({ connectionStatus: "online" });
  mockClient.getConnectionState.mockReturnValue({ status: "connected" });
  mockClient.tmuxStatusLine.mockResolvedValue({
    statusLeft: "[#S]",
    statusCenter: "0:claude*",
    statusRight: "%H:%M %d-%b-%y",
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

describe("useTmuxStatusLine", () => {
  it("fetches status line for a given session", async () => {
    const { result } = renderStatusLineHook("dev");

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual({
      statusLeft: "[#S]",
      statusCenter: "0:claude*",
      statusRight: "%H:%M %d-%b-%y",
      paneBackground: "#1e1e2e",
      paneForeground: "#cdd6f4",
      serverId: "s1",
    });
    expect(mockClient.tmuxStatusLine).toHaveBeenCalledWith("dev");
  });

  it("does not fetch when sessionId is empty", () => {
    const { result } = renderStatusLineHook("");

    expect(result.current.isFetching).toBe(false);
    expect(mockClient.tmuxStatusLine).not.toHaveBeenCalled();
  });

  it("does not fetch when no hosts are connected", () => {
    isConnectedOverride = false;
    mockStore.getClient.mockReturnValue(null);
    mockHosts.value = [];

    const { result } = renderStatusLineHook("dev");

    expect(result.current.isFetching).toBe(false);
    expect(mockClient.tmuxStatusLine).not.toHaveBeenCalled();
  });

  it("handles empty status line response", async () => {
    mockClient.tmuxStatusLine.mockResolvedValue({
      statusLeft: "",
      statusCenter: "",
      statusRight: "",
      paneBackground: "",
      paneForeground: "",
      error: null,
    });

    const { result } = renderStatusLineHook("dev");

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual({
      statusLeft: "",
      statusCenter: "",
      statusRight: "",
      paneBackground: "",
      paneForeground: "",
      serverId: "s1",
    });
  });

  it("handles error response", async () => {
    mockClient.tmuxStatusLine.mockRejectedValue(new Error("tmux not available"));

    const { result } = renderStatusLineHook("dev");

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});
