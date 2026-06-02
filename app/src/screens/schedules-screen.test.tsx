/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SchedulesResult } from "@/hooks/use-schedules";
import type { ScheduleMutationsResult } from "@/hooks/use-schedule-mutations";
import { SchedulesScreen } from "@/screens/schedules-screen";

const { schedulesResult, mutationsResult } = vi.hoisted(() => ({
  schedulesResult: {
    current: null as SchedulesResult | null,
  },
  mutationsResult: {
    current: null as ScheduleMutationsResult | null,
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
  ScrollView: ({ children, refreshControl, ...props }: React.PropsWithChildren<{ refreshControl?: React.ReactNode } & Record<string, unknown>>) =>
    React.createElement("div", { ...props, "data-has-refresh": refreshControl ? "true" : "false" }, refreshControl, children),
  RefreshControl: ({ refreshing }: { refreshing?: boolean }) =>
    React.createElement("div", { "data-refreshing": refreshing ? "true" : "false", "data-testid": "refresh-control" }),
}));

vi.mock("react-native-unistyles", () => {
  const theme = {
    spacing: { 2: 8, 3: 12, 4: 16, 6: 24 },
    fontSize: { sm: 14, lg: 18 },
    colors: {
      surface0: "#111",
      surface1: "#222",
      surface2: "#333",
      foreground: "#fff",
      foregroundMuted: "#999",
      border: "#444",
      palette: {
        green: { 400: "#4ade80" },
        amber: { 500: "#f59e0b" },
        red: { 500: "#ef4444" },
      },
    },
    borderRadius: { md: 6, lg: 8, full: 9999 },
    iconSize: { md: 20 },
    borderWidth: { 1: 1 },
  };

  return {
    StyleSheet: {
      create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
    },
    useUnistyles: () => ({ theme }),
  };
});

vi.mock("@react-navigation/native", () => ({
  useIsFocused: () => true,
}));

vi.mock("expo-router", () => ({
  router: { navigate: vi.fn() },
}));

vi.mock("lucide-react-native", () => ({
  Calendar: () => React.createElement("span", { "data-icon": "Calendar" }),
  ChevronLeft: () => React.createElement("span", { "data-icon": "ChevronLeft" }),
  Pause: () => React.createElement("span", { "data-icon": "Pause" }),
  Pencil: () => React.createElement("span", { "data-icon": "Pencil" }),
  Play: () => React.createElement("span", { "data-icon": "Play" }),
  Plus: () => React.createElement("span", { "data-icon": "Plus" }),
  Trash2: () => React.createElement("span", { "data-icon": "Trash2" }),
  Clock: () => React.createElement("span", { "data-icon": "Clock" }),
}));

vi.mock("@/components/headers/menu-header", () => ({
  MenuHeader: ({
    title,
    rightContent,
  }: {
    title: string;
    rightContent?: React.ReactNode;
  }) =>
    React.createElement("header", { "data-testid": "menu-header" }, title, rightContent),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onPress,
    testID,
  }: React.PropsWithChildren<{ onPress?: () => void; testID?: string }>) =>
    React.createElement("button", { type: "button", onClick: onPress, "data-testid": testID }, children),
}));

vi.mock("@/components/ui/loading-spinner", () => ({
  LoadingSpinner: ({ color, size }: { color: string; size?: string }) =>
    React.createElement("div", {
      "data-color": color,
      "data-size": size,
      "data-testid": "loading-spinner",
    }),
}));

vi.mock("@/components/schedule-create-modal", () => ({
  ScheduleCreateModal: ({ visible }: { visible: boolean }) =>
    visible ? React.createElement("div", { "data-testid": "schedule-create-modal" }) : null,
}));

vi.mock("@/components/schedule-edit-modal", () => ({
  ScheduleEditModal: ({ visible, schedule }: { visible: boolean; schedule: unknown }) =>
    visible && schedule
      ? React.createElement("div", { "data-testid": "schedule-edit-modal" })
      : null,
}));

vi.mock("@/hooks/use-schedules", () => ({
  useSchedules: () => {
    if (!schedulesResult.current) {
      throw new Error("Expected schedules result");
    }
    return schedulesResult.current;
  },
}));

vi.mock("@/hooks/use-schedule-mutations", () => ({
  useScheduleMutations: () => {
    if (!mutationsResult.current) {
      throw new Error("Expected mutations result");
    }
    return mutationsResult.current;
  },
}));

vi.mock("@/utils/host-routes", () => ({
  buildHostOpenProjectRoute: (serverId: string) => `/h/${serverId}/open-project`,
}));

function makeScheduleSummary(overrides: Record<string, unknown> = {}) {
  return {
    id: "schedule-1",
    name: "Daily Report",
    prompt: "Generate daily report",
    cadence: { type: "cron", expression: "0 9 * * *" },
    target: { type: "agent", agentId: "agent-1" },
    status: "active",
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
    nextRunAt: "2026-01-02T09:00:00.000Z",
    lastRunAt: null,
    pausedAt: null,
    expiresAt: null,
    maxRuns: null,
    ...overrides,
  };
}

function makeSchedulesResult(overrides: Partial<SchedulesResult> = {}): SchedulesResult {
  return {
    schedules: [],
    isLoading: false,
    isInitialLoad: false,
    isRevalidating: false,
    error: null,
    refreshAll: vi.fn(),
    ...overrides,
  };
}

function makeMutationsResult(overrides: Partial<ScheduleMutationsResult> = {}): ScheduleMutationsResult {
  return {
    pauseSchedule: vi.fn(),
    resumeSchedule: vi.fn(),
    deleteSchedule: vi.fn(),
    updateSchedule: vi.fn(),
    isPausing: () => false,
    isResuming: () => false,
    isDeleting: () => false,
    isUpdating: () => false,
    ...overrides,
  };
}

describe("SchedulesScreen", () => {
  let container: HTMLElement | null = null;
  let root: Root | null = null;

  beforeEach(() => {
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    schedulesResult.current = makeSchedulesResult();
    mutationsResult.current = makeMutationsResult();
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
    schedulesResult.current = null;
    mutationsResult.current = null;
    vi.unstubAllGlobals();
  });

  it("shows loading spinner during initial load", () => {
    schedulesResult.current = makeSchedulesResult({ isInitialLoad: true, isLoading: true });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    expect(container?.querySelector('[data-testid="loading-spinner"]')).not.toBeNull();
    expect(container?.textContent).not.toContain("No schedules yet");
  });

  it("shows empty state when no schedules exist", () => {
    schedulesResult.current = makeSchedulesResult();

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    expect(container?.querySelector('[data-testid="loading-spinner"]')).toBeNull();
    expect(container?.textContent).toContain("No schedules yet");
    expect(container?.textContent).toContain("Back");
  });

  it("renders schedule list with names and statuses", () => {
    schedulesResult.current = makeSchedulesResult({
      schedules: [
        makeScheduleSummary({ id: "s1", name: "Daily Report", status: "active" }),
        makeScheduleSummary({ id: "s2", name: "Weekly Sync", status: "paused" }),
      ],
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    expect(container?.textContent).toContain("Daily Report");
    expect(container?.textContent).toContain("Weekly Sync");
    expect(container?.textContent).toContain("active");
    expect(container?.textContent).toContain("paused");
  });

  it("shows cron expression for cron cadence", () => {
    schedulesResult.current = makeSchedulesResult({
      schedules: [makeScheduleSummary({ cadence: { type: "cron", expression: "0 9 * * *" } })],
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    expect(container?.textContent).toContain("0 9 * * *");
  });

  it("shows interval for every cadence", () => {
    schedulesResult.current = makeSchedulesResult({
      schedules: [makeScheduleSummary({ cadence: { type: "every", everyMs: 3600000 } })],
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    expect(container?.textContent).toContain("Every 1h");
  });

  it("calls refreshAll when refresh is triggered", () => {
    const refreshAll = vi.fn();
    schedulesResult.current = makeSchedulesResult({
      schedules: [makeScheduleSummary()],
      refreshAll,
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    // The ScrollView mock renders refreshControl as a child with data-testid
    const scrollView = container?.querySelector('[data-has-refresh="true"]');
    expect(scrollView).not.toBeNull();
    expect(refreshAll).not.toHaveBeenCalled();
  });

  it("calls pauseSchedule when pause button is pressed", () => {
    const pauseSchedule = vi.fn();
    mutationsResult.current = makeMutationsResult({ pauseSchedule });
    schedulesResult.current = makeSchedulesResult({
      schedules: [makeScheduleSummary({ id: "schedule-1", status: "active" })],
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    const pauseButton = container?.querySelector('[data-action="pause-schedule-1"]');
    expect(pauseButton).not.toBeNull();

    act(() => {
      pauseButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    expect(pauseSchedule).toHaveBeenCalledWith("schedule-1");
  });

  it("calls resumeSchedule when resume button is pressed", () => {
    const resumeSchedule = vi.fn();
    mutationsResult.current = makeMutationsResult({ resumeSchedule });
    schedulesResult.current = makeSchedulesResult({
      schedules: [makeScheduleSummary({ id: "schedule-1", status: "paused" })],
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    const resumeButton = container?.querySelector('[data-action="resume-schedule-1"]');
    expect(resumeButton).not.toBeNull();

    act(() => {
      resumeButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    expect(resumeSchedule).toHaveBeenCalledWith("schedule-1");
  });

  it("calls deleteSchedule when delete button is pressed", () => {
    const deleteSchedule = vi.fn();
    mutationsResult.current = makeMutationsResult({ deleteSchedule });
    schedulesResult.current = makeSchedulesResult({
      schedules: [makeScheduleSummary({ id: "schedule-1" })],
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    const deleteButton = container?.querySelector('[data-action="delete-schedule-1"]');
    expect(deleteButton).not.toBeNull();

    act(() => {
      deleteButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    expect(deleteSchedule).toHaveBeenCalledWith("schedule-1");
  });

  it("shows the new schedule button and opens the create modal", () => {
    schedulesResult.current = makeSchedulesResult({
      schedules: [makeScheduleSummary()],
    });

    act(() => {
      root?.render(<SchedulesScreen serverId="server-1" />);
    });

    const newButton = container?.querySelector('[data-testid="new-schedule-button"]');
    expect(newButton).not.toBeNull();
    expect(newButton?.textContent).toContain("New");

    // Modal should not be visible initially
    expect(container?.querySelector('[data-testid="schedule-create-modal"]')).toBeNull();

    act(() => {
      newButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    // Modal should be visible after clicking
    expect(container?.querySelector('[data-testid="schedule-create-modal"]')).not.toBeNull();
  });
});
