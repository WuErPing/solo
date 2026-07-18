import { useCallback } from "react";
import { useHostRuntimeClient } from "@/runtime/host-runtime";
import { detectTimezone } from "@/utils/cron-timezone";
import {
  buildTranscript,
  nextAssistantMessageId,
  useScheduleAssistantStore,
} from "@/stores/schedule-assistant-store";

export interface UseScheduleAssistOptions {
  serverId: string;
  contextScheduleId?: string;
}

export interface UseScheduleAssistResult {
  send: (text: string) => Promise<void>;
  isSending: boolean;
}

function classifyTransportError(error: unknown): { code: string; text: string } {
  const message = error instanceof Error ? error.message : String(error);
  if (message.toLowerCase().includes("timeout")) {
    return {
      code: "timeout",
      text: "The assistant took too long to respond. Try again.",
    };
  }
  return { code: "unknown", text: message || "Assistant request failed" };
}

export function useScheduleAssist({
  serverId,
  contextScheduleId,
}: UseScheduleAssistOptions): UseScheduleAssistResult {
  const client = useHostRuntimeClient(serverId);
  const addMessage = useScheduleAssistantStore((state) => state.addMessage);
  const setSending = useScheduleAssistantStore((state) => state.setSending);
  const setProviderInfo = useScheduleAssistantStore((state) => state.setProviderInfo);
  const isSending = useScheduleAssistantStore(
    (state) => state.threads[serverId]?.isSending ?? false,
  );

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed) {
        return;
      }

      // Transcript covers prior turns only — the current message goes in `message`.
      const transcript = buildTranscript(
        useScheduleAssistantStore.getState().threads[serverId]?.messages ?? [],
      );

      addMessage(serverId, {
        id: nextAssistantMessageId(),
        role: "user",
        kind: "text",
        text: trimmed,
        createdAt: Date.now(),
      });

      if (!client) {
        addMessage(serverId, {
          id: nextAssistantMessageId(),
          role: "assistant",
          kind: "error",
          error: "not_connected",
          text: "Not connected to this host.",
          createdAt: Date.now(),
        });
        return;
      }

      setSending(serverId, true);
      try {
        const payload = await client.scheduleAssist({
          message: trimmed,
          timezone: detectTimezone(),
          clientNow: new Date().toISOString(),
          ...(contextScheduleId ? { contextScheduleId } : {}),
          ...(transcript.length > 0 ? { transcript } : {}),
        });

        setProviderInfo(serverId, {
          llmProvider: payload.llmProvider,
          model: payload.model,
        });

        if (payload.kind === "proposal" && payload.proposal) {
          addMessage(serverId, {
            id: nextAssistantMessageId(),
            role: "assistant",
            kind: "proposal",
            proposal: payload.proposal,
            text: payload.proposal.summary,
            createdAt: Date.now(),
          });
        } else if (payload.kind === "error") {
          addMessage(serverId, {
            id: nextAssistantMessageId(),
            role: "assistant",
            kind: "error",
            error: payload.error ?? "unknown",
            text: payload.message ?? "Something went wrong.",
            createdAt: Date.now(),
          });
        } else {
          addMessage(serverId, {
            id: nextAssistantMessageId(),
            role: "assistant",
            kind: "text",
            text: payload.message ?? "",
            createdAt: Date.now(),
          });
        }
      } catch (error) {
        const { code, text: errorText } = classifyTransportError(error);
        addMessage(serverId, {
          id: nextAssistantMessageId(),
          role: "assistant",
          kind: "error",
          error: code,
          text: errorText,
          createdAt: Date.now(),
        });
      } finally {
        setSending(serverId, false);
      }
    },
    [addMessage, client, contextScheduleId, serverId, setProviderInfo, setSending],
  );

  return { send, isSending };
}
