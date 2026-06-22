/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ScheduleCreateModal } from "./schedule-create-modal";

const { createScheduleResult, agentsResult, theme } = vi.hoisted(() => ({
  createScheduleResult: {
    current: null as {
      createSchedule: ReturnType<typeof vi.fn>;
      isCreating: boolean;
    } | null,
  },
  agentsResult: {
    current: null as {
      agents: { id: string; title: string | null; cwd: string }[];
      isInitialLoad: boolean;
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
  View: ({ children, ...props }: React.PropsWithChildren<Record<string, unknown>>) =>
    React.createElement("div", props, children),
  Text: ({ children, ...props }: React.PropsWithChildren<Record<string, unknown>>) =>
    React.createElement("span", props, children),
  Pressable: ({ children, onPress, ...props }: React.PropsWithChildren<{ onPress?: () => void } & Record<string, unknown>>) =>
    React.createElement("button", { type: "button", onClick: onPress, ...props }, children),
  ScrollView: ({ children, ...props }: React.PropsWithChildren<Record<string, unknown>>) =>
    React.createElement("div", props, children),
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
  SegmentedControl: ({ value, onValueChange }: { value: string; onValueChange?: (value: string) => void }) =>
    React.createElement("select", {
      "data-testid": "segmented-control",
      value,
      onChange: (e: React.ChangeEvent<HTMLSelectElement>) => onValueChange?.(e.target.value),
    }),
}));

vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => children,
  TooltipTrigger: ({ children }: { children: React.ReactNode }) => children,
  TooltipContent: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@/hooks/use-all-agents-list", () => ({
  useAllAgentsList: () => {
    if (!agentsResult.current) {
      throw new Error("Expected agents result");
    }
    return agentsResult.current;
  },
}));

vi.mock("@/hooks/use-create-schedule", () => ({
  useCreateSchedule: () => {
    if (!createScheduleResult.current) {
      throw new Error("Expected create schedule result");
    }
    return createScheduleResult.current;
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

    agentsResult.current = {
      agents: [{ id: "agent-1", title: "Test Agent", cwd: "/test" }],
      isInitialLoad: false,
    };
    createScheduleResult.current = {
      createSchedule: vi.fn(async () => ({
        id: "schedule-new",
        name: null,
        prompt: "Generate report",
        cadence: { type: "cron", expression: "0 9 * * *" },
        target: { type: "agent", agentId: "agent-1" },
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
    agentsResult.current = null;
    createScheduleResult.current = null;
    vi.unstubAllGlobals();
  });

  it("renders a working directory input", () => {
    act(() => {
      root?.render(<ScheduleCreateModal visible serverId="server-1" onClose={vi.fn()} />);
    });

    const inputs = container?.querySelectorAll('input[data-placeholder="/path/to/project"]');
    expect(inputs?.length).toBe(1);
  });


});
