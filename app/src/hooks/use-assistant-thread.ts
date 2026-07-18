import { useCallback } from "react";
import { useScheduleAssistantStore, type AssistantMessage } from "@/stores/schedule-assistant-store";

const EMPTY_MESSAGES: AssistantMessage[] = [];

export interface UseAssistantThreadResult {
  messages: AssistantMessage[];
  isSending: boolean;
  llmProvider: string | null;
  model: string | null;
  clearThread: () => void;
}

/** Read-side view of a per-host assistant thread. */
export function useAssistantThread(serverId: string): UseAssistantThreadResult {
  const messages = useScheduleAssistantStore(
    (state) => state.threads[serverId]?.messages ?? EMPTY_MESSAGES,
  );
  const isSending = useScheduleAssistantStore(
    (state) => state.threads[serverId]?.isSending ?? false,
  );
  const llmProvider = useScheduleAssistantStore(
    (state) => state.threads[serverId]?.llmProvider ?? null,
  );
  const model = useScheduleAssistantStore((state) => state.threads[serverId]?.model ?? null);
  const clearThreadForServer = useScheduleAssistantStore((state) => state.clearThread);

  const clearThread = useCallback(() => {
    clearThreadForServer(serverId);
  }, [clearThreadForServer, serverId]);

  return { messages, isSending, llmProvider, model, clearThread };
}
