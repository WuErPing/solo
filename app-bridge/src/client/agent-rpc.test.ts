import { describe, expect, it, vi, afterEach } from "vitest";
import {
  createConnectedClient,
  simulateServerResponse,
  mockAgentSnapshot,
} from "./daemon-client-test-harness.js";

afterEach(() => {
  vi.useRealTimers();
});

function findSentMessage(
  transport: { sentMessages: Array<{ parsed: { type: string; message?: unknown } }> },
  messageType: string,
) {
  return transport.sentMessages.find(
    (m) =>
      m.parsed.type === "session" &&
      (m.parsed as { message?: { type?: string } }).message?.type === messageType,
  );
}

function getRequestId(sent: { parsed: { type: string; message?: unknown } } | undefined): string {
  return (sent?.parsed as { message?: { requestId?: string } }).message?.requestId ?? "";
}

describe("AgentRpc — lifecycle", () => {
  it("createAgent sends create_agent_request and resolves with agent snapshot", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const agentPromise = client.createAgent({
      requestId: "req-create-1",
      provider: "claude-code",
      cwd: "/test/project",
    });

    const sent = findSentMessage(transport, "create_agent_request");
    expect(sent).toBeDefined();
    const msg = (sent!.parsed as { message?: { config?: { provider?: string; cwd?: string } } }).message;
    expect(msg?.config?.provider).toBe("claude-code");
    expect(msg?.config?.cwd).toBe("/test/project");

    simulateServerResponse(transport, {
      type: "status",
      payload: {
        status: "agent_created",
        requestId: "req-create-1",
        agentId: "new-agent",
        agent: mockAgentSnapshot({ id: "new-agent" }),
      },
    });

    const agent = await agentPromise;
    expect(agent.id).toBe("new-agent");
    await cleanup();
  });

  it("createAgent throws on agent_create_failed", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const agentPromise = client.createAgent({
      requestId: "req-create-fail",
      provider: "claude-code",
      cwd: "/test/project",
    });

    simulateServerResponse(transport, {
      type: "status",
      payload: {
        status: "agent_create_failed",
        requestId: "req-create-fail",
        error: "Provider unavailable",
      },
    });

    await expect(agentPromise).rejects.toThrow("Provider unavailable");
    await cleanup();
  });

  it("createAgent requires provider and cwd", async () => {
    const { client, cleanup } = createConnectedClient();

    await expect(
      client.createAgent({ provider: "", cwd: "/test" } as never),
    ).rejects.toThrow();

    await cleanup();
  });

  it("deleteAgent sends delete_agent_request and awaits agent_deleted", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const deletePromise = client.deleteAgent("agent-1");

    const sent = findSentMessage(transport, "delete_agent_request");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "agent_deleted",
      payload: { requestId: getRequestId(sent), agentId: "agent-1" },
    });

    await deletePromise;
    await cleanup();
  });

  it("archiveAgent returns archivedAt", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const archivePromise = client.archiveAgent("agent-1");

    const sent = findSentMessage(transport, "archive_agent_request");
    simulateServerResponse(transport, {
      type: "agent_archived",
      payload: { requestId: getRequestId(sent), agentId: "agent-1", archivedAt: "2026-01-01T00:00:00Z" },
    });

    const result = await archivePromise;
    expect(result.archivedAt).toBe("2026-01-01T00:00:00Z");
    await cleanup();
  });

  it("updateAgent throws when not accepted", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const updatePromise = client.updateAgent("agent-1", { name: "New Name" });

    const sent = findSentMessage(transport, "update_agent_request");
    simulateServerResponse(transport, {
      type: "update_agent_response",
      payload: { requestId: getRequestId(sent), agentId: "agent-1", accepted: false, error: "Agent not found" },
    });

    await expect(updatePromise).rejects.toThrow("Agent not found");
    await cleanup();
  });
});

describe("AgentRpc — interaction", () => {
  it("sendAgentMessage sends request and resolves on accepted response", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const msgPromise = client.sendAgentMessage("agent-1", "Hello world");

    const sent = findSentMessage(transport, "send_agent_message_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { text?: string } }).message?.text).toBe("Hello world");

    simulateServerResponse(transport, {
      type: "send_agent_message_response",
      payload: { requestId: getRequestId(sent), agentId: "agent-1", accepted: true, error: null },
    });

    await msgPromise;
    await cleanup();
  });

  it("sendAgentMessage throws when not accepted", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const msgPromise = client.sendAgentMessage("agent-1", "Hello");

    const sent = findSentMessage(transport, "send_agent_message_request");
    simulateServerResponse(transport, {
      type: "send_agent_message_response",
      payload: { requestId: getRequestId(sent), agentId: "agent-1", accepted: false, error: "Agent busy" },
    });

    await expect(msgPromise).rejects.toThrow("Agent busy");
    await cleanup();
  });

  it("cancelAgent sends cancel_agent_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const cancelPromise = client.cancelAgent("agent-1");

    const sent = findSentMessage(transport, "cancel_agent_request");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "cancel_agent_response",
      payload: { requestId: getRequestId(sent), agentId: "agent-1", agent: null },
    });

    await cancelPromise;
    await cleanup();
  });

  it("setAgentMode sends set_agent_mode_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const modePromise = client.setAgentMode("agent-1", "code");

    const sent = findSentMessage(transport, "set_agent_mode_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { modeId?: string } }).message?.modeId).toBe("code");

    simulateServerResponse(transport, {
      type: "set_agent_mode_response",
      payload: { requestId: getRequestId(sent), agentId: "agent-1", accepted: true, error: null },
    });

    await modePromise;
    await cleanup();
  });

  it("setAgentModel sends set_agent_model_request with nullable modelId", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const modelPromise = client.setAgentModel("agent-1", null);

    const sent = findSentMessage(transport, "set_agent_model_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { modelId?: string | null } }).message?.modelId).toBeNull();

    simulateServerResponse(transport, {
      type: "set_agent_model_response",
      payload: { requestId: getRequestId(sent), agentId: "agent-1", accepted: true, error: null },
    });

    await modelPromise;
    await cleanup();
  });
});

describe("AgentRpc — fetch", () => {
  it("fetchAgent resolves with agent + project", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const fetchPromise = client.fetchAgent("agent-1", "req-fetch-1");

    simulateServerResponse(transport, {
      type: "fetch_agent_response",
      payload: {
        requestId: "req-fetch-1",
        agent: mockAgentSnapshot({ id: "agent-1" }),
        project: null,
        error: null,
      },
    });

    const result = await fetchPromise;
    expect(result).not.toBeNull();
    expect(result!.agent.id).toBe("agent-1");
    await cleanup();
  });

  it("fetchAgent returns null when agent not found", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const fetchPromise = client.fetchAgent("agent-missing", "req-fetch-2");

    simulateServerResponse(transport, {
      type: "fetch_agent_response",
      payload: { requestId: "req-fetch-2", agent: null, error: null },
    });

    const result = await fetchPromise;
    expect(result).toBeNull();
    await cleanup();
  });

  it("fetchAgentTimeline resolves with timeline payload", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const timelinePromise = client.fetchAgentTimeline("agent-1", { requestId: "req-tl-1" });

    simulateServerResponse(transport, {
      type: "fetch_agent_timeline_response",
      payload: {
        requestId: "req-tl-1",
        agentId: "agent-1",
        agent: mockAgentSnapshot({ id: "agent-1" }),
        direction: "tail",
        projection: "projected",
        epoch: "epoch-1",
        reset: true,
        staleCursor: false,
        gap: false,
        window: { minSeq: 0, maxSeq: 0, nextSeq: 1 },
        startCursor: null,
        endCursor: null,
        hasOlder: false,
        hasNewer: false,
        entries: [],
        error: null,
      },
    });

    const result = await timelinePromise;
    expect(result.entries).toEqual([]);
    await cleanup();
  });
});

describe("AgentRpc — permissions", () => {
  it("respondToPermission sends fire-and-forget message", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    client.respondToPermission("agent-1", "perm-1", { behavior: "allow" });

    const sent = findSentMessage(transport, "agent_permission_response");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { agentId?: string } }).message?.agentId).toBe("agent-1");
    await cleanup();
  });

  it("respondToPermissionAndWait awaits agent_permission_resolved", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const waitPromise = client.respondToPermissionAndWait(
      "agent-1",
      "perm-1",
      { behavior: "allow" },
    );

    simulateServerResponse(transport, {
      type: "agent_permission_resolved",
      payload: {
        requestId: "perm-1",
        agentId: "agent-1",
        resolution: { behavior: "allow" },
      },
    });

    const result = await waitPromise;
    expect(result.resolution).toEqual({ behavior: "allow" });
    await cleanup();
  });
});

describe("AgentRpc — ping", () => {
  it("ping sends ping and resolves with RTT", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const pingPromise = client.ping({ requestId: "req-ping-1" });

    const sent = transport.sentMessages.find(
      (m) =>
        m.parsed.type === "session" &&
        (m.parsed as { message?: { type?: string } }).message?.type === "ping",
    );
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { requestId?: string } }).message?.requestId).toBe("req-ping-1");

    simulateServerResponse(transport, {
      type: "pong",
      payload: {
        requestId: "req-ping-1",
        clientSentAt: Date.now(),
        serverReceivedAt: Date.now(),
        serverSentAt: Date.now(),
      },
    });

    const result = await pingPromise;
    expect(result.requestId).toBe("req-ping-1");
    expect(typeof result.rttMs).toBe("number");
    await cleanup();
  });
});
