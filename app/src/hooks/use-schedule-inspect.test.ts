/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { StoredSchedule } from "@server/server/schedule/types";
import { useScheduleInspect } from "./use-schedule-inspect";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    scheduleInspect: vi.fn(),
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

function renderInspectHook(options?: { serverId?: string; scheduleId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () =>
      useScheduleInspect({
        serverId: options?.serverId ?? "server-1",
        scheduleId: options?.scheduleId ?? "schedule-1",
        enabled: options?.enabled,
      }),
    { wrapper },
  );
}

function makeStoredSchedule(overrides: Partial<StoredSchedule> = {}): StoredSchedule {
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
    runs: [],
    ...overrides,
  };
}

afterEach(() => {
  mockClient.scheduleInspect.mockReset();
});

describe("useScheduleInspect", () => {
  it("loads a schedule with runs from the daemon client", async () => {
    const schedule = makeStoredSchedule({
      runs: [
        {
          id: "run-1",
          scheduledFor: "2026-01-01T09:00:00.000Z",
          startedAt: "2026-01-01T09:00:01.000Z",
          endedAt: "2026-01-01T09:01:30.000Z",
          status: "succeeded",
          agentId: "agent-1",
          output: "Report generated",
          error: null,
        },
      ],
    });

    mockClient.scheduleInspect.mockResolvedValueOnce({
      requestId: "req-1",
      schedule,
      error: null,
    });

    const { result } = renderInspectHook();

    await waitFor(() => {
      expect(mockClient.scheduleInspect).toHaveBeenCalledWith({ id: "schedule-1" });
    });

    await waitFor(() => {
      expect(result.current.schedule).not.toBeNull();
    });

    expect(result.current.schedule!.id).toBe("schedule-1");
    expect(result.current.schedule!.runs).toHaveLength(1);
    expect(result.current.schedule!.runs[0]!.status).toBe("succeeded");
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("returns null schedule when not yet loaded", () => {
    mockClient.scheduleInspect.mockReturnValue(new Promise(() => {}));

    const { result } = renderInspectHook();

    expect(result.current.schedule).toBeNull();
    expect(result.current.isLoading).toBe(true);
  });

  it("surfaces error when scheduleInspect fails", async () => {
    mockClient.scheduleInspect.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: null,
      error: "schedule not found",
    });

    const { result } = renderInspectHook();

    await waitFor(() => {
      expect(result.current.error).toBe("schedule not found");
    });

    expect(result.current.schedule).toBeNull();
  });

  it("does not fetch when disabled", async () => {
    const { result } = renderInspectHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.scheduleInspect).not.toHaveBeenCalled();
    expect(result.current.schedule).toBeNull();
    expect(result.current.isLoading).toBe(false);
  });

  it("does not fetch when scheduleId is empty", async () => {
    const { result } = renderInspectHook({ scheduleId: "" });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.scheduleInspect).not.toHaveBeenCalled();
    expect(result.current.schedule).toBeNull();
  });

  it("refetches when refresh is called", async () => {
    const schedule = makeStoredSchedule();
    mockClient.scheduleInspect
      .mockResolvedValueOnce({ requestId: "req-1", schedule, error: null })
      .mockResolvedValueOnce({
        requestId: "req-2",
        schedule: makeStoredSchedule({ name: "Updated Report" }),
        error: null,
      });

    const { result } = renderInspectHook();

    await waitFor(() => {
      expect(result.current.schedule?.name).toBe("Daily Report");
    });

    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => {
      expect(result.current.schedule?.name).toBe("Updated Report");
    });

    expect(mockClient.scheduleInspect).toHaveBeenCalledTimes(2);
  });
});
