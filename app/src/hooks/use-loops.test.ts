/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LoopListItem } from "@server/server/loop/rpc-schemas";
import { useLoops } from "./use-loops";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    loopList: vi.fn(),
  };
  return { mockClient: hoistedClient };
});

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => mockClient,
  useHostRuntimeIsConnected: () => true,
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function renderLoopsHook(options?: { serverId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useLoops({ serverId: options?.serverId ?? "server-1", enabled: options?.enabled }), {
    wrapper,
  });
}

function makeLoopListItem(overrides: Partial<LoopListItem> = {}): LoopListItem {
  return {
    id: "loop-1",
    name: "Refactor loop",
    status: "running",
    cwd: "/repo",
    provider: "claude",
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
    activeIteration: 1,
    ...overrides,
  };
}

afterEach(() => {
  mockClient.loopList.mockReset();
});

describe("useLoops", () => {
  it("loads loops from the daemon client", async () => {
    const loops: LoopListItem[] = [
      makeLoopListItem({ id: "loop-1", name: "Refactor loop" }),
      makeLoopListItem({ id: "loop-2", name: "Test loop", status: "succeeded", activeIteration: null }),
    ];

    mockClient.loopList.mockResolvedValueOnce({
      requestId: "req-1",
      loops,
      error: null,
    });

    const { result } = renderLoopsHook();

    await waitFor(() => {
      expect(mockClient.loopList).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(result.current.loops).toHaveLength(2);
    });

    expect(result.current.loops[0]!.id).toBe("loop-1");
    expect(result.current.loops[1]!.status).toBe("succeeded");
    expect(result.current.isInitialLoad).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("returns empty array when no loops exist", async () => {
    mockClient.loopList.mockResolvedValueOnce({
      requestId: "req-1",
      loops: [],
      error: null,
    });

    const { result } = renderLoopsHook();

    await waitFor(() => {
      expect(result.current.isInitialLoad).toBe(false);
    });

    expect(result.current.loops).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("does not call loopList when disabled", async () => {
    const { result } = renderLoopsHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.loopList).not.toHaveBeenCalled();
    expect(result.current.loops).toEqual([]);
    expect(result.current.isInitialLoad).toBe(false);
  });

  it("does not call loopList when serverId is empty", async () => {
    const { result } = renderLoopsHook({ serverId: "" });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.loopList).not.toHaveBeenCalled();
    expect(result.current.loops).toEqual([]);
  });

  it("surfaces error when loopList fails", async () => {
    mockClient.loopList.mockResolvedValueOnce({
      requestId: "req-1",
      loops: [],
      error: "connection lost",
    });

    const { result } = renderLoopsHook();

    await waitFor(() => {
      expect(result.current.error).toBe("connection lost");
    });

    expect(result.current.loops).toEqual([]);
    expect(result.current.isInitialLoad).toBe(false);
  });

  it("refreshes loops when refreshAll is called", async () => {
    mockClient.loopList
      .mockResolvedValueOnce({
        requestId: "req-1",
        loops: [makeLoopListItem()],
        error: null,
      })
      .mockResolvedValueOnce({
        requestId: "req-2",
        loops: [makeLoopListItem(), makeLoopListItem({ id: "loop-2" })],
        error: null,
      });

    const { result } = renderLoopsHook();

    await waitFor(() => {
      expect(result.current.loops).toHaveLength(1);
    });

    await act(async () => {
      result.current.refreshAll();
    });

    await waitFor(() => {
      expect(result.current.loops).toHaveLength(2);
    });

    expect(mockClient.loopList).toHaveBeenCalledTimes(2);
  });
});
