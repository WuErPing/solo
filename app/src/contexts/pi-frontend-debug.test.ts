import { describe, expect, it } from "vitest";
import { processAgentStreamEvents } from "@/contexts/session-stream-reducers";
import { AgentStreamEventPayloadSchema } from "@server/shared/messages";
import type { AgentStreamEventPayload } from "@server/shared/messages";

let nextSeq = 1;
function makePiEvent(event: AgentStreamEventPayload) {
  const seq = event.type === "timeline" ? nextSeq++ : undefined;
  return {
    event,
    seq,
    epoch: event.type === "timeline" ? "epoch-1" : undefined,
    timestamp: new Date(),
  };
}

function reasoningEvent(text: string): AgentStreamEventPayload {
  return {
    type: "timeline",
    provider: "pi",
    item: { type: "reasoning", text },
  };
}

function assistantEvent(text: string): AgentStreamEventPayload {
  return {
    type: "timeline",
    provider: "pi",
    item: { type: "assistant_message", text },
  };
}

function turnCompletedEvent(): AgentStreamEventPayload {
  return {
    type: "turn_completed",
    provider: "pi",
  };
}

describe("Pi frontend pipeline", () => {
  it("validates and processes Pi events correctly", () => {
    nextSeq = 1;
    const events: AgentStreamEventPayload[] = [
      reasoningEvent("The"),
      reasoningEvent(" user"),
      assistantEvent("Hello"),
      turnCompletedEvent(),
    ];

    // Validate all events
    for (const evt of events) {
      const parsed = AgentStreamEventPayloadSchema.safeParse(evt);
      expect(parsed.success).toBe(true);
    }

    const result = processAgentStreamEvents({
      events: events.map(makePiEvent),
      currentTail: [],
      currentHead: [],
      currentCursor: undefined,
      currentAgent: { status: "running", updatedAt: new Date(), lastActivityAt: new Date() },
    });

    expect(result.tail).toHaveLength(2);
    expect(result.tail[0].kind).toBe("thought");
    expect((result.tail[0] as any).status).toBe("ready");
    expect((result.tail[0] as any).text).toBe("The user");
    expect(result.tail[1].kind).toBe("assistant_message");
    expect((result.tail[1] as any).text).toBe("Hello");
    expect(result.head).toHaveLength(0);
    expect(result.agentChanged).toBe(true);
    expect(result.agent?.status).toBe("idle");
  });
});
