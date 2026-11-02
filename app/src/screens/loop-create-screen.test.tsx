/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LoopCreateScreen } from "./loop-create-screen";

const { createLoopResult, providersResult, theme } = vi.hoisted(() => ({
  createLoopResult: {
    current: null as {
      createLoop: ReturnType<typeof vi.fn>;
      isCreating: boolean;
    } | null,
  },
  providersResult: {
    current: null as {
      entries: {
        provider: string;
        label: string | null;
        description: string | null;
        status: string;
        enabled: boolean;
        modes: { id: string; label: string; description: string | null }[];
        defaultModeId: string | null;
      }[];
      isLoading: boolean;
    } | null,
  },
  theme: {
    spacing: { 1: 4, 2: 8, 3: 12, 4: 16 },
    fontSize: { sm: 14, base: 16 },
    colors: {
      surface0: "#111",
      surface1: "#222",
      surface2: "#333",
      surface3: "#444",
      foreground: "#fff",
      foregroundMuted: "#999",
      border: "#555",
      accent: "#0a84ff",
      palette: { red: { 500: "#ef4444" } },
    },
    borderRadius: { md: 6 },
    borderWidth: { 1: 1 },
  },
}));

vi.mock("react-native", () => ({
  View: ({ children, testID }: React.PropsWithChildren<{ testID?: string }>) =>
    React.createElement("div", { "data-testid": testID }, children),
  Text: ({ children, testID }: React.PropsWithChildren<{ testID?: string }>) =>
    React.createElement("span", { "data-testid": testID }, children),
  ScrollView: ({ children }: React.PropsWithChildren) => React.createElement("div", null, children),
  Pressable: ({
    children,
    onPress,
    disabled,
    testID,
  }: React.PropsWithChildren<{ onPress?: () => void; disabled?: boolean; testID?: string }>) =>
    React.createElement(
      "button",
      { type: "button", onClick: onPress, disabled, "data-testid": testID },
      children,
    ),
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("@/components/adaptive-modal-sheet", () => ({
  AdaptiveTextInput: ({
    value,
    onChangeText,
    placeholder,
  }: {
    value: string;
    onChangeText?: (text: string) => void;
    placeholder?: string;
  }) =>
    React.createElement("input", {
      "data-placeholder": placeholder,
      value,
      onChange: (event: React.ChangeEvent<HTMLInputElement>) => onChangeText?.(event.target.value),
    }),
}));

vi.mock("@/components/headers/back-header", () => ({
  BackHeader: ({ title }: { title: string }) => React.createElement("div", null, title),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onPress,
    disabled,
  }: React.PropsWithChildren<{ onPress?: () => void; disabled?: boolean }>) =>
    React.createElement("button", { type: "button", onClick: onPress, disabled }, children),
}));

vi.mock("@/hooks/use-create-loop", () => ({
  useCreateLoop: () => {
    if (!createLoopResult.current) {
      throw new Error("Expected create loop result");
    }
    return createLoopResult.current;
  },
}));

vi.mock("@/hooks/use-providers-snapshot", () => ({
  useProvidersSnapshot: () => {
    if (!providersResult.current) {
      throw new Error("Expected providers result");
    }
    return providersResult.current;
  },
}));

vi.mock("expo-router", () => ({
  router: { back: vi.fn(), replace: vi.fn() },
}));

vi.mock("lucide-react-native", () => ({
  ChevronDown: () => React.createElement("span", { "data-icon": "ChevronDown" }),
  ChevronRight: () => React.createElement("span", { "data-icon": "ChevronRight" }),
}));

describe("LoopCreateScreen", () => {
  let container: HTMLElement | null = null;
  let root: Root | null = null;

  beforeEach(() => {
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);

    providersResult.current = {
      entries: [
        {
          provider: "claude",
          label: "Claude",
          description: "Anthropic",
          status: "ready",
          enabled: true,
          modes: [{ id: "bypassPermissions", label: "Bypass permissions", description: null }],
          defaultModeId: "default",
        },
      ],
      isLoading: false,
    };
    createLoopResult.current = {
      createLoop: vi.fn(async () => ({ id: "loop-1" })),
      isCreating: false,
    };
  });

  afterEach(() => {
    if (root) {
      act(() => {
        root?.unmount();
      });
    }
    root = null;
    container?.remove();
    container = null;
    providersResult.current = null;
    createLoopResult.current = null;
    vi.unstubAllGlobals();
  });

  function renderScreen() {
    act(() => {
      root?.render(<LoopCreateScreen serverId="server-1" />);
    });
  }

  function setInput(placeholder: string, value: string) {
    const input = container?.querySelector(`input[data-placeholder="${placeholder}"]`) as HTMLInputElement | null;
    act(() => {
      const nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
      nativeInputValueSetter?.call(input, value);
      input?.dispatchEvent(new Event("change", { bubbles: true }));
    });
  }

  function clickButton(text: string, contains = false) {
    const button = Array.from(container?.querySelectorAll("button") ?? []).find((candidate) =>
      contains ? candidate.textContent?.includes(text) : candidate.textContent === text,
    );
    act(() => {
      button?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
  }

  async function submit() {
    await act(async () => {
      const button = Array.from(container?.querySelectorAll("button") ?? []).find(
        (candidate) => candidate.textContent === "Create Loop",
      );
      button?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });
  }

  it("rejects a whitespace-only prompt", async () => {
    renderScreen();

    setInput("e.g. Fix all test failures until CI passes", " \n\t ");
    await submit();

    expect(container?.textContent).toContain("Prompt is required");
    expect(createLoopResult.current?.createLoop).not.toHaveBeenCalled();
  });

  it("preserves custom agent settings when the section is collapsed", async () => {
    renderScreen();

    setInput("e.g. Fix all test failures until CI passes", "  Fix tests  ");
    setInput("e.g. /Users/me/project", "  /workspace/project  ");
    clickButton("Agent Template");
    clickButton("Claude", true);
    setInput("e.g. claude-3-opus (optional)", "  model-a  ");
    setInput("Optional instructions for the agent", "  Be careful  ");
    clickButton("Agent Template");
    await submit();

    await vi.waitFor(() => {
      expect(createLoopResult.current?.createLoop).toHaveBeenCalled();
    });
    expect(createLoopResult.current?.createLoop).toHaveBeenCalledWith(
      expect.objectContaining({
        prompt: "Fix tests",
        cwd: "/workspace/project",
        agentTemplate: {
          provider: "claude",
          cwd: "/workspace/project",
          model: "model-a",
          systemPrompt: "Be careful",
          modeId: "bypassPermissions",
        },
      }),
    );
  });
});
