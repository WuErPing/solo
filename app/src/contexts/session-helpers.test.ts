import { describe, expect, it } from "vitest";
import {
  hasAgentUsageChanged,
  getAgentIdFromUpdate,
  applyToolResultToMessages,
  applyToolErrorToMessages,
  type AgentUpdatePayload,
} from "./session-helpers";
import type { MessageEntry } from "@/stores/session-store";

describe("hasAgentUsageChanged", () => {
  it("returns false when both usages are undefined", () => {
    expect(hasAgentUsageChanged(undefined, undefined)).toBe(false);
  });

  it("returns false when usage fields are identical", () => {
    const usage = {
      inputTokens: 100,
      outputTokens: 200,
      cachedInputTokens: 50,
      totalCostUsd: 0.01,
      contextWindowMaxTokens: 10000,
      contextWindowUsedTokens: 5000,
    };
    expect(hasAgentUsageChanged(usage, usage)).toBe(false);
  });

  it("returns true when inputTokens differs", () => {
    expect(
      hasAgentUsageChanged(
        { inputTokens: 101, outputTokens: 200, cachedInputTokens: 50, totalCostUsd: 0.01 },
        { inputTokens: 100, outputTokens: 200, cachedInputTokens: 50, totalCostUsd: 0.01 },
      ),
    ).toBe(true);
  });

  it("returns true when outputTokens differs", () => {
    expect(
      hasAgentUsageChanged(
        { inputTokens: 100, outputTokens: 201, cachedInputTokens: 50, totalCostUsd: 0.01 },
        { inputTokens: 100, outputTokens: 200, cachedInputTokens: 50, totalCostUsd: 0.01 },
      ),
    ).toBe(true);
  });

  it("returns true when totalCostUsd differs", () => {
    expect(
      hasAgentUsageChanged(
        { inputTokens: 100, outputTokens: 200, cachedInputTokens: 50, totalCostUsd: 0.02 },
        { inputTokens: 100, outputTokens: 200, cachedInputTokens: 50, totalCostUsd: 0.01 },
      ),
    ).toBe(true);
  });

  it("returns true when contextWindowUsedTokens differs", () => {
    expect(
      hasAgentUsageChanged(
        {
          inputTokens: 100,
          outputTokens: 200,
          cachedInputTokens: 50,
          totalCostUsd: 0.01,
          contextWindowMaxTokens: 10000,
          contextWindowUsedTokens: 5001,
        },
        {
          inputTokens: 100,
          outputTokens: 200,
          cachedInputTokens: 50,
          totalCostUsd: 0.01,
          contextWindowMaxTokens: 10000,
          contextWindowUsedTokens: 5000,
        },
      ),
    ).toBe(true);
  });

  it("returns true when incoming has usage but current does not", () => {
    expect(hasAgentUsageChanged({ inputTokens: 100 }, undefined)).toBe(true);
  });
});

describe("getAgentIdFromUpdate", () => {
  it("extracts agentId from remove variant", () => {
    const update: AgentUpdatePayload = {
      kind: "remove",
      agentId: "agent-123",
    };
    expect(getAgentIdFromUpdate(update)).toBe("agent-123");
  });

  it("extracts agent.id from upsert variant", () => {
    const update = {
      kind: "upsert" as const,
      agent: {
        id: "agent-456",
        provider: "claude",
        status: "idle",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      },
    } as unknown as AgentUpdatePayload;
    expect(getAgentIdFromUpdate(update)).toBe("agent-456");
  });
});

describe("applyToolResultToMessages", () => {
  it("marks matching tool call as completed with result", () => {
    const messages: MessageEntry[] = [
      { type: "tool_call", id: "tc-1", timestamp: 0, toolName: "bash", args: null, status: "executing" },
      { type: "tool_call", id: "tc-2", timestamp: 0, toolName: "read", args: null, status: "executing" },
    ];

    const updater = applyToolResultToMessages("tc-1", { output: "done" });
    const result = updater(messages);

    expect(result[0]).toEqual({
      type: "tool_call",
      id: "tc-1",
      timestamp: 0,
      toolName: "bash",
      args: null,
      result: { output: "done" },
      status: "completed",
    });
    expect(result[1].type).toBe("tool_call");
    if (result[1].type === "tool_call") {
      expect(result[1].status).toBe("executing");
    }
  });

  it("does not modify non-tool_call messages", () => {
    const messages: MessageEntry[] = [
      { type: "user", id: "u-1", timestamp: 0, message: "hello" },
      { type: "assistant", id: "a-1", timestamp: 0, message: "hi" },
    ];

    const updater = applyToolResultToMessages("u-1", "result");
    const result = updater(messages);
    expect(result).toStrictEqual(messages);
  });

  it("returns same array if no matching tool call", () => {
    const messages: MessageEntry[] = [
      { type: "tool_call", id: "tc-1", timestamp: 0, toolName: "bash", args: null, status: "executing" },
    ];

    const updater = applyToolResultToMessages("tc-nonexistent", "result");
    const result = updater(messages);
    expect(result).toEqual(messages);
  });
});

describe("applyToolErrorToMessages", () => {
  it("marks matching tool call as failed with error", () => {
    const messages: MessageEntry[] = [
      { type: "tool_call", id: "tc-1", timestamp: 0, toolName: "bash", args: null, status: "executing" },
    ];

    const updater = applyToolErrorToMessages("tc-1", "command failed");
    const result = updater(messages);

    expect(result[0]).toEqual({
      type: "tool_call",
      id: "tc-1",
      timestamp: 0,
      toolName: "bash",
      args: null,
      error: "command failed",
      status: "failed",
    });
  });

  it("does not modify completed tool calls", () => {
    const messages: MessageEntry[] = [
      { type: "tool_call", id: "tc-1", timestamp: 0, toolName: "bash", args: null, status: "completed", result: "ok" },
    ];

    const updater = applyToolErrorToMessages("tc-different", "error");
    const result = updater(messages);
    expect(result).toEqual(messages);
  });
});
