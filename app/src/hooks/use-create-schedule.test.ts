/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useCreateSchedule } from "./use-create-schedule";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    scheduleCreate: vi.fn(),
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

function renderCreateScheduleHook(serverId: string) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(() => useCreateSchedule({ serverId }), { wrapper });
}

afterEach(() => {
  mockClient.scheduleCreate.mockReset();
});

describe("useCreateSchedule", () => {
  it("creates a schedule with cron cadence", async () => {
    mockClient.scheduleCreate.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: {
        id: "schedule-new",
        name: "Daily Report",
        prompt: "Generate report",
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
      },
      error: null,
    });

    const { result } = renderCreateScheduleHook("server-1");

    await act(async () => {
      const summary = await result.current.createSchedule({
        name: "Daily Report",
        prompt: "Generate report",
        cadence: { type: "cron", expression: "0 9 * * *" },
        target: { type: "agent", agentId: "agent-1" },
      });
      expect(summary?.id).toBe("schedule-new");
    });

    expect(mockClient.scheduleCreate).toHaveBeenCalledWith({
      prompt: "Generate report",
      name: "Daily Report",
      cadence: { type: "cron", expression: "0 9 * * *" },
      target: { type: "agent", agentId: "agent-1" },
    });
  });

  it("creates a schedule with every cadence", async () => {
    mockClient.scheduleCreate.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: {
        id: "schedule-new",
        name: null,
        prompt: "Ping",
        cadence: { type: "every", everyMs: 60000 },
        target: { type: "agent", agentId: "agent-1" },
        status: "active",
        createdAt: "2026-01-01T00:00:00.000Z",
        updatedAt: "2026-01-01T00:00:00.000Z",
        nextRunAt: "2026-01-01T00:01:00.000Z",
        lastRunAt: null,
        pausedAt: null,
        expiresAt: null,
        maxRuns: null,
      },
      error: null,
    });

    const { result } = renderCreateScheduleHook("server-1");

    await act(async () => {
      const summary = await result.current.createSchedule({
        prompt: "Ping",
        cadence: { type: "every", everyMs: 60000 },
        target: { type: "agent", agentId: "agent-1" },
      });
      expect(summary?.name).toBeNull();
    });
  });

  it("throws when creation fails", async () => {
    mockClient.scheduleCreate.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: null,
      error: "invalid cadence",
    });

    const { result } = renderCreateScheduleHook("server-1");

    await expect(
      result.current.createSchedule({
        prompt: "Test",
        cadence: { type: "cron", expression: "invalid" },
        target: { type: "agent", agentId: "agent-1" },
      }),
    ).rejects.toThrow("invalid cadence");
  });

  it("tracks pending state during creation", async () => {
    let resolveCreate: (value: {
      requestId: string;
      schedule: { id: string; name: string | null; prompt: string; cadence: { type: "cron"; expression: string }; target: { type: "agent"; agentId: string }; status: string; createdAt: string; updatedAt: string; nextRunAt: string | null; lastRunAt: string | null; pausedAt: string | null; expiresAt: string | null; maxRuns: number | null }; error: string | null;
    }) => void;
    const createPromise = new Promise<{
      requestId: string;
      schedule: { id: string; name: string | null; prompt: string; cadence: { type: "cron"; expression: string }; target: { type: "agent"; agentId: string }; status: string; createdAt: string; updatedAt: string; nextRunAt: string | null; lastRunAt: string | null; pausedAt: string | null; expiresAt: string | null; maxRuns: number | null };
      error: string | null;
    }>((resolve) => {
      resolveCreate = resolve;
    });
    mockClient.scheduleCreate.mockReturnValueOnce(createPromise);

    const { result } = renderCreateScheduleHook("server-1");

    expect(result.current.isCreating).toBe(false);

    const mutationPromise = result.current.createSchedule({
      prompt: "Test",
      cadence: { type: "cron", expression: "0 9 * * *" },
      target: { type: "agent", agentId: "agent-1" },
    });

    await waitFor(() => {
      expect(result.current.isCreating).toBe(true);
    });

    resolveCreate!({
      requestId: "req-1",
      schedule: {
        id: "schedule-new",
        name: null,
        prompt: "Test",
        cadence: { type: "cron", expression: "0 9 * * *" },
        target: { type: "agent", agentId: "agent-1" },
        status: "active",
        createdAt: "2026-01-01T00:00:00.000Z",
        updatedAt: "2026-01-01T00:00:00.000Z",
        nextRunAt: null,
        lastRunAt: null,
        pausedAt: null,
        expiresAt: null,
        maxRuns: null,
      },
      error: null,
    });

    await mutationPromise;

    await waitFor(() => {
      expect(result.current.isCreating).toBe(false);
    });
  });
});
