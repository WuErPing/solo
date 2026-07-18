import { create } from "zustand";
import type { ScheduleAssistResponse } from "@server/shared/messages";

export type ScheduleAssistPayload = ScheduleAssistResponse["payload"];
export type ScheduleAssistProposal = NonNullable<ScheduleAssistPayload["proposal"]>;

export type AssistantMessageKind = "text" | "proposal" | "error" | "receipt";

export interface AssistantMessage {
  id: string;
  role: "user" | "assistant";
  kind: AssistantMessageKind;
  /** Bubble text: user input, assistant text, proposal summary, or receipt label. */
  text?: string;
  proposal?: ScheduleAssistProposal;
  /** Machine code for error messages, e.g. "no_llm_provider" | "timeout" | "unknown". */
  error?: string;
  /** Schedule id a receipt links to (absent for deletes). */
  receiptScheduleId?: string;
  /** True while the confirm mutation is in flight. */
  applying?: boolean;
  /** Inline confirm failure — proposal stays on the message so the user can retry. */
  applyError?: string;
  /** Proposal dismissed by the user. */
  cancelled?: boolean;
  createdAt: number;
}

interface AssistantThread {
  messages: AssistantMessage[];
  isSending: boolean;
  llmProvider: string | null;
  model: string | null;
}

interface ScheduleAssistantState {
  threads: Record<string, AssistantThread>;
  addMessage: (serverId: string, message: AssistantMessage) => void;
  updateMessage: (
    serverId: string,
    messageId: string,
    patch: Partial<Omit<AssistantMessage, "id" | "role" | "createdAt">>,
  ) => void;
  setSending: (serverId: string, sending: boolean) => void;
  setProviderInfo: (serverId: string, info: { llmProvider?: string; model?: string }) => void;
  clearThread: (serverId: string) => void;
}

const EMPTY_THREAD: AssistantThread = {
  messages: [],
  isSending: false,
  llmProvider: null,
  model: null,
};

function getOrCreateThread(threads: Record<string, AssistantThread>, serverId: string) {
  return threads[serverId] ?? { ...EMPTY_THREAD, messages: [] };
}

let messageCounter = 0;

/** Monotonic, unique-enough message id (session-scoped threads, no persistence). */
export function nextAssistantMessageId(): string {
  messageCounter += 1;
  return `am-${Date.now()}-${messageCounter}`;
}

export const useScheduleAssistantStore = create<ScheduleAssistantState>((set) => ({
  threads: {},

  addMessage: (serverId, message) =>
    set((state) => {
      const thread = getOrCreateThread(state.threads, serverId);
      return {
        threads: {
          ...state.threads,
          [serverId]: { ...thread, messages: [...thread.messages, message] },
        },
      };
    }),

  updateMessage: (serverId, messageId, patch) =>
    set((state) => {
      const thread = state.threads[serverId];
      if (!thread) {
        return state;
      }
      return {
        threads: {
          ...state.threads,
          [serverId]: {
            ...thread,
            messages: thread.messages.map((message) =>
              message.id === messageId ? { ...message, ...patch } : message,
            ),
          },
        },
      };
    }),

  setSending: (serverId, sending) =>
    set((state) => {
      const thread = getOrCreateThread(state.threads, serverId);
      return {
        threads: { ...state.threads, [serverId]: { ...thread, isSending: sending } },
      };
    }),

  setProviderInfo: (serverId, info) =>
    set((state) => {
      const thread = getOrCreateThread(state.threads, serverId);
      return {
        threads: {
          ...state.threads,
          [serverId]: {
            ...thread,
            llmProvider: info.llmProvider ?? thread.llmProvider,
            model: info.model ?? thread.model,
          },
        },
      };
    }),

  clearThread: (serverId) =>
    set((state) => {
      const thread = state.threads[serverId];
      if (!thread) {
        return state;
      }
      return {
        threads: { ...state.threads, [serverId]: { ...thread, messages: [], isSending: false } },
      };
    }),
}));

const MAX_TRANSCRIPT_TURNS = 10;

function summarizeProposal(proposal: ScheduleAssistProposal): string {
  const label = proposal.name ?? proposal.scheduleId ?? "schedule";
  return `[proposal] ${proposal.op} "${label}": ${proposal.summary}`;
}

/**
 * Render the thread as plain-text turns for the stateless parse endpoint.
 * Last ≤10 turns, oldest first; proposals collapse to one summary line.
 */
export function buildTranscript(
  messages: readonly AssistantMessage[],
): { role: "user" | "assistant"; content: string }[] {
  const turns: { role: "user" | "assistant"; content: string }[] = [];
  for (const message of messages) {
    let content: string | null = null;
    if (message.kind === "proposal" && message.proposal) {
      content = summarizeProposal(message.proposal);
    } else if (message.text) {
      content = message.text;
    }
    if (content) {
      turns.push({ role: message.role, content });
    }
  }
  return turns.slice(-MAX_TRANSCRIPT_TURNS);
}
