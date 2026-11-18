import { describe, expect, it } from "vitest";
import {
  buildAgentLaunchCommand,
  buildFilePrompt,
  buildTmuxSessionName,
  findSessionPane,
  mergeAgentPickerOptions,
  shellQuoteSingle,
} from "./tmux-agent-launch";
import type { AgentCommandEntry, TmuxAgent, TmuxPane } from "@/hooks/use-tmux-agents";

function makeAgent(sessionName: string): TmuxAgent {
  return {
    sessionName,
    windowName: "main",
    paneId: "%1",
    paneIndex: 0,
    panePid: 100,
    agentName: "claude",
    currentCmd: "claude",
    workingDir: "/tmp/project",
    serverId: "server-1",
    serverLabel: "Host 1",
  };
}

function makePane(sessionName: string): TmuxPane {
  return {
    sessionName,
    windowName: "main",
    paneId: "%2",
    paneIndex: 0,
    panePid: 200,
    currentCmd: "zsh",
    workingDir: "/tmp/project",
    serverId: "server-1",
    serverLabel: "Host 1",
  };
}

function makeHistoryEntry(overrides: Partial<AgentCommandEntry> = {}): AgentCommandEntry {
  return {
    agentName: "claude",
    launchCmd: "claude",
    lastSeen: "2026-11-18T00:00:00Z",
    serverId: "server-1",
    serverLabel: "Host 1",
    ...overrides,
  };
}

describe("shellQuoteSingle", () => {
  it("wraps plain values in single quotes", () => {
    expect(shellQuoteSingle("claude")).toBe("'claude'");
  });

  it("keeps spaces, double quotes, dollars and backticks literal", () => {
    expect(shellQuoteSingle('a b "c" $d `e`')).toBe("'a b \"c\" $d `e`'");
  });

  it("escapes embedded single quotes", () => {
    expect(shellQuoteSingle("it's")).toBe("'it'\\''s'");
  });

  it("handles an empty string", () => {
    expect(shellQuoteSingle("")).toBe("''");
  });
});

describe("buildFilePrompt", () => {
  it("mentions the file path", () => {
    expect(buildFilePrompt("docs/guide.md")).toBe(
      "Read docs/guide.md and follow the instructions in it.",
    );
  });
});

describe("buildAgentLaunchCommand", () => {
  it("appends a quoted prompt for bare claude", () => {
    expect(buildAgentLaunchCommand({ launchCmd: "claude", filePath: "docs/guide.md" })).toBe(
      "claude 'Read docs/guide.md and follow the instructions in it.'",
    );
  });

  it("appends a quoted prompt for bare codex", () => {
    expect(buildAgentLaunchCommand({ launchCmd: "codex", filePath: "a.md" })).toBe(
      "codex 'Read a.md and follow the instructions in it.'",
    );
  });

  it("matches version-suffixed and path-qualified binaries", () => {
    expect(buildAgentLaunchCommand({ launchCmd: "claude-1.0.22", filePath: "a.md" })).toContain(
      "'Read a.md",
    );
    expect(
      buildAgentLaunchCommand({ launchCmd: "/usr/local/bin/claude", filePath: "a.md" }),
    ).toContain("'Read a.md");
  });

  it("shell-quotes paths with spaces and quotes", () => {
    const command = buildAgentLaunchCommand({
      launchCmd: "claude",
      filePath: "docs/it's a guide.md",
    });
    expect(command).toBe("claude 'Read docs/it'\\''s a guide.md and follow the instructions in it.'");
  });

  it("leaves commands with args or subcommands verbatim", () => {
    const launchCmd = "claude --resume abc123";
    expect(buildAgentLaunchCommand({ launchCmd, filePath: "a.md" })).toBe(launchCmd);
  });

  it("leaves non-allowlisted agents verbatim", () => {
    for (const launchCmd of ["kimi", "opencode", "pi", "qodercli", "cursor-agent"]) {
      expect(buildAgentLaunchCommand({ launchCmd, filePath: "a.md" })).toBe(launchCmd);
    }
  });
});

describe("buildTmuxSessionName", () => {
  it("combines sanitized agent name and timestamp", () => {
    expect(buildTmuxSessionName("claude", 1700000000000)).toBe("claude-1700000000000");
  });

  it("strips characters tmux forbids", () => {
    expect(buildTmuxSessionName("My.Agent:v2", 42)).toBe("myagentv2-42");
  });

  it("falls back when nothing survives sanitization", () => {
    expect(buildTmuxSessionName("...", 42)).toBe("agent-42");
  });
});

describe("findSessionPane", () => {
  it("prefers a classified agent over a plain pane", () => {
    const agent = makeAgent("claude-1");
    const pane = makePane("claude-1");
    expect(findSessionPane([agent], [pane], "claude-1")).toBe(agent);
  });

  it("falls back to otherPanes while agent detection lags", () => {
    const pane = makePane("claude-1");
    expect(findSessionPane([], [pane], "claude-1")).toBe(pane);
  });

  it("returns null when the session is absent", () => {
    expect(findSessionPane([makeAgent("other")], [makePane("other")], "claude-1")).toBeNull();
  });
});

describe("mergeAgentPickerOptions", () => {
  it("lists history first, then built-ins and custom names", () => {
    const options = mergeAgentPickerOptions({
      history: [makeHistoryEntry({ launchCmd: "claude --resume x" })],
      customNames: ["myagent"],
    });
    expect(options[0]).toMatchObject({
      agentName: "claude",
      launchCmd: "claude --resume x",
      source: "history",
    });
    const names = options.map((option) => option.agentName);
    expect(names).toContain("codex");
    expect(names).toContain("myagent");
    expect(options.find((option) => option.agentName === "myagent")?.source).toBe("custom");
    expect(options.find((option) => option.agentName === "codex")?.source).toBe("builtin");
  });

  it("dedupes history by launchCmd", () => {
    const options = mergeAgentPickerOptions({
      history: [makeHistoryEntry(), makeHistoryEntry()],
      customNames: [],
    });
    expect(options.filter((option) => option.source === "history")).toHaveLength(1);
  });

  it("skips built-ins already covered by a bare history command", () => {
    const options = mergeAgentPickerOptions({
      history: [makeHistoryEntry({ launchCmd: "/usr/local/bin/claude" })],
      customNames: [],
    });
    expect(options.filter((option) => option.agentName === "claude")).toHaveLength(1);
    expect(options.some((option) => option.source === "builtin" && option.agentName === "claude")).toBe(
      false,
    );
  });

  it("keeps built-ins when history only has args-bearing commands", () => {
    const options = mergeAgentPickerOptions({
      history: [makeHistoryEntry({ launchCmd: "claude --resume x" })],
      customNames: [],
    });
    expect(options.some((option) => option.source === "builtin" && option.agentName === "claude")).toBe(
      true,
    );
  });

  it("dedupes custom names against built-ins and each other", () => {
    const options = mergeAgentPickerOptions({
      history: [],
      customNames: ["claude", "myagent", "myagent"],
    });
    expect(options.filter((option) => option.agentName === "claude")).toHaveLength(1);
    expect(options.filter((option) => option.agentName === "myagent")).toHaveLength(1);
  });
});