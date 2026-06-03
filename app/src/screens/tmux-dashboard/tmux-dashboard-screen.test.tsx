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
  return { Terminal: icon("Terminal"), Monitor: icon("Monitor") };
});

vi.mock("@/components/headers/menu-header", () => ({
  MenuHeader: ({ title }: { title: string }) =>
    React.createElement("div", { "data-testid": "menu-header" }, title),
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

let agentsOverride: typeof mockAgents = [];

vi.mock("@/hooks/use-tmux-agents", () => ({
  useAggregatedTmuxAgents: () => ({
    agents: agentsOverride,
    isLoading: false,
    isInitialLoad: false,
    error: null,
    refreshAll: vi.fn(),
  }),
}));

import { TmuxDashboardScreen } from "./tmux-dashboard-screen";

describe("TmuxDashboardScreen", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows empty state when no agents are detected", () => {
    agentsOverride = [];
    render(<TmuxDashboardScreen />);
    expect(screen.getByText(/no ai agents detected/i)).toBeDefined();
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
});
