/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { StoredSchedule } from "@server/server/schedule/types";
import type { ScheduleAssistProposal } from "@/stores/schedule-assistant-store";
import { ProposalCard, buildUpdateRows } from "./proposal-card";

const { inspectResult, theme } = vi.hoisted(() => ({
  inspectResult: {
    current: null as { schedule: StoredSchedule | null; isLoading: boolean } | null,
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
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onPress,
    disabled,
    testID,
    variant,
  }: React.PropsWithChildren<{
    onPress?: () => void;
    disabled?: boolean;
    testID?: string;
    variant?: string;
  }>) =>
    React.createElement(
      "button",
      { type: "button", onClick: onPress, disabled, "data-testid": testID, "data-variant": variant },
      children,
    ),
}));

vi.mock("@/hooks/use-schedule-inspect", () => ({
  useScheduleInspect: () => {
    if (!inspectResult.current) {
      throw new Error("Expected inspect result");
    }
    return inspectResult.current;
  },
}));

vi.mock("@/utils/cron-timezone", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/utils/cron-timezone")>()),
  detectTimezone: () => "UTC",
}));

function makeCurrentSchedule(overrides: Partial<StoredSchedule> = {}): StoredSchedule {
  return {
    id: "sched-1",
    name: "Nightly",
    prompt: "Summarize the nightly test runs",
    cadence: { type: "cron", expression: "0 1 * * *", timezone: "Asia/Shanghai" },
    target: { type: "agent", agentId: "agent-1" },
    cwd: null,
    status: "active",
    createdAt: "2026-07-01T00:00:00.000Z",
    updatedAt: "2026-07-01T00:00:00.000Z",
    nextRunAt: "2026-07-19T01:00:00.000Z",
    lastRunAt: null,
    pausedAt: null,
    expiresAt: null,
    maxRuns: null,
    runs: [],
    ...overrides,
  };
}

function makeCreateProposal(overrides: Partial<ScheduleAssistProposal> = {}): ScheduleAssistProposal {
  return {
    op: "create",
    name: "Nightly test summary",
    prompt: "Summarize the nightly test runs",
    cadence: { type: "cron", expression: "0 9 * * *" },
    target: { type: "agent", agentId: "agent-1" },
    summary: "Create a daily 09:00 summary",
    nextRunAt: "2026-07-19T01:00:00.000Z",
    ...overrides,
  };
}

describe("ProposalCard", () => {
  let container: HTMLElement | null = null;
  let root: Root | null = null;

  beforeEach(() => {
    vi.stubGlobal("React", React);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    inspectResult.current = { schedule: null, isLoading: false };
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

  function renderCard(props: Partial<React.ComponentProps<typeof ProposalCard>> = {}) {
    const proposal = props.proposal ?? makeCreateProposal();
    act(() => {
      root?.render(
        <ProposalCard
          serverId="server-1"
          messageId="msg-1"
          proposal={proposal}
          onConfirm={vi.fn()}
          onEditInForm={vi.fn()}
          onCancel={vi.fn()}
          {...props}
        />,
      );
    });
  }

  function click(testId: string) {
    const element = container?.querySelector(`[data-testid="${testId}"]`);
    act(() => {
      element?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
  }

  it("renders a create proposal with badge, fields, next run, and warnings", () => {
    renderCard({
      proposal: makeCreateProposal({
        cwd: "~/work/backend",
        maxRuns: 30,
        warnings: ["interpreted 'morning' as 09:00"],
      }),
    });

    const text = container?.textContent ?? "";
    expect(text).toContain("create");
    expect(text).toContain("Nightly test summary");
    expect(text).toContain("Summarize the nightly test runs");
    expect(text).toContain("每天 09:00");
    expect(text).toContain("agent · agent-1");
    expect(text).toContain("~/work/backend");
    expect(text).toContain("30");
    expect(text).toContain("Next run");
    expect(text).toContain("interpreted 'morning' as 09:00");
    expect(container?.querySelector('[data-testid="proposal-confirm-button"]')).not.toBeNull();
    expect(container?.querySelector('[data-testid="proposal-edit-button"]')).not.toBeNull();
    expect(container?.querySelector('[data-testid="proposal-cancel-button"]')).not.toBeNull();
  });

  it("describes an every cadence as every N min", () => {
    renderCard({
      proposal: makeCreateProposal({ cadence: { type: "every", everyMs: 900000 } }),
    });

    expect(container?.textContent).toContain("every 15 min");
  });

  it("summarizes provider and new-agent targets", () => {
    renderCard({
      proposal: makeCreateProposal({ target: { type: "provider", providerId: "claude" } }),
    });
    expect(container?.textContent).toContain("provider · claude");

    renderCard({
      proposal: makeCreateProposal({
        target: { type: "new-agent", config: { provider: "claude", cwd: "/tmp" } },
      }),
    });
    expect(container?.textContent).toContain("new-agent · claude");
  });

  it("calls onConfirm with the message id and proposal", () => {
    const onConfirm = vi.fn();
    const proposal = makeCreateProposal();
    renderCard({ proposal, onConfirm });

    click("proposal-confirm-button");

    expect(onConfirm).toHaveBeenCalledWith("msg-1", proposal);
  });

  it("calls onEditInForm and onCancel", () => {
    const onEditInForm = vi.fn();
    const onCancel = vi.fn();
    const proposal = makeCreateProposal();
    renderCard({ proposal, onEditInForm, onCancel });

    click("proposal-edit-button");
    expect(onEditInForm).toHaveBeenCalledWith(proposal);

    click("proposal-cancel-button");
    expect(onCancel).toHaveBeenCalledWith("msg-1");
  });

  it("shows a per-field diff for update proposals", () => {
    inspectResult.current = { schedule: makeCurrentSchedule(), isLoading: false };
    renderCard({
      proposal: {
        op: "update",
        scheduleId: "sched-1",
        name: "Nightly",
        cadence: { type: "cron", expression: "30 7 * * *" },
        summary: "Move to 07:30",
      },
    });

    const text = container?.textContent ?? "";
    // Changed cadence shows old → new (stored UTC 01:00 → local 09:00 in Asia/Shanghai)
    expect(text).toContain("每天 09:00");
    expect(text).toContain("每天 07:30");
    expect(text).toContain("→");
    // Unchanged prompt is not repeated in the diff
    expect(text).not.toContain("Summarize the nightly test runs");
  });

  it("renders delete proposals with a destructive confirm and no edit button", () => {
    renderCard({
      proposal: {
        op: "delete",
        scheduleId: "sched-1",
        name: "Nightly",
        summary: "Delete nightly",
      },
    });

    const confirm = container?.querySelector('[data-testid="proposal-confirm-button"]');
    expect(confirm?.getAttribute("data-variant")).toBe("destructive");
    expect(container?.querySelector('[data-testid="proposal-edit-button"]')).toBeNull();
    expect(container?.textContent).toContain("Nightly");
  });

  it("disables confirm while applying", () => {
    renderCard({ applying: true });

    const confirm = container?.querySelector(
      '[data-testid="proposal-confirm-button"]',
    ) as HTMLButtonElement | null;
    expect(confirm?.disabled).toBe(true);
  });

  it("shows the inline apply error for retry", () => {
    renderCard({ applyError: "invalid cadence" });

    expect(container?.textContent).toContain("invalid cadence");
    expect(container?.querySelector('[data-testid="proposal-confirm-button"]')).not.toBeNull();
  });

  it("renders a cancelled state without actions", () => {
    renderCard({ cancelled: true });

    expect(container?.textContent).toContain("Cancelled");
    expect(container?.querySelector('[data-testid="proposal-confirm-button"]')).toBeNull();
  });
});

describe("buildUpdateRows", () => {
  const baseProposal: ScheduleAssistProposal = {
    op: "update",
    scheduleId: "sched-1",
    summary: "Update",
  };

  it("returns no rows when nothing changed", () => {
    expect(buildUpdateRows({ ...baseProposal }, makeCurrentSchedule())).toEqual([]);
  });

  it("diffs a changed name against the current value", () => {
    const rows = buildUpdateRows({ ...baseProposal, name: "Renamed" }, makeCurrentSchedule());
    expect(rows).toContainEqual({ label: "Name", before: "Nightly", after: "Renamed" });
  });

  it("omits the name row when the name is unchanged", () => {
    const rows = buildUpdateRows({ ...baseProposal, name: "Nightly" }, makeCurrentSchedule());
    expect(rows.find((row) => row.label === "Name")).toBeUndefined();
  });

  it("diffs a changed prompt", () => {
    const rows = buildUpdateRows(
      { ...baseProposal, prompt: "New prompt" },
      makeCurrentSchedule(),
    );
    expect(rows).toContainEqual({
      label: "Prompt",
      before: "Summarize the nightly test runs",
      after: "New prompt",
    });
  });

  it("diffs a changed cadence with a null before when there is no current schedule", () => {
    const rows = buildUpdateRows(
      { ...baseProposal, cadence: { type: "every", everyMs: 900000 } },
      null,
    );
    expect(rows).toContainEqual({ label: "Cadence", before: null, after: "every 15 min" });
  });

  it("diffs a changed target", () => {
    const rows = buildUpdateRows(
      { ...baseProposal, target: { type: "provider", providerId: "claude" } },
      makeCurrentSchedule(),
    );
    expect(rows).toContainEqual({
      label: "Target",
      before: "agent · agent-1",
      after: "provider · claude",
    });
  });

  it("diffs a changed cwd", () => {
    const rows = buildUpdateRows({ ...baseProposal, cwd: "/new/dir" }, makeCurrentSchedule());
    expect(rows).toContainEqual({ label: "Cwd", before: null, after: "/new/dir" });
  });

  it("diffs a changed maxRuns", () => {
    const rows = buildUpdateRows({ ...baseProposal, maxRuns: 5 }, makeCurrentSchedule());
    expect(rows).toContainEqual({ label: "Max runs", before: null, after: "5" });
  });

  it("diffs a changed expiresAt with a null before", () => {
    const rows = buildUpdateRows(
      { ...baseProposal, expiresAt: "2026-12-31T00:00:00.000Z" },
      makeCurrentSchedule(),
    );
    const row = rows.find((r) => r.label === "Expires");
    expect(row?.before).toBeNull();
    expect(row?.after).toContain("2026");
  });
});
