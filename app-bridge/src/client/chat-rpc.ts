import type {
  CreateChatRoomOptions,
  DaemonClient,
  DeleteChatRoomOptions,
  InspectChatRoomOptions,
  PostChatMessageOptions,
  ReadChatMessagesOptions,
  WaitForChatMessagesOptions,
} from "./daemon-client.js";
import type { SessionOutboundMessage } from "../shared/messages.js";

type ChatCreatePayload = Extract<SessionOutboundMessage, { type: "chat/create/response" }>["payload"];
type ChatListPayload = Extract<SessionOutboundMessage, { type: "chat/list/response" }>["payload"];
type ChatInspectPayload = Extract<SessionOutboundMessage, { type: "chat/inspect/response" }>["payload"];
type ChatDeletePayload = Extract<SessionOutboundMessage, { type: "chat/delete/response" }>["payload"];
type ChatPostPayload = Extract<SessionOutboundMessage, { type: "chat/post/response" }>["payload"];
type ChatReadPayload = Extract<SessionOutboundMessage, { type: "chat/read/response" }>["payload"];
type ChatWaitPayload = Extract<SessionOutboundMessage, { type: "chat/wait/response" }>["payload"];

export class ChatRpc {
  constructor(private readonly client: DaemonClient) {}

  async createChatRoom(options: CreateChatRoomOptions): Promise<ChatCreatePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "chat/create",
        name: options.name,
        ...(options.purpose ? { purpose: options.purpose } : {}),
      },
      responseType: "chat/create/response",
      timeout: 10000,
    });
  }

  async listChatRooms(requestId?: string): Promise<ChatListPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "chat/list",
      },
      responseType: "chat/list/response",
      timeout: 10000,
    });
  }

  async inspectChatRoom(options: InspectChatRoomOptions): Promise<ChatInspectPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "chat/inspect",
        room: options.room,
      },
      responseType: "chat/inspect/response",
      timeout: 10000,
    });
  }

  async deleteChatRoom(options: DeleteChatRoomOptions): Promise<ChatDeletePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "chat/delete",
        room: options.room,
      },
      responseType: "chat/delete/response",
      timeout: 10000,
    });
  }

  async postChatMessage(options: PostChatMessageOptions): Promise<ChatPostPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "chat/post",
        room: options.room,
        body: options.body,
        ...(options.authorAgentId ? { authorAgentId: options.authorAgentId } : {}),
        ...(options.replyToMessageId ? { replyToMessageId: options.replyToMessageId } : {}),
      },
      responseType: "chat/post/response",
      timeout: 10000,
    });
  }

  async readChatMessages(options: ReadChatMessagesOptions): Promise<ChatReadPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "chat/read",
        room: options.room,
        ...(typeof options.limit === "number" ? { limit: options.limit } : {}),
        ...(options.since ? { since: options.since } : {}),
        ...(options.authorAgentId ? { authorAgentId: options.authorAgentId } : {}),
      },
      responseType: "chat/read/response",
      timeout: 10000,
    });
  }

  async waitForChatMessages(options: WaitForChatMessagesOptions): Promise<ChatWaitPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "chat/wait",
        room: options.room,
        ...(options.afterMessageId ? { afterMessageId: options.afterMessageId } : {}),
        ...(typeof options.timeoutMs === "number" ? { timeoutMs: options.timeoutMs } : {}),
      },
      responseType: "chat/wait/response",
      timeout: (options.timeoutMs ?? 0) + 10000,
    });
  }
}
