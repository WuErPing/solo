import { z } from "zod";
import {
  SoloConfigRawSchema,
  SoloLifecycleCommandRawSchema,
  SoloScriptEntryRawSchema,
  SoloWorktreeConfigRawSchema,
  SoloConfigRevisionSchema,
  ProjectConfigRpcErrorSchema,
  type SoloConfigRaw,
  type SoloConfigRevision,
  type SoloScriptEntryRaw,
  type ProjectConfigRpcError,
} from "../utils/solo-config-schema.js";
import { AgentProviderSchema } from "../server/agent/provider-manifest.js";

export {
  SoloConfigRawSchema,
  SoloLifecycleCommandRawSchema,
  SoloScriptEntryRawSchema,
  SoloWorktreeConfigRawSchema,
  type SoloConfigRaw,
  type SoloConfigRevision,
  type SoloScriptEntryRaw,
  type ProjectConfigRpcError,
};

// ---------------------------------------------------------------------------
// Mutable daemon config schemas (shared between server store and client)
// ---------------------------------------------------------------------------

const MutableDaemonProviderModelSchema = z
  .object({
    id: z.string().min(1),
    label: z.string().min(1),
    description: z.string().optional(),
    isDefault: z.boolean().optional(),
  })
  .passthrough();

const MutableDaemonProviderConfigSchema = z
  .object({
    enabled: z.boolean().optional(),
    additionalModels: z.array(MutableDaemonProviderModelSchema).optional(),
  })
  .passthrough();

const LLMProviderModelSchema = z
  .object({
    id: z.string().min(1),
    label: z.string().min(1).optional(),
    description: z.string().optional(),
    isDefault: z.boolean().optional(),
  })
  .passthrough();

export const LLMProviderConfigSchema = z
  .object({
    id: z.string().min(1),
    label: z.string().optional(),
    description: z.string().optional(),
    enabled: z.boolean().optional(),
    baseURL: z.string().optional(),
    apiKey: z.string().optional(),
    models: z.array(LLMProviderModelSchema).optional(),
  })
  .passthrough();

export type LLMProviderConfig = z.infer<typeof LLMProviderConfigSchema>;
export type LLMProviderModel = z.infer<typeof LLMProviderModelSchema>;

export const MutableDaemonConfigSchema = z
  .object({
    mcp: z
      .object({
        injectIntoAgents: z.boolean(),
      })
      .passthrough(),
    providers: z.record(z.string(), MutableDaemonProviderConfigSchema).default({}),
    llmProviders: z.array(LLMProviderConfigSchema).default([]),
    tmuxAgentNames: z.array(z.string()).default([]),
  })
  .passthrough();

export const MutableDaemonConfigPatchSchema = z
  .object({
    mcp: MutableDaemonConfigSchema.shape.mcp.partial().optional(),
    providers: z
      .record(z.string(), MutableDaemonProviderConfigSchema.partial().passthrough())
      .optional(),
    llmProviders: z.array(LLMProviderConfigSchema).optional(),
    tmuxAgentNames: z.array(z.string()).optional(),
  })
  .partial()
  .passthrough();

export type MutableDaemonConfig = z.infer<typeof MutableDaemonConfigSchema>;
export type MutableDaemonConfigPatch = z.infer<typeof MutableDaemonConfigPatchSchema>;

// ---------------------------------------------------------------------------
// Infra request schemas
// ---------------------------------------------------------------------------

export const GetDaemonConfigRequestMessageSchema = z.object({
  type: z.literal("get_daemon_config_request"),
  requestId: z.string(),
});

export const SetDaemonConfigRequestMessageSchema = z.object({
  type: z.literal("set_daemon_config_request"),
  requestId: z.string(),
  config: MutableDaemonConfigPatchSchema,
});

export const ReadProjectConfigRequestMessageSchema = z.object({
  type: z.literal("read_project_config_request"),
  requestId: z.string(),
  repoRoot: z.string(),
});

export const WriteProjectConfigRequestMessageSchema = z.object({
  type: z.literal("write_project_config_request"),
  requestId: z.string(),
  repoRoot: z.string(),
  config: SoloConfigRawSchema,
  expectedRevision: SoloConfigRevisionSchema.nullable(),
});

export const RestartServerRequestMessageSchema = z.object({
  type: z.literal("restart_server_request"),
  reason: z.string().optional(),
  requestId: z.string(),
});

export const ShutdownServerRequestMessageSchema = z.object({
  type: z.literal("shutdown_server_request"),
  requestId: z.string(),
});

export const ClientHeartbeatMessageSchema = z.object({
  type: z.literal("client_heartbeat"),
  deviceType: z.enum(["web", "mobile"]),
  focusedAgentId: z.string().nullable(),
  lastActivityAt: z.string(),
  appVisible: z.boolean(),
  appVisibilityChangedAt: z.string().optional(),
});

export const PingMessageSchema = z.object({
  type: z.literal("ping"),
  requestId: z.string(),
  clientSentAt: z.number().int().optional(),
});

export const ListCommandsDraftConfigSchema = z.object({
  provider: AgentProviderSchema,
  cwd: z.string(),
  modeId: z.string().optional(),
  model: z.string().optional(),
  thinkingOptionId: z.string().optional(),
  featureValues: z.record(z.string(), z.unknown()).optional(),
});

export const ListCommandsRequestSchema = z.object({
  type: z.literal("list_commands_request"),
  agentId: z.string(),
  draftConfig: ListCommandsDraftConfigSchema.optional(),
  requestId: z.string(),
});

export const RegisterPushTokenMessageSchema = z.object({
  type: z.literal("register_push_token"),
  token: z.string(),
});

// ============================================================================
// Session Outbound Messages (infra/base)
// ============================================================================

export const ActivityLogPayloadSchema = z.object({
  id: z.string(),
  timestamp: z.coerce.date(),
  type: z.enum(["transcript", "assistant", "tool_call", "tool_result", "error", "system"]),
  content: z.string(),
  metadata: z.record(z.string(), z.unknown()).optional(),
});

export const ActivityLogMessageSchema = z.object({
  type: z.literal("activity_log"),
  payload: ActivityLogPayloadSchema,
});

export const AssistantChunkMessageSchema = z.object({
  type: z.literal("assistant_chunk"),
  payload: z.object({
    chunk: z.string(),
  }),
});

export const ServerCapabilityStateSchema = z.object({
  enabled: z.boolean(),
  reason: z.string(),
});

export const ServerVoiceCapabilitiesSchema = z.object({
  dictation: ServerCapabilityStateSchema,
  voice: ServerCapabilityStateSchema,
});

export const ServerCapabilitiesSchema = z
  .object({
    voice: ServerVoiceCapabilitiesSchema.optional(),
  })
  .passthrough();

const ServerInfoHostnameSchema = z.unknown().transform((value): string | null => {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
});

const ServerInfoVersionSchema = z.unknown().transform((value): string | null => {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
});

const ServerCapabilitiesFromUnknownSchema = z
  .unknown()
  .optional()
  .transform((value): z.infer<typeof ServerCapabilitiesSchema> | undefined => {
    if (value === undefined) {
      return undefined;
    }
    const parsed = ServerCapabilitiesSchema.safeParse(value);
    if (!parsed.success) {
      return undefined;
    }
    return parsed.data;
  });

export const ServerInfoStatusPayloadSchema = z
  .object({
    status: z.literal("server_info"),
    serverId: z.string().trim().min(1),
    hostname: ServerInfoHostnameSchema.optional(),
    version: ServerInfoVersionSchema.optional(),
    capabilities: ServerCapabilitiesFromUnknownSchema,
    // COMPAT(providersSnapshot): added in v0.1.48, remove gating when all clients use snapshot
    features: z
      .object({
        providersSnapshot: z.boolean().optional(),
      })
      .optional(),
  })
  .passthrough()
  .transform((payload) => ({
    ...payload,
    hostname: payload.hostname ?? null,
    version: payload.version ?? null,
  }));

export const StatusMessageSchema = z.object({
  type: z.literal("status"),
  payload: z
    .object({
      status: z.string(),
    })
    .passthrough(), // Allow additional fields
});

export const PongMessageSchema = z.object({
  type: z.literal("pong"),
  payload: z.object({
    requestId: z.string(),
    clientSentAt: z.number().int().optional(),
    serverReceivedAt: z.number().int(),
    serverSentAt: z.number().int(),
  }),
});

export const RpcErrorMessageSchema = z.object({
  type: z.literal("rpc_error"),
  payload: z.object({
    requestId: z.string(),
    requestType: z.string().optional(),
    error: z.string(),
    code: z.string().optional(),
  }),
});

export const RestartRequestedStatusPayloadSchema = z.object({
  status: z.literal("restart_requested"),
  clientId: z.string(),
  reason: z.string().optional(),
  requestId: z.string(),
});

export const ShutdownRequestedStatusPayloadSchema = z.object({
  status: z.literal("shutdown_requested"),
  clientId: z.string(),
  requestId: z.string(),
});

export const DaemonConfigChangedStatusPayloadSchema = z
  .object({
    status: z.literal("daemon_config_changed"),
    config: MutableDaemonConfigSchema,
  })
  .passthrough();

export const ArtifactMessageSchema = z.object({
  type: z.literal("artifact"),
  payload: z.object({
    type: z.enum(["markdown", "diff", "image", "code"]),
    id: z.string(),
    title: z.string(),
    content: z.string(),
    isBase64: z.boolean(),
  }),
});

// ---------------------------------------------------------------------------
// Infra response schemas
// ---------------------------------------------------------------------------

export const GetDaemonConfigResponseMessageSchema = z.object({
  type: z.literal("get_daemon_config_response"),
  payload: z
    .object({
      requestId: z.string(),
      config: MutableDaemonConfigSchema,
    })
    .passthrough(),
});

export const SetDaemonConfigResponseMessageSchema = z.object({
  type: z.literal("set_daemon_config_response"),
  payload: z
    .object({
      requestId: z.string(),
      config: MutableDaemonConfigSchema,
    })
    .passthrough(),
});

export const ReadProjectConfigResponseMessageSchema = z.object({
  type: z.literal("read_project_config_response"),
  payload: z.discriminatedUnion("ok", [
    z.object({
      requestId: z.string(),
      repoRoot: z.string(),
      ok: z.literal(true),
      config: SoloConfigRawSchema.nullable(),
      revision: SoloConfigRevisionSchema.nullable(),
    }),
    z.object({
      requestId: z.string(),
      repoRoot: z.string(),
      ok: z.literal(false),
      error: ProjectConfigRpcErrorSchema,
    }),
  ]),
});

export const WriteProjectConfigResponseMessageSchema = z.object({
  type: z.literal("write_project_config_response"),
  payload: z.discriminatedUnion("ok", [
    z.object({
      requestId: z.string(),
      repoRoot: z.string(),
      ok: z.literal(true),
      config: SoloConfigRawSchema,
      revision: SoloConfigRevisionSchema,
    }),
    z.object({
      requestId: z.string(),
      repoRoot: z.string(),
      ok: z.literal(false),
      error: ProjectConfigRpcErrorSchema,
    }),
  ]),
});

const AgentSlashCommandSchema = z.object({
  name: z.string(),
  description: z.string(),
  argumentHint: z.string(),
});

export const ListCommandsResponseSchema = z.object({
  type: z.literal("list_commands_response"),
  payload: z.object({
    agentId: z.string(),
    commands: z.array(AgentSlashCommandSchema),
    error: z.string().nullable(),
    requestId: z.string(),
  }),
});

// ============================================================================
// WebSocket Level Messages (WS-only, not session messages)
// ============================================================================

// WebSocket-only messages (not session messages)
export const WSPingMessageSchema = z.object({
  type: z.literal("ping"),
});

export const WSPongMessageSchema = z.object({
  type: z.literal("pong"),
});

export const WSHelloMessageSchema = z.object({
  type: z.literal("hello"),
  clientId: z.string().min(1),
  clientType: z.enum(["mobile", "browser", "cli", "mcp"]),
  protocolVersion: z.number().int(),
  appVersion: z.string().optional(),
  capabilities: z
    .object({
      voice: z.boolean().optional(),
      pushNotifications: z.boolean().optional(),
    })
    .passthrough()
    .optional(),
});

export const WSRecordingStateMessageSchema = z.object({
  type: z.literal("recording_state"),
  isRecording: z.boolean(),
});

// ---------------------------------------------------------------------------
// Type exports
// ---------------------------------------------------------------------------

export type ActivityLogMessage = z.infer<typeof ActivityLogMessageSchema>;
export type AssistantChunkMessage = z.infer<typeof AssistantChunkMessageSchema>;
export type StatusMessage = z.infer<typeof StatusMessageSchema>;
export type ServerCapabilityState = z.infer<typeof ServerCapabilityStateSchema>;
export type ServerVoiceCapabilities = z.infer<typeof ServerVoiceCapabilitiesSchema>;
export type ServerCapabilities = z.infer<typeof ServerCapabilitiesSchema>;
export type ServerInfoStatusPayload = z.infer<typeof ServerInfoStatusPayloadSchema>;
export type RpcErrorMessage = z.infer<typeof RpcErrorMessageSchema>;
export type ArtifactMessage = z.infer<typeof ArtifactMessageSchema>;
export type ActivityLogPayload = z.infer<typeof ActivityLogPayloadSchema>;
export type RestartServerRequestMessage = z.infer<typeof RestartServerRequestMessageSchema>;
export type ShutdownServerRequestMessage = z.infer<typeof ShutdownServerRequestMessageSchema>;
export type ClientHeartbeatMessage = z.infer<typeof ClientHeartbeatMessageSchema>;
export type ListCommandsRequest = z.infer<typeof ListCommandsRequestSchema>;
export type ListCommandsResponse = z.infer<typeof ListCommandsResponseSchema>;
export type RegisterPushTokenMessage = z.infer<typeof RegisterPushTokenMessageSchema>;
export type WSHelloMessage = z.infer<typeof WSHelloMessageSchema>;
