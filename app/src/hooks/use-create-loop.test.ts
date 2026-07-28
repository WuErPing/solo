/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LoopRecord } from "@server/server/loop/rpc-schemas";
import { useCreateLoop } from "./use-create-loop";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    loopRun: vi.fn(),
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
      mutations: { retry: false },
    },
  });
}

function renderCreateLoopHook(serverId: string) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useCreateLoop({ serverId }), { wrapper });
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
  mockClient.loopRun.mockReset();
});

describe("useCreateLoop", () => {
  it("creates a loop and returns the record", async () => {
    mockClient.loopRun.mockResolvedValueOnce({
      requestId: "req-1",
      loop: makeLoopRecord({ id: "loop-1" }),
      error: null,
    });

    const { result } = renderCreateLoopHook("server-1");

    await act(async () => {
      const loop = await result.current.createLoop({ prompt: "Refactor", cwd: "/repo" });
      expect(loop.id).toBe("loop-1");
    });

    expect(mockClient.loopRun).toHaveBeenCalledWith({
      prompt: "Refactor",
      cwd: "/repo",
      name: null,
      verifyPrompt: null,
      verifyChecks: undefined,
      sleepMs: undefined,
      maxIterations: undefined,
      maxTimeMs: undefined,
      agentTemplate: null,
      workerAgentTemplate: null,
      verifierAgentTemplate: null,
    });
  });

  it("passes optional configuration to the daemon", async () => {
    mockClient.loopRun.mockResolvedValueOnce({
      requestId: "req-1",
      loop: makeLoopRecord({ id: "loop-2", name: "Named loop" }),
      error: null,
    });

    const { result } = renderCreateLoopHook("server-1");

    await act(async () => {
      await result.current.createLoop({
        prompt: "Refactor",
        cwd: "/repo",
        name: "Named loop",
        verifyChecks: ["npm test"],
        maxIterations: 5,
      });
    });

    expect(mockClient.loopRun).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "Named loop",
        verifyChecks: ["npm test"],
        maxIterations: 5,
      }),
    );
  });

  it("throws when loop creation fails", async () => {
    mockClient.loopRun.mockResolvedValueOnce({
      requestId: "req-1",
      loop: null,
      error: "provider not ready",
    });

    const { result } = renderCreateLoopHook("server-1");

    await expect(result.current.createLoop({ prompt: "Refactor", cwd: "/repo" })).rejects.toThrow(
      "provider not ready",
    );
  });

  it("throws when response has no loop", async () => {
    mockClient.loopRun.mockResolvedValueOnce({
      requestId: "req-1",
      loop: null,
      error: null,
    });

    const { result } = renderCreateLoopHook("server-1");

    await expect(result.current.createLoop({ prompt: "Refactor", cwd: "/repo" })).rejects.toThrow(
      "Loop creation failed",
    );
  });
});
