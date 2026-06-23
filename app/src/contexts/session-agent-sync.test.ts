import { describe, expect, it, vi } from "vitest";
import {
  createApplyAgentUpdatePayload,
  createApplyWorkspaceSetupProgress,
} from "./session-agent-sync";
import type { AgentUpdatePayload, WorkspaceSetupProgressPayload } from "./session-helpers";
import type { Agent } from "@/stores/session-store";

function createMockApplyAgentUpdatePayloadDeps(overrides?: {
  setAgents?: ReturnType<typeof vi.fn>;
  setPendingPermissions?: ReturnType<typeof vi.fn>;
  setQueuedMessages?: ReturnType<typeof vi.fn>;
  setAgentTimelineCursor?: ReturnType<typeof vi.fn>;
  setAgentAuthoritativeHistoryApplied?: ReturnType<typeof vi.fn>;
  applyAuthoritativeAgentSnapshot?: ReturnType<typeof vi.fn>;
}) {
  return {
    serverId: "server-1",
    queryClient: { getQueryData: vi.fn(), setQueryData: vi.fn() } as any,
    setAgents: overrides?.setAgents ?? vi.fn(),
    setPendingPermissions: overrides?.setPendingPermissions ?? vi.fn(),
    setQueuedMessages: overrides?.setQueuedMessages ?? vi.fn(),
    setAgentTimelineCursor: overrides?.setAgentTimelineCursor ?? vi.fn(),
    setAgentAuthoritativeHistoryApplied: overrides?.setAgentAuthoritativeHistoryApplied ?? vi.fn(),
    applyAuthoritativeAgentSnapshot: overrides?.applyAuthoritativeAgentSnapshot ?? vi.fn(),
    previousAgentStatusRef: { current: new Map() },
  };
}

describe("createApplyAgentUpdatePayload", () => {
  it("handles remove — deletes agent from store via setAgents", () => {
    const setAgents = vi.fn();
    const setPendingPermissions = vi.fn();
    const setQueuedMessages = vi.fn();
    const setAgentTimelineCursor = vi.fn();
    const setAgentAuthoritativeHistoryApplied = vi.fn();
    const applyAuthoritativeAgentSnapshot = vi.fn();

    const deps = createMockApplyAgentUpdatePayloadDeps({
      setAgents,
      setPendingPermissions,
      setQueuedMessages,
      setAgentTimelineCursor,
      setAgentAuthoritativeHistoryApplied,
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
