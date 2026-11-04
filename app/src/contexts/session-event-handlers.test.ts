import { describe, expect, it, vi } from "vitest";
import type { DaemonClient } from "@server/client/daemon-client";
import {
  subscribeToSessionEvents,
  type SubscribeToSessionEventsDeps,
} from "./session-event-handlers";

function createHarness() {
  const handlers = new Map<string, (message: unknown) => void>();
  const client = {
    on: vi.fn((type: string, handler: (message: unknown) => void) => {
      handlers.set(type, handler);
      return vi.fn();
    }),
  } as unknown as DaemonClient;
  const setPendingPermissions = vi.fn();
  const deps = {
    client,
    serverId: "server-1",
    setPendingPermissions,
  } as unknown as SubscribeToSessionEventsDeps;

  return {
    setPendingPermissions,
    subscribe: () => subscribeToSessionEvents(deps),
    emitAgentStream: (event: Record<string, unknown>) => {
      handlers.get("agent_stream")?.({
        type: "agent_stream",
        payload: {
          agentId: "agent-1",
          event,
          timestamp: "2026-11-03T15:00:00Z",
        },
      });
    },
  };
}

describe("subscribeToSessionEvents permissions", () => {
  it("tracks permission requests and resolutions from agent streams", () => {
    const harness = createHarness();
    const unsubscribe = harness.subscribe();

    harness.emitAgentStream({
      type: "permission_requested",
      provider: "kimi",
      request: {
        id: "perm-1",
        provider: "kimi",
        name: "file_editor",
        kind: "tool",
      },
    });

    expect(harness.setPendingPermissions).toHaveBeenCalledTimes(1);
    const addUpdater = harness.setPendingPermissions.mock.calls[0][1];
    const pending = addUpdater(new Map());
    expect(pending.get("agent-1:perm-1")).toMatchObject({
      agentId: "agent-1",
      request: { id: "perm-1" },
    });

    harness.emitAgentStream({
      type: "permission_resolved",
      provider: "kimi",
      requestId: "perm-1",
    });

    expect(harness.setPendingPermissions).toHaveBeenCalledTimes(2);
    const removeUpdater = harness.setPendingPermissions.mock.calls[1][1];
    expect(removeUpdater(pending).size).toBe(0);
    unsubscribe();
  });
});
