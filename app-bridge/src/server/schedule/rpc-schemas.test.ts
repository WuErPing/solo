import { describe, expect, it } from "vitest";
import {
  ScheduleCreateRequestSchema,
  ScheduleCreateResponseSchema,
  ScheduleListResponseSchema,
  ScheduleUpdateRequestSchema,
} from "./rpc-schemas.js";

const baseSchedule = {
  id: "sched-1",
  name: null,
  prompt: "Run the daily report",
  cadence: { type: "cron", expression: "0 9 * * *", timezone: "UTC" } as const,
  target: { type: "agent", agentId: "agent-1" } as const,
  status: "active" as const,
  createdAt: "2026-06-22T00:00:00Z",
  updatedAt: "2026-06-22T00:00:00Z",
  nextRunAt: "2026-06-22T09:00:00Z",
  lastRunAt: null,
  pausedAt: null,
  expiresAt: null,
  maxRuns: null,
};

describe("ScheduleCreateRequestSchema", () => {
  it("accepts a non-UUID agentId", () => {
    const result = ScheduleCreateRequestSchema.safeParse({
      type: "schedule/create",
      requestId: "req-1",
      prompt: "Run the daily report",
      cadence: { type: "cron", expression: "0 9 * * *", timezone: "UTC" },
      target: { type: "agent", agentId: "agent-abc-123" },
    });
    expect(result.success).toBe(true);
  });

  it("rejects an empty agentId", () => {
    const result = ScheduleCreateRequestSchema.safeParse({
      type: "schedule/create",
      requestId: "req-1",
      prompt: "Run the daily report",
      cadence: { type: "cron", expression: "0 9 * * *" },
      target: { type: "agent", agentId: "" },
    });
    expect(result.success).toBe(false);
  });

  it("accepts a null cwd", () => {
    const result = ScheduleCreateRequestSchema.safeParse({
      type: "schedule/create",
      requestId: "req-1",
      prompt: "Run the daily report",
      cadence: { type: "cron", expression: "0 9 * * *" },
      target: { type: "agent", agentId: "agent-1" },
      cwd: null,
    });
    expect(result.success).toBe(true);
  });
});

describe("ScheduleUpdateRequestSchema", () => {
  it("accepts a non-UUID agentId", () => {
    const result = ScheduleUpdateRequestSchema.safeParse({
      type: "schedule/update",
      requestId: "req-1",
      scheduleId: "sched-1",
      prompt: "Updated prompt",
      cadence: { type: "every", everyMs: 60000 },
      target: { type: "agent", agentId: "agent-abc-123" },
    });
    expect(result.success).toBe(true);
  });
});

describe("ScheduleCreateResponseSchema", () => {
  it("accepts a schedule without cwd (daemon omits null cwd)", () => {
    const result = ScheduleCreateResponseSchema.safeParse({
      type: "schedule/create/response",
      payload: {
        requestId: "req-1",
        schedule: baseSchedule,
        error: null,
      },
    });
    expect(result.success).toBe(true);
  });
});

describe("ScheduleListResponseSchema", () => {
  it("accepts schedules without cwd", () => {
    const result = ScheduleListResponseSchema.safeParse({
      type: "schedule/list/response",
      payload: {
        requestId: "req-1",
        schedules: [baseSchedule],
        error: null,
      },
    });
    expect(result.success).toBe(true);
  });

  it("falls back to unknown target for a provider schedule missing providerId", () => {
    const result = ScheduleListResponseSchema.safeParse({
      type: "schedule/list/response",
      payload: {
        requestId: "req-1",
        schedules: [
          {
            ...baseSchedule,
            id: "sched-broken",
            target: { type: "provider" },
          },
          baseSchedule,
        ],
        error: null,
      },
    });
    expect(result.success).toBe(true);
    if (!result.success) return;
    expect(result.data.payload.schedules).toHaveLength(2);
    expect(result.data.payload.schedules[0].target.type).toBe("unknown");
    expect(result.data.payload.schedules[1].target.type).toBe("agent");
  });
});
