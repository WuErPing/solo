import { describe, expect, it } from "vitest";
import { WSOutboundMessageSchema } from "@server/shared/messages";

describe("Pi Zod shape", () => {
  it("preserves turn_completed shape after parsing", () => {
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

    const parsed = WSOutboundMessageSchema.parse(message);
    if (parsed.type !== "session") throw new Error("expected session type");
    const { message: msg } = parsed;
    if (msg.type !== "agent_stream") throw new Error("expected agent_stream");
    const event = msg.payload.event;
    if (event.type !== "turn_completed") throw new Error("expected turn_completed");
    expect(event.provider).toBe("pi");
    expect(event.usage).toBeDefined();
    expect(event.usage?.inputTokens).toBe(1);
  });

  it("preserves reasoning timeline shape after parsing", () => {
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

    const parsed = WSOutboundMessageSchema.parse(message);
    if (parsed.type !== "session") throw new Error("expected session type");
    const { message: msg } = parsed;
    if (msg.type !== "agent_stream") throw new Error("expected agent_stream");
    const event = msg.payload.event;
    if (event.type !== "timeline") throw new Error("expected timeline");
    const { item } = event;
    if (item.type !== "reasoning") throw new Error("expected reasoning");
    expect(item.text).toBe("Let me think...");
  });
});
