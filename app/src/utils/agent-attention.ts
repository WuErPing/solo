interface ShouldClearAgentAttentionInput {
  agentId: string | null | undefined;
  isConnected: boolean;
  requiresAttention: boolean | null | undefined;
  attentionReason?: "finished" | "error" | "permission" | null | undefined;
  trigger?: AgentAttentionClearTrigger;
  hasDeferredFocusEntryClear?: boolean;
}

export type AgentAttentionClearTrigger =
  | "focus-entry"
  | "input-focus"
  | "prompt-send"
  | "agent-blur";

export function shouldClearAgentAttention(input: ShouldClearAgentAttentionInput): boolean {
  const agentId = input.agentId?.trim();
  if (!agentId) {
    return false;
  }
  if (!input.isConnected) {
    return false;
  }
  if (!input.requiresAttention) {
    return false;
  }
  if (input.attentionReason === "permission") {
    return false;
  }
  if (input.trigger === "focus-entry" && input.hasDeferredFocusEntryClear === true) {
    return false;
  }
  return true;
}
