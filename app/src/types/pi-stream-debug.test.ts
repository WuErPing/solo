import { describe, expect, it } from "vitest";
import { applyStreamEvent } from "@/types/stream";
import type { AgentStreamEventPayload } from "@server/shared/messages";

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

describe("Pi stream events", () => {
  it("finalizes thought when assistant message starts", () => {
    let tail: ReturnType<typeof applyStreamEvent>["tail"] = [];
    let head: ReturnType<typeof applyStreamEvent>["head"] = [];

    // Pi thinking deltas
    const thinkingWords = ["The", " user", " said", " hi"];
    for (const word of thinkingWords) {
      const r = applyStreamEvent({ tail, head, event: reasoningEvent(word), timestamp: new Date() });
      tail = r.tail;
      head = r.head;
    }

    // Pi text delta starts
    const r2 = applyStreamEvent({ tail, head, event: assistantEvent("Hello"), timestamp: new Date() });
    tail = r2.tail;
    head = r2.head;

    // Turn completed
    const r3 = applyStreamEvent({ tail, head, event: turnCompletedEvent(), timestamp: new Date() });
    tail = r3.tail;
    head = r3.head;

    // Check tail
    expect(tail).toHaveLength(2);
    expect(tail[0].kind).toBe("thought");
    expect((tail[0] as any).status).toBe("ready");
    expect((tail[0] as any).text).toBe("The user said hi");
    expect(tail[1].kind).toBe("assistant_message");
    expect((tail[1] as any).text).toBe("Hello");
    expect(head).toHaveLength(0);
  });
});
