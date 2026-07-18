/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MutableDaemonConfig } from "@server/shared/messages";
import { ScheduleAssistantPanel } from "./schedule-assistant-panel";
import { useScheduleAssistantStore } from "@/stores/schedule-assistant-store";

const { mockRouter, mockSend, daemonConfigResult, theme } = vi.hoisted(() => ({
  mockRouter: { push: vi.fn() },
  mockSend: vi.fn(),
  daemonConfigResult: {
    current: null as { config: MutableDaemonConfig | null; isLoading: boolean } | null,
  },
  theme: {
    spacing: { 1: 4, 2: 8, 3: 12, 4: 16, 6: 24 },
    fontSize: { xs: 12, sm: 14, base: 16, lg: 18 },
    fontWeight: { medium: "500" },
    colors: {
      surface0: "#111",
      surface1: "#222",
      surface2: "#333",
      surface3: "#444",
      foreground: "#fff",
      foregroundMuted: "#999",
      border: "#555",
      accent: "#0a84ff",
      palette: {
        green: { 400: "#4ade80" },
        blue: { 400: "#60a5fa" },
        amber: { 500: "#f59e0b" },
        red: { 500: "#ef4444" },
      },
    },
    borderRadius: { md: 6, lg: 8, full: 9999 },
    borderWidth: { 1: 1 },
  },
}));

vi.mock("react-native", () => ({
  Platform: { OS: "web" },
  View: ({ children, testID, ...props }: React.PropsWithChildren<{ testID?: string } & Record<string, unknown>>) =>
    React.createElement("div", { "data-testid": testID, ...props }, children),
  Text: ({ children, testID, numberOfLines, ...props }: React.PropsWithChildren<{ testID?: string; numberOfLines?: number } & Record<string, unknown>>) =>
    React.createElement("span", { "data-testid": testID, ...props }, children),
  Pressable: ({ children, onPress, testID, disabled, ...props }: React.PropsWithChildren<{ onPress?: () => void; testID?: string; disabled?: boolean } & Record<string, unknown>>) =>
    React.createElement("button", { type: "button", onClick: onPress, disabled, "data-testid": testID, ...props }, children),
  ScrollView: ({ children, testID, contentContainerStyle, keyboardShouldPersistTaps, showsVerticalScrollIndicator, ...props }: React.PropsWithChildren<{ testID?: string; contentContainerStyle?: unknown; keyboardShouldPersistTaps?: string; showsVerticalScrollIndicator?: boolean } & Record<string, unknown>>) =>
    React.createElement("div", { "data-testid": testID, ...props }, children),
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("expo-router", () => ({
  router: mockRouter,
}));

vi.mock("lucide-react-native", () => ({
  Send: () => React.createElement("span", { "data-icon": "Send" }),
}));

vi.mock("@/components/adaptive-modal-sheet", () => ({
  AdaptiveModalSheet: ({
    children,
    visible,
    subtitle,
  }: {
    children: React.ReactNode;
    visible: boolean;
    subtitle?: React.ReactNode;
  }) =>
    visible
      ? React.createElement("div", { "data-testid": "modal-sheet" }, subtitle, children)
      : null,
  AdaptiveTextInput: ({
    value,
    onChangeText,
    placeholder,
    testID,
  }: {
    value: string;
    onChangeText?: (text: string) => void;
    placeholder?: string;
    testID?: string;
  }) =>
    React.createElement("input", {
      "data-testid": testID,
      placeholder,
      value,
      onChange: (e: React.ChangeEvent<HTMLInputElement>) => onChangeText?.(e.target.value),
    }),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
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

vi.mock("@/hooks/use-schedule-assist", () => ({
  useScheduleAssist: () => ({ send: mockSend, isSending: false }),
}));

vi.mock("@/hooks/use-proposal-confirm", () => ({
  useProposalConfirm: () => ({ confirmProposal: vi.fn() }),
}));

vi.mock("@/hooks/use-daemon-config", () => ({
  useDaemonConfig: () => {
    if (!daemonConfigResult.current) {
      throw new Error("Expected daemon config result");
    }
    return daemonConfigResult.current;
  },
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => null,
  useHostRuntimeIsConnected: () => true,
  useHosts: () => [{ serverId: "server-1", label: "Workstation" }],
}));

vi.mock("@/hooks/use-schedule-inspect", () => ({
  useScheduleInspect: () => ({ schedule: null, isLoading: false }),
}));

vi.mock("@/components/schedule-create-modal", () => ({
  ScheduleCreateModal: ({ visible }: { visible: boolean }) =>
    visible ? React.createElement("div", { "data-testid": "schedule-create-modal" }) : null,
}));

vi.mock("@/components/schedule-edit-modal", () => ({
  ScheduleEditModal: ({ visible }: { visible: boolean }) =>
    visible ? React.createElement("div", { "data-testid": "schedule-edit-modal" }) : null,
}));

function configWithProvider(): MutableDaemonConfig {
  return {
    mcp: { injectIntoAgents: false },
    providers: {},
    llmProviders: [
      {
        id: "openai",
        enabled: true,
        baseURL: "https://api.openai.com/v1",
        apiKey: "sk-test",
        models: [{ id: "gpt-5", isDefault: true }],
      },
    ],
    tmuxAgentNames: [],
  } as MutableDaemonConfig;
}

function configWithoutProvider(): MutableDaemonConfig {
  return {
    mcp: { injectIntoAgents: false },
    providers: {},
    llmProviders: [],
    tmuxAgentNames: [],
  } as MutableDaemonConfig;
}

describe("ScheduleAssistantPanel", () => {
  let container: HTMLElement | null = null;
  let root: Root | null = null;

  beforeEach(() => {
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    useScheduleAssistantStore.setState({ threads: {} });
    daemonConfigResult.current = { config: configWithProvider(), isLoading: false };
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
    daemonConfigResult.current = null;
    mockRouter.push.mockReset();
    mockSend.mockReset();
    vi.unstubAllGlobals();
  });

  function renderPanel(props: Partial<React.ComponentProps<typeof ScheduleAssistantPanel>> = {}) {
    act(() => {
      root?.render(
        <ScheduleAssistantPanel
          visible
          onClose={vi.fn()}
          serverId="server-1"
          {...props}
        />,
      );
    });
  }

  function composerInput(): HTMLInputElement | null {
    return container?.querySelector('[data-testid="assistant-composer-input"]') ?? null;
  }

  it("shows suggestion chips in the empty state and fills the composer on tap", () => {
    renderPanel();

    const chips = container?.querySelectorAll('[data-testid="assistant-suggestion-chip"]');
    expect(chips?.length).toBe(3);
    expect(container?.textContent).toContain(
      "Every morning at 9, run the daily standup summary",
    );
    expect(container?.textContent).toContain("Pause the disk cleanup");
    expect(container?.textContent).toContain("What runs this week?");

    act(() => {
      chips?.[0]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    expect(composerInput()?.value).toBe("Every morning at 9, run the daily standup summary");
  });

  it("shows the host and llm provider indicator", () => {
    useScheduleAssistantStore.getState().setProviderInfo("server-1", {
      llmProvider: "openai",
      model: "gpt-5",
    });

    renderPanel();

    expect(container?.textContent).toContain("Workstation");
    expect(container?.textContent).toContain("openai · gpt-5");
  });

  it("renders a setup card with a settings deep link when no provider is configured", () => {
    daemonConfigResult.current = { config: configWithoutProvider(), isLoading: false };

    renderPanel();

    expect(container?.textContent).toContain("LLM provider");
    const settingsButton = container?.querySelector('[data-testid="assistant-setup-settings"]');
    expect(settingsButton).not.toBeNull();
    expect(container?.querySelectorAll('[data-testid="assistant-suggestion-chip"]')).toHaveLength(0);

    act(() => {
      settingsButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    expect(mockRouter.push).toHaveBeenCalledWith("/settings/general");
    expect((composerInput() as HTMLInputElement | null)).not.toBeNull();
  });

  it("does not show the setup card while config is still loading", () => {
    daemonConfigResult.current = { config: null, isLoading: true };

    renderPanel();

    expect(container?.querySelector('[data-testid="assistant-setup-settings"]')).toBeNull();
    expect(container?.querySelectorAll('[data-testid="assistant-suggestion-chip"]')).toHaveLength(3);
  });

  it("sends the composer text through useScheduleAssist", () => {
    renderPanel();

    const input = composerInput();
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
      setter?.call(input, "every day at 9 summarize tests");
      input?.dispatchEvent(new Event("change", { bubbles: true }));
    });

    act(() => {
      container
        ?.querySelector('[data-testid="assistant-composer-send"]')
        ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    expect(mockSend).toHaveBeenCalledWith("every day at 9 summarize tests");
  });

  it("shows a pending bubble while sending", () => {
    useScheduleAssistantStore.getState().addMessage("server-1", {
      id: "m1",
      role: "user",
      kind: "text",
      text: "hello",
      createdAt: Date.now(),
    });
    useScheduleAssistantStore.getState().setSending("server-1", true);

    renderPanel();

    expect(container?.querySelector('[data-testid="assistant-pending"]')).not.toBeNull();
    expect(container?.textContent).toContain("Thinking");
  });

  it("shows a host chip row when multiple hosts are available", () => {
    renderPanel({
      availableHosts: [
        { serverId: "server-1", label: "Workstation" },
        { serverId: "server-2", label: "Laptop" },
      ],
    });

    const chips = container?.querySelectorAll('[data-testid="assistant-host-chip"]');
    expect(chips?.length).toBe(2);
    expect(container?.textContent).toContain("Workstation");
    expect(container?.textContent).toContain("Laptop");
  });
});
