/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LoopRecord } from "@server/server/loop/rpc-schemas";
import { useLoopInspect } from "./use-loop-inspect";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    loopInspect: vi.fn(),
  };
  return { mockClient: hoistedClient };
});

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => mockClient,
  useHostRuntimeIsConnected: () => true,
}));

vi.mock("@/hooks/use-app-visible", () => ({
  useAppVisible: () => true,
}));

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function renderInspectHook(options?: { serverId?: string; loopId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () =>
      useLoopInspect({
        serverId: options?.serverId ?? "server-1",
        loopId: options?.loopId ?? "loop-1",
        enabled: options?.enabled,
      }),
    { wrapper },
  );
}

function makeLoopRecord(overrides: Partial<LoopRecord> = {}): LoopRecord {
  return {
    id: "loop-1",
    name: "Refactor loop",
    prompt: "Refactor the parser",
    cwd: "/repo",
    provider: "claude",
    model: null,
    workerProvider: null,
    workerModel: null,
    verifierProvider: null,
    verifierModel: null,
    verifyPrompt: null,
    verifyChecks: [],
    archive: false,
    sleepMs: 0,
    maxIterations: null,
    maxTimeMs: null,
    status: "running",
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
    startedAt: "2026-01-01T00:00:00.000Z",
    completedAt: null,
    stopRequestedAt: null,
    iterations: [],
    logs: [],
    nextLogSeq: 1,
    activeIteration: 1,
    activeWorkerAgentId: null,
    activeVerifierAgentId: null,
    agentTemplate: null,
    workerAgentTemplate: null,
    verifierAgentTemplate: null,
    ...overrides,
  };
}

afterEach(() => {
  mockClient.loopInspect.mockReset();
});

describe("useLoopInspect", () => {
  it("loads a loop from the daemon client", async () => {
    const loop = makeLoopRecord({
      iterations: [
        {
          index: 1,
          workerAgentId: "agent-1",
          workerStartedAt: "2026-01-01T09:00:00.000Z",
          workerCompletedAt: null,
          verifierAgentId: null,
          status: "running",
          workerOutcome: null,
          failureReason: null,
          verifyChecks: [],
          verifyPrompt: null,
        },
      ],
    });

    mockClient.loopInspect.mockResolvedValueOnce({
      requestId: "req-1",
      loop,
      error: null,
    });

    const { result } = renderInspectHook();

    await waitFor(() => {
      expect(mockClient.loopInspect).toHaveBeenCalledWith({ id: "loop-1" });
    });

    await waitFor(() => {
      expect(result.current.loop).not.toBeNull();
    });

    expect(result.current.loop!.id).toBe("loop-1");
    expect(result.current.loop!.iterations).toHaveLength(1);
    expect(result.current.loop!.iterations[0]!.status).toBe("running");
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("returns null loop when not yet loaded", () => {
    mockClient.loopInspect.mockReturnValue(new Promise(() => {}));

    const { result } = renderInspectHook();

    expect(result.current.loop).toBeNull();
    expect(result.current.isLoading).toBe(true);
  });

  it("surfaces error when loopInspect fails", async () => {
    mockClient.loopInspect.mockResolvedValueOnce({
      requestId: "req-1",
      loop: null,
      error: "loop not found",
    });

    const { result } = renderInspectHook();

    await waitFor(() => {
      expect(result.current.error).toBe("loop not found");
    });

    expect(result.current.loop).toBeNull();
  });

  it("does not fetch when disabled", async () => {
    const { result } = renderInspectHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.loopInspect).not.toHaveBeenCalled();
    expect(result.current.loop).toBeNull();
    expect(result.current.isLoading).toBe(false);
  });

  it("does not fetch when loopId is empty", async () => {
    const { result } = renderInspectHook({ loopId: "" });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.loopInspect).not.toHaveBeenCalled();
    expect(result.current.loop).toBeNull();
  });

  it("refetches when refresh is called", async () => {
    const loop = makeLoopRecord();
    mockClient.loopInspect
      .mockResolvedValueOnce({ requestId: "req-1", loop, error: null })
      .mockResolvedValueOnce({
        requestId: "req-2",
        loop: makeLoopRecord({ name: "Updated loop" }),
        error: null,
      });

    const { result } = renderInspectHook();

    await waitFor(() => {
      expect(result.current.loop?.name).toBe("Refactor loop");
    });

    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => {
      expect(result.current.loop?.name).toBe("Updated loop");
    });

    expect(mockClient.loopInspect).toHaveBeenCalledTimes(2);
  });
});
