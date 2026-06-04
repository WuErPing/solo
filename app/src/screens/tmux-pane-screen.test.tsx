/**
 * @vitest-environment jsdom
 */
import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const { mockSendKeys, mockRefetch, mockTheme } = vi.hoisted(() => ({
  mockSendKeys: vi.fn(() => Promise.resolve({})),
  mockRefetch: vi.fn(),
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
  };
});

vi.mock("expo-router", () => ({
  router: {
    back: vi.fn(),
    push: vi.fn(),
  },
}));

vi.mock("@/components/headers/back-header", () => ({
  BackHeader: ({ title, onBack }: { title: string; onBack?: () => void }) =>
    React.createElement("div", { "data-testid": "back-header" },
      React.createElement("button", { onClick: onBack, "aria-label": "Back" }, "Back"),
      title,
    ),
}));

vi.mock("@/components/ansi-text-renderer", () => ({
  AnsiTextContent: ({ segments, style }: { segments: { text: string }[]; style?: unknown }) =>
    React.createElement("span", { style }, segments.map((s: { text: string }) => s.text).join("")),
}));

vi.mock("@/utils/ansi-parser", () => ({
  parseAnsi: (input: string) => [{ text: input, style: {} }],
}));


vi.mock("@/hooks/use-tmux-capture-pane", () => ({
  useTmuxCapturePane: () => ({
    content: "$ ls\nfile1.txt\nfile2.txt\n$ _",
    isLoading: false,
    error: null,
    refetch: mockRefetch,
  }),
}));

vi.mock("@/hooks/use-tmux-theme", () => ({
  useTmuxTheme: () => ({
    theme: null,
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => ({
    tmuxSendKeys: mockSendKeys,
  }),
  getHostRuntimeStore: () => ({
    getClient: () => ({
      tmuxSendKeys: mockSendKeys,
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
});
