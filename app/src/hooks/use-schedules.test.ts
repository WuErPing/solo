/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ScheduleSummary } from "@server/server/schedule/types";
import { useSchedules } from "./use-schedules";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    scheduleList: vi.fn(),
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

function renderSchedulesHook(options?: { serverId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useSchedules({ serverId: options?.serverId ?? "server-1", enabled: options?.enabled }), {
    wrapper,
  });
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
  mockClient.scheduleList.mockReset();
});

describe("useSchedules", () => {
  it("loads schedules from the daemon client", async () => {
    const schedules: ScheduleSummary[] = [
      makeScheduleSummary({ id: "schedule-1", name: "Daily Report" }),
      makeScheduleSummary({ id: "schedule-2", name: "Weekly Sync", status: "paused" }),
    ];

    mockClient.scheduleList.mockResolvedValueOnce({
      requestId: "req-1",
      schedules,
      error: null,
    });

    const { result } = renderSchedulesHook();

    await waitFor(() => {
      expect(mockClient.scheduleList).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(result.current.schedules).toHaveLength(2);
    });

    expect(result.current.schedules[0]!.id).toBe("schedule-1");
    expect(result.current.schedules[1]!.status).toBe("paused");
    expect(result.current.isInitialLoad).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("returns empty array when no schedules exist", async () => {
    mockClient.scheduleList.mockResolvedValueOnce({
      requestId: "req-1",
      schedules: [],
      error: null,
    });

    const { result } = renderSchedulesHook();

    await waitFor(() => {
      expect(result.current.isInitialLoad).toBe(false);
    });

    expect(result.current.schedules).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("does not call scheduleList when disabled", async () => {
    const { result } = renderSchedulesHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.scheduleList).not.toHaveBeenCalled();
    expect(result.current.schedules).toEqual([]);
    expect(result.current.isInitialLoad).toBe(false);
  });

  it("does not call scheduleList when serverId is empty", async () => {
    const { result } = renderSchedulesHook({ serverId: "" });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.scheduleList).not.toHaveBeenCalled();
    expect(result.current.schedules).toEqual([]);
  });

  it("surfaces error when scheduleList fails", async () => {
    mockClient.scheduleList.mockResolvedValueOnce({
      requestId: "req-1",
      schedules: [],
      error: "connection lost",
    });

    const { result } = renderSchedulesHook();

    await waitFor(() => {
      expect(result.current.error).toBe("connection lost");
    });

    expect(result.current.schedules).toEqual([]);
    expect(result.current.isInitialLoad).toBe(false);
  });

  it("refreshes schedules when refreshAll is called", async () => {
    mockClient.scheduleList
      .mockResolvedValueOnce({
        requestId: "req-1",
        schedules: [makeScheduleSummary()],
        error: null,
      })
      .mockResolvedValueOnce({
        requestId: "req-2",
        schedules: [makeScheduleSummary(), makeScheduleSummary({ id: "schedule-2" })],
        error: null,
      });

    const { result } = renderSchedulesHook();

    await waitFor(() => {
      expect(result.current.schedules).toHaveLength(1);
    });

    await act(async () => {
      result.current.refreshAll();
    });

    await waitFor(() => {
      expect(result.current.schedules).toHaveLength(2);
    });

    expect(mockClient.scheduleList).toHaveBeenCalledTimes(2);
  });
});
