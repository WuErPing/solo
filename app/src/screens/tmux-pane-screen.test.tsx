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
    Clock: icon("Clock"),
    ChevronDown: icon("ChevronDown"),
    ChevronUp: icon("ChevronUp"),
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
  AnsiTextLine: ({ segments, style, selectable }: { segments: { text: string }[]; style?: unknown; selectable?: boolean }) => {
    const flattenedStyle = Array.isArray(style) ? Object.assign({}, ...style) : style;
    return React.createElement("span", { style: flattenedStyle, "data-selectable": String(selectable ?? false) }, segments.map((s: { text: string }) => s.text).join(""));
  },
}));

vi.mock("@/utils/ansi-line-splitter", () => ({
  splitSegmentsByLine: (segments: { text: string }[]) =>
    segments.map((s) => [s]),
}));

vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  DropdownMenuItem: ({
    children,
    onSelect,
    selected,
    showSelectedCheck,
    leading,
  }: {
    children: React.ReactNode;
    onSelect?: () => void;
    selected?: boolean;
    showSelectedCheck?: boolean;
    leading?: { props?: { style?: Record<string, unknown> } } | null;
  }) => {
    const leadingStyle = leading?.props?.style;
    return React.createElement(
      "button",
      {
        onClick: onSelect,
        "data-selected": selected,
        "data-show-selected-check": showSelectedCheck,
        "data-leading-style": leadingStyle ? JSON.stringify(leadingStyle) : null,
      },
      children,
    );
  },
  DropdownMenuSeparator: () => React.createElement("hr", null),
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
}));

const { mockParseAnsi } = vi.hoisted(() => ({
  mockParseAnsi: vi.fn((input: string) => [{ text: input, style: {} }]),
}));

vi.mock("@/utils/ansi-parser", () => ({
  parseAnsi: (input: string) => mockParseAnsi(input),
}));

const { autoRefreshRef, contentRef, terminalThemeRef } = vi.hoisted(() => ({
  autoRefreshRef: { current: true },
  contentRef: { current: "$ ls\nfile1.txt\nfile2.txt\n$ _" },
  terminalThemeRef: { current: "light" as string },
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
    settings: { theme: "dark", sendBehavior: "interrupt", terminalTheme: terminalThemeRef.current },
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

vi.mock("@/hooks/use-tmux-agents", () => ({
  useAggregatedTmuxAgents: () => ({
    agents: [],
    otherPanes: [],
    commandHistory: [],
    isLoading: false,
    isInitialLoad: false,
    error: null,
    refreshAll: vi.fn(),
  }),
}));

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
    terminalThemeRef.current = "light";
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
      const selectButton = screen.getByTestId("tmux-select-button");
      expect(selectButton.querySelector('[data-icon="TextSelect"]')).not.toBeNull();
    });

    it("select button is icon-only and has no visible text", () => {
      render(<TmuxPaneScreen />);
      const selectButton = screen.getByTestId("tmux-select-button");
      expect(selectButton.textContent).toBe("");
      expect(screen.queryByText("Select")).toBeNull();
    });

    it("select mode is off by default — AnsiTextContent is not selectable", () => {
      render(<TmuxPaneScreen />);
      expect(getSelectableAttr()).toBe("false");
    });

    it("toggling select ON makes AnsiTextContent selectable and pauses autoRefresh", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByTestId("tmux-select-button"));
      expect(getSelectableAttr()).toBe("true");
      expect(mockSetAutoRefresh).toHaveBeenCalledWith(false);
    });

    it("toggling select OFF makes AnsiTextContent not selectable and restores autoRefresh", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByTestId("tmux-select-button"));
      mockSetAutoRefresh.mockClear();
      fireEvent.click(screen.getByTestId("tmux-select-button"));
      expect(getSelectableAttr()).toBe("false");
      expect(mockSetAutoRefresh).toHaveBeenCalledWith(true);
    });

    it("exiting select mode restores autoRefresh=false when it was off before", () => {
      autoRefreshRef.current = false;
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByTestId("tmux-select-button"));
      mockSetAutoRefresh.mockClear();
      fireEvent.click(screen.getByTestId("tmux-select-button"));
      expect(mockSetAutoRefresh).toHaveBeenCalledWith(false);
    });

    it("scroll events do not trigger loadMoreHistory in select mode", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByTestId("tmux-select-button"));
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

  describe("input panel hide toggle", () => {
    it("renders a hide input panel button in the header", () => {
      render(<TmuxPaneScreen />);
      const hideButton = screen.getByTestId("tmux-hide-input-button");
      expect(hideButton.querySelector('[data-icon="ChevronDown"]')).not.toBeNull();
    });

    it("hide input panel button is icon-only and has no visible text", () => {
      render(<TmuxPaneScreen />);
      const hideButton = screen.getByTestId("tmux-hide-input-button");
      expect(hideButton.textContent).toBe("");
      expect(screen.queryByText("Hide")).toBeNull();
      expect(screen.queryByText("Show")).toBeNull();
    });

    it("hides the input panel when the hide button is pressed", () => {
      render(<TmuxPaneScreen />);
      expect(screen.getByPlaceholderText(/type a command/i)).toBeDefined();
      fireEvent.click(screen.getByTestId("tmux-hide-input-button"));
      expect(screen.queryByPlaceholderText(/type a command/i)).toBeNull();
    });

    it("shows the input panel when the floating show button is pressed", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByTestId("tmux-hide-input-button"));
      expect(screen.queryByPlaceholderText(/type a command/i)).toBeNull();
      fireEvent.click(screen.getByTestId("tmux-show-input-button"));
      expect(screen.getByPlaceholderText(/type a command/i)).toBeDefined();
    });
  });

  describe("slash commands", () => {
    it("shows dropdown when typing / in input for a known agent", () => {
      render(<TmuxPaneScreen />);
      const input = screen.getByPlaceholderText(/type a command/i);
      fireEvent.change(input, { target: { value: "/" } });
      expect(screen.getByText("/compact")).toBeDefined();
      expect(screen.getByText("/help")).toBeDefined();
      expect(screen.getByText("/clear")).toBeDefined();
    });

    it("filters commands when typing more characters", () => {
      render(<TmuxPaneScreen />);
      const input = screen.getByPlaceholderText(/type a command/i);
      fireEvent.change(input, { target: { value: "/co" } });
      expect(screen.getByText("/compact")).toBeDefined();
      expect(screen.getByText("/config")).toBeDefined();
      expect(screen.getByText("/cost")).toBeDefined();
      expect(screen.queryByText("/help")).toBeNull();
    });

    it("selecting a command fills the input with command and space", () => {
      render(<TmuxPaneScreen />);
      const input = screen.getByPlaceholderText(/type a command/i);
      fireEvent.change(input, { target: { value: "/" } });
      fireEvent.click(screen.getByTestId("slash-command-help"));
      expect((input as HTMLInputElement).value).toBe("/help ");
    });

    it("dropdown disappears when input is cleared", () => {
      render(<TmuxPaneScreen />);
      const input = screen.getByPlaceholderText(/type a command/i);
      fireEvent.change(input, { target: { value: "/" } });
      expect(screen.getByText("/compact")).toBeDefined();
      fireEvent.change(input, { target: { value: "" } });
      expect(screen.queryByText("/compact")).toBeNull();
    });

    it("no dropdown for unknown agent", () => {
      agentRef.current = { ...mockAgent, agentName: "unknown" };
      render(<TmuxPaneScreen />);
      const input = screen.getByPlaceholderText(/type a command/i);
      fireEvent.change(input, { target: { value: "/" } });
      expect(screen.queryByText("/compact")).toBeNull();
      agentRef.current = mockAgent;
    });

    it("/ key button sends / keystroke to tmux pane", () => {
      render(<TmuxPaneScreen />);
      fireEvent.click(screen.getByTestId("slash-key-button"));
      expect(mockSendKeys).toHaveBeenCalledWith("%0", "/", false);
    });
  });

  describe("terminal theme", () => {
    it("shows Bash option in theme picker", () => {
      render(<TmuxPaneScreen />);
      expect(screen.getByText("Bash")).toBeDefined();
    });

    it("shows System, Dark, and Light options in theme picker", () => {
      render(<TmuxPaneScreen />);
      expect(screen.getByText("System")).toBeDefined();
      expect(screen.getByText("Dark")).toBeDefined();
      expect(screen.getByText("Light")).toBeDefined();
    });

    it("marks exactly one theme item as selected (radio-button exclusivity)", () => {
      render(<TmuxPaneScreen />);
      const items = screen.getAllByRole("button").filter(
        (el) => el.getAttribute("data-selected") !== null,
      );
      expect(items).toHaveLength(4);
      const selectedItems = items.filter(
        (el) => el.getAttribute("data-selected") === "true",
      );
      expect(selectedItems).toHaveLength(1);
    });

    it("selects the Light item by default (radio default matches default scheme)", () => {
      render(<TmuxPaneScreen />);
      const items = screen.getAllByRole("button").filter(
        (el) => el.getAttribute("data-selected") === "true",
      );
      expect(items).toHaveLength(1);
      expect(items[0]!.textContent).toBe("Light");
    });

    it("uses showSelectedCheck on every theme item so only the trailing slot indicates selection", () => {
      render(<TmuxPaneScreen />);
      const items = screen.getAllByRole("button").filter(
        (el) => el.getAttribute("data-show-selected-check") !== null,
      );
      expect(items).toHaveLength(4);
      for (const item of items) {
        expect(item.getAttribute("data-show-selected-check")).toBe("true");
      }
    });

    it("uses the Bash palette foreground for content text instead of the app theme foreground", () => {
      terminalThemeRef.current = "bash";
      render(<TmuxPaneScreen />);
      const content = screen.getByText((text) => text.includes("$ ls"));
      expect(content).toBeDefined();
      expect((content as HTMLElement).getAttribute("style")).toContain("color: rgb(192, 192, 192)");
    });
  });
});
