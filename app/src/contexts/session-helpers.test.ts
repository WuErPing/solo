import { describe, expect, it } from "vitest";
import {
  hasAgentUsageChanged,
  getAgentIdFromUpdate,
  type AgentUpdatePayload,
} from "./session-helpers";

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
