/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { UsageQuotaSnapshot } from "@server/client/usage-rpc";
import { useUsage } from "./use-usage";

const { mockClient } = vi.hoisted(() => {
  const hoistedClient = {
    usageQuotaList: vi.fn(),
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

function renderUsageHook(options?: { serverId?: string; enabled?: boolean }) {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  return renderHook(
    () => useUsage({ serverId: options?.serverId ?? "server-1", enabled: options?.enabled }),
    { wrapper },
  );
}

function makeUsageQuotaSnapshot(overrides: Partial<UsageQuotaSnapshot> = {}): UsageQuotaSnapshot {
  return {
    provider: "anthropic",
    plan: { name: "Pro", tier: "pro" },
    quotas: [
      {
        name: "5h",
        label: "5h",
        used: 10,
        limit: 100,
        usedPct: 10,
        unit: "requests",
        resetAt: null,
        resetIn: "2h 30m",
      },
    ],
    fetchedAt: "2026-01-01T00:00:00.000Z",
    ...overrides,
  };
}

afterEach(() => {
  mockClient.usageQuotaList.mockReset();
});

describe("useUsage", () => {
  it("loads usage snapshots from the daemon client", async () => {
    const snapshots: UsageQuotaSnapshot[] = [
      makeUsageQuotaSnapshot({ provider: "anthropic" }),
      makeUsageQuotaSnapshot({ provider: "openai", plan: null }),
    ];

    mockClient.usageQuotaList.mockResolvedValueOnce({
      requestId: "req-1",
      snapshots,
      cachedAt: "2026-01-01T00:00:00.000Z",
      error: null,
    });

    const { result } = renderUsageHook();

    await waitFor(() => {
      expect(mockClient.usageQuotaList).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(result.current.snapshots).toHaveLength(2);
    });

    expect(result.current.snapshots[0]!.provider).toBe("anthropic");
    expect(result.current.snapshots[1]!.plan).toBeNull();
    expect(result.current.cachedAt).toBe("2026-01-01T00:00:00.000Z");
    expect(result.current.isInitialLoad).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("returns empty array when no snapshots exist", async () => {
    mockClient.usageQuotaList.mockResolvedValueOnce({
      requestId: "req-1",
      snapshots: [],
      cachedAt: "2026-01-01T00:00:00.000Z",
      error: null,
    });

    const { result } = renderUsageHook();

    await waitFor(() => {
      expect(result.current.isInitialLoad).toBe(false);
    });

    expect(result.current.snapshots).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("does not call usageQuotaList when disabled", async () => {
    const { result } = renderUsageHook({ enabled: false });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.usageQuotaList).not.toHaveBeenCalled();
    expect(result.current.snapshots).toEqual([]);
    expect(result.current.isInitialLoad).toBe(false);
  });

  it("does not call usageQuotaList when serverId is empty", async () => {
    const { result } = renderUsageHook({ serverId: "" });

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockClient.usageQuotaList).not.toHaveBeenCalled();
    expect(result.current.snapshots).toEqual([]);
  });

  it("surfaces error when usageQuotaList fails", async () => {
    mockClient.usageQuotaList.mockResolvedValueOnce({
      requestId: "req-1",
      snapshots: [],
      cachedAt: "2026-01-01T00:00:00.000Z",
      error: "connection lost",
    });

    const { result } = renderUsageHook();

    await waitFor(() => {
      expect(result.current.error).toBe("connection lost");
    });

    expect(result.current.snapshots).toEqual([]);
    expect(result.current.isInitialLoad).toBe(false);
  });

  it("surfaces per-provider errors", async () => {
    mockClient.usageQuotaList.mockResolvedValueOnce({
      requestId: "req-1",
      snapshots: [makeUsageQuotaSnapshot()],
      errors: { openai: "rate limited" },
      cachedAt: "2026-01-01T00:00:00.000Z",
      error: null,
    });

    const { result } = renderUsageHook();

    await waitFor(() => {
      expect(result.current.snapshots).toHaveLength(1);
    });

    expect(result.current.errors).toEqual({ openai: "rate limited" });
    expect(result.current.error).toBeNull();
  });

  it("refreshes usage when refreshAll is called", async () => {
    mockClient.usageQuotaList
      .mockResolvedValueOnce({
        requestId: "req-1",
        snapshots: [makeUsageQuotaSnapshot()],
        cachedAt: "2026-01-01T00:00:00.000Z",
        error: null,
      })
      .mockResolvedValueOnce({
        requestId: "req-2",
        snapshots: [makeUsageQuotaSnapshot(), makeUsageQuotaSnapshot({ provider: "openai" })],
        cachedAt: "2026-01-01T00:01:00.000Z",
        error: null,
      });

    const { result } = renderUsageHook();

    await waitFor(() => {
      expect(result.current.snapshots).toHaveLength(1);
    });

    await act(async () => {
      result.current.refreshAll();
    });

    await waitFor(() => {
      expect(result.current.snapshots).toHaveLength(2);
    });

    expect(mockClient.usageQuotaList).toHaveBeenCalledTimes(2);
  });

  it("passes forceRefresh to the daemon client when requested", async () => {
    mockClient.usageQuotaList.mockResolvedValue({
      requestId: "req-1",
      snapshots: [makeUsageQuotaSnapshot()],
      cachedAt: "2026-01-01T00:00:00.000Z",
      error: null,
    });

    const { result } = renderUsageHook();

    await waitFor(() => {
      expect(result.current.snapshots).toHaveLength(1);
    });

    expect(mockClient.usageQuotaList).toHaveBeenNthCalledWith(1, undefined);

    await act(async () => {
      result.current.refreshAll({ forceRefresh: true });
    });

    await waitFor(() => {
      expect(mockClient.usageQuotaList).toHaveBeenCalledTimes(2);
    });

    expect(mockClient.usageQuotaList).toHaveBeenNthCalledWith(2, { forceRefresh: true });
  });
});
