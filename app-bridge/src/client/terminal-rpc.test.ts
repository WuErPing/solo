import { describe, expect, it } from "vitest";
import {
  createConnectedClient,
  simulateServerResponse,
} from "./daemon-client-test-harness.js";

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

describe("TerminalRpc", () => {
  it("listTerminals sends list_terminals_request and resolves with terminals", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.listTerminals("/test/project", "req-list-terminals");

    const sent = findSentMessage(transport, "list_terminals_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");

    simulateServerResponse(transport, {
      type: "list_terminals_response" as const,
      payload: {
        terminals: [
          { id: "term-1", name: "shell" as const },
        ],
        requestId: "req-list-terminals",
      },
    });

    const result = await promise;
    expect(result.terminals).toHaveLength(1);
    expect(result.terminals[0]!.id).toBe("term-1");
    await cleanup();
  });

  it("createTerminal sends create_terminal_request and resolves with terminal", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.createTerminal(
      "/test/project",
      "my-term",
      "req-create-terminal",
    );

    const sent = findSentMessage(transport, "create_terminal_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");
    expect((sent!.parsed as { message?: { name?: string } }).message?.name).toBe("my-term");

    simulateServerResponse(transport, {
      type: "create_terminal_response" as const,
      payload: {
        terminal: {
          id: "term-1",
          name: "my-term",
          cwd: "/test/project",
        },
        error: null,
        requestId: "req-create-terminal",
      },
    });

    const result = await promise;
    expect(result.terminal!.id).toBe("term-1");
    expect(result.error).toBeNull();
    await cleanup();
  });

  it("killTerminal sends kill_terminal_request and resolves with success", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.killTerminal("term-1", "req-kill-terminal");

    const sent = findSentMessage(transport, "kill_terminal_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { terminalId?: string } }).message?.terminalId).toBe(
      "term-1",
    );

    simulateServerResponse(transport, {
      type: "kill_terminal_response" as const,
      payload: {
        terminalId: "term-1",
        success: true,
        requestId: "req-kill-terminal",
      },
    });

    const result = await promise;
    expect(result.success).toBe(true);
    await cleanup();
  });

  it("tmuxListAgents sends tmux/list_agents and resolves with agents", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.tmuxListAgents("req-tmux-list-agents");

    const sent = findSentMessage(transport, "tmux/list_agents");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "tmux/list_agents/response" as const,
      payload: {
        requestId: "req-tmux-list-agents",
        agents: [],
        otherPanes: [],
        commandHistory: [],
        error: null,
      },
    });

    const result = await promise;
    expect(result.agents).toHaveLength(0);
    expect(result.error).toBeNull();
    await cleanup();
  });

  it("closeItems sends close_items_request and resolves with results", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.closeItems(
      { agentIds: ["agent-1"], terminalIds: ["term-1"] },
      "req-close-items",
    );

    const sent = findSentMessage(transport, "close_items_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { agentIds?: string[] } }).message?.agentIds).toEqual([
      "agent-1",
    ]);
    expect((sent!.parsed as { message?: { terminalIds?: string[] } }).message?.terminalIds).toEqual(
      ["term-1"],
    );

    simulateServerResponse(transport, {
      type: "close_items_response" as const,
      payload: {
        agents: [
          { agentId: "agent-1", archivedAt: "2026-01-01T00:00:00Z" },
        ],
        terminals: [
          { terminalId: "term-1", success: true },
        ],
        requestId: "req-close-items",
      },
    });

    const result = await promise;
    expect(result.agents).toHaveLength(1);
    expect(result.agents[0]!.agentId).toBe("agent-1");
    expect(result.terminals).toHaveLength(1);
    expect(result.terminals[0]!.success).toBe(true);
    await cleanup();
  });
});
