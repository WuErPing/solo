import { z } from "zod";
import { AgentProviderSchema } from "../agent/provider-manifest.js";

export const ScheduleStatusSchema = z.enum(["active", "paused", "completed"]);
export type ScheduleStatus = z.infer<typeof ScheduleStatusSchema>;

export const ScheduleCadenceSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("every"),
    everyMs: z.number().int().positive(),
    timezone: z.string().optional(),
  }),
  z.object({
    type: z.literal("cron"),
    expression: z.string().trim().min(1),
    timezone: z.string().optional(),
  }),
]);
export type ScheduleCadence = z.infer<typeof ScheduleCadenceSchema>;

export const ScheduleNewAgentConfigSchema = z.object({
  provider: AgentProviderSchema,
  cwd: z.string().trim().min(1),
  modeId: z.string().trim().min(1).optional(),
  model: z.string().trim().min(1).optional(),
  thinkingOptionId: z.string().trim().min(1).optional(),
  title: z.string().trim().min(1).nullable().optional(),
  approvalPolicy: z.string().trim().min(1).optional(),
  sandboxMode: z.string().trim().min(1).optional(),
  networkAccess: z.boolean().optional(),
  webSearch: z.boolean().optional(),
  extra: z
    .object({
      codex: z.record(z.string(), z.unknown()).optional(),
      claude: z.record(z.string(), z.unknown()).optional(),
    })
    .partial()
    .optional(),
  systemPrompt: z.string().optional(),
  mcpServers: z.record(z.string(), z.unknown()).optional(),
});

// Discriminated union of the three supported targets plus a synthetic
// "unknown" type for schedules persisted with an obsolete target type (e.g.
// an old experimental "clone-agent" type). They still appear in the list so
// the user can open them and re-save with a supported target.
const ScheduleTargetDiscriminatedSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("agent"),
    agentId: z.string().min(1),
  }),
  z.object({
    type: z.literal("provider"),
    providerId: AgentProviderSchema,
  }),
  z.object({
    type: z.literal("new-agent"),
    config: ScheduleNewAgentConfigSchema,
  }),
  z.object({
    type: z.literal("unknown"),
  }),
]);

export const ScheduleTargetSchema = z.preprocess((value) => {
  if (typeof value !== "object" || value === null) {
    return value;
  }
  const target = value as {
    type?: unknown;
    agentId?: unknown;
    providerId?: unknown;
    config?: unknown;
  };
  const type = target.type;

  // Known type but missing required fields → fall back to "unknown" so the
  // schedule still appears in the list and can be repaired by the user.
  if (type === "agent") {
    return typeof target.agentId === "string" && target.agentId.length > 0
      ? value
      : { type: "unknown" };
  }
  if (type === "provider") {
    return typeof target.providerId === "string" && target.providerId.length > 0
      ? value
      : { type: "unknown" };
  }
  if (type === "new-agent") {
    return typeof target.config === "object" && target.config !== null
      ? value
      : { type: "unknown" };
  }
  return { type: "unknown" };
}, ScheduleTargetDiscriminatedSchema);
export type ScheduleTarget = z.infer<typeof ScheduleTargetSchema>;

export const ScheduleRunSchema = z.object({
  id: z.string(),
  scheduledFor: z.string(),
  startedAt: z.string(),
  endedAt: z.string().nullable(),
  status: z.enum(["running", "succeeded", "failed"]),
  agentId: z.string().min(1).nullable(),
  output: z.string().nullable(),
  error: z.string().nullable(),
});
export type ScheduleRun = z.infer<typeof ScheduleRunSchema>;

export const StoredScheduleSchema = z.object({
  id: z.string(),
  name: z.string().nullable(),
  prompt: z.string().min(1),
  cadence: ScheduleCadenceSchema,
  target: ScheduleTargetSchema,
  cwd: z.string().nullable().optional(),
  status: ScheduleStatusSchema,
  createdAt: z.string(),
  updatedAt: z.string(),
  nextRunAt: z.string().nullable(),
  lastRunAt: z.string().nullable(),
  pausedAt: z.string().nullable(),
  expiresAt: z.string().nullable(),
  maxRuns: z.number().int().positive().nullable(),
  runs: z.array(ScheduleRunSchema),
});
export type StoredSchedule = z.infer<typeof StoredScheduleSchema>;

export const ScheduleSummarySchema = StoredScheduleSchema.omit({
  runs: true,
});
export type ScheduleSummary = z.infer<typeof ScheduleSummarySchema>;

export interface CreateScheduleInput {
  name?: string | null;
  prompt: string;
  cadence: ScheduleCadence;
  target: ScheduleTarget;
  cwd?: string | null;
  maxRuns?: number | null;
  expiresAt?: string | null;
}

export interface ScheduleExecutionResult {
  agentId: string | null;
  output: string | null;
}
