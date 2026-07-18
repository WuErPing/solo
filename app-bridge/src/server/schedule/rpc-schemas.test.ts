import { describe, expect, it } from "vitest";
import {
  ScheduleAssistRequestSchema,
  ScheduleAssistResponseSchema,
  ScheduleCreateRequestSchema,
  ScheduleCreateResponseSchema,
  ScheduleListResponseSchema,
  ScheduleUpdateRequestSchema,
} from "./rpc-schemas.js";
import { ScheduleNewAgentConfigSchema } from "./types.js";
import { AgentSessionConfigSchema } from "../../shared/agent-session-config.js";

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

describe("ScheduleNewAgentConfigSchema", () => {
  it("is identical to AgentSessionConfigSchema", () => {
    expect(ScheduleNewAgentConfigSchema).toBe(AgentSessionConfigSchema);
  });

  it("accepts all AgentSessionConfig fields", () => {
    const result = ScheduleNewAgentConfigSchema.safeParse({
      provider: "claude",
      cwd: "/project",
      modeId: "code",
      model: "claude-3-opus",
      thinkingOptionId: "extended",
      featureValues: { temperature: 0.2 },
      title: "My Agent",
      approvalPolicy: "dangerous-only",
      sandboxMode: "none",
      networkAccess: true,
      webSearch: true,
      extra: { codex: { foo: "bar" } },
      systemPrompt: "be helpful",
      mcpServers: {
        fs: { type: "stdio", command: "mcp-fs" },
      },
    });
    expect(result.success).toBe(true);
  });
});

describe("ScheduleAssistRequestSchema", () => {
  it("accepts a minimal valid request", () => {
    const result = ScheduleAssistRequestSchema.safeParse({
      type: "schedule/assist",
      requestId: "req-1",
      message: "Every weekday at 9am, summarize the nightly test runs",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
    });
    expect(result.success).toBe(true);
  });

  it("accepts a full request with contextScheduleId and transcript", () => {
    const result = ScheduleAssistRequestSchema.safeParse({
      type: "schedule/assist",
      requestId: "req-2",
      message: "Move it to 7:30",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
      contextScheduleId: "sched-1",
      transcript: [
        { role: "user", content: "Every weekday at 9am, summarize the nightly test runs" },
        { role: "assistant", content: "[proposal] create \"Nightly test summary\"" },
      ],
    });
    expect(result.success).toBe(true);
  });

  it("rejects an empty message", () => {
    const result = ScheduleAssistRequestSchema.safeParse({
      type: "schedule/assist",
      requestId: "req-3",
      message: "",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
    });
    expect(result.success).toBe(false);
  });

  it("rejects a message over 2000 chars", () => {
    const result = ScheduleAssistRequestSchema.safeParse({
      type: "schedule/assist",
      requestId: "req-4",
      message: "a".repeat(2001),
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
    });
    expect(result.success).toBe(false);
  });

  it("rejects a missing timezone", () => {
    const result = ScheduleAssistRequestSchema.safeParse({
      type: "schedule/assist",
      requestId: "req-5",
      message: "Every weekday at 9am, summarize the nightly test runs",
      clientNow: "2026-07-18T09:00:00+08:00",
    });
    expect(result.success).toBe(false);
  });

  it("rejects a transcript over 10 turns", () => {
    const result = ScheduleAssistRequestSchema.safeParse({
      type: "schedule/assist",
      requestId: "req-6",
      message: "Move it to 7:30",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
      transcript: Array.from({ length: 11 }, (_, i) => ({
        role: i % 2 === 0 ? "user" : "assistant",
        content: `turn ${i}`,
      })),
    });
    expect(result.success).toBe(false);
  });
});

describe("ScheduleAssistResponseSchema", () => {
  it("accepts a proposal response with a full proposal", () => {
    const result = ScheduleAssistResponseSchema.safeParse({
      type: "schedule/assist/response",
      payload: {
        requestId: "req-1",
        kind: "proposal",
        proposal: {
          op: "create",
          name: "Nightly test summary",
          prompt: "Summarize the nightly test runs",
          cadence: { type: "cron", expression: "0 9 * * 1-5", timezone: "Asia/Shanghai" },
          target: { type: "agent", agentId: "agent-1" },
          cwd: "/project",
          maxRuns: 30,
          expiresAt: "2026-08-31T00:00:00+08:00",
          summary: "Create \"Nightly test summary\" every weekday at 09:00",
          warnings: ["interpreted 'morning' as 09:00"],
          nextRunAt: "2026-07-21T09:00:00+08:00",
        },
        error: null,
        llmProvider: "openai",
        model: "gpt-5",
      },
    });
    expect(result.success).toBe(true);
  });

  it("accepts a clarify response without a proposal", () => {
    const result = ScheduleAssistResponseSchema.safeParse({
      type: "schedule/assist/response",
      payload: {
        requestId: "req-2",
        kind: "clarify",
        message: "Which schedule do you mean — \"Nightly test summary\" or \"Disk cleanup\"?",
        error: null,
        llmProvider: "openai",
        model: "gpt-5",
      },
    });
    expect(result.success).toBe(true);
  });

  it("accepts an answer response", () => {
    const result = ScheduleAssistResponseSchema.safeParse({
      type: "schedule/assist/response",
      payload: {
        requestId: "req-3",
        kind: "answer",
        message: "3 schedules run today: Nightly test summary (09:00), …",
        error: null,
      },
    });
    expect(result.success).toBe(true);
  });

  it("accepts an error response with an error code", () => {
    const result = ScheduleAssistResponseSchema.safeParse({
      type: "schedule/assist/response",
      payload: {
        requestId: "req-4",
        kind: "error",
        message: "No LLM provider configured on this host.",
        error: "no_llm_provider",
      },
    });
    expect(result.success).toBe(true);
  });

  it("accepts error: null (daemon sends null)", () => {
    const result = ScheduleAssistResponseSchema.safeParse({
      type: "schedule/assist/response",
      payload: {
        requestId: "req-5",
        kind: "answer",
        message: "Nothing runs today.",
        proposal: null,
        error: null,
      },
    });
    expect(result.success).toBe(true);
  });

  it("rejects an unknown kind", () => {
    const result = ScheduleAssistResponseSchema.safeParse({
      type: "schedule/assist/response",
      payload: {
        requestId: "req-6",
        kind: "apply",
        error: null,
      },
    });
    expect(result.success).toBe(false);
  });
});
