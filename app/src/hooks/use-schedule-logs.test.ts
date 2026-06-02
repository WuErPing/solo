/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ScheduleRun } from "@server/server/schedule/types";
import { useScheduleLogs } from "./use-schedule-logs";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    scheduleLogs: vi.fn(),
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

function renderLogsHook(options?: { serverId?: string; scheduleId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () =>
      useScheduleLogs({
        serverId: options?.serverId ?? "server-1",
        scheduleId: options?.scheduleId ?? "schedule-1",
        enabled: options?.enabled,
      }),
    { wrapper },
  );
}

function makeScheduleRun(overrides: Partial<ScheduleRun> = {}): ScheduleRun {
  return {
    id: "run-1",
    scheduledFor: "2026-01-01T09:00:00.000Z",
    startedAt: "2026-01-01T09:00:01.000Z",
    endedAt: "2026-01-01T09:01:30.000Z",
    status: "succeeded",
    agentId: "agent-1",
    output: "Report generated successfully",
    error: null,
    ...overrides,
  };
}

afterEach(() => {
  mockClient.scheduleLogs.mockReset();
});

describe("useScheduleLogs", () => {
  it("loads runs from the daemon client", async () => {
    const runs: ScheduleRun[] = [
      makeScheduleRun({ id: "run-1", status: "succeeded" }),
      makeScheduleRun({ id: "run-2", status: "failed", error: "timeout", output: null }),
    ];

    mockClient.scheduleLogs.mockResolvedValueOnce({
      requestId: "req-1",
      runs,
      error: null,
    });

    const { result } = renderLogsHook();

    await waitFor(() => {
      expect(mockClient.scheduleLogs).toHaveBeenCalledWith({ id: "schedule-1" });
    });

    await waitFor(() => {
      expect(result.current.runs).toHaveLength(2);
    });

    expect(result.current.runs[0]!.status).toBe("succeeded");
    expect(result.current.runs[1]!.error).toBe("timeout");
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("returns empty array when no runs exist", async () => {
    mockClient.scheduleLogs.mockResolvedValueOnce({
      requestId: "req-1",
      runs: [],
      error: null,
    });

    const { result } = renderLogsHook();

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.runs).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("surfaces error when scheduleLogs fails", async () => {
    mockClient.scheduleLogs.mockResolvedValueOnce({
      requestId: "req-1",
      runs: [],
      error: "schedule not found",
    });

    const { result } = renderLogsHook();

    await waitFor(() => {
      expect(result.current.error).toBe("schedule not found");
    });

    expect(result.current.runs).toEqual([]);
  });

  it("does not fetch when disabled", async () => {
    const { result } = renderLogsHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.scheduleLogs).not.toHaveBeenCalled();
    expect(result.current.runs).toEqual([]);
    expect(result.current.isLoading).toBe(false);
  });

  it("does not fetch when scheduleId is empty", async () => {
    const { result } = renderLogsHook({ scheduleId: "" });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.scheduleLogs).not.toHaveBeenCalled();
    expect(result.current.runs).toEqual([]);
  });

  it("refetches when refresh is called", async () => {
    mockClient.scheduleLogs
      .mockResolvedValueOnce({
        requestId: "req-1",
        runs: [makeScheduleRun()],
        error: null,
      })
      .mockResolvedValueOnce({
        requestId: "req-2",
        runs: [makeScheduleRun(), makeScheduleRun({ id: "run-2" })],
        error: null,
      });

    const { result } = renderLogsHook();

    await waitFor(() => {
      expect(result.current.runs).toHaveLength(1);
    });

    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => {
      expect(result.current.runs).toHaveLength(2);
    });

    expect(mockClient.scheduleLogs).toHaveBeenCalledTimes(2);
  });
});
