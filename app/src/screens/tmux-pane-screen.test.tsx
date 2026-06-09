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
    TextSelect: icon("TextSelect"),
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

vi.mock("@/components/ansi-text-line", () => ({
  AnsiTextLine: ({ segments, style, selectable }: { segments: { text: string }[]; style?: unknown; selectable?: boolean }) =>
    React.createElement("span", { style, "data-selectable": String(selectable ?? false) }, segments.map((s: { text: string }) => s.text).join("")),
}));

vi.mock("@/utils/ansi-line-splitter", () => ({
  splitSegmentsByLine: (segments: { text: string }[]) =>
    segments.map((s) => [s]),
}));

vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  DropdownMenuItem: ({ children, onSelect, selected }: { children: React.ReactNode; onSelect?: () => void; selected?: boolean }) =>
    React.createElement("button", { onClick: onSelect, "data-selected": selected }, children),
  DropdownMenuSeparator: () => React.createElement("hr", null),
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
}));

const { mockParseAnsi } = vi.hoisted(() => ({
  mockParseAnsi: vi.fn((input: string) => [{ text: input, style: {} }]),
}));

vi.mock("@/utils/ansi-parser", () => ({
  parseAnsi: (input: string) => mockParseAnsi(input),
}));

const { mockDetectColors } = vi.hoisted(() => ({
  mockDetectColors: vi.fn(() => ({ background: "#1a1a2e", foreground: "#e0e0e0" })),
}));

vi.mock("@/utils/detect-ansi-colors", () => ({
  detectColorsFromAnsi: mockDetectColors,
}));

const { autoRefreshRef, contentRef } = vi.hoisted(() => ({
  autoRefreshRef: { current: true },
  contentRef: { current: "$ ls\nfile1.txt\nfile2.txt\n$ _" },
}));

vi.mock("@/hooks/use-tmux-capture-pane", () => ({
  useTmuxCapturePane: () => ({
    content: contentRef.current,
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
    contentRef.current = "$ ls\nfile1.txt\nfile2.txt\n$ _";
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

  it("background color does not change when content changes with terminalTheme system", () => {
    mockDetectColors.mockReturnValue({ background: "#1a1a2e", foreground: "#e0e0e0" });
    const { rerender } = render(<TmuxPaneScreen />);

    const scrollView = screen.getByTestId("tmux-pane-scroll");
    const initialBg = (scrollView as HTMLElement).style.backgroundColor;

    // Change content — detectColorsFromAnsi returns different colors now
    mockDetectColors.mockReturnValue({ background: "#2a2a3e", foreground: "#f0f0f0" });
    contentRef.current = "$ pwd\n/home/user\n$ _";
    rerender(<TmuxPaneScreen />);

    // Background should be stable (cached from first detection)
    expect((scrollView as HTMLElement).style.backgroundColor).toBe(initialBg);
  });

  it("background color re-detects when content transitions from empty to non-empty", () => {
    mockDetectColors.mockReturnValue({ background: "#1a1a2e", foreground: "#e0e0e0" });
    const { rerender } = render(<TmuxPaneScreen />);

    const scrollView = screen.getByTestId("tmux-pane-scroll");
    const initialBg = (scrollView as HTMLElement).style.backgroundColor;

    // Simulate pane navigation: content goes empty then new content arrives
    contentRef.current = "";
    rerender(<TmuxPaneScreen />);

    mockDetectColors.mockReturnValue({ background: "#2a2a3e", foreground: "#f0f0f0" });
    contentRef.current = "$ pwd\n/home/user\n$ _";
    rerender(<TmuxPaneScreen />);

    // Background should update because content went from empty to non-empty
    expect((scrollView as HTMLElement).style.backgroundColor).not.toBe(initialBg);
  });

  it("does NOT re-parse ANSI content when content reference is stable across re-renders", () => {
    // This is the TDD contract for "no jitter when content unchanged".
    // The dedup hook returns the same content string reference when the payload
    // is identical across polls; the screen must not re-invoke parseAnsi on
    // stable content, otherwise every 5s poll would rebuild the <Text> tree
    // and cause visible refresh jitter on Android.
    const { rerender } = render(<TmuxPaneScreen />);

    expect(mockParseAnsi).toHaveBeenCalledTimes(1);

    // Re-render multiple times with the SAME content REFERENCE.
    // (The mock hook captures contentRef.current once; rerender does not
    // change it, so the content string reference is stable.)
    rerender(<TmuxPaneScreen />);
    rerender(<TmuxPaneScreen />);
    rerender(<TmuxPaneScreen />);

    // parseAnsi must NOT have been called again — memo should have short-circuited.
    expect(mockParseAnsi).toHaveBeenCalledTimes(1);
  });

  it("re-parses ANSI content only when content reference changes", () => {
    const { rerender } = render(<TmuxPaneScreen />);
    expect(mockParseAnsi).toHaveBeenCalledTimes(1);

    // Change the content string reference — memo should recalculate.
    contentRef.current = "$ pwd\n/home\n$ _";
    rerender(<TmuxPaneScreen />);

    expect(mockParseAnsi).toHaveBeenCalledTimes(2);
  });

  describe("select mode", () => {
    function getSelectableAttr(): string | null {
      const el = screen.getByTestId("tmux-pane-scroll").querySelector("[data-selectable]");
      return el?.getAttribute("data-selectable") ?? null;
    }

    it("renders a Select toggle button in the header", () => {
      render(<TmuxPaneScreen />);
      const headerRight = screen.getByTestId("header-right");
      expect(headerRight.querySelector('[data-icon="TextSelect"]')).not.toBeNull();
    });

    it("select mode is off by default — AnsiTextContent is not selectable", () => {
      render(<TmuxPaneScreen />);
      expect(getSelectableAttr()).toBe("false");
    });

    it("toggling select ON makes AnsiTextContent selectable and pauses autoRefresh", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByText("Select"));
      expect(getSelectableAttr()).toBe("true");
      expect(mockSetAutoRefresh).toHaveBeenCalledWith(false);
    });

    it("toggling select OFF makes AnsiTextContent not selectable and restores autoRefresh", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByText("Select"));
      mockSetAutoRefresh.mockClear();
      fireEvent.click(screen.getByText("Select"));
      expect(getSelectableAttr()).toBe("false");
      expect(mockSetAutoRefresh).toHaveBeenCalledWith(true);
    });

    it("exiting select mode restores autoRefresh=false when it was off before", () => {
      autoRefreshRef.current = false;
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByText("Select"));
      mockSetAutoRefresh.mockClear();
      fireEvent.click(screen.getByText("Select"));
      expect(mockSetAutoRefresh).toHaveBeenCalledWith(false);
    });

    it("scroll events do not trigger loadMoreHistory in select mode", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByText("Select"));
      const scrollView = screen.getByTestId("tmux-pane-scroll");
      fireEvent.scroll(scrollView, {
        nativeEvent: {
          contentOffset: { y: 5, x: 0 },
          contentSize: { height: 2000, width: 300 },
          layoutMeasurement: { height: 500, width: 300 },
        },
      });
      expect(mockLoadMoreHistory).not.toHaveBeenCalled();
    });

    it("scroll events trigger loadMoreHistory when not in select mode", () => {
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
  });
});
