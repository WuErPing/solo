/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ScheduleEditModal } from "./schedule-edit-modal";
import type { ScheduleSummary } from "@server/server/schedule/types";

const { mutationsResult, providersResult, theme } = vi.hoisted(() => ({
  mutationsResult: {
    current: null as {
      updateSchedule: ReturnType<typeof vi.fn>;
      isUpdating: (scheduleId: string) => boolean;
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

vi.mock("@/hooks/use-schedule-mutations", () => ({
  useScheduleMutations: () => {
    if (!mutationsResult.current) {
      throw new Error("Expected schedule mutations result");
    }
    return mutationsResult.current;
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

function makeSchedule(overrides: Partial<ScheduleSummary> = {}): ScheduleSummary {
  return {
    id: "schedule-1",
    name: "Daily Report",
    prompt: "Generate daily report",
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
    ...overrides,
  };
}

describe("ScheduleEditModal", () => {
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
        { provider: "claude", label: "Claude", description: "Anthropic", status: "ready", enabled: true },
        { provider: "codex", label: "Codex", description: "OpenAI", status: "ready", enabled: true },
      ],
      isLoading: false,
    };
    mutationsResult.current = {
      updateSchedule: vi.fn(async () => makeSchedule()),
      isUpdating: () => false,
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
    mutationsResult.current = null;
    vi.unstubAllGlobals();
  });

  function renderModal(schedule: ScheduleSummary | null = makeSchedule()) {
    act(() => {
      root?.render(<ScheduleEditModal visible serverId="server-1" onClose={vi.fn()} schedule={schedule} />);
    });
  }

  function selectProviderCard(index = 0) {
    const cards = container?.querySelectorAll('[data-testid="schedule-edit-provider-card"]');
    act(() => {
      cards?.[index]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
  }

  async function submit() {
    const button = container?.querySelector('[data-testid="schedule-edit-submit-button"]');
    await act(async () => {
      button?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });
  }

  it("renders provider cards", () => {
    renderModal();

    const cards = container?.querySelectorAll('[data-testid="schedule-edit-provider-card"]');
    expect(cards?.length).toBe(2);
  });

  it("selects the existing provider when schedule target is provider", () => {
    renderModal(makeSchedule({ target: { type: "provider", providerId: "codex" } }));

    const cards = container?.querySelectorAll('[data-testid="schedule-edit-provider-card"]');
    expect(cards?.[0]?.getAttribute("data-selected")).toBe("false");
    expect(cards?.[1]?.getAttribute("data-selected")).toBe("true");
  });

  it("clears selection when existing schedule has non-provider target", () => {
    renderModal(makeSchedule({ target: { type: "agent", agentId: "agent-1" } }));

    const cards = container?.querySelectorAll('[data-testid="schedule-edit-provider-card"]');
    expect(cards?.[0]?.getAttribute("data-selected")).toBe("false");
    expect(cards?.[1]?.getAttribute("data-selected")).toBe("false");
  });

  it("updates schedule with provider target", async () => {
    renderModal();

    selectProviderCard(1);
    await submit();

    await vi.waitFor(() => {
      expect(mutationsResult.current?.updateSchedule).toHaveBeenCalled();
    });

    expect(mutationsResult.current?.updateSchedule).toHaveBeenCalledWith(
      expect.objectContaining({
        scheduleId: "schedule-1",
        target: { type: "provider", providerId: "codex" },
      }),
    );
  });

  it("shows validation error when no provider chosen", async () => {
    renderModal(makeSchedule({ target: { type: "agent", agentId: "agent-1" } }));

    await submit();

    const error = container?.querySelector('[data-testid="schedule-edit-error"]');
    expect(error).not.toBeNull();
    expect(error?.textContent).toContain("provider");
    expect(mutationsResult.current?.updateSchedule).not.toHaveBeenCalled();
  });
});
