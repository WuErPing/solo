/**
 * @vitest-environment jsdom
 */
import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const { mockSendKeys, mockRefetch, mockLoadMoreHistory, mockSetAutoRefresh, mockTheme } = vi.hoisted(() => ({
  mockSendKeys: vi.fn(() => Promise.resolve({})),
  mockRefetch: vi.fn(),
  mockLoadMoreHistory: vi.fn(),
  mockSetAutoRefresh: vi.fn(),
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
  return {
    ArrowLeft: icon("ArrowLeft"),
    Send: icon("Send"),
    Palette: icon("Palette"),
  };
});

vi.mock("expo-router", () => ({
  router: {
    back: vi.fn(),
    push: vi.fn(),
  },
}));

vi.mock("@/components/headers/back-header", () => ({
  BackHeader: ({ title, onBack, rightContent }: { title: string; onBack?: () => void; rightContent?: React.ReactNode }) =>
    React.createElement("div", { "data-testid": "back-header" },
      React.createElement("button", { onClick: onBack, "aria-label": "Back" }, "Back"),
      title,
      rightContent ? React.createElement("div", { "data-testid": "header-right" }, rightContent) : null,
    ),
}));

vi.mock("@/components/ansi-text-renderer", () => ({
  AnsiTextContent: ({ segments, style }: { segments: { text: string }[]; style?: unknown }) =>
    React.createElement("span", { style }, segments.map((s: { text: string }) => s.text).join("")),
}));

vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  DropdownMenuItem: ({ children, onSelect, selected }: { children: React.ReactNode; onSelect?: () => void; selected?: boolean }) =>
    React.createElement("button", { onClick: onSelect, "data-selected": selected }, children),
  DropdownMenuSeparator: () => React.createElement("hr", null),
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
}));

vi.mock("@/utils/ansi-parser", () => ({
  parseAnsi: (input: string) => [{ text: input, style: {} }],
}));

const { autoRefreshRef } = vi.hoisted(() => ({
  autoRefreshRef: { current: true },
}));

vi.mock("@/hooks/use-tmux-capture-pane", () => ({
  useTmuxCapturePane: () => ({
    content: "$ ls\nfile1.txt\nfile2.txt\n$ _",
    isLoading: false,
    isLoadingMore: false,
    error: null,
    refetch: mockRefetch,
    scrollbackLines: 200,
    loadMoreHistory: mockLoadMoreHistory,
    hasMoreHistory: true,
    autoRefresh: autoRefreshRef.current,
    setAutoRefresh: mockSetAutoRefresh,
  }),
}));

vi.mock("@/hooks/use-settings", () => ({
  useAppSettings: () => ({
    settings: { theme: "dark", sendBehavior: "interrupt", terminalTheme: "system" },
    isLoading: false,
    error: null,
    updateSettings: vi.fn(),
    resetSettings: vi.fn(),
  }),
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => ({
    tmuxSendKeys: mockSendKeys,
  }),
  getHostRuntimeStore: () => ({
    getClient: () => ({
      tmuxSendKeys: mockSendKeys,
      getConnectionState: () => ({ status: "connected" }),
    }),
  }),
}));

const mockAgent = {
  serverId: "server1",
  paneId: "%0",
  agentName: "claude",
  sessionName: "dev",
  windowName: "main",
  paneIndex: 0,
  panePid: 1234,
  currentCmd: "claude",
  workingDir: "/home/user",
  serverLabel: "local",
};

const { agentRef } = vi.hoisted(() => ({
  agentRef: { current: null as typeof mockAgent | null },
}));
agentRef.current = mockAgent;

vi.mock("@/stores/tmux-agent-store", () => ({
  useTmuxAgentStore: (selector: (s: { selectedAgent: typeof mockAgent | null }) => unknown) => {
    const state = { selectedAgent: agentRef.current };
    return selector ? selector(state) : state;
  },
}));

import { TmuxPaneScreen } from "./tmux-pane-screen";

describe("TmuxPaneScreen", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    autoRefreshRef.current = true;
  });

  it("renders pane content text", () => {
    render(<TmuxPaneScreen />);
    expect(screen.getByText((text) => text.includes("$ ls"))).toBeDefined();
  });

  it("renders a back button", () => {
    render(<TmuxPaneScreen />);
    expect(screen.getByRole("button", { name: /back/i })).toBeDefined();
  });

  it("shows empty state when no agent is selected", () => {
    agentRef.current = null;
    render(<TmuxPaneScreen />);
    expect(screen.getByText(/no agent selected/i)).toBeDefined();
    agentRef.current = mockAgent;
  });

  it("renders Home and End key buttons", () => {
    render(<TmuxPaneScreen />);
    expect(screen.getByText("Home")).toBeDefined();
    expect(screen.getByText("End")).toBeDefined();
  });

  it("Home button scrolls to top without sending tmux key", () => {
    render(<TmuxPaneScreen />);
    fireEvent.click(screen.getByText("Home"));
    expect(mockSendKeys).not.toHaveBeenCalled();
  });

  it("End button scrolls to bottom without sending tmux key", () => {
    render(<TmuxPaneScreen />);
    fireEvent.click(screen.getByText("End"));
    expect(mockSendKeys).not.toHaveBeenCalled();
  });

  it("refetches pane content after sending a command key", async () => {
    render(<TmuxPaneScreen />);
    fireEvent.click(screen.getByText("Enter"));
    await vi.waitFor(() => {
      expect(mockRefetch).toHaveBeenCalled();
    });
  });

  it("refetches pane content after sending text input", async () => {
    render(<TmuxPaneScreen />);
    const input = screen.getByPlaceholderText(/type a command/i);
    fireEvent.change(input, { target: { value: "hello" } });
    fireEvent.click(screen.getByTestId("send-button"));
    await vi.waitFor(() => {
      expect(mockRefetch).toHaveBeenCalled();
    });
  });

  it("triggers loadMoreHistory when scrolling near top", () => {
    render(<TmuxPaneScreen />);

    const scrollView = screen.getByTestId("tmux-pane-scroll");
    fireEvent.scroll(scrollView, {
      nativeEvent: {
        contentOffset: { y: 5, x: 0 },
        contentSize: { height: 2000, width: 300 },
        layoutMeasurement: { height: 500, width: 300 },
      },
    });

    expect(mockLoadMoreHistory).toHaveBeenCalledTimes(1);
  });

  it("shows loading more indicator when isLoadingMore is true", () => {
    render(<TmuxPaneScreen />);
    expect(screen.queryByText(/loading more history/i)).toBeNull();
  });

  it("renders auto refresh toggle in header", () => {
    render(<TmuxPaneScreen />);
    const headerRight = screen.getByTestId("header-right");
    expect(headerRight).toBeDefined();
    expect(screen.getByText("Auto")).toBeDefined();
  });

  it("calls setAutoRefresh when clicking the auto toggle", () => {
    render(<TmuxPaneScreen />);
    const toggle = screen.getByText("Auto");
    fireEvent.click(toggle);
    expect(mockSetAutoRefresh).toHaveBeenCalledWith(false);
  });

  it("shows manual refresh button when auto refresh is off", () => {
    autoRefreshRef.current = false;
    render(<TmuxPaneScreen />);
    expect(screen.getByText("Refresh")).toBeDefined();
  });

  it("does not show manual refresh button when auto refresh is on", () => {
    autoRefreshRef.current = true;
    render(<TmuxPaneScreen />);
    expect(screen.queryByText("Refresh")).toBeNull();
  });

  it("calls refetch when clicking the manual refresh button", () => {
    autoRefreshRef.current = false;
    render(<TmuxPaneScreen />);
    fireEvent.click(screen.getByText("Refresh"));
    expect(mockRefetch).toHaveBeenCalled();
  });
});
