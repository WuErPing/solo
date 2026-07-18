import type { z } from "zod";
import {
  AgentRefreshedStatusPayloadSchema,
  parseServerInfoStatusPayload,
  RestartRequestedStatusPayloadSchema,
  ShutdownRequestedStatusPayloadSchema,
  SessionInboundMessageSchema,
  type ServerInfoStatusPayload,
  WSOutboundMessageSchema,
} from "../shared/messages.js";
import type {
  AgentStreamEventPayload,
  AgentSnapshotPayload,
  ProjectPlacementPayload,
  AgentPermissionResolvedMessage,
  CreateAgentRequestMessage,
  CreateSoloWorktreeRequest,
  FileDownloadTokenResponse,
  FileExplorerResponse,
  FetchAgentTimelineResponseMessage,
  GitSetupOptions,
  CheckoutStatusResponse,
  CheckoutCommitResponse,
  CheckoutMergeResponse,
  CheckoutMergeFromBaseResponse,
  CheckoutPullResponse,
  CheckoutPushResponse,
  CheckoutPrCreateResponse,
  CheckoutPrStatusResponse,
  PullRequestTimelineResponse,
  CheckoutSwitchBranchResponse,
  StashSaveResponse,
  StashPopResponse,
  StashListResponse,
  ValidateBranchResponse,
  BranchSuggestionsResponse,
  GitHubSearchResponse,
  GitHubSearchRequest,
  DirectorySuggestionsResponse,
  SoloWorktreeListResponse,
  SoloWorktreeArchiveResponse,
  ProjectIconResponse,
  ListAvailableEditorsResponseMessage,
  OpenInEditorResponseMessage,
  OpenProjectResponseMessage,
  ArchiveWorkspaceResponseMessage,
  RemoveProjectResponseMessage,
  WorkspaceSetupStatusResponseMessage,
  ListCommandsResponse,
  ListProviderFeaturesResponseMessage,
  ListProviderModelsResponseMessage,
  ListProviderModesResponseMessage,
  ListAvailableProvidersResponse,
  GetProvidersSnapshotResponseMessage,
  RefreshProvidersSnapshotResponseMessage,
  ProviderDiagnosticResponseMessage,
  ListTerminalsResponse,
  CreateTerminalResponse,
  SubscribeTerminalResponse,
  TerminalState,
  CloseItemsResponse,
  KillTerminalResponse,
  CaptureTerminalResponse,
  TerminalInput,
  SessionInboundMessage,
  SessionOutboundMessage,
  SendAgentMessageRequest,
  EditorTargetId,
  SoloConfigRaw,
  SoloConfigRevision,
} from "../shared/messages.js";
import type {
  AgentPermissionRequest,
  AgentPermissionResponse,
  AgentPersistenceHandle,
  AgentProvider,
  AgentSessionConfig,
} from "../server/agent/agent-sdk-types.js";
import type { MutableDaemonConfig, MutableDaemonConfigPatch } from "../shared/messages.js";
import type { AgentSessionConfig as WireAgentSessionConfig } from "../shared/agent-session-config.js";
import { isRelayClientWebSocketUrl } from "../shared/daemon-endpoints.js";
import { asUint8Array } from "../shared/terminal-stream-protocol.js";
import {
  createRelayE2eeTransportFactory,
  createWebSocketTransportFactory,
  decodeMessageData,
  defaultWebSocketFactory,
  describeTransportClose,
  describeTransportError,
  type DaemonTransport,
  type DaemonTransportFactory,
  type WebSocketFactory,
} from "./daemon-client-transport.js";
import { DaemonClientRuntimeMetrics } from "./daemon-client-runtime-metrics.js";
import { AgentRpc } from "./agent-rpc.js";
import { ScheduleRpc } from "./schedule-rpc.js";
import { ChatRpc } from "./chat-rpc.js";
import { WorkspaceRpc } from "./workspace-rpc.js";
import { GitRpc } from "./git-rpc.js";
import { TerminalRpc } from "./terminal-rpc.js";

import { consoleLogger, toErrorInfo, type Logger } from "../shared/logger.js";

export type { Logger } from "../shared/logger.js";

const perfNow: () => number =
  typeof performance !== "undefined" && typeof performance.now === "function"
    ? () => performance.now()
    : () => Date.now();

export type {
  DaemonTransport,
  DaemonTransportFactory,
  WebSocketFactory,
  WebSocketLike,
} from "./daemon-client-transport.js";

export type TerminalStreamEvent =
  | { terminalId: string; type: "output"; data: Uint8Array }
  | { terminalId: string; type: "snapshot"; state: TerminalState };

export type ConnectionState =
  | { status: "idle" }
  | { status: "connecting"; attempt: number }
  | { status: "connected" }
  | { status: "disconnected"; reason?: string }
  | { status: "disposed" };

export type DaemonEvent =
  | {
      type: "agent_update";
      agentId: string;
      payload: Extract<SessionOutboundMessage, { type: "agent_update" }>["payload"];
    }
  | {
      type: "workspace_update";
      workspaceId: string;
      payload: Extract<SessionOutboundMessage, { type: "workspace_update" }>["payload"];
    }
  | {
      type: "workspace_setup_progress";
      workspaceId: string;
      payload: Extract<SessionOutboundMessage, { type: "workspace_setup_progress" }>["payload"];
    }
  | {
      type: "agent_stream";
      agentId: string;
      event: AgentStreamEventPayload;
      timestamp: string;
      seq?: number;
      epoch?: string;
    }
  | { type: "status"; payload: { status: string } & Record<string, unknown> }
  | { type: "agent_deleted"; agentId: string }
  | {
      type: "agent_permission_request";
      agentId: string;
      request: AgentPermissionRequest;
    }
  | {
      type: "agent_permission_resolved";
      agentId: string;
      requestId: string;
      resolution: AgentPermissionResponse;
    }
  | {
      type: "providers_snapshot_update";
      payload: Extract<SessionOutboundMessage, { type: "providers_snapshot_update" }>["payload"];
    }
  | { type: "error"; message: string };

export type DaemonEventHandler = (event: DaemonEvent) => void;

export interface DaemonClientConfig {
  url: string;
  clientId: string;
  clientType?: "mobile" | "browser" | "cli" | "mcp";
  appVersion?: string;
  runtimeGeneration?: number | null;
  authHeader?: string;
  suppressSendErrors?: boolean;
  transportFactory?: DaemonTransportFactory;
  webSocketFactory?: WebSocketFactory;
  logger?: Logger;
  connectTimeoutMs?: number;
  e2ee?: {
    enabled?: boolean;
    daemonPublicKeyB64?: string;
  };
  reconnect?: {
    enabled?: boolean;
    baseDelayMs?: number;
    maxDelayMs?: number;
  };
  runtimeMetricsIntervalMs?: number;
  runtimeMetricsWindowMs?: number;
}

export interface SendMessageOptions {
  messageId?: string;
  images?: Array<{ data: string; mimeType: string }>;
  attachments?: SendAgentMessageRequest["attachments"];
}

type AgentConfigOverrides = Partial<Omit<AgentSessionConfig, "provider" | "cwd">>;

export interface CreateAgentRequestOptions extends AgentConfigOverrides {
  config?: AgentSessionConfig;
  provider?: AgentProvider;
  cwd?: string;
  workspaceId?: string;
  initialPrompt?: string;
  clientMessageId?: string;
  outputSchema?: Record<string, unknown>;
  images?: CreateAgentRequestMessage["images"];
  attachments?: CreateAgentRequestMessage["attachments"];
  git?: GitSetupOptions;
  worktreeName?: string;
  requestId?: string;
  labels?: Record<string, string>;
}

export interface CreateSoloWorktreeInput extends Pick<
  CreateSoloWorktreeRequest,
  "cwd" | "worktreeSlug" | "attachments" | "refName" | "action" | "githubPrNumber"
> {}

type CheckoutStatusPayload = CheckoutStatusResponse["payload"];
type SubscribeCheckoutDiffPayload = Extract<
  SessionOutboundMessage,
  { type: "subscribe_checkout_diff_response" }
>["payload"];
type CheckoutDiffPayload = Omit<SubscribeCheckoutDiffPayload, "subscriptionId">;
type CheckoutCommitPayload = CheckoutCommitResponse["payload"];
type CheckoutMergePayload = CheckoutMergeResponse["payload"];
type CheckoutMergeFromBasePayload = CheckoutMergeFromBaseResponse["payload"];
type CheckoutPullPayload = CheckoutPullResponse["payload"];
type CheckoutPushPayload = CheckoutPushResponse["payload"];
type CheckoutPrCreatePayload = CheckoutPrCreateResponse["payload"];
type CheckoutPrStatusPayload = CheckoutPrStatusResponse["payload"];
type PullRequestTimelinePayload = PullRequestTimelineResponse["payload"];
type CheckoutSwitchBranchPayload = CheckoutSwitchBranchResponse["payload"];
type StashSavePayload = StashSaveResponse["payload"];
type StashPopPayload = StashPopResponse["payload"];
type StashListPayload = StashListResponse["payload"];
type ValidateBranchPayload = ValidateBranchResponse["payload"];
type BranchSuggestionsPayload = BranchSuggestionsResponse["payload"];
type GitHubSearchPayload = GitHubSearchResponse["payload"];
type DirectorySuggestionsPayload = DirectorySuggestionsResponse["payload"];
type SoloWorktreeListPayload = SoloWorktreeListResponse["payload"];
type SoloWorktreeArchivePayload = SoloWorktreeArchiveResponse["payload"];
type CreateSoloWorktreePayload = Extract<
  SessionOutboundMessage,
  { type: "create_solo_worktree_response" }
>["payload"];
type FileExplorerPayload = FileExplorerResponse["payload"];
type FileDownloadTokenPayload = FileDownloadTokenResponse["payload"];
type ListProviderFeaturesPayload = ListProviderFeaturesResponseMessage["payload"];
type ListProviderModelsPayload = ListProviderModelsResponseMessage["payload"];
type ListProviderModesPayload = ListProviderModesResponseMessage["payload"];
type ListAvailableProvidersPayload = ListAvailableProvidersResponse["payload"];
type GetProvidersSnapshotPayload = GetProvidersSnapshotResponseMessage["payload"];
type RefreshProvidersSnapshotPayload = RefreshProvidersSnapshotResponseMessage["payload"];
type ProviderDiagnosticPayload = ProviderDiagnosticResponseMessage["payload"];
type ReadProjectConfigPayload = Extract<
  SessionOutboundMessage,
  { type: "read_project_config_response" }
>["payload"];
type WriteProjectConfigPayload = Extract<
  SessionOutboundMessage,
  { type: "write_project_config_response" }
>["payload"];
type ListCommandsPayload = ListCommandsResponse["payload"];
type ListCommandsDraftConfig = Pick<
  AgentSessionConfig,
  "provider" | "cwd" | "modeId" | "model" | "thinkingOptionId" | "featureValues"
>;
export interface WriteProjectConfigInput {
  repoRoot: string;
  config: SoloConfigRaw;
  expectedRevision: SoloConfigRevision | null;
  requestId?: string;
}
interface ListCommandsOptions {
  requestId?: string;
  draftConfig?: ListCommandsDraftConfig;
}
type SetVoiceModePayload = Extract<
  SessionOutboundMessage,
  { type: "set_voice_mode_response" }
>["payload"];
type AgentPermissionResolvedPayload = AgentPermissionResolvedMessage["payload"];
type ListTerminalsPayload = ListTerminalsResponse["payload"];
type CreateTerminalPayload = CreateTerminalResponse["payload"];
type SubscribeTerminalPayload = SubscribeTerminalResponse["payload"];
type CloseItemsPayload = CloseItemsResponse["payload"];
type KillTerminalPayload = KillTerminalResponse["payload"];
type CaptureTerminalPayload = CaptureTerminalResponse["payload"];
type ChatCreatePayload = Extract<
  SessionOutboundMessage,
  { type: "chat/create/response" }
>["payload"];
type ChatListPayload = Extract<SessionOutboundMessage, { type: "chat/list/response" }>["payload"];
type ChatInspectPayload = Extract<
  SessionOutboundMessage,
  { type: "chat/inspect/response" }
>["payload"];
type ChatDeletePayload = Extract<
  SessionOutboundMessage,
  { type: "chat/delete/response" }
>["payload"];
type ChatPostPayload = Extract<SessionOutboundMessage, { type: "chat/post/response" }>["payload"];
type ChatReadPayload = Extract<SessionOutboundMessage, { type: "chat/read/response" }>["payload"];
type ChatWaitPayload = Extract<SessionOutboundMessage, { type: "chat/wait/response" }>["payload"];
type LoopRunPayload = Extract<SessionOutboundMessage, { type: "loop/run/response" }>["payload"];
type LoopListPayload = Extract<SessionOutboundMessage, { type: "loop/list/response" }>["payload"];
type LoopInspectPayload = Extract<
  SessionOutboundMessage,
  { type: "loop/inspect/response" }
>["payload"];
type LoopLogsPayload = Extract<SessionOutboundMessage, { type: "loop/logs/response" }>["payload"];
type LoopStopPayload = Extract<SessionOutboundMessage, { type: "loop/stop/response" }>["payload"];
type LoopUpdatePayload = Extract<SessionOutboundMessage, { type: "loop/update/response" }>["payload"];
type LoopDeletePayload = Extract<SessionOutboundMessage, { type: "loop/delete/response" }>["payload"];
type LoopTemplateListPayload = Extract<SessionOutboundMessage, { type: "loop/template/list/response" }>["payload"];
type LoopTemplateGetPayload = Extract<SessionOutboundMessage, { type: "loop/template/get/response" }>["payload"];
type LoopTemplateDeletePayload = Extract<SessionOutboundMessage, { type: "loop/template/delete/response" }>["payload"];
type ScheduleCreatePayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/create/response" }
>["payload"];
type ScheduleListPayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/list/response" }
>["payload"];
type ScheduleInspectPayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/inspect/response" }
>["payload"];
type ScheduleLogsPayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/logs/response" }
>["payload"];
type SchedulePausePayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/pause/response" }
>["payload"];
type ScheduleResumePayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/resume/response" }
>["payload"];
type ScheduleDeletePayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/delete/response" }
>["payload"];
type ScheduleUpdatePayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/update/response" }
>["payload"];
type ScheduleAssistPayload = Extract<
  SessionOutboundMessage,
  { type: "schedule/assist/response" }
>["payload"];
type TmuxListAgentsPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/list_agents/response" }
>["payload"];
type TmuxCapturePanePayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/capture_pane/response" }
>["payload"];
type TmuxSendKeysPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/send_keys/response" }
>["payload"];
type TmuxNewSessionPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/new_session/response" }
>["payload"];
type TmuxKillSessionPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/kill_session/response" }
>["payload"];
type TmuxDeleteCommandHistoryPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/delete_command_history/response" }
>["payload"];
type TmuxStatusLinePayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/status_line/response" }
>["payload"];
export type FetchAgentTimelinePayload = FetchAgentTimelineResponseMessage["payload"];

export type FetchAgentTimelineDirection = FetchAgentTimelinePayload["direction"];
export type FetchAgentTimelineProjection = FetchAgentTimelinePayload["projection"];
export type FetchAgentTimelineCursor = NonNullable<FetchAgentTimelinePayload["startCursor"]>;
export interface FetchAgentTimelineOptions {
  direction?: FetchAgentTimelineDirection;
  cursor?: FetchAgentTimelineCursor;
  limit?: number;
  projection?: FetchAgentTimelineProjection;
  requestId?: string;
}

type AgentRefreshedStatusPayload = z.infer<typeof AgentRefreshedStatusPayloadSchema>;
type RestartRequestedStatusPayload = z.infer<typeof RestartRequestedStatusPayloadSchema>;
type ShutdownRequestedStatusPayload = z.infer<typeof ShutdownRequestedStatusPayloadSchema>;
type FetchAgentsPayload = Extract<
  SessionOutboundMessage,
  { type: "fetch_agents_response" }
>["payload"];
type FetchAgentsRequest = Extract<SessionInboundMessage, { type: "fetch_agents_request" }>;
export type FetchAgentsOptions = Omit<FetchAgentsRequest, "type" | "requestId"> & {
  requestId?: string;
};
export type FetchAgentsEntry = FetchAgentsPayload["entries"][number];
export type FetchAgentsPageInfo = FetchAgentsPayload["pageInfo"];
type FetchAgentHistoryPayload = Extract<
  SessionOutboundMessage,
  { type: "fetch_agent_history_response" }
>["payload"];
type FetchAgentHistoryRequest = Extract<
  SessionInboundMessage,
  { type: "fetch_agent_history_request" }
>;
export type FetchAgentHistoryOptions = Omit<FetchAgentHistoryRequest, "type" | "requestId"> & {
  requestId?: string;
};
export type FetchAgentHistoryEntry = FetchAgentHistoryPayload["entries"][number];
export type FetchAgentHistoryPageInfo = FetchAgentHistoryPayload["pageInfo"];
type FetchWorkspacesPayload = Extract<
  SessionOutboundMessage,
  { type: "fetch_workspaces_response" }
>["payload"];
type FetchWorkspacesRequest = Extract<SessionInboundMessage, { type: "fetch_workspaces_request" }>;
export type FetchWorkspacesOptions = Omit<FetchWorkspacesRequest, "type" | "requestId"> & {
  requestId?: string;
};
export type FetchWorkspacesEntry = FetchWorkspacesPayload["entries"][number];
export type FetchWorkspacesPageInfo = FetchWorkspacesPayload["pageInfo"];
export interface CreateChatRoomOptions {
  name: string;
  purpose?: string | null;
  requestId?: string;
}
export interface InspectChatRoomOptions {
  room: string;
  requestId?: string;
}
export interface DeleteChatRoomOptions {
  room: string;
  requestId?: string;
}
export interface PostChatMessageOptions {
  room: string;
  body: string;
  authorAgentId?: string;
  replyToMessageId?: string | null;
  requestId?: string;
}
export interface ReadChatMessagesOptions {
  room: string;
  limit?: number;
  since?: string;
  authorAgentId?: string;
  requestId?: string;
}
export interface WaitForChatMessagesOptions {
  room: string;
  afterMessageId?: string | null;
  timeoutMs?: number;
  requestId?: string;
}
export interface RunLoopOptions {
  prompt: string;
  cwd: string;
  verifyPrompt?: string | null;
  verifyChecks?: string[];
  name?: string | null;
  sleepMs?: number;
  maxIterations?: number;
  maxTimeMs?: number;
  agentTemplate?: WireAgentSessionConfig | null;
  workerAgentTemplate?: WireAgentSessionConfig | null;
  verifierAgentTemplate?: WireAgentSessionConfig | null;
  requestId?: string;
}
export interface InspectLoopOptions {
  id: string;
  requestId?: string;
}
export interface LoopLogsOptions {
  id: string;
  afterSeq?: number;
  requestId?: string;
}
export interface StopLoopOptions {
  id: string;
  requestId?: string;
}
export interface UpdateLoopOptions {
  id: string;
  name?: string | null;
  archive?: boolean | null;
  prompt?: string | null;
  cwd?: string | null;
  verifyChecks?: string[] | null;
  maxIterations?: number | null;
  agentTemplate?: WireAgentSessionConfig | null;
  workerAgentTemplate?: WireAgentSessionConfig | null;
  verifierAgentTemplate?: WireAgentSessionConfig | null;
  requestId?: string;
}
export interface DeleteLoopOptions {
  id: string;
  requestId?: string;
}
export interface GetLoopTemplateOptions {
  templateID: string;
  requestId?: string;
}
export interface DeleteLoopTemplateOptions {
  templateID: string;
  requestId?: string;
}
export interface CreateScheduleOptions {
  prompt: string;
  name?: string | null;
  cwd?: string | null;
  cadence:
    | {
        type: "every";
        everyMs: number;
      }
    | {
        type: "cron";
        expression: string;
      };
  target:
    | {
        type: "self";
        agentId: string;
      }
    | {
        type: "agent";
        agentId: string;
      }
    | {
        type: "new-agent";
        config: {
          provider: AgentProvider;
          cwd: string;
          modeId?: string;
          model?: string;
          thinkingOptionId?: string;
          title?: string | null;
          approvalPolicy?: string;
          sandboxMode?: string;
          networkAccess?: boolean;
          webSearch?: boolean;
          extra?: AgentSessionConfig["extra"];
          systemPrompt?: string;
          mcpServers?: AgentSessionConfig["mcpServers"];
        };
      };
  maxRuns?: number;
  expiresAt?: string;
  requestId?: string;
}
export interface InspectScheduleOptions {
  id: string;
  requestId?: string;
}
export interface UpdateScheduleOptions {
  id: string;
  prompt: string;
  name?: string | null;
  cwd?: string | null;
  cadence:
    | {
        type: "every";
        everyMs: number;
      }
    | {
        type: "cron";
        expression: string;
      };
  target: CreateScheduleOptions["target"];
  maxRuns?: number;
  expiresAt?: string;
  requestId?: string;
}
export interface ScheduleAssistOptions {
  message: string;
  timezone: string;
  clientNow: string;
  contextScheduleId?: string;
  transcript?: Array<{ role: "user" | "assistant"; content: string }>;
  requestId?: string;
}
type ListAvailableEditorsPayload = ListAvailableEditorsResponseMessage["payload"];
type OpenInEditorPayload = OpenInEditorResponseMessage["payload"];
type OpenProjectPayload = OpenProjectResponseMessage["payload"];
type ArchiveWorkspacePayload = ArchiveWorkspaceResponseMessage["payload"];
type RemoveProjectPayload = RemoveProjectResponseMessage["payload"];
type WorkspaceSetupStatusPayload = WorkspaceSetupStatusResponseMessage["payload"];
export type EditorTargetDescriptor = ListAvailableEditorsPayload["editors"][number];

export interface FetchAgentResult {
  agent: AgentSnapshotPayload;
  project: ProjectPlacementPayload | null;
}

export interface WaitForFinishResult {
  status: "idle" | "error" | "permission" | "timeout";
  final: AgentSnapshotPayload | null;
  error: string | null;
  lastMessage: string | null;
}

interface Waiter<T> {
  predicate: (msg: SessionOutboundMessage) => T | null;
  resolve: (value: T) => void;
  reject: (error: Error) => void;
  timeoutHandle: ReturnType<typeof setTimeout> | null;
}

interface WaitHandle<T> {
  promise: Promise<T>;
  cancel: (error: Error) => void;
}

type RpcWaitResult<T> = { kind: "ok"; value: T } | { kind: "error"; error: DaemonRpcError };
type GetDaemonConfigResponse = Extract<
  SessionOutboundMessage,
  { type: "get_daemon_config_response" }
>;
type SetDaemonConfigResponse = Extract<
  SessionOutboundMessage,
  { type: "set_daemon_config_response" }
>;
type CorrelatedResponseMessage =
  | Extract<SessionOutboundMessage, { payload: { requestId: string } }>
  | GetDaemonConfigResponse
  | SetDaemonConfigResponse;
type CorrelatedResponseType = CorrelatedResponseMessage["type"];
type CorrelatedResponsePayload<TType extends CorrelatedResponseType> = Extract<
  CorrelatedResponseMessage,
  { type: TType }
>["payload"];

class DaemonRpcError extends Error {
  readonly requestId: string;
  readonly requestType?: string;
  readonly code?: string;

  constructor(params: { requestId: string; error: string; requestType?: string; code?: string }) {
    const parts = [params.error];
    if (params.requestType) parts.push(`requestType=${params.requestType}`);
    if (params.code) parts.push(`code=${params.code}`);
    super(parts.join(" "));
    this.name = "DaemonRpcError";
    this.requestId = params.requestId;
    this.requestType = params.requestType;
    this.code = params.code;
  }
}

const DEFAULT_RECONNECT_BASE_DELAY_MS = 1500;
const DEFAULT_RECONNECT_MAX_DELAY_MS = 30000;
const DEFAULT_CONNECT_TIMEOUT_MS = 15000;

/** Default timeout for waiting for connection before sending queued messages */
const DEFAULT_SEND_QUEUE_TIMEOUT_MS = 10000;

function normalizeClientId(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

function hashForLog(value: string): string {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) | 0;
  }
  return `h_${Math.abs(hash).toString(16)}`;
}

function toReasonCode(reason: string | null | undefined): string | null {
  if (!reason) {
    return null;
  }
  const normalized = reason.toLowerCase();
  if (normalized.includes("timed out")) {
    return "connect_timeout";
  }
  if (normalized.includes("disposed")) {
    return "disposed";
  }
  if (normalized.includes("client closed")) {
    return "client_closed";
  }
  if (normalized.includes("transport")) {
    return "transport_error";
  }
  if (normalized.includes("failed to connect")) {
    return "connect_failed";
  }
  return "unknown";
}

interface PendingSend {
  message: SessionInboundMessage;
  resolve: () => void;
  reject: (error: Error) => void;
  timeoutHandle: ReturnType<typeof setTimeout>;
}

export class DaemonClient {
  private transport: DaemonTransport | null = null;
  private transportCleanup: Array<() => void> = [];
  private rawMessageListeners: Set<(message: SessionOutboundMessage) => void> = new Set();
  private messageHandlers: Map<
    SessionOutboundMessage["type"],
    Set<(message: SessionOutboundMessage) => void>
  > = new Map();
  private eventListeners: Set<DaemonEventHandler> = new Set();
  private waiters: Set<Waiter<unknown>> = new Set();
  private connectionListeners: Set<(status: ConnectionState) => void> = new Set();
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private connectTimeout: ReturnType<typeof setTimeout> | null = null;
  private pendingGenericTransportErrorTimeout: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private shouldReconnect = true;
  private connectPromise: Promise<void> | null = null;
  private connectResolve: (() => void) | null = null;
  private connectReject: ((error: Error) => void) | null = null;
  private lastErrorValue: string | null = null;
  private connectionState: ConnectionState = { status: "idle" };
  private logger: Logger;
  private pendingSendQueue: PendingSend[] = [];
  private readonly logConnectionPath: "direct" | "relay";
  private readonly logServerId: string | null;
  private readonly logClientIdHash: string;
  private readonly logGeneration: number | null;
  private lastServerInfoMessage: ServerInfoStatusPayload | null = null;
  private runtimeMetricsInterval: ReturnType<typeof setInterval> | null = null;
  private runtimeMetrics: DaemonClientRuntimeMetrics | null = null;
  private readonly agentRpc = new AgentRpc(this);
  private readonly scheduleRpc = new ScheduleRpc(this);
  private readonly chatRpc = new ChatRpc(this);
  private readonly workspaceRpc = new WorkspaceRpc(this);
  private readonly gitRpc = new GitRpc(this);
  private readonly terminalRpc = new TerminalRpc(this);

  constructor(private config: DaemonClientConfig) {
    this.logger = config.logger ?? consoleLogger;
    this.logConnectionPath = isRelayClientWebSocketUrl(this.config.url) ? "relay" : "direct";
    let parsedUrlForLog: URL | null = null;
    try {
      parsedUrlForLog = new URL(this.config.url);
    } catch {
      parsedUrlForLog = null;
    }
    const parsedServerIdForLog = normalizeClientId(parsedUrlForLog?.searchParams.get("serverId"));
    this.logServerId = parsedServerIdForLog ?? parsedUrlForLog?.host ?? null;
    const resolvedClientId = normalizeClientId(this.config.clientId);
    if (!resolvedClientId) {
      throw new Error("Daemon client requires a non-empty clientId");
    }
    this.config.clientId = resolvedClientId;
    this.logClientIdHash = hashForLog(resolvedClientId);
    this.logGeneration =
      typeof this.config.runtimeGeneration === "number" &&
      Number.isFinite(this.config.runtimeGeneration)
        ? this.config.runtimeGeneration
        : null;
    const runtimeMetricsIntervalMs =
      typeof config.runtimeMetricsIntervalMs === "number" && config.runtimeMetricsIntervalMs > 0
        ? config.runtimeMetricsIntervalMs
        : 0;
    if (runtimeMetricsIntervalMs > 0) {
      const runtimeMetricsWindowMs =
        typeof config.runtimeMetricsWindowMs === "number" && config.runtimeMetricsWindowMs > 0
          ? Math.max(config.runtimeMetricsWindowMs, runtimeMetricsIntervalMs)
          : undefined;
      this.runtimeMetrics = new DaemonClientRuntimeMetrics(
        this.logger,
        {
          connectionPath: this.logConnectionPath,
          serverId: this.logServerId,
          getConnectionStatus: () => this.connectionState.status,
        },
        runtimeMetricsWindowMs ? { windowMs: runtimeMetricsWindowMs } : undefined,
      );
      this.runtimeMetricsInterval = setInterval(() => {
        this.runtimeMetrics?.flush();
      }, runtimeMetricsIntervalMs);
    }
  }

  // ============================================================================
  // Connection
  // ============================================================================

  async connect(): Promise<void> {
    if (this.connectionState.status === "disposed") {
      throw new Error("Daemon client is disposed");
    }
    if (this.connectionState.status === "connected") {
      return;
    }
    if (this.connectPromise) {
      return this.connectPromise;
    }

    this.shouldReconnect = true;
    this.connectPromise = new Promise((resolve, reject) => {
      this.connectResolve = resolve;
      this.connectReject = reject;
      this.attemptConnect();
    });

    return this.connectPromise;
  }

  private attemptConnect(): void {
    if (this.connectionState.status === "disposed") {
      this.rejectConnect(new Error("Daemon client is disposed"));
      return;
    }
    if (!this.shouldReconnect) {
      this.rejectConnect(new Error("Daemon client is closed"));
      return;
    }

    if (this.connectionState.status === "connecting") {
      return;
    }

    const headers: Record<string, string> = {};
    if (this.config.authHeader) {
      headers["Authorization"] = this.config.authHeader;
    }

    try {
      // Reconnect can overlap with browser close/error delivery ordering.
      // Always dispose previous transport before constructing the next one.
      this.disposeTransport();
      const baseTransportFactory =
        this.config.transportFactory ??
        createWebSocketTransportFactory(this.config.webSocketFactory ?? defaultWebSocketFactory);
      const shouldUseRelayE2ee =
        this.config.e2ee?.enabled === true && isRelayClientWebSocketUrl(this.config.url);

      let transportFactory = baseTransportFactory;
      if (shouldUseRelayE2ee) {
        const daemonPublicKeyB64 = this.config.e2ee?.daemonPublicKeyB64;
        if (!daemonPublicKeyB64) {
          throw new Error("daemonPublicKeyB64 is required for relay E2EE");
        }
        transportFactory = createRelayE2eeTransportFactory({
          baseFactory: baseTransportFactory,
          daemonPublicKeyB64,
          logger: this.logger,
        });
      }
      const transportUrl = this.resolveTransportUrlForAttempt();
      const transport = transportFactory({ url: transportUrl, headers });
      this.transport = transport;
      this.lastServerInfoMessage = null;

      this.updateConnectionState(
        {
          status: "connecting",
          attempt: this.reconnectAttempt,
        },
        { event: "CONNECT_REQUEST" },
      );
      this.resetConnectTimeout();
      const timeoutMs = Math.max(1, this.config.connectTimeoutMs ?? DEFAULT_CONNECT_TIMEOUT_MS);
      this.connectTimeout = setTimeout(() => {
        if (this.connectionState.status !== "connecting") {
          return;
        }
        this.lastErrorValue = "Connection timed out";
        this.disposeTransport(1001, "Connection timed out");
        this.scheduleReconnect({
          reason: "Connection timed out",
          event: "CONNECT_TIMEOUT",
          reasonCode: "connect_timeout",
        });
      }, timeoutMs);

      this.transportCleanup = [
        transport.onOpen(() => {
          if (this.pendingGenericTransportErrorTimeout) {
            clearTimeout(this.pendingGenericTransportErrorTimeout);
            this.pendingGenericTransportErrorTimeout = null;
          }
          this.lastErrorValue = null;
          this.sendHelloMessage();
        }),
        transport.onClose((event) => {
          this.resetConnectTimeout();
          if (this.pendingGenericTransportErrorTimeout) {
            clearTimeout(this.pendingGenericTransportErrorTimeout);
            this.pendingGenericTransportErrorTimeout = null;
          }
          const reason = describeTransportClose(event);
          if (reason) {
            this.lastErrorValue = reason;
          }
          this.scheduleReconnect({
            reason,
            event: "TRANSPORT_CLOSE",
            reasonCode: "transport_closed",
          });
        }),
        transport.onError((event) => {
          this.resetConnectTimeout();
          const reason = describeTransportError(event);
          const isGeneric = reason === "Transport error";
          // Browser WebSocket.onerror often provides no useful details and is followed
          // by a close event (often with code 1006). Prefer surfacing the close details
          // instead of immediately disconnecting with a generic "Transport error".
          if (isGeneric) {
            this.lastErrorValue ??= reason;
            if (!this.pendingGenericTransportErrorTimeout) {
              this.pendingGenericTransportErrorTimeout = setTimeout(() => {
                this.pendingGenericTransportErrorTimeout = null;
                if (
                  this.connectionState.status === "connected" ||
                  this.connectionState.status === "connecting"
                ) {
                  this.lastErrorValue = reason;
                  this.scheduleReconnect({
                    reason,
                    event: "TRANSPORT_ERROR",
                    reasonCode: "transport_error",
                  });
                }
              }, 250);
            }
            return;
          }

          if (this.pendingGenericTransportErrorTimeout) {
            clearTimeout(this.pendingGenericTransportErrorTimeout);
            this.pendingGenericTransportErrorTimeout = null;
          }
          this.lastErrorValue = reason;
          this.scheduleReconnect({
            reason,
            event: "TRANSPORT_ERROR",
            reasonCode: "transport_error",
          });
        }),
        transport.onMessage((data) => this.handleTransportMessage(data)),
      ];
    } catch (error) {
      this.resetConnectTimeout();
      const message = error instanceof Error ? error.message : "Failed to connect";
      this.lastErrorValue = message;
      this.scheduleReconnect({
        reason: message,
        event: "CONNECT_FAILED",
        reasonCode: "connect_failed",
      });
      this.rejectConnect(error instanceof Error ? error : new Error(message));
    }
  }

  private resolveConnect(): void {
    if (this.connectResolve) {
      this.connectResolve();
    }
    this.connectPromise = null;
    this.connectResolve = null;
    this.connectReject = null;
  }

  private rejectConnect(error: Error): void {
    if (this.connectReject) {
      this.connectReject(error);
    }
    this.connectPromise = null;
    this.connectResolve = null;
    this.connectReject = null;
  }

  async close(): Promise<void> {
    if (this.connectionState.status === "disposed") {
      return;
    }
    this.shouldReconnect = false;
    this.connectPromise = null;
    this.connectResolve = null;
    this.connectReject = null;
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }
    this.resetConnectTimeout();
    this.disposeTransport(1000, "Client closed");
    this.clearWaiters(new Error("Daemon client closed"));
    this.rejectPendingSendQueue(new Error("Daemon client closed"));
    this.terminalRpc.clearSlots();
    this.lastServerInfoMessage = null;
    if (this.runtimeMetricsInterval) {
      clearInterval(this.runtimeMetricsInterval);
      this.runtimeMetricsInterval = null;
      this.runtimeMetrics?.flush({ final: true });
      this.runtimeMetrics = null;
    }
    this.updateConnectionState(
      { status: "disposed" },
      { event: "DISPOSE", reason: "Client closed", reasonCode: "disposed" },
    );
  }

  ensureConnected(): void {
    if (this.connectionState.status === "disposed") {
      return;
    }
    if (!this.shouldReconnect) {
      this.shouldReconnect = true;
    }
    if (
      this.connectionState.status === "connected" ||
      this.connectionState.status === "connecting"
    ) {
      return;
    }
    void this.connect();
  }

  getConnectionState(): ConnectionState {
    return this.connectionState;
  }

  subscribeConnectionStatus(listener: (status: ConnectionState) => void): () => void {
    this.connectionListeners.add(listener);
    try {
      listener(this.connectionState);
    } catch (error) {
      this.logger.warn({ err: toErrorInfo(error) }, "connection_listener_failed");
    }
    return () => {
      this.connectionListeners.delete(listener);
    };
  }

  get isConnected(): boolean {
    return this.connectionState.status === "connected";
  }

  get isConnecting(): boolean {
    return this.connectionState.status === "connecting";
  }

  get lastError(): string | null {
    return this.lastErrorValue;
  }

  // ============================================================================
  // Message Subscription
  // ============================================================================

  subscribe(handler: DaemonEventHandler): () => void {
    this.eventListeners.add(handler);
    return () => this.eventListeners.delete(handler);
  }

  subscribeRawMessages(handler: (message: SessionOutboundMessage) => void): () => void {
    this.rawMessageListeners.add(handler);
    return () => {
      this.rawMessageListeners.delete(handler);
    };
  }

  on<TType extends SessionOutboundMessage["type"]>(
    type: TType,
    handler: (message: Extract<SessionOutboundMessage, { type: TType }>) => void,
  ): () => void;
  on(handler: DaemonEventHandler): () => void;
  on(
    arg1: SessionOutboundMessage["type"] | DaemonEventHandler,
    arg2?: (message: SessionOutboundMessage) => void,
  ): () => void {
    if (typeof arg1 === "function") {
      return this.subscribe(arg1);
    }

    const type = arg1 as SessionOutboundMessage["type"];
    const handler = arg2 as (message: SessionOutboundMessage) => void;

    if (!this.messageHandlers.has(type)) {
      this.messageHandlers.set(type, new Set());
    }
    this.messageHandlers.get(type)!.add(handler);

    return () => {
      const handlers = this.messageHandlers.get(type);
      if (!handlers) {
        return;
      }
      handlers.delete(handler);
      if (handlers.size === 0) {
        this.messageHandlers.delete(type);
      }
    };
  }

  // ============================================================================
  // Core Send Helpers
  // ============================================================================

  /**
   * Send a session message. For fire-and-forget messages (heartbeats, etc.),
   * failures are suppressed if `suppressSendErrors` is configured.
   * For RPC methods that wait for responses, use `sendSessionMessageOrThrow` instead.
   */
  /** @internal */
  sendSessionMessage(message: SessionInboundMessage): void {
    if (!this.transport || this.connectionState.status !== "connected") {
      if (this.config.suppressSendErrors) {
        return;
      }
      throw new Error(`Transport not connected (status: ${this.connectionState.status})`);
    }
    const payload = SessionInboundMessageSchema.parse(message);
    try {
      this.transport.send(JSON.stringify({ type: "session", message: payload }));
    } catch (error) {
      if (this.config.suppressSendErrors) {
        return;
      }
      throw error instanceof Error ? error : new Error(String(error));
    }
  }

  /** @internal */
  sendBinaryFrame(frame: Uint8Array): void {
    if (!this.transport || this.connectionState.status !== "connected") {
      if (this.config.suppressSendErrors) {
        return;
      }
      throw new Error(`Transport not connected (status: ${this.connectionState.status})`);
    }
    try {
      this.transport.send(frame);
    } catch (error) {
      if (this.config.suppressSendErrors) {
        return;
      }
      throw error instanceof Error ? error : new Error(String(error));
    }
  }

  /** @internal */
  recordBinaryFrame(kind: string, bytes: number, handlerMs: number): void {
    this.runtimeMetrics?.recordBinaryFrame(kind, bytes, handlerMs);
  }

  /**
   * Send a session message for RPC methods that create waiters.
   * If the connection is still being established ("connecting"), the message
   * is queued and will be sent once connected (or rejected after timeout).
   * This prevents waiters from hanging forever when called during connection.
   */
  private sendSessionMessageOrThrow(message: SessionInboundMessage): Promise<void> {
    const status = this.connectionState.status;

    // If connected, send immediately
    if (this.transport && status === "connected") {
      const payload = SessionInboundMessageSchema.parse(message);
      this.transport.send(JSON.stringify({ type: "session", message: payload }));
      return Promise.resolve();
    }

    // If connecting, queue the message to be sent once connected
    if (status === "connecting") {
      return new Promise((resolve, reject) => {
        const timeoutHandle = setTimeout(() => {
          // Remove from queue
          const idx = this.pendingSendQueue.findIndex((p) => p.resolve === resolve);
          if (idx !== -1) {
            this.pendingSendQueue.splice(idx, 1);
          }
          reject(new Error(`Timed out waiting for connection to send message`));
        }, DEFAULT_SEND_QUEUE_TIMEOUT_MS);

        this.pendingSendQueue.push({ message, resolve, reject, timeoutHandle });
      });
    }

    // Not connected and not connecting - fail immediately
    return Promise.reject(new Error(`Transport not connected (status: ${status})`));
  }

  /**
   * Flush pending send queue - called when connection is established.
   */
  private flushPendingSendQueue(): void {
    const queue = this.pendingSendQueue;
    this.pendingSendQueue = [];

    for (const pending of queue) {
      clearTimeout(pending.timeoutHandle);
      try {
        if (this.transport && this.connectionState.status === "connected") {
          const payload = SessionInboundMessageSchema.parse(pending.message);
          this.transport.send(JSON.stringify({ type: "session", message: payload }));
          pending.resolve();
        } else {
          pending.reject(new Error("Connection lost before message could be sent"));
        }
      } catch (error) {
        pending.reject(error instanceof Error ? error : new Error(String(error)));
      }
    }
  }

  /**
   * Reject all pending sends - called when connection fails or is closed.
   */
  private rejectPendingSendQueue(error: Error): void {
    const queue = this.pendingSendQueue;
    this.pendingSendQueue = [];

    for (const pending of queue) {
      clearTimeout(pending.timeoutHandle);
      pending.reject(error);
    }
  }

  /** @internal */
  async sendRequest<T>(params: {
    requestId: string;
    message: SessionInboundMessage;
    timeout: number;
    select: (msg: SessionOutboundMessage) => T | null;
    options?: { skipQueue?: boolean };
  }): Promise<T> {
    const { promise, cancel } = this.waitForWithCancel<RpcWaitResult<T>>(
      (msg) => {
        if (msg.type === "rpc_error" && msg.payload.requestId === params.requestId) {
          return {
            kind: "error",
            error: new DaemonRpcError({
              requestId: msg.payload.requestId,
              error: msg.payload.error,
              requestType: msg.payload.requestType,
              code: msg.payload.code,
            }),
          };
        }
        const value = params.select(msg);
        if (value === null) {
          return null;
        }
        return { kind: "ok", value };
      },
      params.timeout,
      params.options,
    );

    try {
      await this.sendSessionMessageOrThrow(params.message);
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));
      cancel(err);
      void promise.catch(() => undefined);
      throw err;
    }

    const result = await promise;
    if (result.kind === "error") {
      throw result.error;
    }
    return result.value;
  }

  /** @internal */
  async sendCorrelatedRequest<
    TResponseType extends CorrelatedResponseType,
    TResult = CorrelatedResponsePayload<TResponseType>,
  >(params: {
    requestId: string;
    message: SessionInboundMessage;
    timeout: number;
    responseType: TResponseType;
    options?: { skipQueue?: boolean };
    selectPayload?: (payload: CorrelatedResponsePayload<TResponseType>) => TResult | null;
  }): Promise<TResult> {
    return this.sendRequest({
      requestId: params.requestId,
      message: params.message,
      timeout: params.timeout,
      options: params.options,
      select: (msg) => {
        const correlated = msg as CorrelatedResponseMessage;
        if (correlated.type !== params.responseType) {
          return null;
        }
        const payload = correlated.payload as unknown as CorrelatedResponsePayload<TResponseType>;
        if (payload.requestId !== params.requestId) {
          return null;
        }
        if (!params.selectPayload) {
          return payload as TResult;
        }
        return params.selectPayload(payload);
      },
    });
  }

  /** @internal */
  sendCorrelatedSessionRequest<
    TResponseType extends CorrelatedResponseType,
    TResult = CorrelatedResponsePayload<TResponseType>,
  >(params: {
    requestId?: string;
    message: { type: SessionInboundMessage["type"] } & Record<string, unknown>;
    responseType: TResponseType;
    timeout: number;
    selectPayload?: (payload: CorrelatedResponsePayload<TResponseType>) => TResult | null;
  }): Promise<TResult> {
    const resolvedRequestId = this.createRequestId(params.requestId);
    const message = SessionInboundMessageSchema.parse({
      ...params.message,
      requestId: resolvedRequestId,
    });
    return this.sendCorrelatedRequest({
      requestId: resolvedRequestId,
      message,
      responseType: params.responseType,
      timeout: params.timeout,
      options: { skipQueue: true },
      ...(params.selectPayload ? { selectPayload: params.selectPayload } : {}),
    });
  }

  /** @internal */
  sendSessionMessageStrict(message: SessionInboundMessage): void {
    if (!this.transport || this.connectionState.status !== "connected") {
      throw new Error("Transport not connected");
    }
    const payload = SessionInboundMessageSchema.parse(message);
    try {
      this.transport.send(JSON.stringify({ type: "session", message: payload }));
    } catch (error) {
      throw error instanceof Error ? error : new Error(String(error));
    }
  }

  async clearAgentAttention(agentId: string | string[]): Promise<void> {
    return this.agentRpc.clearAgentAttention(agentId);
  }

  sendHeartbeat(params: {
    deviceType: "web" | "mobile";
    focusedAgentId: string | null;
    lastActivityAt: string;
    appVisible: boolean;
    appVisibilityChangedAt?: string;
  }): void {
    this.agentRpc.sendHeartbeat(params);
  }

  registerPushToken(token: string): void {
    this.agentRpc.registerPushToken(token);
  }

  async ping(params?: { requestId?: string; timeoutMs?: number }): Promise<{
    requestId: string;
    clientSentAt: number;
    serverReceivedAt: number;
    serverSentAt: number;
    rttMs: number;
  }> {
    return this.agentRpc.ping(params);
  }

  // ============================================================================
  // Agent RPCs (requestId-correlated)
  // ============================================================================

  async fetchAgents(options?: FetchAgentsOptions): Promise<FetchAgentsPayload> {
    return this.agentRpc.fetchAgents(options);
  }

  async fetchAgentHistory(options?: FetchAgentHistoryOptions): Promise<FetchAgentHistoryPayload> {
    return this.agentRpc.fetchAgentHistory(options);
  }

  async fetchWorkspaces(options?: FetchWorkspacesOptions): Promise<FetchWorkspacesPayload> {
    return this.workspaceRpc.fetchWorkspaces(options);
  }

  async openProject(cwd: string, requestId?: string): Promise<OpenProjectPayload> {
    return this.workspaceRpc.openProject(cwd, requestId);
  }

  async startWorkspaceScript(
    workspaceId: string,
    scriptName: string,
    requestId?: string,
  ): Promise<
    Extract<SessionOutboundMessage, { type: "start_workspace_script_response" }>["payload"]
  > {
    return this.workspaceRpc.startWorkspaceScript(workspaceId, scriptName, requestId);
  }

  async listAvailableEditors(requestId?: string): Promise<ListAvailableEditorsPayload> {
    return this.workspaceRpc.listAvailableEditors(requestId);
  }

  async openInEditor(
    path: string,
    editorId: EditorTargetId,
    requestId?: string,
  ): Promise<OpenInEditorPayload> {
    return this.workspaceRpc.openInEditor(path, editorId, requestId);
  }

  async archiveWorkspace(
    workspaceId: string,
    requestId?: string,
  ): Promise<ArchiveWorkspacePayload> {
    return this.workspaceRpc.archiveWorkspace(workspaceId, requestId);
  }

  async removeProject(
    workspaceIds: string[],
    requestId?: string,
  ): Promise<RemoveProjectPayload> {
    return this.workspaceRpc.removeProject(workspaceIds, requestId);
  }

  async fetchWorkspaceSetupStatus(
    workspaceId: string,
    requestId?: string,
  ): Promise<WorkspaceSetupStatusPayload> {
    return this.workspaceRpc.fetchWorkspaceSetupStatus(workspaceId, requestId);
  }

  async fetchAgent(agentId: string, requestId?: string): Promise<FetchAgentResult | null> {
    return this.agentRpc.fetchAgent(agentId, requestId);
  }

  // ============================================================================
  // Agent Lifecycle
  // ============================================================================

  async createAgent(options: CreateAgentRequestOptions): Promise<AgentSnapshotPayload> {
    return this.agentRpc.createAgent(options);
  }

  async deleteAgent(agentId: string): Promise<void> {
    return this.agentRpc.deleteAgent(agentId);
  }

  async archiveAgent(agentId: string): Promise<{ archivedAt: string }> {
    return this.agentRpc.archiveAgent(agentId);
  }

  async updateAgent(
    agentId: string,
    updates: { name?: string; labels?: Record<string, string> },
  ): Promise<void> {
    return this.agentRpc.updateAgent(agentId, updates);
  }

  async resumeAgent(
    handle: AgentPersistenceHandle,
    overrides?: Partial<AgentSessionConfig>,
  ): Promise<AgentSnapshotPayload> {
    return this.agentRpc.resumeAgent(handle, overrides);
  }

  async refreshAgent(agentId: string, requestId?: string): Promise<AgentRefreshedStatusPayload> {
    return this.agentRpc.refreshAgent(agentId, requestId);
  }

  async fetchAgentTimeline(
    agentId: string,
    options: FetchAgentTimelineOptions = {},
  ): Promise<FetchAgentTimelinePayload> {
    return this.agentRpc.fetchAgentTimeline(agentId, options);
  }

  // ============================================================================
  // Agent Interaction
  // ============================================================================

  async sendAgentMessage(
    agentId: string,
    text: string,
    options?: SendMessageOptions,
  ): Promise<void> {
    return this.agentRpc.sendAgentMessage(agentId, text, options);
  }

  async sendMessage(agentId: string, text: string, options?: SendMessageOptions): Promise<void> {
    return this.agentRpc.sendMessage(agentId, text, options);
  }

  async cancelAgent(agentId: string): Promise<void> {
    return this.agentRpc.cancelAgent(agentId);
  }

  async setAgentMode(agentId: string, modeId: string): Promise<void> {
    return this.agentRpc.setAgentMode(agentId, modeId);
  }

  async setAgentModel(agentId: string, modelId: string | null): Promise<void> {
    return this.agentRpc.setAgentModel(agentId, modelId);
  }

  async setAgentFeature(agentId: string, featureId: string, value: unknown): Promise<void> {
    return this.agentRpc.setAgentFeature(agentId, featureId, value);
  }

  async setAgentThinkingOption(agentId: string, thinkingOptionId: string | null): Promise<void> {
    return this.agentRpc.setAgentThinkingOption(agentId, thinkingOptionId);
  }

  async restartServer(reason?: string, requestId?: string): Promise<RestartRequestedStatusPayload> {
    return this.agentRpc.restartServer(reason, requestId);
  }

  async shutdownServer(requestId?: string): Promise<ShutdownRequestedStatusPayload> {
    return this.agentRpc.shutdownServer(requestId);
  }

  // ============================================================================
  // Audio / Voice
  // ============================================================================

  async setVoiceMode(enabled: boolean, agentId?: string): Promise<SetVoiceModePayload> {
    return this.terminalRpc.setVoiceMode(enabled, agentId);
  }

  async sendVoiceAudioChunk(audio: string, format: string, isLast = false): Promise<void> {
    return this.terminalRpc.sendVoiceAudioChunk(audio, format, isLast);
  }

  async startDictationStream(dictationId: string, format: string): Promise<void> {
    return this.terminalRpc.startDictationStream(dictationId, format);
  }

  sendDictationStreamChunk(dictationId: string, seq: number, audio: string, format: string): void {
    return this.terminalRpc.sendDictationStreamChunk(dictationId, seq, audio, format);
  }

  async finishDictationStream(
    dictationId: string,
    finalSeq: number,
  ): Promise<{ dictationId: string; text: string }> {
    return this.terminalRpc.finishDictationStream(dictationId, finalSeq);
  }

  cancelDictationStream(dictationId: string): void {
    return this.terminalRpc.cancelDictationStream(dictationId);
  }

  async abortRequest(): Promise<void> {
    return this.terminalRpc.abortRequest();
  }

  async audioPlayed(id: string): Promise<void> {
    return this.terminalRpc.audioPlayed(id);
  }

  // ============================================================================
  // Git Operations
  // ============================================================================

  async getCheckoutStatus(
    cwd: string,
    options?: { requestId?: string },
  ): Promise<CheckoutStatusPayload> {
    return this.gitRpc.getCheckoutStatus(cwd, options);
  }

  async getCheckoutDiff(
    cwd: string,
    compare: { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean },
    requestId?: string,
  ): Promise<CheckoutDiffPayload> {
    return this.gitRpc.getCheckoutDiff(cwd, compare, requestId);
  }

  async subscribeCheckoutDiff(
    cwd: string,
    compare: { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean },
    options?: { subscriptionId?: string; requestId?: string },
  ): Promise<SubscribeCheckoutDiffPayload> {
    return this.gitRpc.subscribeCheckoutDiff(cwd, compare, options);
  }

  unsubscribeCheckoutDiff(subscriptionId: string): void {
    return this.gitRpc.unsubscribeCheckoutDiff(subscriptionId);
  }

  async checkoutCommit(
    cwd: string,
    input: { message?: string; addAll?: boolean },
    requestId?: string,
  ): Promise<CheckoutCommitPayload> {
    return this.gitRpc.checkoutCommit(cwd, input, requestId);
  }

  async checkoutMerge(
    cwd: string,
    input: { baseRef?: string; strategy?: "merge" | "squash"; requireCleanTarget?: boolean },
    requestId?: string,
  ): Promise<CheckoutMergePayload> {
    return this.gitRpc.checkoutMerge(cwd, input, requestId);
  }

  async checkoutMergeFromBase(
    cwd: string,
    input: { baseRef?: string; requireCleanTarget?: boolean },
    requestId?: string,
  ): Promise<CheckoutMergeFromBasePayload> {
    return this.gitRpc.checkoutMergeFromBase(cwd, input, requestId);
  }

  async checkoutPull(cwd: string, requestId?: string): Promise<CheckoutPullPayload> {
    return this.gitRpc.checkoutPull(cwd, requestId);
  }

  async checkoutPush(cwd: string, requestId?: string): Promise<CheckoutPushPayload> {
    return this.gitRpc.checkoutPush(cwd, requestId);
  }

  async checkoutPrCreate(
    cwd: string,
    input: { title?: string; body?: string; baseRef?: string },
    requestId?: string,
  ): Promise<CheckoutPrCreatePayload> {
    return this.gitRpc.checkoutPrCreate(cwd, input, requestId);
  }

  async checkoutPrStatus(cwd: string, requestId?: string): Promise<CheckoutPrStatusPayload> {
    return this.gitRpc.checkoutPrStatus(cwd, requestId);
  }

  async pullRequestTimeline(
    input: { cwd: string; prNumber: number; repoOwner: string; repoName: string },
    requestId?: string,
  ): Promise<PullRequestTimelinePayload> {
    return this.gitRpc.pullRequestTimeline(input, requestId);
  }

  async checkoutSwitchBranch(
    cwd: string,
    branch: string,
    requestId?: string,
  ): Promise<CheckoutSwitchBranchPayload> {
    return this.gitRpc.checkoutSwitchBranch(cwd, branch, requestId);
  }

  async stashSave(
    cwd: string,
    options?: { branch?: string },
    requestId?: string,
  ): Promise<StashSavePayload> {
    return this.gitRpc.stashSave(cwd, options, requestId);
  }

  async stashPop(cwd: string, stashIndex: number, requestId?: string): Promise<StashPopPayload> {
    return this.gitRpc.stashPop(cwd, stashIndex, requestId);
  }

  async stashList(
    cwd: string,
    options?: { soloOnly?: boolean },
    requestId?: string,
  ): Promise<StashListPayload> {
    return this.gitRpc.stashList(cwd, options, requestId);
  }

  async getSoloWorktreeList(
    input: { cwd?: string; repoRoot?: string },
    requestId?: string,
  ): Promise<SoloWorktreeListPayload> {
    return this.gitRpc.getSoloWorktreeList(input, requestId);
  }

  async archiveSoloWorktree(
    input: { worktreePath?: string; repoRoot?: string; branchName?: string },
    requestId?: string,
  ): Promise<SoloWorktreeArchivePayload> {
    return this.gitRpc.archiveSoloWorktree(input, requestId);
  }

  async createSoloWorktree(
    input: CreateSoloWorktreeInput,
    requestId?: string,
  ): Promise<CreateSoloWorktreePayload> {
    return this.gitRpc.createSoloWorktree(input, requestId);
  }

  async validateBranch(
    options: { cwd: string; branchName: string },
    requestId?: string,
  ): Promise<ValidateBranchPayload> {
    return this.gitRpc.validateBranch(options, requestId);
  }

  async getBranchSuggestions(
    options: { cwd: string; query?: string; limit?: number },
    requestId?: string,
  ): Promise<BranchSuggestionsPayload> {
    return this.gitRpc.getBranchSuggestions(options, requestId);
  }

  async searchGitHub(
    options: { cwd: string; query: string; limit?: number; kinds?: GitHubSearchRequest["kinds"] },
    requestId?: string,
  ): Promise<GitHubSearchPayload> {
    return this.gitRpc.searchGitHub(options, requestId);
  }

  async getDirectorySuggestions(
    options: {
      query: string;
      limit?: number;
      cwd?: string;
      includeFiles?: boolean;
      includeDirectories?: boolean;
    },
    requestId?: string,
  ): Promise<DirectorySuggestionsPayload> {
    return this.gitRpc.getDirectorySuggestions(options, requestId);
  }

  // ============================================================================
  // File Explorer
  // ============================================================================

  async exploreFileSystem(
    cwd: string,
    path: string,
    mode: "list" | "file" = "list",
    requestId?: string,
  ): Promise<FileExplorerPayload> {
    return this.workspaceRpc.exploreFileSystem(cwd, path, mode, requestId);
  }

  async requestDownloadToken(
    cwd: string,
    path: string,
    requestId?: string,
  ): Promise<FileDownloadTokenPayload> {
    return this.workspaceRpc.requestDownloadToken(cwd, path, requestId);
  }

  async requestProjectIcon(
    cwd: string,
    requestId?: string,
  ): Promise<ProjectIconResponse["payload"]> {
    return this.workspaceRpc.requestProjectIcon(cwd, requestId);
  }

  // ============================================================================
  // Provider Models / Commands
  // ============================================================================

  async listProviderModels(
    provider: AgentProvider,
    options?: { cwd?: string; requestId?: string },
  ): Promise<ListProviderModelsPayload> {
    return this.workspaceRpc.listProviderModels(provider, options);
  }

  async listProviderModes(
    provider: AgentProvider,
    options?: { cwd?: string; requestId?: string },
  ): Promise<ListProviderModesPayload> {
    return this.workspaceRpc.listProviderModes(provider, options);
  }

  async listProviderFeatures(
    draftConfig: ListCommandsDraftConfig,
    options?: { requestId?: string },
  ): Promise<ListProviderFeaturesPayload> {
    return this.workspaceRpc.listProviderFeatures(draftConfig, options);
  }

  async listAvailableProviders(options?: {
    requestId?: string;
  }): Promise<ListAvailableProvidersPayload> {
    return this.workspaceRpc.listAvailableProviders(options);
  }

  async getProvidersSnapshot(options?: {
    cwd?: string;
    requestId?: string;
  }): Promise<GetProvidersSnapshotPayload> {
    return this.workspaceRpc.getProvidersSnapshot(options);
  }

  async getDaemonConfig(
    requestId?: string,
  ): Promise<{ requestId: string; config: MutableDaemonConfig }> {
    return this.workspaceRpc.getDaemonConfig(requestId);
  }

  async patchDaemonConfig(
    config: MutableDaemonConfigPatch,
    requestId?: string,
  ): Promise<{ requestId: string; config: MutableDaemonConfig }> {
    return this.workspaceRpc.patchDaemonConfig(config, requestId);
  }

  async readProjectConfig(repoRoot: string, requestId?: string): Promise<ReadProjectConfigPayload> {
    return this.workspaceRpc.readProjectConfig(repoRoot, requestId);
  }

  async writeProjectConfig(input: WriteProjectConfigInput): Promise<WriteProjectConfigPayload> {
    return this.workspaceRpc.writeProjectConfig(input);
  }

  async refreshProvidersSnapshot(options?: {
    cwd?: string;
    providers?: AgentProvider[];
    requestId?: string;
  }): Promise<RefreshProvidersSnapshotPayload> {
    return this.workspaceRpc.refreshProvidersSnapshot(options);
  }

  async getProviderDiagnostic(
    provider: AgentProvider,
    options?: { requestId?: string },
  ): Promise<ProviderDiagnosticPayload> {
    return this.workspaceRpc.getProviderDiagnostic(provider, options);
  }

  async listCommands(agentId: string, requestId?: string): Promise<ListCommandsPayload>;
  async listCommands(agentId: string, options?: ListCommandsOptions): Promise<ListCommandsPayload>;
  async listCommands(
    agentId: string,
    requestIdOrOptions?: string | ListCommandsOptions,
  ): Promise<ListCommandsPayload> {
    if (typeof requestIdOrOptions === "string") {
      return this.workspaceRpc.listCommands(agentId, requestIdOrOptions);
    }
    return this.workspaceRpc.listCommands(agentId, requestIdOrOptions);
  }

  // ============================================================================
  // Permissions
  // ============================================================================

  async respondToPermission(
    agentId: string,
    requestId: string,
    response: AgentPermissionResponse,
  ): Promise<void> {
    return this.agentRpc.respondToPermission(agentId, requestId, response);
  }

  async respondToPermissionAndWait(
    agentId: string,
    requestId: string,
    response: AgentPermissionResponse,
    timeout = 15000,
  ): Promise<AgentPermissionResolvedPayload> {
    return this.agentRpc.respondToPermissionAndWait(agentId, requestId, response, timeout);
  }

  // ============================================================================
  // Waiting / Streaming Helpers
  // ============================================================================

  async waitForAgentUpsert(
    agentId: string,
    predicate: (snapshot: AgentSnapshotPayload) => boolean,
    timeout = 60000,
  ): Promise<AgentSnapshotPayload> {
    return this.agentRpc.waitForAgentUpsert(agentId, predicate, timeout);
  }

  async waitForFinish(agentId: string, timeout = 60000): Promise<WaitForFinishResult> {
    return this.agentRpc.waitForFinish(agentId, timeout);
  }

  // ============================================================================
  // Terminals
  // ============================================================================

  subscribeTerminals(input: { cwd: string }): void {
    return this.terminalRpc.subscribeTerminals(input);
  }

  unsubscribeTerminals(input: { cwd: string }): void {
    return this.terminalRpc.unsubscribeTerminals(input);
  }

  async listTerminals(cwd?: string, requestId?: string): Promise<ListTerminalsPayload> {
    return this.terminalRpc.listTerminals(cwd, requestId);
  }

  async createTerminal(
    cwd: string,
    name?: string,
    requestId?: string,
    options?: { agentId?: string; command?: string; args?: string[] },
  ): Promise<CreateTerminalPayload> {
    return this.terminalRpc.createTerminal(cwd, name, requestId, options);
  }

  async subscribeTerminal(
    terminalId: string,
    requestId?: string,
  ): Promise<SubscribeTerminalPayload> {
    return this.terminalRpc.subscribeTerminal(terminalId, requestId);
  }

  unsubscribeTerminal(terminalId: string): void {
    return this.terminalRpc.unsubscribeTerminal(terminalId);
  }

  sendTerminalInput(terminalId: string, message: TerminalInput["message"]): void {
    return this.terminalRpc.sendTerminalInput(terminalId, message);
  }

  async killTerminal(terminalId: string, requestId?: string): Promise<KillTerminalPayload> {
    return this.terminalRpc.killTerminal(terminalId, requestId);
  }

  async closeItems(
    input: { agentIds?: string[]; terminalIds?: string[] },
    requestId?: string,
  ): Promise<CloseItemsPayload> {
    return this.terminalRpc.closeItems(input, requestId);
  }

  async captureTerminal(
    terminalId: string,
    options?: { start?: number; end?: number; stripAnsi?: boolean },
    requestId?: string,
  ): Promise<CaptureTerminalPayload> {
    return this.terminalRpc.captureTerminal(terminalId, options, requestId);
  }

  async createChatRoom(options: CreateChatRoomOptions): Promise<ChatCreatePayload> {
    return this.chatRpc.createChatRoom(options);
  }

  async listChatRooms(requestId?: string): Promise<ChatListPayload> {
    return this.chatRpc.listChatRooms(requestId);
  }

  async inspectChatRoom(options: InspectChatRoomOptions): Promise<ChatInspectPayload> {
    return this.chatRpc.inspectChatRoom(options);
  }

  async deleteChatRoom(options: DeleteChatRoomOptions): Promise<ChatDeletePayload> {
    return this.chatRpc.deleteChatRoom(options);
  }

  async postChatMessage(options: PostChatMessageOptions): Promise<ChatPostPayload> {
    return this.chatRpc.postChatMessage(options);
  }

  async readChatMessages(options: ReadChatMessagesOptions): Promise<ChatReadPayload> {
    return this.chatRpc.readChatMessages(options);
  }

  async waitForChatMessages(options: WaitForChatMessagesOptions): Promise<ChatWaitPayload> {
    return this.chatRpc.waitForChatMessages(options);
  }

  async scheduleCreate(options: CreateScheduleOptions): Promise<ScheduleCreatePayload> {
    return this.scheduleRpc.scheduleCreate(options);
  }

  async scheduleList(requestId?: string): Promise<ScheduleListPayload> {
    return this.scheduleRpc.scheduleList(requestId);
  }

  async scheduleInspect(options: InspectScheduleOptions): Promise<ScheduleInspectPayload> {
    return this.scheduleRpc.scheduleInspect(options);
  }

  async scheduleLogs(options: InspectScheduleOptions): Promise<ScheduleLogsPayload> {
    return this.scheduleRpc.scheduleLogs(options);
  }

  async schedulePause(options: InspectScheduleOptions): Promise<SchedulePausePayload> {
    return this.scheduleRpc.schedulePause(options);
  }

  async scheduleResume(options: InspectScheduleOptions): Promise<ScheduleResumePayload> {
    return this.scheduleRpc.scheduleResume(options);
  }

  async scheduleDelete(options: InspectScheduleOptions): Promise<ScheduleDeletePayload> {
    return this.scheduleRpc.scheduleDelete(options);
  }

  async scheduleUpdate(options: UpdateScheduleOptions): Promise<ScheduleUpdatePayload> {
    return this.scheduleRpc.scheduleUpdate(options);
  }

  async scheduleAssist(options: ScheduleAssistOptions): Promise<ScheduleAssistPayload> {
    return this.scheduleRpc.scheduleAssist(options);
  }

  async tmuxListAgents(requestId?: string): Promise<TmuxListAgentsPayload> {
    return this.terminalRpc.tmuxListAgents(requestId);
  }

  async tmuxCapturePane(
    paneId: string,
    startLine?: number,
    lastContentHash?: string,
    cols?: number,
    requestId?: string,
  ): Promise<TmuxCapturePanePayload> {
    return this.terminalRpc.tmuxCapturePane(paneId, startLine, lastContentHash, cols, requestId);
  }

  async tmuxSendKeys(paneId: string, keys: string, sendEnter?: boolean, requestId?: string): Promise<TmuxSendKeysPayload> {
    return this.terminalRpc.tmuxSendKeys(paneId, keys, sendEnter, requestId);
  }

  async tmuxStatusLine(sessionId: string, requestId?: string): Promise<TmuxStatusLinePayload> {
    return this.terminalRpc.tmuxStatusLine(sessionId, requestId);
  }

  async tmuxNewSession(name: string, options?: { workingDir?: string; command?: string }, requestId?: string): Promise<TmuxNewSessionPayload> {
    return this.terminalRpc.tmuxNewSession(name, options, requestId);
  }

  async tmuxKillSession(sessionName: string, requestId?: string): Promise<TmuxKillSessionPayload> {
    return this.terminalRpc.tmuxKillSession(sessionName, requestId);
  }

  async tmuxDeleteCommandHistory(launchCmd: string, requestId?: string): Promise<TmuxDeleteCommandHistoryPayload> {
    return this.terminalRpc.tmuxDeleteCommandHistory(launchCmd, requestId);
  }

  async loopRun(options: RunLoopOptions): Promise<LoopRunPayload> {
    return this.scheduleRpc.loopRun(options);
  }

  async loopList(requestId?: string): Promise<LoopListPayload> {
    return this.scheduleRpc.loopList(requestId);
  }

  async loopInspect(options: string | InspectLoopOptions): Promise<LoopInspectPayload> {
    return this.scheduleRpc.loopInspect(options);
  }

  async loopLogs(options: string | LoopLogsOptions, afterSeq?: number): Promise<LoopLogsPayload> {
    return this.scheduleRpc.loopLogs(options, afterSeq);
  }

  async loopStop(options: string | StopLoopOptions): Promise<LoopStopPayload> {
    return this.scheduleRpc.loopStop(options);
  }

  async loopUpdate(options: UpdateLoopOptions): Promise<LoopUpdatePayload> {
    return this.scheduleRpc.loopUpdate(options);
  }

  async loopDelete(options: string | DeleteLoopOptions): Promise<LoopDeletePayload> {
    return this.scheduleRpc.loopDelete(options);
  }

  async loopTemplateList(requestId?: string): Promise<LoopTemplateListPayload> {
    return this.scheduleRpc.loopTemplateList(requestId);
  }

  async loopTemplateGet(options: string | GetLoopTemplateOptions): Promise<LoopTemplateGetPayload> {
    return this.scheduleRpc.loopTemplateGet(options);
  }

  async loopTemplateDelete(options: string | DeleteLoopTemplateOptions): Promise<LoopTemplateDeletePayload> {
    return this.scheduleRpc.loopTemplateDelete(options);
  }

  onTerminalStreamEvent(handler: (event: TerminalStreamEvent) => void): () => void {
    return this.terminalRpc.onTerminalStreamEvent(handler);
  }

  async waitForTerminalStreamEvent(
    predicate: (event: TerminalStreamEvent) => boolean,
    timeout = 5000,
  ): Promise<TerminalStreamEvent> {
    return this.terminalRpc.waitForTerminalStreamEvent(predicate, timeout);
  }

  // ============================================================================
  // Internals
  // ============================================================================

  /** @internal */
  createRequestId(requestId?: string): string {
    return requestId ?? crypto.randomUUID();
  }

  getLastServerInfoMessage(): ServerInfoStatusPayload | null {
    return this.lastServerInfoMessage;
  }

  private resolveTransportUrlForAttempt(): string {
    return this.config.url;
  }

  private sendHelloMessage(): void {
    if (!this.transport) {
      this.scheduleReconnect({
        reason: "Transport unavailable before hello",
        event: "HELLO_TRANSPORT_MISSING",
        reasonCode: "transport_error",
      });
      return;
    }

    try {
      this.transport.send(
        JSON.stringify({
          type: "hello",
          clientId: this.config.clientId,
          clientType: this.config.clientType ?? "cli",
          protocolVersion: 2,
          ...(this.config.appVersion ? { appVersion: this.config.appVersion } : {}),
        }),
      );
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to send hello message";
      this.lastErrorValue = message;
      this.scheduleReconnect({
        reason: message,
        event: "HELLO_SEND_FAILED",
        reasonCode: "transport_error",
      });
    }
  }

  private disposeTransport(code = 1001, reason = "Reconnecting"): void {
    this.cleanupTransport();
    if (this.transport) {
      try {
        this.transport.close(code, reason);
      } catch (error) {
        this.logger.debug({ err: toErrorInfo(error) }, "transport_close_failed");
      }
      this.transport = null;
    }
  }

  private cleanupTransport(): void {
    this.resetConnectTimeout();
    if (this.pendingGenericTransportErrorTimeout) {
      clearTimeout(this.pendingGenericTransportErrorTimeout);
      this.pendingGenericTransportErrorTimeout = null;
    }
    for (const cleanup of this.transportCleanup) {
      try {
        cleanup();
      } catch (error) {
        this.logger.warn({ err: toErrorInfo(error) }, "transport_cleanup_handler_failed");
      }
    }
    this.transportCleanup = [];
  }

  private resetConnectTimeout(): void {
    if (!this.connectTimeout) {
      return;
    }
    clearTimeout(this.connectTimeout);
    this.connectTimeout = null;
  }

  private handleTransportMessage(data: unknown): void {
    const rawData =
      data && typeof data === "object" && "data" in data ? (data as { data: unknown }).data : data;

    if (
      typeof Blob !== "undefined" &&
      rawData instanceof Blob &&
      typeof rawData.arrayBuffer === "function"
    ) {
      void rawData
        .arrayBuffer()
        .then((buffer) => {
          this.handleTransportMessage(buffer);
          return;
        })
        .catch(() => {
          // Ignore failed blob decoding and allow reconnect logic to recover.
        });
      return;
    }

    const rawBytes = asUint8Array(rawData);
    if (rawBytes && this.terminalRpc.tryHandleBinaryFrame(rawBytes)) {
      return;
    }
    const payload = decodeMessageData(rawData);
    if (!payload) {
      return;
    }
    this.handleJsonPayload(payload, rawBytes?.byteLength);
  }

  private handleJsonPayload(payload: string, rawBytesLength: number | undefined): void {
    const bytes = rawBytesLength ?? payload.length;
    const startMs = perfNow();
    let parsedJson: unknown;
    try {
      parsedJson = JSON.parse(payload);
    } catch (error) {
      this.logger.debug({ err: toErrorInfo(error) }, "json_parse_failed");
      return;
    }

    const parsed = WSOutboundMessageSchema.safeParse(parsedJson);
    if (!parsed.success) {
      const msgType = (parsedJson as { type?: string })?.type ?? "unknown";
      this.logger.warn({ msgType, error: parsed.error.message }, "Message validation failed");
      return;
    }

    if (parsed.data.type === "pong") {
      this.runtimeMetrics?.recordMessage("pong", bytes, perfNow() - startMs);
      return;
    }

    this.handleSessionMessage(parsed.data.message);
    const msgType = parsed.data.message.type;
    this.runtimeMetrics?.recordMessage(msgType, bytes, perfNow() - startMs);
    if (parsed.data.message.type === "agent_stream") {
      this.runtimeMetrics?.recordAgentStream(parsed.data.message.payload);
    }
  }

  private updateConnectionState(
    next: ConnectionState,
    metadata?: { event: string; reason?: string; reasonCode?: string },
  ): void {
    const previous = this.connectionState;
    this.connectionState = next;
    const reasonFromNext =
      next.status === "disconnected" && typeof next.reason === "string" ? next.reason : null;
    const reason = metadata?.reason ?? reasonFromNext;
    const reasonCode = metadata?.reasonCode ?? toReasonCode(reason);
    this.logger.debug(
      {
        serverId: this.logServerId,
        clientIdHash: this.logClientIdHash,
        from: previous.status,
        to: next.status,
        event: metadata?.event ?? "STATE_UPDATE",
        connectionPath: this.logConnectionPath,
        generation: this.logGeneration,
        reasonCode,
        reason,
      },
      "DaemonClientTransition",
    );
    for (const listener of this.connectionListeners) {
      try {
        listener(next);
      } catch (error) {
        this.logger.warn({ err: toErrorInfo(error) }, "connection_listener_failed");
      }
    }
  }

  setReconnectEnabled(enabled: boolean): void {
    this.config = { ...this.config, reconnect: { ...this.config.reconnect, enabled } };
  }

  private scheduleReconnect(input?: {
    reason?: string;
    event?: string;
    reasonCode?: string;
  }): void {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }
    const wasDisposed = this.connectionState.status === "disposed";
    const reason = input?.reason;

    if (typeof reason === "string" && reason.trim().length > 0) {
      this.lastErrorValue = reason.trim();
    }

    // Clear all pending waiters and queued sends since the connection was lost
    // and responses from the previous connection will never arrive.
    this.clearWaiters(new Error(reason ?? "Connection lost"));
    this.rejectPendingSendQueue(new Error(reason ?? "Connection lost"));
    this.terminalRpc.clearSlots();
    this.lastServerInfoMessage = null;

    if (wasDisposed) {
      this.rejectConnect(new Error(reason ?? "Daemon client is disposed"));
      return;
    }
    this.emitDisconnectedStateForReconnect(reason, input);
    if (!this.shouldReconnect || this.config.reconnect?.enabled === false) {
      this.rejectConnect(new Error(reason ?? "Transport disconnected before connect"));
      return;
    }

    this.armReconnectTimer();
  }

  private emitDisconnectedStateForReconnect(
    reason: string | undefined,
    input: { reason?: string; event?: string; reasonCode?: string } | undefined,
  ): void {
    this.updateConnectionState(
      {
        status: "disconnected",
        ...(reason ? { reason } : {}),
      },
      {
        event: input?.event ?? "TRANSPORT_CLOSE",
        ...(reason ? { reason } : {}),
        ...(input?.reasonCode ? { reasonCode: input.reasonCode } : {}),
      },
    );
  }

  private armReconnectTimer(): void {
    const attempt = this.reconnectAttempt;
    const baseDelay = this.config.reconnect?.baseDelayMs ?? DEFAULT_RECONNECT_BASE_DELAY_MS;
    const maxDelay = this.config.reconnect?.maxDelayMs ?? DEFAULT_RECONNECT_MAX_DELAY_MS;
    const delay = Math.min(baseDelay * 2 ** attempt, maxDelay);
    this.reconnectAttempt = attempt + 1;
    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null;
      if (!this.shouldReconnect) {
        return;
      }
      this.attemptConnect();
    }, delay);
  }

  private handleSessionMessage(msg: SessionOutboundMessage): void {
    if (msg.type === "status") {
      const serverInfo = parseServerInfoStatusPayload(msg.payload);
      if (serverInfo) {
        this.lastServerInfoMessage = serverInfo;
        if (this.connectionState.status === "connecting") {
          this.resetConnectTimeout();
          this.reconnectAttempt = 0;
          this.updateConnectionState({ status: "connected" }, { event: "HELLO_SERVER_INFO" });
          this.gitRpc.resubscribe();
          this.terminalRpc.resubscribe();
          this.flushPendingSendQueue();
          this.resolveConnect();
        }
      }
    }

    if (msg.type === "terminal_stream_exit") {
      this.terminalRpc.removeSlot(msg.payload.terminalId);
    }

    if (this.rawMessageListeners.size > 0) {
      for (const handler of this.rawMessageListeners) {
        try {
          handler(msg);
        } catch (error) {
          this.logger.warn({ err: toErrorInfo(error) }, "raw_message_listener_failed");
        }
      }
    }

    const handlers = this.messageHandlers.get(msg.type);
    if (handlers) {
      for (const handler of handlers) {
        try {
          handler(msg);
        } catch (error) {
          this.logger.warn({ err: toErrorInfo(error) }, "message_handler_failed");
        }
      }
    }

    const event = this.toEvent(msg);
    if (event) {
      for (const handler of this.eventListeners) {
        handler(event);
      }
    }

    this.resolveWaiters(msg);
  }

  private resolveWaiters(msg: SessionOutboundMessage): void {
    for (const waiter of Array.from(this.waiters)) {
      const result = waiter.predicate(msg);
      if (result !== null) {
        this.waiters.delete(waiter);
        if (waiter.timeoutHandle) {
          clearTimeout(waiter.timeoutHandle);
        }
        waiter.resolve(result);
      }
    }
  }

  private clearWaiters(error: Error): void {
    for (const waiter of Array.from(this.waiters)) {
      if (waiter.timeoutHandle) {
        clearTimeout(waiter.timeoutHandle);
      }
      waiter.reject(error);
    }
    this.waiters.clear();
  }

  private toEvent(msg: SessionOutboundMessage): DaemonEvent | null {
    switch (msg.type) {
      case "agent_update":
        return {
          type: "agent_update",
          agentId: msg.payload.kind === "upsert" ? msg.payload.agent.id : msg.payload.agentId,
          payload: msg.payload,
        };
      case "workspace_update":
        return {
          type: "workspace_update",
          workspaceId: msg.payload.kind === "upsert" ? msg.payload.workspace.id : msg.payload.id,
          payload: msg.payload,
        };
      case "workspace_setup_progress":
        return {
          type: "workspace_setup_progress",
          workspaceId: msg.payload.workspaceId,
          payload: msg.payload,
        };
      case "agent_stream":
        return {
          type: "agent_stream",
          agentId: msg.payload.agentId,
          event: msg.payload.event,
          timestamp: msg.payload.timestamp,
          ...(typeof msg.payload.seq === "number" ? { seq: msg.payload.seq } : {}),
          ...(typeof msg.payload.epoch === "string" ? { epoch: msg.payload.epoch } : {}),
        };
      case "status":
        return { type: "status", payload: msg.payload };
      case "agent_deleted":
        return { type: "agent_deleted", agentId: msg.payload.agentId };
      case "agent_permission_request":
        return {
          type: "agent_permission_request",
          agentId: msg.payload.agentId,
          request: msg.payload.request,
        };
      case "agent_permission_resolved":
        return {
          type: "agent_permission_resolved",
          agentId: msg.payload.agentId,
          requestId: msg.payload.requestId,
          resolution: msg.payload.resolution,
        };
      case "providers_snapshot_update":
        return {
          type: "providers_snapshot_update",
          payload: msg.payload,
        };
      default:
        return null;
    }
  }

  /** @internal */
  waitForWithCancel<T>(
    predicate: (msg: SessionOutboundMessage) => T | null,
    timeout = 30000,
    _options?: { skipQueue?: boolean },
  ): WaitHandle<T> {
    // Capture stack trace at call site, not inside setTimeout
    const timeoutError = new Error(`Timeout waiting for message (${timeout}ms)`);

    let waiter: Waiter<T> | null = null;
    let settled = false;
    let rejectFn: ((error: Error) => void) | null = null;

    const promise = new Promise<T>((resolve, reject) => {
      const wrappedResolve = (value: T) => {
        if (settled) return;
        settled = true;
        resolve(value);
      };
      const wrappedReject = (error: Error) => {
        if (settled) return;
        settled = true;
        reject(error);
      };
      rejectFn = wrappedReject;

      const timeoutHandle =
        timeout > 0
          ? setTimeout(() => {
              if (waiter) {
                this.waiters.delete(waiter as Waiter<unknown>);
              }
              wrappedReject(timeoutError);
            }, timeout)
          : null;

      waiter = {
        predicate,
        resolve: wrappedResolve,
        reject: wrappedReject,
        timeoutHandle,
      };
      this.waiters.add(waiter as Waiter<unknown>);
    });

    const cancel = (error: Error) => {
      if (settled) {
        return;
      }

      if (waiter) {
        this.waiters.delete(waiter as Waiter<unknown>);
        if (waiter.timeoutHandle) {
          clearTimeout(waiter.timeoutHandle);
        }
      }

      if (rejectFn) {
        rejectFn(error);
        return;
      }

      // Extremely unlikely: cancel called before the Promise executor ran.
      queueMicrotask(() => {
        if (!settled && rejectFn) {
          rejectFn(error);
        }
      });
    };

    return { promise, cancel };
  }
}
