import { describe, expect, it } from "vitest";
import { shouldFinalizePendingCreate } from "@/utils/pending-create-finalize";
import type { StreamItem } from "@/types/stream";

function userMessage(id: string, text = "hi"): StreamItem {
  return { kind: "user_message", id, text, timestamp: new Date("2025-01-01T00:00:00Z") };
}

describe("shouldFinalizePendingCreate", () => {
  it("finalizes when stream has user_message with matching clientMessageId", () => {
    expect(
      shouldFinalizePendingCreate({
        streamItems: [userMessage("msg_123")],
        clientMessageId: "msg_123",
        canFinalize: true,
      }),
    ).toBe(true);
  });

  it("does not finalize when user_message id is empty (daemon didn't propagate clientMessageId)", () => {
    expect(
      shouldFinalizePendingCreate({
        streamItems: [userMessage("")],
        clientMessageId: "msg_123",
        canFinalize: true,
      }),
    ).toBe(false);
  });

  it("does not finalize when user_message has a different id", () => {
    expect(
      shouldFinalizePendingCreate({
        streamItems: [userMessage("msg_456")],
        clientMessageId: "msg_123",
        canFinalize: true,
      }),
    ).toBe(false);
  });

  it("does not finalize when canFinalize is false", () => {
    expect(
      shouldFinalizePendingCreate({
        streamItems: [userMessage("msg_123")],
        clientMessageId: "msg_123",
        canFinalize: false,
      }),
    ).toBe(false);
  });

  it("does not finalize when stream is empty", () => {
    expect(
      shouldFinalizePendingCreate({
        streamItems: [],
        clientMessageId: "msg_123",
        canFinalize: true,
      }),
    ).toBe(false);
  });

  it("does not finalize when stream only has assistant messages", () => {
    const assistant: StreamItem = {
      kind: "assistant_message",
      id: "a1",
      text: "hello",
      timestamp: new Date("2025-01-01T00:00:00Z"),
    };
    expect(
      shouldFinalizePendingCreate({
        streamItems: [assistant],
        clientMessageId: "msg_123",
        canFinalize: true,
      }),
    ).toBe(false);
  });
});
