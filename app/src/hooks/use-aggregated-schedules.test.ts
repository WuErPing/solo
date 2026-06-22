/**
 * @vitest-environment jsdom
 */
import React from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useAggregatedSchedules } from "./use-aggregated-schedules";

const { mockStore } = vi.hoisted(() => {
  const clients: Record<string, { scheduleList: ReturnType<typeof vi.fn> }> = {};
  const hostListListeners = new Set<() => void>();
  const serverListeners = new Map<string, Set<() => void>>();
  const state = {
    hosts: [] as { serverId: string; label: string; connectionStatus: string }[],
  };

  const store = {
    getHosts: vi.fn(() => state.hosts),
    subscribeHostList: vi.fn((listener: () => void) => {
      hostListListeners.add(listener);
      return () => {
        hostListListeners.delete(listener);
      };
    }),
    subscribe: vi.fn((serverId: string, listener: () => void) => {
      if (!serverListeners.has(serverId)) {
        serverListeners.set(serverId, new Set());
      }
      serverListeners.get(serverId)!.add(listener);
      return () => {
        serverListeners.get(serverId)?.delete(listener);
      };
    }),
    getSnapshot: vi.fn((serverId: string) => {
      const host = state.hosts.find((h) => h.serverId === serverId);
      if (!host) return null;
      return {
        connectionStatus: host.connectionStatus,
      };
    }),
    getClient: vi.fn((serverId: string) => {
      return clients[serverId] ?? null;
    }),
    _setHosts: (newHosts: typeof state.hosts) => {
      state.hosts = newHosts;
      hostListListeners.forEach((l) => l());
      state.hosts.forEach((host) => {
        serverListeners.get(host.serverId)?.forEach((l) => l());
      });
    },
    _setClients: (newClients: typeof clients) => {
      Object.keys(clients).forEach((k) => delete clients[k]);
      Object.assign(clients, newClients);
    },
  };

  return { mockStore: store, mockClients: clients };
});

vi.mock("@/runtime/host-runtime", async () => {
  const actual = await vi.importActual<typeof import("react")>("react");
  return {
    getHostRuntimeStore: () => mockStore,
    useHosts: () => {
      return actual.useSyncExternalStore(
        (callback) => mockStore.subscribeHostList(callback),
        () => mockStore.getHosts(),
        () => mockStore.getHosts(),
      );
    },
    isHostRuntimeConnected: (snapshot: { connectionStatus: string } | null) =>
      snapshot?.connectionStatus === "online",
  };
});

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

function renderAggregatedHook() {
  const queryClient = createQueryClient();
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
  return renderHook(() => useAggregatedSchedules(), { wrapper });
}

afterEach(() => {
  mockStore._setHosts([]);
  mockStore._setClients({});
});

describe("useAggregatedSchedules", () => {
  it("aggregates schedules from all connected hosts", async () => {
    mockStore._setHosts([
      { serverId: "host-1", label: "Host One", connectionStatus: "online" },
      { serverId: "host-2", label: "Host Two", connectionStatus: "online" },
    ]);
    mockStore._setClients({
      "host-1": {
        scheduleList: vi.fn(async () => ({
          schedules: [
            { id: "s1", name: "Daily", prompt: "p1", cadence: { type: "cron", expression: "0 9 * * *" }, target: { type: "agent", agentId: "a1" }, cwd: null, status: "active", createdAt: "", updatedAt: "", nextRunAt: null, lastRunAt: null, pausedAt: null, expiresAt: null, maxRuns: null },
          ],
          error: null,
        })),
      },
      "host-2": {
        scheduleList: vi.fn(async () => ({
          schedules: [
            { id: "s2", name: "Weekly", prompt: "p2", cadence: { type: "cron", expression: "0 10 * * 1" }, target: { type: "agent", agentId: "a2" }, cwd: null, status: "paused", createdAt: "", updatedAt: "", nextRunAt: null, lastRunAt: null, pausedAt: null, expiresAt: null, maxRuns: null },
          ],
          error: null,
        })),
      },
    });

    const { result } = renderAggregatedHook();

    await waitFor(() => {
      expect(result.current.schedules).toHaveLength(2);
    });

    expect(result.current.schedules[0]).toMatchObject({
      id: "s1",
      serverId: "host-1",
      serverLabel: "Host One",
    });
    expect(result.current.schedules[1]).toMatchObject({
      id: "s2",
      serverId: "host-2",
      serverLabel: "Host Two",
    });
    expect(result.current.isInitialLoad).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("ignores offline hosts", async () => {
    mockStore._setHosts([
      { serverId: "host-1", label: "Host One", connectionStatus: "online" },
      { serverId: "host-2", label: "Host Two", connectionStatus: "offline" },
    ]);
    mockStore._setClients({
      "host-1": {
        scheduleList: vi.fn(async () => ({
          schedules: [
            { id: "s1", name: "Daily", prompt: "p1", cadence: { type: "cron", expression: "0 9 * * *" }, target: { type: "agent", agentId: "a1" }, cwd: null, status: "active", createdAt: "", updatedAt: "", nextRunAt: null, lastRunAt: null, pausedAt: null, expiresAt: null, maxRuns: null },
          ],
          error: null,
        })),
      },
    });

    const { result } = renderAggregatedHook();

    await waitFor(() => {
      expect(result.current.schedules).toHaveLength(1);
    });

    expect(result.current.schedules[0]?.serverId).toBe("host-1");
  });

  it("updates when a host comes online", async () => {
    mockStore._setHosts([{ serverId: "host-1", label: "Host One", connectionStatus: "offline" }]);
    mockStore._setClients({});

    const { result } = renderAggregatedHook();

    expect(result.current.schedules).toHaveLength(0);

    mockStore._setClients({
      "host-1": {
        scheduleList: vi.fn(async () => ({
          schedules: [
            { id: "s1", name: "Daily", prompt: "p1", cadence: { type: "cron", expression: "0 9 * * *" }, target: { type: "agent", agentId: "a1" }, cwd: null, status: "active", createdAt: "", updatedAt: "", nextRunAt: null, lastRunAt: null, pausedAt: null, expiresAt: null, maxRuns: null },
          ],
          error: null,
        })),
      },
    });
    mockStore._setHosts([{ serverId: "host-1", label: "Host One", connectionStatus: "online" }]);

    await waitFor(() => {
      expect(result.current.schedules).toHaveLength(1);
    });
  });
});
