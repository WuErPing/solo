/**
 * @vitest-environment jsdom
 */
import React from "react";
import { createRoot, type Root } from "react-dom/client";
import { act, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LLMProviderConfig } from "@server/shared/messages";

const { theme, configState, patchConfigMock } = vi.hoisted(() => ({
  theme: {
    spacing: { 1: 4, "1.5": 6, 2: 8, 3: 12, 4: 16, 6: 24 },
    iconSize: { sm: 14, md: 20 },
    fontSize: { xs: 11, sm: 13, base: 15 },
    fontWeight: { normal: "400", medium: "500" },
    borderRadius: { md: 6, lg: 8 },
    opacity: { 50: 0.5 },
    colors: {
      surface1: "#111",
      surface2: "#222",
      surface3: "#333",
      foreground: "#fff",
      foregroundMuted: "#aaa",
      border: "#555",
      accent: "#0a84ff",
      destructive: "#ff0000",
      statusSuccess: "#00ff00",
      statusWarning: "#ff9500",
      statusDanger: "#ff0000",
      palette: { red: { 300: "#ff6b6b" }, white: "#fff" },
    },
  },
  configState: {
    config: null as { llmProviders?: LLMProviderConfig[] } | null,
  },
  patchConfigMock: vi.fn<(patch: { llmProviders?: LLMProviderConfig[] }) => Promise<void>>(
    async () => undefined,
  ),
}));

vi.mock("react-native", () => ({
  View: ({ children, testID }: { children?: React.ReactNode; testID?: string }) =>
    React.createElement("div", { "data-testid": testID }, children),
  Text: ({ children }: { children?: React.ReactNode }) =>
    React.createElement("span", null, children),
  TextInput: ({
    value,
    onChangeText,
    placeholder,
    testID,
    secureTextEntry,
  }: {
    value?: string;
    onChangeText?: (text: string) => void;
    placeholder?: string;
    testID?: string;
    secureTextEntry?: boolean;
  }) =>
    React.createElement("input", {
      "data-testid": testID,
      placeholder,
      value: value ?? "",
      type: secureTextEntry ? "password" : "text",
      onChange: (event: React.ChangeEvent<HTMLInputElement>) => {
        onChangeText?.(event.target.value);
      },
    }),
  Pressable: ({
    children,
    onPress,
    accessibilityRole,
    accessibilityLabel,
    disabled,
    testID,
  }: {
    children?:
      | React.ReactNode
      | ((state: { pressed: boolean; hovered: boolean }) => React.ReactNode);
    onPress?: (event: React.MouseEvent) => void;
    accessibilityRole?: string;
    accessibilityLabel?: string;
    disabled?: boolean;
    testID?: string;
  }) =>
    React.createElement(
      "div",
      {
        role: accessibilityRole,
        "aria-label": accessibilityLabel,
        "aria-disabled": disabled ? "true" : undefined,
        "data-testid": testID,
        onClick: disabled ? undefined : onPress,
      },
      typeof children === "function" ? children({ pressed: false, hovered: false }) : children,
    ),
  Switch: ({
    value,
    onValueChange,
    testID,
  }: {
    value: boolean;
    onValueChange?: (next: boolean) => void;
    testID?: string;
  }) =>
    React.createElement("div", {
      role: "switch",
      "aria-checked": value ? "true" : "false",
      "data-testid": testID,
      onClick: () => onValueChange?.(!value),
    }),
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) =>
      typeof factory === "function" ? (factory as (t: typeof theme) => unknown)(theme) : factory,
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("lucide-react-native", () => {
  const icon = (name: string) => () => React.createElement("span", { "data-icon": name });
  return {
    Plus: icon("Plus"),
    Pencil: icon("Pencil"),
    Trash2: icon("Trash2"),
    RotateCw: icon("RotateCw"),
  };
});

vi.mock("@/components/adaptive-modal-sheet", () => ({
  AdaptiveModalSheet: ({
    children,
    visible,
  }: {
    children?: React.ReactNode;
    visible?: boolean;
  }) => (visible ? React.createElement("div", { "data-testid": "llm-provider-modal" }, children) : null),
  AdaptiveTextInput: ({
    value,
    onChangeText,
    placeholder,
    testID,
    secureTextEntry,
    editable,
  }: {
    value?: string;
    onChangeText?: (text: string) => void;
    placeholder?: string;
    testID?: string;
    secureTextEntry?: boolean;
    editable?: boolean;
  }) =>
    React.createElement("input", {
      "data-testid": testID,
      placeholder,
      value: value ?? "",
      type: secureTextEntry ? "password" : "text",
      disabled: editable === false,
      onChange: (event: React.ChangeEvent<HTMLInputElement>) => {
        onChangeText?.(event.target.value);
      },
    }),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onPress,
    disabled,
    testID,
    variant,
  }: {
    children?: React.ReactNode;
    onPress?: () => void;
    disabled?: boolean;
    testID?: string;
    variant?: string;
  }) =>
    React.createElement(
      "button",
      { type: "button", onClick: disabled ? undefined : onPress, "data-testid": testID, "data-variant": variant },
      children,
    ),
}));

vi.mock("@/hooks/use-daemon-config", () => ({
  useDaemonConfig: () => ({
    config: configState.config,
    isLoading: false,
    patchConfig: patchConfigMock,
  }),
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeIsConnected: () => true,
}));

vi.mock("@/utils/confirm-dialog", () => ({
  confirmDialog: vi.fn(async () => true),
}));

import { LlmProvidersSection } from "./llm-providers-section";

const openAIProvider: LLMProviderConfig = {
  id: "openai",
  label: "OpenAI",
  description: "OpenAI API",
  enabled: true,
  baseURL: "https://api.openai.com/v1",
  apiKey: "sk-test",
  models: [
    { id: "gpt-5", label: "GPT-5" },
    { id: "gpt-5-mini", label: "GPT-5 Mini" },
  ],
};

describe("LlmProvidersSection", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;

  beforeEach(() => {
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);

    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);

    configState.config = null;
    patchConfigMock.mockReset();
    patchConfigMock.mockResolvedValue(undefined);
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
    vi.unstubAllGlobals();
  });

  function render(): void {
    act(() => {
      root?.render(<LlmProvidersSection serverId="server-1" />);
    });
  }

  it("renders an empty state when no LLM providers are configured", () => {
    render();

    expect(container?.textContent).toContain("No LLM providers configured");
  });

  it("shows the add-provider form when the add button is pressed", () => {
    render();

    expect(container?.querySelector('[data-testid="llm-provider-form"]')).toBeNull();

    const addButton = container?.querySelector<HTMLElement>('[data-testid="add-llm-provider-button"]');
    expect(addButton).not.toBeNull();

    act(() => {
      addButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    expect(container?.querySelector('[data-testid="llm-provider-form"]')).not.toBeNull();
  });

  it("creates a new LLM provider through patchConfig when the form is submitted", async () => {
    render();

    const addButton = container?.querySelector<HTMLElement>('[data-testid="add-llm-provider-button"]');
    act(() => {
      addButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    const idInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-id"]');
    const labelInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-label"]');
    const baseURLInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-baseurl"]');
    const apiKeyInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-apikey"]');

    act(() => {
      fireEvent.change(idInput!, { target: { value: "anthropic" } });
    });
    act(() => {
      fireEvent.change(labelInput!, { target: { value: "Anthropic" } });
    });
    act(() => {
      fireEvent.change(baseURLInput!, { target: { value: "https://api.anthropic.com/v1" } });
    });
    act(() => {
      fireEvent.change(apiKeyInput!, { target: { value: "sk-ant" } });
    });

    const submitButton = container?.querySelector<HTMLElement>('[data-testid="llm-provider-submit"]');
    await act(async () => {
      submitButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(patchConfigMock).toHaveBeenCalledTimes(1);
    expect(patchConfigMock).toHaveBeenCalledWith({
      llmProviders: [
        {
          id: "anthropic",
          label: "Anthropic",
          description: "",
          enabled: true,
          baseURL: "https://api.anthropic.com/v1",
          apiKey: "sk-ant",
          models: [],
        },
      ],
    });
  });

  it("saves comma-separated models with the first marked default when creating a provider", async () => {
    render();

    const addButton = container?.querySelector<HTMLElement>('[data-testid="add-llm-provider-button"]');
    act(() => {
      addButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    const idInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-id"]');
    const baseURLInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-baseurl"]');
    const apiKeyInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-apikey"]');
    const modelsInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-models"]');

    act(() => {
      fireEvent.change(idInput!, { target: { value: "deepseek" } });
    });
    act(() => {
      fireEvent.change(baseURLInput!, { target: { value: "https://api.deepseek.com" } });
    });
    act(() => {
      fireEvent.change(apiKeyInput!, { target: { value: "sk-test" } });
    });
    act(() => {
      fireEvent.change(modelsInput!, { target: { value: "deepseek-chat, deepseek-reasoner" } });
    });

    const submitButton = container?.querySelector<HTMLElement>('[data-testid="llm-provider-submit"]');
    await act(async () => {
      submitButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(patchConfigMock).toHaveBeenCalledTimes(1);
    const call = patchConfigMock.mock.calls[0][0] as unknown as { llmProviders: LLMProviderConfig[] };
    expect(call.llmProviders[0].models).toEqual([
      { id: "deepseek-chat", isDefault: true },
      { id: "deepseek-reasoner" },
    ]);
  });

  it("prefills models when editing and preserves an existing default marker", async () => {
    configState.config = {
      llmProviders: [
        {
          ...openAIProvider,
          models: [{ id: "gpt-5" }, { id: "gpt-5-mini", isDefault: true }],
        },
      ],
    };
    render();

    const editButton = container?.querySelector<HTMLElement>('[data-testid="edit-llm-provider-openai"]');
    act(() => {
      editButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    const modelsInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-models"]');
    expect(modelsInput?.value).toBe("gpt-5, gpt-5-mini");

    const submitButton = container?.querySelector<HTMLElement>('[data-testid="llm-provider-submit"]');
    await act(async () => {
      submitButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(patchConfigMock).toHaveBeenCalledTimes(1);
    const call = patchConfigMock.mock.calls[0][0] as unknown as { llmProviders: LLMProviderConfig[] };
    expect(call.llmProviders[0].models).toEqual([
      { id: "gpt-5" },
      { id: "gpt-5-mini", isDefault: true },
    ]);
  });

  it("moves the default marker to the first model when the previous default is removed", async () => {
    configState.config = {
      llmProviders: [
        {
          ...openAIProvider,
          models: [{ id: "gpt-5" }, { id: "gpt-5-mini", isDefault: true }],
        },
      ],
    };
    render();

    const editButton = container?.querySelector<HTMLElement>('[data-testid="edit-llm-provider-openai"]');
    act(() => {
      editButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    const modelsInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-models"]');
    act(() => {
      fireEvent.change(modelsInput!, { target: { value: "gpt-5-nano, gpt-5" } });
    });

    const submitButton = container?.querySelector<HTMLElement>('[data-testid="llm-provider-submit"]');
    await act(async () => {
      submitButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(patchConfigMock).toHaveBeenCalledTimes(1);
    const call = patchConfigMock.mock.calls[0][0] as unknown as { llmProviders: LLMProviderConfig[] };
    expect(call.llmProviders[0].models).toEqual([
      { id: "gpt-5-nano", isDefault: true },
      { id: "gpt-5" },
    ]);
  });

  it("renders existing providers and allows deletion", async () => {
    configState.config = { llmProviders: [openAIProvider] };
    render();

    expect(container?.textContent).toContain("OpenAI");
    expect(container?.textContent).toContain("2 models");

    const deleteButton = container?.querySelector<HTMLElement>('[data-testid="delete-llm-provider-openai"]');
    expect(deleteButton).not.toBeNull();

    await act(async () => {
      deleteButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(patchConfigMock).toHaveBeenCalledTimes(1);
    expect(patchConfigMock).toHaveBeenCalledWith({ llmProviders: [] });
  });

  it("allows editing an existing provider", async () => {
    configState.config = { llmProviders: [openAIProvider] };
    render();

    const editButton = container?.querySelector<HTMLElement>('[data-testid="edit-llm-provider-openai"]');
    expect(editButton).not.toBeNull();

    act(() => {
      editButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
    });

    const labelInput = container?.querySelector<HTMLInputElement>('[data-testid="llm-provider-label"]');
    expect(labelInput?.value).toBe("OpenAI");

    act(() => {
      fireEvent.change(labelInput!, { target: { value: "OpenAI Updated" } });
    });

    const submitButton = container?.querySelector<HTMLElement>('[data-testid="llm-provider-submit"]');
    await act(async () => {
      submitButton?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(patchConfigMock).toHaveBeenCalledTimes(1);
    const call = patchConfigMock.mock.calls[0][0] as unknown as { llmProviders: LLMProviderConfig[] };
    expect(call.llmProviders[0].label).toBe("OpenAI Updated");
    expect(call.llmProviders[0].id).toBe("openai");
  });

  it("toggles provider enabled state through patchConfig", async () => {
    configState.config = { llmProviders: [openAIProvider] };
    render();

    const switchEl = container?.querySelector<HTMLElement>('[data-testid="llm-provider-switch-openai"]');
    expect(switchEl?.getAttribute("aria-checked")).toBe("true");

    await act(async () => {
      switchEl?.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(patchConfigMock).toHaveBeenCalledTimes(1);
    const call = patchConfigMock.mock.calls[0][0] as unknown as { llmProviders: LLMProviderConfig[] };
    expect(call.llmProviders[0].enabled).toBe(false);
  });
});
