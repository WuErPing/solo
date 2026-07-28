/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LoopRecord } from "@server/server/loop/rpc-schemas";
import { useLoopMutations } from "./use-loop-mutations";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    loopUpdate: vi.fn(),
    loopDelete: vi.fn(),
    loopStop: vi.fn(),
    loopTemplateDelete: vi.fn(),
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

function renderMutationsHook(serverId: string) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useLoopMutations({ serverId }), { wrapper });
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
  mockClient.loopUpdate.mockReset();
  mockClient.loopDelete.mockReset();
  mockClient.loopStop.mockReset();
  mockClient.loopTemplateDelete.mockReset();
});

describe("useLoopMutations", () => {
  it("updates a loop and returns the updated record", async () => {
    mockClient.loopUpdate.mockResolvedValueOnce({
      requestId: "req-1",
      loop: makeLoopRecord({ name: "Renamed loop" }),
      error: null,
    });

    const { result } = renderMutationsHook("server-1");

    await act(async () => {
      const loop = await result.current.updateLoop({ loopId: "loop-1", name: "Renamed loop" });
      expect(loop.name).toBe("Renamed loop");
    });

    expect(mockClient.loopUpdate).toHaveBeenCalledWith({ id: "loop-1", name: "Renamed loop" });
  });

  it("deletes a loop and returns the loop id", async () => {
    mockClient.loopDelete.mockResolvedValueOnce({
      requestId: "req-1",
      id: "loop-1",
      error: null,
    });

    const { result } = renderMutationsHook("server-1");

    await act(async () => {
      const id = await result.current.deleteLoop("loop-1");
      expect(id).toBe("loop-1");
    });

    expect(mockClient.loopDelete).toHaveBeenCalledWith({ id: "loop-1" });
  });

  it("stops a loop and returns the updated record", async () => {
    mockClient.loopStop.mockResolvedValueOnce({
      requestId: "req-1",
      loop: makeLoopRecord({ status: "stopped" }),
      error: null,
    });

    const { result } = renderMutationsHook("server-1");

    await act(async () => {
      const loop = await result.current.stopLoop("loop-1");
      expect(loop.status).toBe("stopped");
    });

    expect(mockClient.loopStop).toHaveBeenCalledWith({ id: "loop-1" });
  });

  it("deletes a template and returns the template id", async () => {
    mockClient.loopTemplateDelete.mockResolvedValueOnce({
      requestId: "req-1",
      templateID: "template-1",
      error: null,
    });

    const { result } = renderMutationsHook("server-1");

    await act(async () => {
      const id = await result.current.deleteTemplate("template-1");
      expect(id).toBe("template-1");
    });

    expect(mockClient.loopTemplateDelete).toHaveBeenCalledWith({ templateID: "template-1" });
  });

  it("throws when update fails", async () => {
    mockClient.loopUpdate.mockResolvedValueOnce({
      requestId: "req-1",
      loop: null,
      error: "not found",
    });

    const { result } = renderMutationsHook("server-1");

    await expect(result.current.updateLoop({ loopId: "loop-1", name: "x" })).rejects.toThrow("not found");
  });

  it("throws when delete fails", async () => {
    mockClient.loopDelete.mockResolvedValueOnce({
      requestId: "req-1",
      id: "loop-1",
      error: "not found",
    });

    const { result } = renderMutationsHook("server-1");

    await expect(result.current.deleteLoop("loop-1")).rejects.toThrow("not found");
  });

  it("throws when stop fails", async () => {
    mockClient.loopStop.mockResolvedValueOnce({
      requestId: "req-1",
      loop: null,
      error: "already stopped",
    });

    const { result } = renderMutationsHook("server-1");

    await expect(result.current.stopLoop("loop-1")).rejects.toThrow("already stopped");
  });

  it("throws when template delete fails", async () => {
    mockClient.loopTemplateDelete.mockResolvedValueOnce({
      requestId: "req-1",
      templateID: "template-1",
      error: "not found",
    });

    const { result } = renderMutationsHook("server-1");

    await expect(result.current.deleteTemplate("template-1")).rejects.toThrow("not found");
  });

  it("tracks pending mutation states", async () => {
    let resolveUpdate: (value: { requestId: string; loop: LoopRecord; error: string | null }) => void;
    const updatePromise = new Promise<{ requestId: string; loop: LoopRecord; error: string | null }>((resolve) => {
      resolveUpdate = resolve;
    });
    mockClient.loopUpdate.mockReturnValueOnce(updatePromise);

    const { result } = renderMutationsHook("server-1");

    expect(result.current.isUpdating).toBe(false);

    const mutationPromise = result.current.updateLoop({ loopId: "loop-1", name: "x" });

    await waitFor(() => {
      expect(result.current.isUpdating).toBe(true);
    });

    resolveUpdate!({
      requestId: "req-1",
      loop: makeLoopRecord({ name: "x" }),
      error: null,
    });

    await mutationPromise;

    await waitFor(() => {
      expect(result.current.isUpdating).toBe(false);
    });
  });
});
