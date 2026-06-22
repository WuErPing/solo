import { describe, expect, it } from "vitest";
import { buildScheduleCwdItems, type AggregatedScheduleForCounts } from "./use-schedule-project-counts";

function makeSchedule(
  target: AggregatedScheduleForCounts["target"],
  serverId = "host1",
  cwd: string | null = null,
): AggregatedScheduleForCounts {
  return {
    id: `sched-${Math.random().toString(36).slice(2)}`,
    name: null,
    prompt: "do something",
    cadence: { type: "every", everyMs: 60000 },
    target,
    cwd,
    status: "active",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    nextRunAt: null,
    lastRunAt: null,
    pausedAt: null,
    expiresAt: null,
    maxRuns: null,
    serverId,
    serverLabel: "host1",
  };
}

describe("buildScheduleCwdItems", () => {
  it("returns empty array for empty schedules", () => {
    expect(buildScheduleCwdItems([])).toEqual([]);
  });

  it("extracts cwd from new-agent target", () => {
    const schedules = [
      makeSchedule({
        type: "new-agent",
        config: { provider: "claude", cwd: "/repo" },
      }),
    ];
    const result = buildScheduleCwdItems(schedules);
    expect(result).toEqual([{ cwd: "/repo", serverId: "host1" }]);
  });

  it("skips agent targets (no cwd)", () => {
    const schedules = [
      makeSchedule({ type: "agent", agentId: "00000000-0000-0000-0000-000000000001" }),
    ];
    const result = buildScheduleCwdItems(schedules);
    expect(result).toEqual([]);
  });

  it("skips new-agent targets with empty cwd", () => {
    const schedules = [
      makeSchedule({
        type: "new-agent",
        config: { provider: "claude", cwd: "  " },
      }),
    ];
    const result = buildScheduleCwdItems(schedules);
    expect(result).toEqual([]);
  });

  it("maps multiple schedules", () => {
    const schedules = [
      makeSchedule(
        { type: "new-agent", config: { provider: "claude", cwd: "/repo1" } },
        "host1",
      ),
      makeSchedule(
        { type: "new-agent", config: { provider: "claude", cwd: "/repo2" } },
        "host2",
      ),
      makeSchedule(
        { type: "agent", agentId: "00000000-0000-0000-0000-000000000001" },
        "host1",
      ),
    ];
    const result = buildScheduleCwdItems(schedules);
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({ cwd: "/repo1", serverId: "host1" });
    expect(result[1]).toEqual({ cwd: "/repo2", serverId: "host2" });
  });

  it("prefers top-level cwd over new-agent config cwd", () => {
    const schedules = [
      makeSchedule(
        { type: "new-agent", config: { provider: "claude", cwd: "/config-cwd" } },
        "host1",
        "/top-level-cwd",
      ),
    ];
    const result = buildScheduleCwdItems(schedules);
    expect(result).toEqual([{ cwd: "/top-level-cwd", serverId: "host1" }]);
  });

  it("extracts cwd from agent target when top-level cwd is present", () => {
    const schedules = [
      makeSchedule(
        { type: "agent", agentId: "00000000-0000-0000-0000-000000000001" },
        "host1",
        "/agent-target-cwd",
      ),
    ];
    const result = buildScheduleCwdItems(schedules);
    expect(result).toEqual([{ cwd: "/agent-target-cwd", serverId: "host1" }]);
  });

  it("skips schedules with empty top-level cwd", () => {
    const schedules = [
      makeSchedule(
        { type: "agent", agentId: "00000000-0000-0000-0000-000000000001" },
        "host1",
        "  ",
      ),
    ];
    const result = buildScheduleCwdItems(schedules);
    expect(result).toEqual([]);
  });
});
