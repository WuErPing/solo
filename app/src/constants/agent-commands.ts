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
