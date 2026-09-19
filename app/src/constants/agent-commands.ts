export interface AgentCommand {
  label: string;
  command: string;
}

export const AGENT_COMMANDS: Record<string, AgentCommand[]> = {
  claude: [
    { label: "compact", command: "/compact" },
    { label: "clear", command: "/clear" },
    { label: "help", command: "/help" },
    { label: "config", command: "/config" },
    { label: "memory", command: "/memory" },
    { label: "model", command: "/model" },
    { label: "cost", command: "/cost" },
    { label: "doctor", command: "/doctor" },
    { label: "permissions", command: "/permissions" },
    { label: "mcp", command: "/mcp" },
    { label: "rewind", command: "/rewind" },
  ],
  qoder: [
    { label: "compact", command: "/compact" },
    { label: "clear", command: "/clear" },
    { label: "model", command: "/model" },
    { label: "models", command: "/models" },
    { label: "help", command: "/help" },
    { label: "simplify", command: "/simplify" },
    { label: "verify", command: "/verify" },
    { label: "run", command: "/run" },
    { label: "loop", command: "/loop" },
    { label: "mcp-config", command: "/mcp-config" },
    { label: "security-scan", command: "/security-scan" },
  ],
  aider: [
    { label: "help", command: "/help" },
    { label: "model", command: "/model" },
    { label: "clear", command: "/clear" },
    { label: "cost", command: "/cost" },
    { label: "tokens", command: "/tokens" },
    { label: "diff", command: "/diff" },
    { label: "undo", command: "/undo" },
    { label: "redo", command: "/redo" },
    { label: "run", command: "/run" },
    { label: "web", command: "/web" },
    { label: "map", command: "/map" },
    { label: "add", command: "/add" },
    { label: "drop", command: "/drop" },
    { label: "ls", command: "/ls" },
  ],
  hermes: [
    { label: "tools", command: "/tools" },
    { label: "model", command: "/model" },
    { label: "personality", command: "/personality" },
  ],
};

export function filterSlashCommands(
  agentName: string,
  input: string,
): AgentCommand[] {
  if (!input.startsWith("/")) return [];
  const query = input.slice(1).toLowerCase();
  const commands = AGENT_COMMANDS[agentName];
  if (!commands) return [];
  if (!query) return commands;
  return commands.filter((c) => c.label.startsWith(query));
}
