import { BUILT_IN_AGENT_NAMES } from "@/constants/tmux-agent-names";
import type { AgentCommandEntry, TmuxAgent, TmuxPane } from "@/hooks/use-tmux-agents";

// Interactive CLIs that accept an initial prompt as a positional argument.
const PROMPT_ARG_AGENTS = ["claude", "codex"];

export function shellQuoteSingle(value: string): string {
  return `'${value.replace(/'/g, "'\\''")}'`;
}

export function buildFilePrompt(filePath: string): string {
  return `Read ${filePath} and follow the instructions in it.`;
}

function commandBaseName(token: string): string {
  return token.slice(token.lastIndexOf("/") + 1);
}

// Mirrors the daemon's matchAgentCommand: exact name or version-suffixed
// binary ("claude-1.0.22" → "claude").
function matchPromptArgAgent(token: string): boolean {
  const base = commandBaseName(token);
  return PROMPT_ARG_AGENTS.some((name) => {
    if (base === name) return true;
    if (!base.startsWith(name)) return false;
    const rest = base.slice(name.length);
    return /^\d/.test(rest) || /^-\d/.test(rest);
  });
}

export function buildAgentLaunchCommand(input: {
  launchCmd: string;
  filePath: string;
}): string {
  const trimmed = input.launchCmd.trim();
  // Commands carrying args/subcommands (e.g. "claude --resume x") run verbatim.
  if (/\s/.test(trimmed) || !matchPromptArgAgent(trimmed)) {
    return input.launchCmd;
  }
  return `${input.launchCmd} ${shellQuoteSingle(buildFilePrompt(input.filePath))}`;
}

export function buildTmuxSessionName(agentName: string, now: number): string {
  // tmux session names must not contain '.' or ':'.
  const sanitized = agentName.toLowerCase().replace(/[^a-z0-9_-]/g, "");
  return `${sanitized || "agent"}-${now}`;
}

export function findSessionPane(
  agents: TmuxAgent[],
  otherPanes: TmuxPane[],
  sessionName: string,
): TmuxAgent | TmuxPane | null {
  return (
    agents.find((agent) => agent.sessionName === sessionName) ??
    otherPanes.find((pane) => pane.sessionName === sessionName) ??
    null
  );
}

export interface AgentPickerOption {
  key: string;
  agentName: string;
  launchCmd: string;
  source: "history" | "builtin" | "custom";
  lastSeen?: string;
}

export function mergeAgentPickerOptions(input: {
  history: AgentCommandEntry[];
  customNames: string[];
}): AgentPickerOption[] {
  const options: AgentPickerOption[] = [];
  const seenCommands = new Set<string>();
  const historyBareNames = new Set<string>();
  for (const entry of input.history) {
    if (seenCommands.has(entry.launchCmd)) continue;
    seenCommands.add(entry.launchCmd);
    options.push({
      key: `history:${entry.launchCmd}`,
      agentName: entry.agentName,
      launchCmd: entry.launchCmd,
      source: "history",
      lastSeen: entry.lastSeen,
    });
    const token = entry.launchCmd.trim();
    if (!/\s/.test(token)) {
      historyBareNames.add(commandBaseName(token));
    }
  }

  const builtInSet = new Set(BUILT_IN_AGENT_NAMES);
  const seenNames = new Set<string>();
  for (const name of [...BUILT_IN_AGENT_NAMES, ...input.customNames]) {
    if (seenNames.has(name) || historyBareNames.has(name)) continue;
    seenNames.add(name);
    options.push({
      key: `name:${name}`,
      agentName: name,
      launchCmd: name,
      source: builtInSet.has(name) ? "builtin" : "custom",
    });
  }
  return options;
}