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
      surface2: "#333",
      primary: "#0af",
      border: "#333",
      destructive: "#f00",
      terminal: {
        background: "#000",
        foreground: "#fff",
        cursor: "#fff",
        cursorAccent: "#000",
        selectionBackground: "#fff",
        selectionForeground: "#000",
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
    Send: icon("Send"),
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

const { contentRef, autoRefreshRef, paneColsRef, hookArgsRef, mockEmulatorHandle, emulatorPropsRef } = vi.hoisted(() => ({
  contentRef: { current: "$ ls\nfile1.txt\n$ _" },
  autoRefreshRef: { current: true },
  paneColsRef: { current: null as number | null },
  hookArgsRef: { current: { cols: undefined as number | undefined } },
  mockEmulatorHandle: {
    writeOutput: vi.fn(),
    clear: vi.fn(),
  },
  emulatorPropsRef: {
    current: {
      forceCols: undefined as number | undefined,
      allowHorizontalScroll: false,
    },
  },
}));

vi.mock("@/hooks/use-tmux-capture-pane", () => ({
  useTmuxCapturePane: (
    _serverId: string,
    _paneId: string,
    _enabled: boolean,
    cols?: number,
  ) => {
    hookArgsRef.current.cols = cols;
    return {
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
      paneCols: paneColsRef.current,
    };
  },
}));

vi.mock("@/components/terminal-emulator", () => ({
  default: function MockTerminalEmulator({
    onResize,
    snapshotText,
    forceCols,
    allowHorizontalScroll,
  }: {
    onResize?: (size: { rows: number; cols: number }) => void;
    snapshotText?: string;
    forceCols?: number;
    allowHorizontalScroll?: boolean;
  }) {
    emulatorPropsRef.current = {
      forceCols,
      allowHorizontalScroll: allowHorizontalScroll ?? false,
    };

    React.useEffect(() => {
      onResize?.({ rows: 24, cols: 80 });
    }, [onResize]);

    React.useEffect(() => {
      if (!snapshotText) return;
      mockEmulatorHandle.clear();
      mockEmulatorHandle.writeOutput(snapshotText);
    }, [snapshotText]);

    return React.createElement("div", { "data-testid": "tmux-xterm-surface" }, "xterm");
  },
}));

vi.mock("@/utils/to-xterm-theme", () => ({
  toXtermTheme: () => ({}),
}));

vi.mock("@/utils/tmux-rpc", () => ({
  withLiveTmuxClient: async (_serverId: string, fn: (client: unknown) => Promise<unknown>) => {
    const client = { tmuxSendKeys: mockSendKeys };
    return fn(client);
  },
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

import { TmuxPaneXtermScreen } from "./tmux-pane-xterm-screen";

describe("TmuxPaneXtermScreen", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    contentRef.current = "$ ls\nfile1.txt\n$ _";
    autoRefreshRef.current = true;
    paneColsRef.current = null;
    hookArgsRef.current.cols = undefined;
    emulatorPropsRef.current = { forceCols: undefined, allowHorizontalScroll: false };
  });

  it("renders the xterm surface", () => {
    render(<TmuxPaneXtermScreen />);
    expect(screen.getByTestId("tmux-xterm-surface")).toBeDefined();
  });

  it("shows empty state when no agent is selected", () => {
    agentRef.current = null;
    render(<TmuxPaneXtermScreen />);
    expect(screen.getByText(/no agent selected/i)).toBeDefined();
    agentRef.current = mockAgent;
  });

  it("writes new content to the emulator when content changes", () => {
    const { rerender } = render(<TmuxPaneXtermScreen />);
    expect(mockEmulatorHandle.writeOutput).toHaveBeenCalledWith(contentRef.current);
    expect(mockEmulatorHandle.clear).toHaveBeenCalled();

    mockEmulatorHandle.writeOutput.mockClear();
    mockEmulatorHandle.clear.mockClear();
    contentRef.current = "$ pwd\n/home\n$ _";
    rerender(<TmuxPaneXtermScreen />);

    expect(mockEmulatorHandle.clear).toHaveBeenCalled();
    expect(mockEmulatorHandle.writeOutput).toHaveBeenCalledWith(contentRef.current);
  });

  it("passes measured cols to useTmuxCapturePane", () => {
    render(<TmuxPaneXtermScreen />);
    expect(hookArgsRef.current.cols).toBe(80);
  });

  it("sends typed input with Enter when the send button is pressed", async () => {
    render(<TmuxPaneXtermScreen />);
    const input = screen.getByTestId("tmux-xterm-input");
    fireEvent.change(input, { target: { value: "hello world" } });
    fireEvent.click(screen.getByTestId("tmux-xterm-send-button"));

    await vi.waitFor(() => {
      expect(mockSendKeys).toHaveBeenCalledWith("%0", "hello world", true);
    });
  });

  it("sends a virtual key when a key button is pressed", async () => {
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-key-Enter"));

    await vi.waitFor(() => {
      expect(mockSendKeys).toHaveBeenCalledWith("%0", "Enter", false);
    });
  });

  it("refetches content after sending input", async () => {
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-key-Enter"));

    await vi.waitFor(() => {
      expect(mockRefetch).toHaveBeenCalled();
    });
  });

  it("toggles auto refresh off when the Auto toggle is pressed", () => {
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByText("Auto"));
    expect(mockSetAutoRefresh).toHaveBeenCalledWith(false);
  });

  it("shows a manual refresh button when auto refresh is off", () => {
    autoRefreshRef.current = false;
    render(<TmuxPaneXtermScreen />);
    expect(screen.getByTestId("tmux-xterm-refresh-button")).toBeDefined();
  });

  it("calls refetch when the manual refresh button is pressed", () => {
    autoRefreshRef.current = false;
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-refresh-button"));
    expect(mockRefetch).toHaveBeenCalled();
  });

  it("hides the input panel when the hide button is pressed", () => {
    render(<TmuxPaneXtermScreen />);
    expect(screen.getByTestId("tmux-xterm-input")).toBeDefined();
    fireEvent.click(screen.getByTestId("tmux-xterm-hide-input-button"));
    expect(screen.queryByTestId("tmux-xterm-input")).toBeNull();
  });

  it("shows the input panel when the floating show button is pressed", () => {
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-hide-input-button"));
    expect(screen.queryByTestId("tmux-xterm-input")).toBeNull();
    fireEvent.click(screen.getByTestId("tmux-xterm-show-input-button"));
    expect(screen.getByTestId("tmux-xterm-input")).toBeDefined();
  });

  it("loads more history when the History button is pressed", () => {
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-load-more-button"));
    expect(mockLoadMoreHistory).toHaveBeenCalled();
  });

  it("toggles to original mode when Width button is pressed", () => {
    render(<TmuxPaneXtermScreen />);
    const button = screen.getByTestId("tmux-xterm-width-toggle-button");
    expect(button.textContent).toBe("Width");
    fireEvent.click(button);
    expect(button.textContent).toBe("Fit");
  });

  it("passes forceCols to TerminalEmulator in original mode", () => {
    paneColsRef.current = 120;
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-width-toggle-button"));
    expect(emulatorPropsRef.current.forceCols).toBe(120);
  });

  it("passes allowHorizontalScroll=true in original mode", () => {
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-width-toggle-button"));
    expect(emulatorPropsRef.current.allowHorizontalScroll).toBe(true);
  });

  it("passes allowHorizontalScroll=false in fit mode", () => {
    render(<TmuxPaneXtermScreen />);
    expect(emulatorPropsRef.current.allowHorizontalScroll).toBe(false);
  });

  it("omits cols argument in original mode", () => {
    render(<TmuxPaneXtermScreen />);
    expect(hookArgsRef.current.cols).toBe(80);
    fireEvent.click(screen.getByTestId("tmux-xterm-width-toggle-button"));
    expect(hookArgsRef.current.cols).toBeUndefined();
  });

  it("restores cols argument when switching back to fit mode", () => {
    render(<TmuxPaneXtermScreen />);
    fireEvent.click(screen.getByTestId("tmux-xterm-width-toggle-button"));
    expect(hookArgsRef.current.cols).toBeUndefined();
    fireEvent.click(screen.getByTestId("tmux-xterm-width-toggle-button"));
    expect(hookArgsRef.current.cols).toBe(80);
  });
});
