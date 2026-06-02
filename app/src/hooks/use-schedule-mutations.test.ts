/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ScheduleSummary } from "@server/server/schedule/types";
import { useSessionStore } from "@/stores/session-store";
import { useScheduleMutations } from "./use-schedule-mutations";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    schedulePause: vi.fn(),
    scheduleResume: vi.fn(),
    scheduleDelete: vi.fn(),
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

  return renderHook(() => useScheduleMutations({ serverId }), { wrapper });
}

function makeScheduleSummary(overrides: Partial<ScheduleSummary> = {}): ScheduleSummary {
  return {
    id: "schedule-1",
    name: "Daily Report",
    prompt: "Generate daily report",
    cadence: { type: "cron", expression: "0 9 * * *" },
    target: { type: "agent", agentId: "agent-1" },
    status: "active",
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
    nextRunAt: "2026-01-02T09:00:00.000Z",
    lastRunAt: null,
    pausedAt: null,
    expiresAt: null,
    maxRuns: null,
    ...overrides,
  };
}

afterEach(() => {
  mockClient.schedulePause.mockReset();
  mockClient.scheduleResume.mockReset();
  mockClient.scheduleDelete.mockReset();
  useSessionStore.setState({ sessions: {}, agentLastActivity: new Map() });
});

describe("useScheduleMutations", () => {
  it("pauses a schedule and returns updated summary", async () => {
    mockClient.schedulePause.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: makeScheduleSummary({ status: "paused" }),
      error: null,
    });

    const { result } = renderMutationsHook("server-1");

    await act(async () => {
      const summary = await result.current.pauseSchedule("schedule-1");
      expect(summary?.status).toBe("paused");
    });

    expect(mockClient.schedulePause).toHaveBeenCalledWith({ id: "schedule-1" });
  });

  it("resumes a schedule and returns updated summary", async () => {
    mockClient.scheduleResume.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: makeScheduleSummary({ status: "active" }),
      error: null,
    });

    const { result } = renderMutationsHook("server-1");

    await act(async () => {
      const summary = await result.current.resumeSchedule("schedule-1");
      expect(summary?.status).toBe("active");
    });

    expect(mockClient.scheduleResume).toHaveBeenCalledWith({ id: "schedule-1" });
  });

  it("deletes a schedule and returns scheduleId", async () => {
    mockClient.scheduleDelete.mockResolvedValueOnce({
      requestId: "req-1",
      scheduleId: "schedule-1",
      error: null,
    });

    const { result } = renderMutationsHook("server-1");

    await act(async () => {
      const scheduleId = await result.current.deleteSchedule("schedule-1");
      expect(scheduleId).toBe("schedule-1");
    });

    expect(mockClient.scheduleDelete).toHaveBeenCalledWith({ id: "schedule-1" });
  });

  it("throws when pause fails", async () => {
    mockClient.schedulePause.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: null,
      error: "not found",
    });

    const { result } = renderMutationsHook("server-1");

    await expect(result.current.pauseSchedule("schedule-1")).rejects.toThrow("not found");
  });

  it("throws when resume fails", async () => {
    mockClient.scheduleResume.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: null,
      error: "already active",
    });

    const { result } = renderMutationsHook("server-1");

    await expect(result.current.resumeSchedule("schedule-1")).rejects.toThrow("already active");
  });

  it("throws when delete fails", async () => {
    mockClient.scheduleDelete.mockResolvedValueOnce({
      requestId: "req-1",
      scheduleId: "schedule-1",
      error: "not found",
    });

    const { result } = renderMutationsHook("server-1");

    await expect(result.current.deleteSchedule("schedule-1")).rejects.toThrow("not found");
  });

  it("tracks pending mutation states", async () => {
    let resolvePause: (value: { requestId: string; schedule: ScheduleSummary; error: string | null }) => void;
    const pausePromise = new Promise<{
      requestId: string;
      schedule: ScheduleSummary;
      error: string | null;
    }>((resolve) => {
      resolvePause = resolve;
    });
    mockClient.schedulePause.mockReturnValueOnce(pausePromise);

    const { result } = renderMutationsHook("server-1");

    expect(result.current.isPausing("schedule-1")).toBe(false);

    // Start the mutation but don't await it yet
    const mutationPromise = result.current.pauseSchedule("schedule-1");

    await waitFor(() => {
      expect(result.current.isPausing("schedule-1")).toBe(true);
    });

    resolvePause!({
      requestId: "req-1",
      schedule: makeScheduleSummary({ status: "paused" }),
      error: null,
    });

    await mutationPromise;

    await waitFor(() => {
      expect(result.current.isPausing("schedule-1")).toBe(false);
    });
  });
});
