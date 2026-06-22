import { describe, expect, it, vi, afterEach } from "vitest";
import { createConnectedClient, simulateServerResponse } from "./daemon-client-test-harness.js";

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

const mockRoom = {
  id: "room-1",
  name: "Test Room",
  purpose: null,
  createdAt: "2026-06-22T00:00:00Z",
  updatedAt: "2026-06-22T00:00:00Z",
  messageCount: 0,
  lastMessageAt: null,
};

const mockMessage = {
  id: "msg-1",
  roomId: "room-1",
  authorAgentId: "agent-1",
  body: "Hello world",
  replyToMessageId: null,
  mentionAgentIds: [],
  createdAt: "2026-06-22T00:00:00Z",
};

describe("ChatRpc", () => {
  it("createChatRoom sends chat/create and resolves with room", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.createChatRoom({ name: "Test Room", requestId: "req-chat-create" });

    const sent = findSentMessage(transport, "chat/create");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { name?: string } }).message?.name).toBe("Test Room");

    simulateServerResponse(transport, {
      type: "chat/create/response",
      payload: { requestId: "req-chat-create", room: mockRoom, error: null },
    });

    const result = await promise;
    expect(result.room?.id).toBe("room-1");
    await cleanup();
  });

  it("listChatRooms sends chat/list and resolves with array", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.listChatRooms("req-chat-list");

    simulateServerResponse(transport, {
      type: "chat/list/response",
      payload: { requestId: "req-chat-list", rooms: [mockRoom], error: null },
    });

    const result = await promise;
    expect(result.rooms).toHaveLength(1);
    await cleanup();
  });

  it("deleteChatRoom sends chat/delete", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.deleteChatRoom({ room: "room-1", requestId: "req-chat-del" });

    const sent = findSentMessage(transport, "chat/delete");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "chat/delete/response",
      payload: { requestId: "req-chat-del", room: mockRoom, error: null },
    });

    await promise;
    await cleanup();
  });

  it("postChatMessage sends chat/post and resolves with message", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.postChatMessage({
      room: "room-1",
      body: "Hello world",
      requestId: "req-chat-post",
    });

    const sent = findSentMessage(transport, "chat/post");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { body?: string } }).message?.body).toBe("Hello world");

    simulateServerResponse(transport, {
      type: "chat/post/response",
      payload: { requestId: "req-chat-post", message: mockMessage, error: null },
    });

    const result = await promise;
    expect(result.message?.id).toBe("msg-1");
    await cleanup();
  });

  it("readChatMessages sends chat/read and resolves with messages", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.readChatMessages({ room: "room-1", requestId: "req-chat-read" });

    simulateServerResponse(transport, {
      type: "chat/read/response",
      payload: { requestId: "req-chat-read", messages: [mockMessage], error: null },
    });

    const result = await promise;
    expect(result.messages).toHaveLength(1);
    await cleanup();
  });
});
