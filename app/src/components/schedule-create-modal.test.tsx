/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ScheduleCreateModal } from "./schedule-create-modal";

const { createScheduleResult, providersResult, theme } = vi.hoisted(() => ({
  createScheduleResult: {
    current: null as {
      createSchedule: ReturnType<typeof vi.fn>;
      isCreating: boolean;
    } | null,
  },
  providersResult: {
    current: null as {
      entries: { provider: string; label: string | null; description: string | null; status: string; enabled: boolean }[];
      isLoading: boolean;
    } | null,
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
        red: { 500: "#ef4444" },
      },
    },
    borderRadius: { md: 6, lg: 8, full: 9999 },
    borderWidth: { 1: 1 },
  },
}));

vi.mock("react-native", () => ({
  Platform: { OS: "web" },
  View: ({
    children,
    testID,
    ...props
  }: React.PropsWithChildren<{ testID?: string } & Record<string, unknown>>) =>
    React.createElement("div", { "data-testid": testID, ...props }, children),
  Text: ({
    children,
    testID,
    ...props
  }: React.PropsWithChildren<{ testID?: string } & Record<string, unknown>>) =>
    React.createElement("span", { "data-testid": testID, ...props }, children),
  Pressable: ({
    children,
    onPress,
    testID,
    ...props
  }: React.PropsWithChildren<{ onPress?: () => void; testID?: string } & Record<string, unknown>>) =>
    React.createElement("button", { type: "button", onClick: onPress, "data-testid": testID, ...props }, children),
  ScrollView: ({
    children,
    testID,
    ...props
  }: React.PropsWithChildren<{ testID?: string } & Record<string, unknown>>) =>
    React.createElement("div", { "data-testid": testID, ...props }, children),
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("@/components/adaptive-modal-sheet", () => ({
  AdaptiveModalSheet: ({ children, visible }: { children: React.ReactNode; visible: boolean }) =>
    visible ? React.createElement("div", { "data-testid": "modal-sheet" }, children) : null,
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
      "data-placeholder": placeholder,
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

vi.mock("@/components/ui/segmented-control", () => ({
  SegmentedControl: ({
    options,
    value,
    onValueChange,
    testID,
  }: {
    options: { value: string; label: string; testID?: string }[];
    value: string;
    onValueChange?: (value: string) => void;
    testID?: string;
  }) =>
    React.createElement(
      "div",
      { "data-testid": testID },
      options.map((option) =>
        React.createElement(
          "button",
          {
            key: option.value,
            type: "button",
            "data-testid": option.testID,
            "data-value": option.value,
            "data-selected": option.value === value,
            onClick: () => onValueChange?.(option.value),
          },
          option.label,
        ),
      ),
    ),
}));

vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => children,
  TooltipTrigger: ({ children }: { children: React.ReactNode }) => children,
  TooltipContent: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@/hooks/use-create-schedule", () => ({
  useCreateSchedule: () => {
    if (!createScheduleResult.current) {
      throw new Error("Expected create schedule result");
    }
    return createScheduleResult.current;
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

vi.mock("@/utils/cron-timezone", () => ({
  detectTimezone: () => "UTC",
  cronToUTC: (expr: string) => expr,
  cronFromUTC: (expr: string) => expr,
  describeCron: (expr: string) => expr,
}));

vi.mock("lucide-react-native", () => ({
  HelpCircle: () => React.createElement("span", { "data-icon": "HelpCircle" }),
}));

describe("ScheduleCreateModal", () => {
  let container: HTMLElement | null = null;
  let root: Root | null = null;

  beforeEach(() => {
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);

    providersResult.current = {
      entries: [{ provider: "claude", label: "Claude", description: "Anthropic", status: "ready", enabled: true }],
      isLoading: false,
    };
    createScheduleResult.current = {
      createSchedule: vi.fn(async () => ({
        id: "schedule-new",
        name: null,
        prompt: "Generate report",
        cadence: { type: "cron", expression: "0 9 * * *" },
        target: { type: "provider", providerId: "claude" },
        cwd: "/workspace/project",
        status: "active",
        createdAt: "2026-01-01T00:00:00.000Z",
        updatedAt: "2026-01-01T00:00:00.000Z",
        nextRunAt: null,
        lastRunAt: null,
        pausedAt: null,
        expiresAt: null,
        maxRuns: null,
      })),
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
    createScheduleResult.current = null;
    vi.unstubAllGlobals();
  });

  function renderModal() {
    act(() => {
      root?.render(<ScheduleCreateModal visible serverId="server-1" onClose={vi.fn()} />);
    });
  }

  function setPrompt(value: string) {
    const input = container?.querySelector('input[data-testid="schedule-create-prompt-input"]') as HTMLInputElement | undefined;
    act(() => {
      input?.dispatchEvent(new InputEvent("change", { bubbles: true }));
      const nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
      nativeInputValueSetter?.call(input, value);
      input?.dispatchEvent(new Event("change", { bubbles: true }));
    });
  }

  function setCwd(value: string) {
    const input = container?.querySelector('input[data-testid="schedule-create-cwd-input"]') as HTMLInputElement | undefined;
    act(() => {
      const nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
      nativeInputValueSetter?.call(input, value);
      input?.dispatchEvent(new Event("change", { bubbles: true }));
    });
  }

  function selectProviderCard(index = 0) {
    const cards = container?.querySelectorAll('[data-testid="schedule-create-provider-card"]');
    act(() => {
      cards?.[index]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
  }

  async function submit() {
    const button = container?.querySelector('[data-testid="schedule-create-submit-button"]');
    await act(async () => {
      button?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      // Let the async createSchedule promise and subsequent state updates flush.
      await Promise.resolve();
    });
  }

  it("renders provider cards", () => {
    renderModal();

    const providerCards = container?.querySelectorAll('[data-testid="schedule-create-provider-card"]');
    expect(providerCards?.length).toBe(1);
  });

  it("creates a provider schedule with schedule cwd", async () => {
    renderModal();

    selectProviderCard(0);
    setCwd("/workspace/project");
    setPrompt("Generate report");
    await submit();

    await vi.waitFor(() => {
      expect(createScheduleResult.current?.createSchedule).toHaveBeenCalled();
    });

    expect(createScheduleResult.current?.createSchedule).toHaveBeenCalledWith(
      expect.objectContaining({
        target: { type: "provider", providerId: "claude" },
        cwd: "/workspace/project",
        prompt: "Generate report",
      }),
    );
  });

  it("shows validation error when no provider chosen", async () => {
    renderModal();

    setPrompt("Generate report");
    await submit();

    const error = container?.querySelector('[data-testid="schedule-create-error"]');
    expect(error).not.toBeNull();
    expect(error?.textContent).toContain("provider");
    expect(createScheduleResult.current?.createSchedule).not.toHaveBeenCalled();
  });

  it("seeds fields from initialValues", () => {
    act(() => {
      root?.render(
        <ScheduleCreateModal
          visible
          serverId="server-1"
          onClose={vi.fn()}
          initialValues={{
            name: "Nightly",
            prompt: "Summarize the nightly test runs",
            cadence: { type: "cron", expression: "30 7 * * *" },
            target: { type: "provider", providerId: "claude" },
            cwd: "/work/backend",
            maxRuns: 10,
            expiresAt: "2026-08-31T00:00:00.000Z",
          }}
        />,
      );
    });

    const nameInput = container?.querySelector(
      'input[data-testid="schedule-create-name-input"]',
    ) as HTMLInputElement | null;
    expect(nameInput?.value).toBe("Nightly");

    const promptInput = container?.querySelector(
      'input[data-testid="schedule-create-prompt-input"]',
    ) as HTMLInputElement | null;
    expect(promptInput?.value).toBe("Summarize the nightly test runs");

    const cwdInput = container?.querySelector(
      'input[data-testid="schedule-create-cwd-input"]',
    ) as HTMLInputElement | null;
    expect(cwdInput?.value).toBe("/work/backend");

    // Cron expression appears in the local/UTC preview row
    expect(container?.textContent).toContain("30 7 * * *");

    // Provider target preselected
    const selectedCard = container?.querySelector(
      '[data-testid="schedule-create-provider-card"][data-selected="true"]',
    );
    expect(selectedCard).not.toBeNull();
  });

  it("leaves target unselected for non-provider initialValues targets", () => {
    act(() => {
      root?.render(
        <ScheduleCreateModal
          visible
          serverId="server-1"
          onClose={vi.fn()}
          initialValues={{
            prompt: "Summarize",
            cadence: { type: "every", everyMs: 900000 },
            target: { type: "agent", agentId: "agent-1" },
          }}
        />,
      );
    });

    const selectedCard = container?.querySelector(
      '[data-testid="schedule-create-provider-card"][data-selected="true"]',
    );
    expect(selectedCard).toBeNull();

    const intervalInput = container?.querySelector(
      'input[data-testid="schedule-create-interval-input"]',
    ) as HTMLInputElement | null;
    expect(intervalInput?.value).toBe("900000");
  });

  it("passes seeded maxRuns and expiresAt through on submit", async () => {
    act(() => {
      root?.render(
        <ScheduleCreateModal
          visible
          serverId="server-1"
          onClose={vi.fn()}
          initialValues={{
            name: "Nightly",
            prompt: "Summarize the nightly test runs",
            cadence: { type: "cron", expression: "0 9 * * *" },
            target: { type: "provider", providerId: "claude" },
            maxRuns: 10,
            expiresAt: "2026-08-31T00:00:00.000Z",
          }}
        />,
      );
    });

    await submit();

    await vi.waitFor(() => {
      expect(createScheduleResult.current?.createSchedule).toHaveBeenCalled();
    });

    expect(createScheduleResult.current?.createSchedule).toHaveBeenCalledWith(
      expect.objectContaining({
        maxRuns: 10,
        expiresAt: "2026-08-31T00:00:00.000Z",
      }),
    );
  });
});
