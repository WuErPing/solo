import { z } from "zod";
import { AgentProviderSchema } from "../agent/provider-manifest.js";
import { AgentSessionConfigSchema } from "../../shared/agent-session-config.js";

export const LoopLogEntrySchema = z.object({
  seq: z.number().int().positive(),
  timestamp: z.string(),
  iteration: z.number().int().positive().nullable(),
  source: z.enum(["loop", "worker", "verifier", "verify-check"]),
  level: z.enum(["info", "error"]),
  text: z.string(),
});

export const LoopVerifyCheckResultSchema = z.object({
  command: z.string(),
  exitCode: z.number().int(),
  passed: z.boolean(),
  stdout: z.string(),
  stderr: z.string(),
  startedAt: z.string(),
  completedAt: z.string(),
});

export const LoopVerifyPromptResultSchema = z.object({
  passed: z.boolean(),
  reason: z.string(),
  verifierAgentId: z.string().nullable(),
  startedAt: z.string(),
  completedAt: z.string(),
});

export const LoopIterationRecordSchema = z.object({
  index: z.number().int().positive(),
  workerAgentId: z.string().nullable(),
  workerStartedAt: z.string(),
  workerCompletedAt: z.string().nullable(),
  verifierAgentId: z.string().nullable(),
  status: z.enum(["running", "succeeded", "failed", "stopped"]),
  workerOutcome: z.enum(["completed", "failed", "canceled"]).nullable(),
  failureReason: z.string().nullable(),
  verifyChecks: z.array(LoopVerifyCheckResultSchema),
  verifyPrompt: LoopVerifyPromptResultSchema.nullable(),
});

export const LoopRecordSchema = z.object({
  id: z.string(),
  templateID: z.string().optional(),
  name: z.string().nullable(),
  prompt: z.string(),
  cwd: z.string(),
  provider: AgentProviderSchema,
  // The legacy/deprecated provider+model overrides are serialized by the Go
  // daemon with `omitempty`, so the keys are absent (not null) when unset.
  // They must be `.optional()` here or the whole response fails validation
  // and gets silently dropped by the client.
  model: z.string().optional().nullable(),
  workerProvider: AgentProviderSchema.optional().nullable(),
  workerModel: z.string().optional().nullable(),
  verifierProvider: AgentProviderSchema.optional().nullable(),
  verifierModel: z.string().optional().nullable(),
  verifyPrompt: z.string().nullable(),
  verifyChecks: z.array(z.string()),
  archive: z.boolean(),
  sleepMs: z.number().int().nonnegative(),
  maxIterations: z.number().int().positive().nullable(),
  maxTimeMs: z.number().int().positive().nullable(),
  status: z.enum(["running", "succeeded", "failed", "stopped"]),
  createdAt: z.string(),
  updatedAt: z.string(),
  startedAt: z.string(),
  completedAt: z.string().nullable(),
  stopRequestedAt: z.string().nullable(),
  iterations: z.array(LoopIterationRecordSchema),
  logs: z.array(LoopLogEntrySchema),
  nextLogSeq: z.number().int().positive(),
  activeIteration: z.number().int().positive().nullable(),
  activeWorkerAgentId: z.string().nullable(),
  activeVerifierAgentId: z.string().nullable(),
  agentTemplate: AgentSessionConfigSchema.optional().nullable(),
  workerAgentTemplate: AgentSessionConfigSchema.optional().nullable(),
  verifierAgentTemplate: AgentSessionConfigSchema.optional().nullable(),
});

export const LoopListItemSchema = z.object({
  id: z.string(),
  templateID: z.string().optional(),
  name: z.string().nullable(),
  status: z.enum(["running", "succeeded", "failed", "stopped"]),
  cwd: z.string(),
  provider: AgentProviderSchema,
  // `model` is serialized with `omitempty` by the Go daemon (key absent when
  // unset), so it must be `.optional()` and not just `.nullable()`.
  model: z.string().optional().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
  activeIteration: z.number().int().positive().nullable(),
});

export const LoopRunRequestSchema = z.object({
  type: z.literal("loop/run"),
  requestId: z.string(),
  prompt: z.string().trim().min(1),
  cwd: z.string(),
  provider: AgentProviderSchema.optional(),
  model: z.string().trim().min(1).optional(),
  workerProvider: AgentProviderSchema.optional(),
  workerModel: z.string().trim().min(1).optional(),
  verifierProvider: AgentProviderSchema.optional(),
  verifierModel: z.string().trim().min(1).optional(),
  verifyPrompt: z.string().trim().min(1).optional(),
  verifyChecks: z.array(z.string().trim().min(1)).optional(),
  archive: z.boolean().optional(),
  name: z.string().trim().min(1).optional(),
  sleepMs: z.number().int().nonnegative().optional(),
  maxIterations: z.number().int().positive().optional(),
  maxTimeMs: z.number().int().positive().optional(),
  templateID: z.string().trim().min(1).optional(),
  agentTemplate: AgentSessionConfigSchema.optional().nullable(),
  workerAgentTemplate: AgentSessionConfigSchema.optional().nullable(),
  verifierAgentTemplate: AgentSessionConfigSchema.optional().nullable(),
});

export const LoopListRequestSchema = z.object({
  type: z.literal("loop/list"),
  requestId: z.string(),
});

export const LoopInspectRequestSchema = z.object({
  type: z.literal("loop/inspect"),
  requestId: z.string(),
  id: z.string().trim().min(1),
});

export const LoopLogsRequestSchema = z.object({
  type: z.literal("loop/logs"),
  requestId: z.string(),
  id: z.string().trim().min(1),
  afterSeq: z.number().int().nonnegative().optional(),
});

export const LoopStopRequestSchema = z.object({
  type: z.literal("loop/stop"),
  requestId: z.string(),
  id: z.string().trim().min(1),
});

export const LoopUpdateRequestSchema = z.object({
  type: z.literal("loop/update"),
  requestId: z.string(),
  id: z.string().trim().min(1),
  name: z.string().trim().min(1).optional(),
  archive: z.boolean().optional(),
  prompt: z.string().trim().min(1).optional(),
  cwd: z.string().trim().min(1).optional(),
  verifyChecks: z.array(z.string().trim().min(1)).optional(),
  maxIterations: z.number().int().positive().optional(),
});

export const LoopDeleteRequestSchema = z.object({
  type: z.literal("loop/delete"),
  requestId: z.string(),
  id: z.string().trim().min(1),
});

export const LoopRunResponseSchema = z.object({
  type: z.literal("loop/run/response"),
  payload: z.object({
    requestId: z.string(),
    loop: LoopRecordSchema.nullable(),
    error: z.string().nullable(),
  }),
});

export const LoopListResponseSchema = z.object({
  type: z.literal("loop/list/response"),
  payload: z.object({
    requestId: z.string(),
    loops: z.array(LoopListItemSchema),
    error: z.string().nullable(),
  }),
});

export const LoopInspectResponseSchema = z.object({
  type: z.literal("loop/inspect/response"),
  payload: z.object({
    requestId: z.string(),
    loop: LoopRecordSchema.nullable(),
    error: z.string().nullable(),
  }),
});

export const LoopLogsResponseSchema = z.object({
  type: z.literal("loop/logs/response"),
  payload: z.object({
    requestId: z.string(),
    loop: LoopRecordSchema.nullable(),
    entries: z.array(LoopLogEntrySchema),
    nextCursor: z.number().int().nonnegative(),
    error: z.string().nullable(),
  }),
});

export const LoopStopResponseSchema = z.object({
  type: z.literal("loop/stop/response"),
  payload: z.object({
    requestId: z.string(),
    loop: LoopRecordSchema.nullable(),
    error: z.string().nullable(),
  }),
});

export const LoopUpdateResponseSchema = z.object({
  type: z.literal("loop/update/response"),
  payload: z.object({
    requestId: z.string(),
    loop: LoopRecordSchema.nullable(),
    error: z.string().nullable(),
  }),
});

export const LoopDeleteResponseSchema = z.object({
  type: z.literal("loop/delete/response"),
  payload: z.object({
    requestId: z.string(),
    id: z.string(),
    error: z.string().nullable(),
  }),
});

export const LoopTemplateSummarySchema = z.object({
  id: z.string(),
  name: z.string(),
  cwd: z.string(),
  provider: z.string().optional(),
  model: z.string().optional(),
  instanceCount: z.number().int().nonnegative(),
  lastRunAt: z.string().optional(),
  latestStatus: z.string().optional(),
});

export const LoopTemplateListRequestSchema = z.object({
  type: z.literal("loop/template/list"),
  requestId: z.string(),
});

export const LoopTemplateListResponseSchema = z.object({
  type: z.literal("loop/template/list/response"),
  payload: z.object({
    requestId: z.string(),
    templates: z.array(LoopTemplateSummarySchema),
    error: z.string().nullable(),
  }),
});

export const LoopTemplateGetRequestSchema = z.object({
  type: z.literal("loop/template/get"),
  requestId: z.string(),
  templateID: z.string().trim().min(1),
});

export const LoopTemplateGetResponseSchema = z.object({
  type: z.literal("loop/template/get/response"),
  payload: z.object({
    requestId: z.string(),
    template: LoopTemplateSummarySchema.nullable(),
    instances: z.array(LoopListItemSchema),
    latestRecord: LoopRecordSchema.nullable().optional(),
    error: z.string().nullable(),
  }),
});

export const LoopTemplateDeleteRequestSchema = z.object({
  type: z.literal("loop/template/delete"),
  requestId: z.string(),
  templateID: z.string(),
});

export const LoopTemplateDeleteResponseSchema = z.object({
  type: z.literal("loop/template/delete/response"),
  payload: z.object({
    requestId: z.string(),
    templateID: z.string(),
    error: z.string().nullable(),
  }),
});

export type LoopLogEntry = z.infer<typeof LoopLogEntrySchema>;
export type LoopVerifyCheckResult = z.infer<typeof LoopVerifyCheckResultSchema>;
export type LoopVerifyPromptResult = z.infer<typeof LoopVerifyPromptResultSchema>;
export type LoopIterationRecord = z.infer<typeof LoopIterationRecordSchema>;
export type LoopRecord = z.infer<typeof LoopRecordSchema>;
export type LoopListItem = z.infer<typeof LoopListItemSchema>;
export type LoopTemplateSummary = z.infer<typeof LoopTemplateSummarySchema>;
export type LoopRunRequest = z.infer<typeof LoopRunRequestSchema>;
export type LoopListRequest = z.infer<typeof LoopListRequestSchema>;
export type LoopInspectRequest = z.infer<typeof LoopInspectRequestSchema>;
export type LoopLogsRequest = z.infer<typeof LoopLogsRequestSchema>;
export type LoopStopRequest = z.infer<typeof LoopStopRequestSchema>;
export type LoopRunResponse = z.infer<typeof LoopRunResponseSchema>;
export type LoopListResponse = z.infer<typeof LoopListResponseSchema>;
export type LoopInspectResponse = z.infer<typeof LoopInspectResponseSchema>;
export type LoopLogsResponse = z.infer<typeof LoopLogsResponseSchema>;
export type LoopStopResponse = z.infer<typeof LoopStopResponseSchema>;
export type LoopUpdateRequest = z.infer<typeof LoopUpdateRequestSchema>;
export type LoopDeleteRequest = z.infer<typeof LoopDeleteRequestSchema>;
export type LoopUpdateResponse = z.infer<typeof LoopUpdateResponseSchema>;
export type LoopDeleteResponse = z.infer<typeof LoopDeleteResponseSchema>;
export type LoopTemplateListRequest = z.infer<typeof LoopTemplateListRequestSchema>;
export type LoopTemplateListResponse = z.infer<typeof LoopTemplateListResponseSchema>;
export type LoopTemplateGetRequest = z.infer<typeof LoopTemplateGetRequestSchema>;
export type LoopTemplateGetResponse = z.infer<typeof LoopTemplateGetResponseSchema>;
export type LoopTemplateDeleteRequest = z.infer<typeof LoopTemplateDeleteRequestSchema>;
export type LoopTemplateDeleteResponse = z.infer<typeof LoopTemplateDeleteResponseSchema>;
