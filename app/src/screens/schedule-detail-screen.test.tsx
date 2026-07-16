/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ScheduleInspectResult } from "@/hooks/use-schedule-inspect";
import type { StoredSchedule } from "@server/server/schedule/types";
import { ScheduleDetailScreen } from "@/screens/schedule-detail-screen";

const { inspectResult } = vi.hoisted(() => ({
  inspectResult: {
    current: null as ScheduleInspectResult | null,
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

vi.mock("react-native-unistyles", () => {
  const theme = {
    spacing: { 1: 4, 2: 8, 3: 12, 4: 16, 6: 24 },
    fontSize: { xs: 12, sm: 14, base: 16, lg: 18 },
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

vi.mock("expo-router", () => ({
  router: { navigate: vi.fn(), back: vi.fn() },
}));

vi.mock("@react-navigation/native", () => ({
  useIsFocused: () => true,
}));

vi.mock("lucide-react-native", () => ({
  ArrowLeft: () => React.createElement("span", { "data-icon": "ArrowLeft" }),
  Calendar: () => React.createElement("span", { "data-icon": "Calendar" }),
  Clock: () => React.createElement("span", { "data-icon": "Clock" }),
  CheckCircle: () => React.createElement("span", { "data-icon": "CheckCircle" }),
  XCircle: () => React.createElement("span", { "data-icon": "XCircle" }),
  Loader: () => React.createElement("span", { "data-icon": "Loader" }),
  Globe: () => React.createElement("span", { "data-icon": "Globe" }),
}));

vi.mock("@/components/headers/menu-header", () => ({
  MenuHeader: ({
    title,
    leftContent,
  }: {
    title: string;
    leftContent?: React.ReactNode;
  }) =>
    React.createElement("header", { "data-testid": "menu-header" }, leftContent, title),
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

vi.mock("@/hooks/use-schedule-inspect", () => ({
  useScheduleInspect: () => {
    if (!inspectResult.current) {
      throw new Error("Expected inspect result");
    }
    return inspectResult.current;
  },
}));

vi.mock("@/utils/host-routes", () => ({
  buildHostSchedulesRoute: (serverId: string) => `/h/${serverId}/schedules`,
}));

vi.mock("@/utils/cron-timezone", () => ({
  detectTimezone: () => "Asia/Shanghai",
  cronFromUTC: (expr: string, tz: string) => {
    if (tz === "Asia/Shanghai") {
      const parts = expr.trim().split(/\s+/);
      if (parts.length >= 2) {
        const h = (parseInt(parts[1], 10) + 8) % 24;
        parts[1] = String(h);
      }
      return parts.join(" ");
    }
    return expr;
  },
  describeCron: (expr: string) => {
    const parts = expr.trim().split(/\s+/);
    if (parts.length === 5) {
      const [m, h, dom, mon, dow] = parts;
      if (dom === "*" && mon === "*" && dow === "*" && /^\d+$/.test(h) && /^\d+$/.test(m)) {
        return `每天 ${h.padStart(2, "0")}:${m.padStart(2, "0")}`;
      }
    }
    return expr;
  },
}));

function makeStoredSchedule(overrides: Partial<StoredSchedule> = {}): StoredSchedule {
  return {
    id: "schedule-1",
    name: "Daily Report",
    prompt: "Generate daily report",
    cadence: { type: "cron", expression: "0 9 * * *" },
    target: { type: "agent", agentId: "agent-1" },
    cwd: null,
    status: "active",
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
    nextRunAt: "2026-01-02T09:00:00.000Z",
    lastRunAt: "2026-01-01T09:00:00.000Z",
    pausedAt: null,
    expiresAt: null,
    maxRuns: null,
    runs: [],
    ...overrides,
  };
}

function makeInspectResult(overrides: Partial<ScheduleInspectResult> = {}): ScheduleInspectResult {
  return {
    schedule: null,
    isLoading: false,
    isRevalidating: false,
    error: null,
    refresh: vi.fn(),
    ...overrides,
  };
}

describe("ScheduleDetailScreen", () => {
  let container: HTMLElement | null = null;
  let root: Root | null = null;

  beforeEach(() => {
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    inspectResult.current = makeInspectResult();
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
    inspectResult.current = null;
    vi.unstubAllGlobals();
  });

  it("shows loading spinner during initial load", () => {
    inspectResult.current = makeInspectResult({ isLoading: true });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.querySelector('[data-testid="loading-spinner"]')).not.toBeNull();
  });

  it("displays schedule name and prompt", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({ name: "Daily Report", prompt: "Generate the daily summary" }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("Daily Report");
    expect(container?.textContent).toContain("Generate the daily summary");
  });

  it("displays working directory when present", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({ cwd: "/workspace/project" }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("Working Directory");
    expect(container?.textContent).toContain("/workspace/project");
  });

  it("hides working directory when not set", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({ cwd: null }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).not.toContain("Working Directory");
  });

  it("displays cron cadence expression", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({ cadence: { type: "cron", expression: "0 9 * * *" } }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("0 9 * * *");
  });

  it("displays interval cadence in human-readable form", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({ cadence: { type: "every", everyMs: 3600000 } }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("Every 1h");
  });

  it("displays schedule status", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({ status: "paused" }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("paused");
  });

  it("displays run history when runs exist", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({
        runs: [
          {
            id: "run-1",
            scheduledFor: "2026-01-01T09:00:00.000Z",
            startedAt: "2026-01-01T09:00:01.000Z",
            endedAt: "2026-01-01T09:01:30.000Z",
            status: "succeeded",
            agentId: "agent-1",
            output: "Report generated",
            error: null,
          },
          {
            id: "run-2",
            scheduledFor: "2026-01-02T09:00:00.000Z",
            startedAt: "2026-01-02T09:00:02.000Z",
            endedAt: null,
            status: "failed",
            agentId: "agent-1",
            output: null,
            error: "timeout exceeded",
          },
        ],
      }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("succeeded");
    expect(container?.textContent).toContain("failed");
    expect(container?.textContent).toContain("Report generated");
    expect(container?.textContent).toContain("timeout exceeded");
  });

  it("shows empty state when no runs exist", () => {
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({ runs: [] }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("No runs yet");
  });

  it("shows error state when schedule fails to load", () => {
    inspectResult.current = makeInspectResult({ error: "schedule not found" });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    expect(container?.textContent).toContain("schedule not found");
  });

  it("converts UTC cron expression to local time for display", () => {
    // UTC expression: "25 16 * * *" = 16:25 UTC = 00:25 Asia/Shanghai
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({
        cadence: { type: "cron", expression: "25 16 * * *", timezone: "Asia/Shanghai" },
      }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    // Should show local time expression, not UTC
    expect(container?.textContent).toContain("每天 00:25");
    // Raw UTC expression should also be shown
    expect(container?.textContent).toContain("25 16 * * *");
  });

  it("displays next run time in local timezone", () => {
    // nextRunAt is 16:25 UTC = 00:25 next day in Asia/Shanghai
    inspectResult.current = makeInspectResult({
      schedule: makeStoredSchedule({
        cadence: { type: "cron", expression: "25 16 * * *", timezone: "Asia/Shanghai" },
        nextRunAt: "2026-06-03T16:25:00.000Z",
      }),
    });

    act(() => {
      root?.render(<ScheduleDetailScreen serverId="server-1" scheduleId="schedule-1" />);
    });

    const text = container?.textContent ?? "";
    // Should show 00:25 (local), NOT 4:25 PM (UTC)
    expect(text).toContain("00:25");
    expect(text).not.toMatch(/4:25/);
  });
});
