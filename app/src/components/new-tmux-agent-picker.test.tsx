/**
 * @vitest-environment jsdom
 */
import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NewTmuxAgentPicker } from "@/components/new-tmux-agent-picker";
import type { AgentCommandEntry } from "@/hooks/use-tmux-agents";

const { mockLaunch, mockAgentsResult, mockConfigResult } = vi.hoisted(() => ({
  mockLaunch: vi.fn(),
  mockAgentsResult: {
    commandHistory: [] as AgentCommandEntry[],
    isLoading: false,
  },
  mockConfigResult: {
    config: { tmuxAgentNames: ["myagent"] } as { tmuxAgentNames: string[] } | null,
  },
}));

vi.mock("react-native-unistyles", () => {
  const mockTheme = {
    colorScheme: "light",
    colors: {
      surface0: "#fff",
      border: "#ccc",
      foreground: "#111",
      foregroundMuted: "#666",
      destructive: "#f00",
      primary: "#00f",
    },
    spacing: { 1: 4, 2: 8, 3: 12 },
    fontSize: { xs: 11, sm: 13 },
    borderRadius: { md: 8 },
  };
  return {
    StyleSheet: {
      create: (factory: unknown) => (typeof factory === "function" ? factory(mockTheme) : factory),
    },
    useUnistyles: () => ({ theme: mockTheme }),
  };
});

vi.mock("lucide-react-native", () => ({
  SquareTerminal: () => null,
}));

vi.mock("@/components/adaptive-modal-sheet", () => ({
  AdaptiveModalSheet: ({
    title,
    visible,
    children,
  }: {
    title: string;
    visible: boolean;
    children: React.ReactNode;
  }) =>
    visible ? (
      <div>
        <div>{title}</div>
        {children}
      </div>
    ) : null,
}));

vi.mock("@/hooks/use-tmux-agents", () => ({
  useAggregatedTmuxAgents: () => mockAgentsResult,
}));

vi.mock("@/hooks/use-daemon-config", () => ({
  useDaemonConfig: () => mockConfigResult,
}));

vi.mock("@/hooks/use-tmux-agent-launch", () => ({
  useTmuxAgentLaunch: () => ({ launch: mockLaunch, isLaunching: false }),
}));

function makeHistoryEntry(overrides: Partial<AgentCommandEntry> = {}): AgentCommandEntry {
  return {
    agentName: "claude",
    launchCmd: "claude --resume x",
    lastSeen: "2026-11-18T00:00:00Z",
    serverId: "server-1",
    serverLabel: "Host 1",
    ...overrides,
  };
}

function renderPicker(onClose = vi.fn()) {
  render(
    <NewTmuxAgentPicker
      serverId="server-1"
      filePath="docs/guide.md"
      workspaceRoot="/repo"
      visible
      onClose={onClose}
    />,
  );
  return { onClose };
}

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  mockLaunch.mockReset();
  mockLaunch.mockResolvedValue({ ok: true });
  mockAgentsResult.commandHistory = [];
  mockAgentsResult.isLoading = false;
  mockConfigResult.config = { tmuxAgentNames: ["myagent"] };
});

describe("NewTmuxAgentPicker", () => {
  it("lists current-host history, built-ins and custom names", () => {
    mockAgentsResult.commandHistory = [
      makeHistoryEntry(),
      makeHistoryEntry({ serverId: "server-2", launchCmd: "codex --other" }),
    ];
    renderPicker();

    expect(screen.getByTestId("new-tmux-option-history:claude --resume x")).toBeTruthy();
    expect(screen.queryByTestId("new-tmux-option-history:codex --other")).toBeNull();
    expect(screen.getByTestId("new-tmux-option-name:codex")).toBeTruthy();
    expect(screen.getByTestId("new-tmux-option-name:myagent")).toBeTruthy();
  });

  it("shows built-in rows while history is still loading", () => {
    mockAgentsResult.isLoading = true;
    renderPicker();

    expect(screen.getByTestId("new-tmux-option-name:claude")).toBeTruthy();
    expect(screen.getByTestId("new-tmux-history-loading")).toBeTruthy();
  });

  it("launches the selected agent with file and workspace context", async () => {
    mockAgentsResult.commandHistory = [makeHistoryEntry()];
    const { onClose } = renderPicker();

    fireEvent.click(screen.getByTestId("new-tmux-option-history:claude --resume x"));

    await waitFor(() => {
      expect(mockLaunch).toHaveBeenCalledWith({
        serverId: "server-1",
        agentName: "claude",
        launchCmd: "claude --resume x",
        filePath: "docs/guide.md",
        workspaceRoot: "/repo",
      });
    });
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it("keeps the sheet open and shows the error when launch fails", async () => {
    mockLaunch.mockResolvedValue({ ok: false, error: "tmux is not installed or not in PATH" });
    const { onClose } = renderPicker();

    fireEvent.click(screen.getByTestId("new-tmux-option-name:claude"));

    const errorNode = await screen.findByTestId("new-tmux-launch-error");
    expect(errorNode.textContent).toContain("tmux is not installed or not in PATH");
    expect(onClose).not.toHaveBeenCalled();
  });
});