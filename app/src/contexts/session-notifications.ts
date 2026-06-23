import type { StreamItem } from "@/types/stream";
import type { SessionState } from "@/stores/session-store";
import type { AgentAttentionNotificationPayload, NotificationPermissionRequest } from "@server/shared/agent-attention-notification";
import { buildAgentAttentionNotificationPayload } from "@server/shared/agent-attention-notification";

export const findLatestAssistantMessageText = (items: StreamItem[]): string | null => {
  for (let i = items.length - 1; i >= 0; i -= 1) {
    const item = items[i];
    if (item.kind === "assistant_message") {
      return item.text;
    }
  }
  return null;
};

export const getLatestPermissionRequest = (
  session: SessionState | undefined,
  agentId: string,
): NotificationPermissionRequest | null => {
  if (!session) {
    return null;
  }

  let latest: NotificationPermissionRequest | null = null;
  for (const pending of session.pendingPermissions.values()) {
    if (pending.agentId === agentId) {
      latest = pending.request;
    }
  }
  if (latest) {
    return latest;
  }

  const agentPending = session.agents.get(agentId)?.pendingPermissions;
  if (agentPending && agentPending.length > 0) {
    return agentPending[agentPending.length - 1] as NotificationPermissionRequest;
  }

  return null;
};

export interface NotifyAttentionDeps {
  serverId: string;
  appStateRef: { current: string };
  attentionNotifiedRef: { current: Map<string, number> };
  getSessionState: () => SessionState | undefined;
  isAppActivelyVisible: (appState: string) => boolean;
  sendNotification: (params: {
    title: string;
    body?: string;
    data?: Record<string, unknown>;
  }) => void;
}

export function createNotifyAgentAttention(deps: NotifyAttentionDeps) {
  return (params: {
    agentId: string;
    reason: "finished" | "error" | "permission";
    timestamp: string;
    notification?: AgentAttentionNotificationPayload;
  }) => {
    const appState = deps.appStateRef.current;
    const session = deps.getSessionState();
    const attentionFocusedAgentId = session?.focusedAgentId ?? null;
    if (params.reason === "error") {
      return;
    }
    const isActivelyVisible = deps.isAppActivelyVisible(appState);
    const isAwayFromAgent = !isActivelyVisible || attentionFocusedAgentId !== params.agentId;
    if (!isAwayFromAgent) {
      return;
    }

    const timestampMs = new Date(params.timestamp).getTime();
    const lastNotified = deps.attentionNotifiedRef.current.get(params.agentId);
    if (lastNotified && lastNotified >= timestampMs) {
      return;
    }
    deps.attentionNotifiedRef.current.set(params.agentId, timestampMs);

    const head = session?.agentStreamHead.get(params.agentId) ?? [];
    const tail = session?.agentStreamTail.get(params.agentId) ?? [];
    const assistantMessage =
      findLatestAssistantMessageText(head) ?? findLatestAssistantMessageText(tail);
    const permissionRequest = getLatestPermissionRequest(session, params.agentId);

    const notification =
      params.notification ??
      buildAgentAttentionNotificationPayload({
        reason: params.reason,
        serverId: deps.serverId,
        agentId: params.agentId,
        assistantMessage: params.reason === "finished" ? assistantMessage : null,
        permissionRequest: params.reason === "permission" ? permissionRequest : null,
      });

    deps.sendNotification({
      title: notification.title,
      body: notification.body,
      data: notification.data,
    });
  };
}
