import { describe, expect, it } from "vitest";
import { WSOutboundMessageSchema } from "@server/shared/messages";

describe("Pi Zod validation", () => {
  it("validates turn_completed with usage", () => {
    const message = {
      type: "session",
      message: {
        type: "agent_stream",
        payload: {
          agentId: "agent-1",
          event: {
            type: "turn_completed",
            provider: "pi",
            usage: {
              inputTokens: 1,
              outputTokens: 68,
              totalCostUsd: 0.00137,
            },
          },
          timestamp: "2026-05-24T13:00:00Z",
        },
      },
    };

    const parsed = WSOutboundMessageSchema.safeParse(message);
    expect(parsed.success).toBe(true);
  });

  it("validates turn_completed without usage", () => {
    const message = {
      type: "session",
      message: {
        type: "agent_stream",
        payload: {
          agentId: "agent-1",
          event: {
            type: "turn_completed",
            provider: "pi",
          },
          timestamp: "2026-05-24T13:00:00Z",
        },
      },
    };

    const parsed = WSOutboundMessageSchema.safeParse(message);
    expect(parsed.success).toBe(true);
  });

  it("validates reasoning timeline", () => {
    const message = {
      type: "session",
      message: {
        type: "agent_stream",
        payload: {
          agentId: "agent-1",
          event: {
            type: "timeline",
            provider: "pi",
            item: {
              type: "reasoning",
              text: "Let me think...",
            },
          },
          timestamp: "2026-05-24T13:00:00Z",
          seq: 1,
          epoch: "epoch-1",
        },
      },
    };

    const parsed = WSOutboundMessageSchema.safeParse(message);
    expect(parsed.success).toBe(true);
  });

  it("validates assistant_message timeline", () => {
    const message = {
      type: "session",
      message: {
        type: "agent_stream",
        payload: {
          agentId: "agent-1",
          event: {
            type: "timeline",
            provider: "pi",
            item: {
              type: "assistant_message",
              text: "Hello",
            },
          },
          timestamp: "2026-05-24T13:00:00Z",
          seq: 2,
          epoch: "epoch-1",
        },
      },
    };

    const parsed = WSOutboundMessageSchema.safeParse(message);
    expect(parsed.success).toBe(true);
  });

  it("validates tool_call timeline", () => {
    const message = {
      type: "session",
      message: {
        type: "agent_stream",
        payload: {
          agentId: "agent-1",
          event: {
            type: "timeline",
            provider: "pi",
            item: {
              type: "tool_call",
              callId: "call-1",
              name: "bash",
              detail: {
                type: "unknown",
                input: { command: "ls" },
                output: null,
              },
              status: "running",
              error: null,
            },
          },
          timestamp: "2026-05-24T13:00:00Z",
          seq: 3,
          epoch: "epoch-1",
        },
      },
    };

    const parsed = WSOutboundMessageSchema.safeParse(message);
    expect(parsed.success).toBe(true);
  });
});
