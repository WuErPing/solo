export const AGENT_STATUSES = [
  "initializing",
  "idle",
  "running",
  "error",
  "closed",
] as const;

export type AgentStatus = (typeof AGENT_STATUSES)[number];

// Backward-compatible alias
export type AgentLifecycleStatus = AgentStatus;
