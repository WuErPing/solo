import type { DaemonClient } from "@server/client/daemon-client";
import type { AgentAttachment } from "@server/shared/messages";
import type { AgentPersistenceHandle } from "@server/server/agent/agent-sdk-types";
import type { StreamItem } from "@/types/stream";
import type { AttachmentMetadata } from "@/attachments/types";
import { generateMessageId } from "@/types/stream";
import { encodeImages } from "@/utils/encode-images";

export interface SendAgentMessageDeps {
  serverId: string;
  client: DaemonClient | null;
  setAgentStreamHead: (
    serverId: string,
    updater: (prev: Map<string, StreamItem[]>) => Map<string, StreamItem[]>,
  ) => void;
  setAgentStreamTail: (
    serverId: string,
    updater: (prev: Map<string, StreamItem[]>) => Map<string, StreamItem[]>,
  ) => void;
  getCurrentHead: (agentId: string) => StreamItem[] | undefined;
  getAgent: (
    agentId: string,
  ) => { status: string; persistence: AgentPersistenceHandle | null } | undefined;
  /** Called when a send fails so the caller can surface it to the user. */
  onError?: (error: unknown) => void;
}

/**
 * Rolls back an optimistically-appended message from the stream maps so a
 * failed send does not leave a phantom "sent" message in the UI.
 */
function removeOptimisticMessage(
  deps: SendAgentMessageDeps,
  agentId: string,
  messageId: string,
): void {
  const filterItem = (items: StreamItem[] | undefined): StreamItem[] | undefined =>
    items?.filter((item) => item.id !== messageId);

  deps.setAgentStreamHead(deps.serverId, (prev) => {
    const head = prev.get(agentId);
    if (!head) return prev;
    const updated = new Map(prev);
    updated.set(agentId, filterItem(head) ?? []);
    return updated;
  });
  deps.setAgentStreamTail(deps.serverId, (prev) => {
    const tail = prev.get(agentId);
    if (!tail) return prev;
    const updated = new Map(prev);
    updated.set(agentId, filterItem(tail) ?? []);
    return updated;
  });
}

export function createSendAgentMessage(deps: SendAgentMessageDeps) {
  return async (
    agentId: string,
    message: string,
    images?: AttachmentMetadata[],
    attachments?: AgentAttachment[],
  ): Promise<void> => {
    const messageId = generateMessageId();
    const userMessage: StreamItem = {
      kind: "user_message",
      id: messageId,
      text: message,
      timestamp: new Date(),
    };

    const currentHead = deps.getCurrentHead(agentId);
    if (currentHead && currentHead.length > 0) {
      deps.setAgentStreamHead(deps.serverId, (prev) => {
        const head = prev.get(agentId) || [];
        const updated = new Map(prev);
        updated.set(agentId, [...head, userMessage]);
        return updated;
      });
    } else {
      deps.setAgentStreamTail(deps.serverId, (prev) => {
        const currentStream = prev.get(agentId) || [];
        const updated = new Map(prev);
        updated.set(agentId, [...currentStream, userMessage]);
        return updated;
      });
    }

    if (!deps.client) {
      console.warn("[Session] sendAgentMessage skipped: daemon unavailable");
      removeOptimisticMessage(deps, agentId, messageId);
      deps.onError?.(new Error("Daemon unavailable — message not sent"));
      return;
    }

    const agent = deps.getAgent(agentId);
    if (agent?.status === "closed" && agent.persistence) {
      await deps.client.resumeAgent(agent.persistence);
    }

    const imagesData = await encodeImages(images);
    void deps.client
      .sendAgentMessage(agentId, message, {
        messageId,
        ...(imagesData && imagesData.length > 0 ? { images: imagesData } : {}),
        ...(attachments && attachments.length > 0 ? { attachments } : {}),
      })
      .catch((error) => {
        console.error("[Session] Failed to send agent message:", error);
        removeOptimisticMessage(deps, agentId, messageId);
        deps.onError?.(error);
      });
  };
}
