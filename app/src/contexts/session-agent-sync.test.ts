import { describe, expect, it, vi } from "vitest";
import {
  createApplyAgentUpdatePayload,
  createApplyWorkspaceSetupProgress,
} from "./session-agent-sync";
import type { AgentUpdatePayload, WorkspaceSetupProgressPayload } from "./session-helpers";
import type { Agent } from "@/stores/session-store";

function createMockApplyAgentUpdatePayloadDeps(
  overrides?: Partial<Record<string, ReturnType<typeof vi.fn>>>,
) {
  return {
    serverId: "server-1",
    queryClient: { getQueryData: vi.fn(), setQueryData: vi.fn() } as any,
    setAgents: vi.fn(),
    setPendingPermissions: vi.fn(),
    setQueuedMessages: vi.fn(),
    setAgentTimelineCursor: vi.fn(),
    setAgentAuthoritativeHistoryApplied: vi.fn(),
    setAgentStreamTail: vi.fn(),
    setAgentStreamHead: vi.fn(),
    setInitializingAgents: vi.fn(),
    setFileExplorer: vi.fn(),
    applyAuthoritativeAgentSnapshot: vi.fn(),
    previousAgentStatusRef: { current: new Map() },
    ...overrides,
  };
}

describe("createApplyAgentUpdatePayload", () => {
  it("handles remove — deletes agent from store via setAgents", () => {
    const setAgents = vi.fn();
    const setPendingPermissions = vi.fn();
    const setQueuedMessages = vi.fn();
    const setAgentTimelineCursor = vi.fn();
    const setAgentAuthoritativeHistoryApplied = vi.fn();
    const setAgentStreamTail = vi.fn();
    const setAgentStreamHead = vi.fn();
    const setInitializingAgents = vi.fn();
    const setFileExplorer = vi.fn();
    const applyAuthoritativeAgentSnapshot = vi.fn();

    const deps = createMockApplyAgentUpdatePayloadDeps({
      setAgents,
      setPendingPermissions,
      setQueuedMessages,
      setAgentTimelineCursor,
      setAgentAuthoritativeHistoryApplied,
      setAgentStreamTail,
      setAgentStreamHead,
      setInitializingAgents,
      setFileExplorer,
      applyAuthoritativeAgentSnapshot,
    });

    const handler = createApplyAgentUpdatePayload(deps);

    const update: AgentUpdatePayload = {
      kind: "remove",
      agentId: "agent-to-remove",
    };

    handler(update);

    // Should call setAgents to delete the agent
    expect(setAgents).toHaveBeenCalledTimes(1);
    expect(setAgents.mock.calls[0][0]).toBe("server-1");
    const updater = setAgents.mock.calls[0][1];
    const prev = new Map([["agent-to-remove", {} as Agent]]);
    const result = updater(prev);
    expect(result.has("agent-to-remove")).toBe(false);

    // Should clear pending permissions
    expect(setPendingPermissions).toHaveBeenCalledTimes(1);

    // Should clear queued messages
    expect(setQueuedMessages).toHaveBeenCalledTimes(1);

    // Should clear timeline cursor
    expect(setAgentTimelineCursor).toHaveBeenCalledTimes(1);

    // Should release per-agent stream buffers and explorer state
    for (const setter of [setAgentStreamTail, setAgentStreamHead, setInitializingAgents, setFileExplorer]) {
      expect(setter).toHaveBeenCalledTimes(1);
      expect(setter.mock.calls[0][0]).toBe("server-1");
      const mapUpdater = setter.mock.calls[0][1];
      const before = new Map([["agent-to-remove", ["item"]]]);
      const after = mapUpdater(before);
      expect(after.has("agent-to-remove")).toBe(false);
      expect(mapUpdater(new Map())).toBeInstanceOf(Map);
    }

    // Should mark authoritative history as not applied
    expect(setAgentAuthoritativeHistoryApplied).toHaveBeenCalledWith("server-1", "agent-to-remove", false);

    // Should NOT call applyAuthoritativeAgentSnapshot for remove
    expect(applyAuthoritativeAgentSnapshot).not.toHaveBeenCalled();
  });

  it("handles upsert — calls applyAuthoritativeAgentSnapshot with normalized agent", () => {
    const setAgents = vi.fn();
    const applyAuthoritativeAgentSnapshot = vi.fn();

    const deps = createMockApplyAgentUpdatePayloadDeps({
      setAgents,
      applyAuthoritativeAgentSnapshot,
    });

    const handler = createApplyAgentUpdatePayload(deps);

    const update = {
      kind: "upsert" as const,
      agent: {
        id: "agent-1",
        provider: "claude",
        status: "idle",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        cwd: "/home/user/project",
      },
    } as unknown as AgentUpdatePayload;

    handler(update);

    // Should NOT call setAgents for upsert (that's done via applyAuthoritativeAgentSnapshot)
    expect(setAgents).not.toHaveBeenCalled();

    // Should call applyAuthoritativeAgentSnapshot with the normalized agent
    expect(applyAuthoritativeAgentSnapshot).toHaveBeenCalledTimes(1);
    const agent = applyAuthoritativeAgentSnapshot.mock.calls[0][0];
    expect(agent.id).toBe("agent-1");
    expect(agent.serverId).toBe("server-1");
    expect(agent.projectPlacement).toBeDefined();
  });
});

describe("createApplyWorkspaceSetupProgress", () => {
  it("forwards payload to upsertWorkspaceSetupProgress with serverId", () => {
    const upsertWorkspaceSetupProgress = vi.fn();

    const handler = createApplyWorkspaceSetupProgress({
      serverId: "server-1",
      upsertWorkspaceSetupProgress,
    });

    const payload: WorkspaceSetupProgressPayload = {
      workspaceId: "ws-1",
      status: "running",
      detail: {
        steps: [],
      },
      error: null,
    } as unknown as WorkspaceSetupProgressPayload;

    handler(payload);

    expect(upsertWorkspaceSetupProgress).toHaveBeenCalledTimes(1);
    expect(upsertWorkspaceSetupProgress).toHaveBeenCalledWith({
      serverId: "server-1",
      payload,
    });
  });
});
