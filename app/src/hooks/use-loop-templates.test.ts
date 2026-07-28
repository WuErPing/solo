/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { LoopListItem, LoopRecord, LoopTemplateSummary } from "@server/server/loop/rpc-schemas";
import { useLoopTemplates, useLoopTemplateDetail } from "./use-loop-templates";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    loopTemplateList: vi.fn(),
    loopTemplateGet: vi.fn(),
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

function renderTemplatesHook(options?: { serverId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useLoopTemplates({ serverId: options?.serverId ?? "server-1", enabled: options?.enabled }), {
    wrapper,
  });
}

function renderTemplateDetailHook(options?: { serverId?: string; templateId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () =>
      useLoopTemplateDetail({
        serverId: options?.serverId ?? "server-1",
        templateId: options?.templateId ?? "template-1",
        enabled: options?.enabled,
      }),
    { wrapper },
  );
}

function makeLoopTemplateSummary(overrides: Partial<LoopTemplateSummary> = {}): LoopTemplateSummary {
  return {
    id: "template-1",
    name: "Weekly refactor",
    cwd: "/repo",
    instanceCount: 2,
    ...overrides,
  };
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
  mockClient.loopTemplateList.mockReset();
  mockClient.loopTemplateGet.mockReset();
});

describe("useLoopTemplates", () => {
  it("loads templates from the daemon client", async () => {
    const templates: LoopTemplateSummary[] = [
      makeLoopTemplateSummary({ id: "template-1", name: "Weekly refactor" }),
      makeLoopTemplateSummary({ id: "template-2", name: "Nightly test" }),
    ];

    mockClient.loopTemplateList.mockResolvedValueOnce({
      requestId: "req-1",
      templates,
      error: null,
    });

    const { result } = renderTemplatesHook();

    await waitFor(() => {
      expect(mockClient.loopTemplateList).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(result.current.templates).toHaveLength(2);
    });

    expect(result.current.templates[0]!.id).toBe("template-1");
    expect(result.current.templates[1]!.name).toBe("Nightly test");
    expect(result.current.isInitialLoad).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("returns empty array when no templates exist", async () => {
    mockClient.loopTemplateList.mockResolvedValueOnce({
      requestId: "req-1",
      templates: [],
      error: null,
    });

    const { result } = renderTemplatesHook();

    await waitFor(() => {
      expect(result.current.isInitialLoad).toBe(false);
    });

    expect(result.current.templates).toEqual([]);
  });

  it("does not fetch when disabled", async () => {
    const { result } = renderTemplatesHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.loopTemplateList).not.toHaveBeenCalled();
    expect(result.current.templates).toEqual([]);
  });

  it("refreshes templates when refreshAll is called", async () => {
    mockClient.loopTemplateList
      .mockResolvedValueOnce({
        requestId: "req-1",
        templates: [makeLoopTemplateSummary()],
        error: null,
      })
      .mockResolvedValueOnce({
        requestId: "req-2",
        templates: [makeLoopTemplateSummary(), makeLoopTemplateSummary({ id: "template-2" })],
        error: null,
      });

    const { result } = renderTemplatesHook();

    await waitFor(() => {
      expect(result.current.templates).toHaveLength(1);
    });

    await act(async () => {
      result.current.refreshAll();
    });

    await waitFor(() => {
      expect(result.current.templates).toHaveLength(2);
    });

    expect(mockClient.loopTemplateList).toHaveBeenCalledTimes(2);
  });
});

describe("useLoopTemplateDetail", () => {
  it("loads template detail from the daemon client", async () => {
    mockClient.loopTemplateGet.mockResolvedValueOnce({
      requestId: "req-1",
      template: makeLoopTemplateSummary(),
      instances: [makeLoopListItem()],
      latestRecord: makeLoopRecord(),
      error: null,
    });

    const { result } = renderTemplateDetailHook();

    await waitFor(() => {
      expect(mockClient.loopTemplateGet).toHaveBeenCalledWith({ templateID: "template-1" });
    });

    await waitFor(() => {
      expect(result.current.template).not.toBeNull();
    });

    expect(result.current.template!.id).toBe("template-1");
    expect(result.current.instances).toHaveLength(1);
    expect(result.current.latestRecord).not.toBeNull();
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("surfaces error when template detail fails", async () => {
    mockClient.loopTemplateGet.mockResolvedValueOnce({
      requestId: "req-1",
      template: null,
      instances: [],
      latestRecord: null,
      error: "template not found",
    });

    const { result } = renderTemplateDetailHook();

    await waitFor(() => {
      expect(result.current.error).toBe("template not found");
    });

    expect(result.current.template).toBeNull();
  });

  it("does not fetch when disabled", async () => {
    const { result } = renderTemplateDetailHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.loopTemplateGet).not.toHaveBeenCalled();
    expect(result.current.template).toBeNull();
  });

  it("does not fetch when templateId is empty", async () => {
    const { result } = renderTemplateDetailHook({ templateId: "" });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.loopTemplateGet).not.toHaveBeenCalled();
    expect(result.current.template).toBeNull();
  });
});
