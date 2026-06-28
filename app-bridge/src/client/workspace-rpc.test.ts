import { describe, expect, it, vi, afterEach } from "vitest";
import {
  createConnectedClient,
  simulateServerResponse,
  mockWorkspaceDescriptor,
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

describe("WorkspaceRpc", () => {
  it("openProject sends open_project_request and resolves with workspace", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.openProject("/test/project", "req-open-project");

    const sent = findSentMessage(transport, "open_project_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");

    simulateServerResponse(transport, {
      type: "open_project_response",
      payload: {
        requestId: "req-open-project",
        workspace: mockWorkspaceDescriptor(),
        error: null,
      },
    });

    const result = await promise;
    expect(result.workspace?.id).toBe("ws-1");
    await cleanup();
  });

  it("fetchWorkspaces sends fetch_workspaces_request and resolves with entries", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.fetchWorkspaces({ requestId: "req-fetch-workspaces" });

    const sent = findSentMessage(transport, "fetch_workspaces_request");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "fetch_workspaces_response",
      payload: {
        requestId: "req-fetch-workspaces",
        entries: [mockWorkspaceDescriptor()],
        pageInfo: { nextCursor: null, prevCursor: null, hasMore: false },
      },
    });

    const result = await promise;
    expect(result.entries).toHaveLength(1);
    await cleanup();
  });

  it("listProviderModels sends list_provider_models_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.listProviderModels("claude-code", { requestId: "req-list-models" });

    const sent = findSentMessage(transport, "list_provider_models_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { provider?: string } }).message?.provider).toBe(
      "claude-code",
    );

    simulateServerResponse(transport, {
      type: "list_provider_models_response",
      payload: {
        provider: "claude-code",
        models: [],
        error: null,
        fetchedAt: "2026-06-22T00:00:00Z",
        requestId: "req-list-models",
      },
    });

    const result = await promise;
    expect(result.provider).toBe("claude-code");
    await cleanup();
  });

  it("listCommands sends list_commands_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.listCommands("agent-1", "req-list-commands");

    const sent = findSentMessage(transport, "list_commands_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { agentId?: string } }).message?.agentId).toBe("agent-1");

    simulateServerResponse(transport, {
      type: "list_commands_response",
      payload: {
        agentId: "agent-1",
        commands: [{ name: "/test", description: "Test command", argumentHint: "" }],
        error: null,
        requestId: "req-list-commands",
      },
    });

    const result = await promise;
    expect(result.commands).toHaveLength(1);
    await cleanup();
  });

  it("getDaemonConfig sends get_daemon_config_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.getDaemonConfig("req-daemon-config");

    const sent = findSentMessage(transport, "get_daemon_config_request");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "get_daemon_config_response",
      payload: {
        requestId: "req-daemon-config",
        config: {
          mcp: { injectIntoAgents: false },
          providers: {},
          llmProviders: [],
          tmuxAgentNames: [],
        },
      },
    });

    const result = await promise;
    expect(result.config.mcp.injectIntoAgents).toBe(false);
    await cleanup();
  });

  it("exploreFileSystem sends file_explorer_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.exploreFileSystem("/test", "/test", "list", "req-explore");

    const sent = findSentMessage(transport, "file_explorer_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test");

    simulateServerResponse(transport, {
      type: "file_explorer_response",
      payload: {
        cwd: "/test",
        path: "/test",
        mode: "list" as const,
        directory: { path: "/test", entries: [] },
        file: null,
        error: null,
        requestId: "req-explore",
      },
    });

    const result = await promise;
    expect(result.directory?.entries).toHaveLength(0);
    await cleanup();
  });

  it("readProjectConfig sends read_project_config_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.readProjectConfig("/test", "req-read-config");

    const sent = findSentMessage(transport, "read_project_config_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { repoRoot?: string } }).message?.repoRoot).toBe("/test");

    simulateServerResponse(transport, {
      type: "read_project_config_response",
      payload: {
        requestId: "req-read-config",
        repoRoot: "/test",
        ok: true as const,
        config: null,
        revision: null,
      },
    });

    const result = await promise;
    expect(result.ok).toBe(true);
    await cleanup();
  });
});
