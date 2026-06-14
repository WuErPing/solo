/**
 * @vitest-environment jsdom
 */
import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const { mockPush, mockSetSelectedAgent, mockTheme } = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockSetSelectedAgent: vi.fn(),
  mockTheme: {
    colors: {
      background: "#000",
      foreground: "#fff",
      foregroundMuted: "#888",
      surface0: "#111",
      surface1: "#222",
      primary: "#0af",
      border: "#333",
      destructive: "#f00",
      terminal: {
        background: "#000",
        foreground: "#fff",
        black: "#000",
        red: "#f00",
        green: "#0f0",
        yellow: "#ff0",
        blue: "#00f",
        magenta: "#f0f",
        cyan: "#0ff",
        white: "#fff",
        brightBlack: "#888",
        brightRed: "#f88",
        brightGreen: "#8f8",
        brightYellow: "#ff8",
        brightBlue: "#88f",
        brightMagenta: "#f8f",
        brightCyan: "#8ff",
        brightWhite: "#fff",
      },
    },
  },
}));

vi.mock("expo-router", () => ({
  router: { push: mockPush, back: vi.fn() },
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => {
      if (typeof factory === "function") {
        return factory(mockTheme);
      }
      return factory;
    },
  },
  useUnistyles: () => ({ theme: mockTheme, rt: {}, breakpoint: undefined }),
  UnistylesRuntime: { setTheme: vi.fn(), themeName: "light" },
}));

vi.mock("lucide-react-native", () => {
  const icon = (name: string) => {
    const Component = (props: Record<string, unknown>) =>
      React.createElement("span", { "data-icon": name, ...props });
    Component.displayName = `Icon(${name})`;
    return Component;
  };
  return { Terminal: icon("Terminal"), Monitor: icon("Monitor"), RefreshCw: icon("RefreshCw") };
});

vi.mock("@/components/headers/back-header", () => ({
  BackHeader: ({ title, rightContent }: { title: string; rightContent?: React.ReactNode }) =>
    React.createElement("div", { "data-testid": "back-header" }, title, rightContent),
}));

vi.mock("@/constants/layout", () => ({
  useIsCompactFormFactor: () => false,
}));

vi.mock("@/app/_layout", () => ({
  useStoreReady: () => true,
}));

vi.mock("@/stores/tmux-agent-store", () => ({
  useTmuxAgentStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector({ setSelectedAgent: mockSetSelectedAgent }),
}));

const mockAgents = [
  {
    serverId: "s1", paneId: "%0", agentName: "claude", sessionName: "dev",
    windowName: "main", paneIndex: 0, panePid: 100, currentCmd: "claude",
    workingDir: "/a", serverLabel: "local",
  },
  {
    serverId: "s1", paneId: "%1", agentName: "pi", sessionName: "dev",
    windowName: "main", paneIndex: 1, panePid: 200, currentCmd: "node",
    workingDir: "/b", serverLabel: "local",
  },
];

const mockExitedAgent = {
  serverId: "s1", paneId: "%2", agentName: "claude", sessionName: "dev",
  windowName: "main", paneIndex: 2, panePid: 300, currentCmd: "bash",
  workingDir: "/c", serverLabel: "local", status: "exited",
};

let agentsOverride: typeof mockAgents = [];
let isInitialLoadOverride = false;
let isLoadingOverride = false;
const mockRefreshAll = vi.fn();

const mockStatusLines = [
  { sessionName: "dev", serverId: "s1", statusLeft: "[#S]", statusCenter: "0:claude*", statusRight: "\"Analyze tmux session\" 22:45 06-Jun-26" },
];

vi.mock("@/hooks/use-tmux-agents", () => ({
  useAggregatedTmuxAgents: () => ({
    agents: agentsOverride,
    isLoading: isLoadingOverride,
    isInitialLoad: isInitialLoadOverride,
    error: null,
    refreshAll: mockRefreshAll,
  }),
}));

vi.mock("@/hooks/use-tmux-status-lines", () => ({
  useTmuxStatusLines: () => mockStatusLines,
}));

import { TmuxDashboardScreen } from "./tmux-dashboard-screen";

describe("TmuxDashboardScreen", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    agentsOverride = [];
    isInitialLoadOverride = false;
    isLoadingOverride = false;
  });

  it("shows empty state when no agents are detected", () => {
    agentsOverride = [];
    render(<TmuxDashboardScreen />);
    expect(screen.getByText(/no ai agents detected/i)).toBeDefined();
  });

  it("shows loading state when queries are disabled after browser refresh", () => {
    agentsOverride = [];
    isInitialLoadOverride = true;
    isLoadingOverride = true;
    render(<TmuxDashboardScreen />);
    expect(screen.getByText(/scanning tmux panes/i)).toBeDefined();
    expect(screen.queryByText(/no ai agents detected/i)).toBeNull();
  });

  it("calls refreshAll when Refresh button is pressed", () => {
    agentsOverride = [];
    render(<TmuxDashboardScreen />);
    const refreshButton = screen.getByText("Refresh");
    fireEvent.click(refreshButton);
    expect(mockRefreshAll).toHaveBeenCalled();
  });

  it("renders agent cards and navigates on press", () => {
    agentsOverride = mockAgents;
    render(<TmuxDashboardScreen />);

    const claudeElements = screen.getAllByText("claude");
    // Second element is the agent card name (first is the name filter card)
    fireEvent.click(claudeElements[1]);

    expect(mockSetSelectedAgent).toHaveBeenCalledWith(mockAgents[0]);
    expect(mockPush).toHaveBeenCalledWith("/tmux-pane");
  });

  it("renders agent details in compact S:/W:/P:/PID: format", () => {
    agentsOverride = mockAgents;
    render(<TmuxDashboardScreen />);

    // Should show compact format on a single line per agent
    const compactLines = screen.getAllByText("S:dev W:main P:%0 PID:100");
    expect(compactLines.length).toBe(1);

    // Old 4-line format should NOT be present
    expect(screen.queryByText(/Session:/)).toBeNull();
    expect(screen.queryByText(/Window:/)).toBeNull();
    expect(screen.queryByText(/Pane:/)).toBeNull();
    expect(screen.queryByText(/^PID:/)).toBeNull();
  });

  it("renders only statusRight in agent cards (statusLeft and statusCenter are redundant with compact detail line)", () => {
    agentsOverride = mockAgents;
    render(<TmuxDashboardScreen />);

    // statusRight pane title and time/date should be rendered
    expect(screen.getAllByText('"Analyze tmux session"').length).toBe(2);
    expect(screen.getAllByText("22:45 06-Jun-26").length).toBe(2);

    // statusLeft and statusCenter should NOT be rendered (redundant with S:/W:/P:/PID:)
    expect(screen.queryByText("[#S]")).toBeNull();
    expect(screen.queryByText("0:claude*")).toBeNull();
  });

  it("renders exited agent with exited badge", () => {
    agentsOverride = [mockExitedAgent];
    render(<TmuxDashboardScreen />);
    expect(screen.getByText("exited")).toBeDefined();
    expect(screen.getAllByText("claude").length).toBeGreaterThanOrEqual(1);
  });

  it("renders mix of active and exited agents with correct count", () => {
    agentsOverride = [...mockAgents, mockExitedAgent];
    render(<TmuxDashboardScreen />);
    // Badge should show "3 agent(s), 1 exited"
    expect(screen.getByText("3 agent(s), 1 exited")).toBeDefined();
    // Only one "exited" badge (the exited agent card)
    expect(screen.getAllByText("exited").length).toBe(1);
  });
});
