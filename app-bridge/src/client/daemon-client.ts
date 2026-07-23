import type { z } from "zod";
import {
  AgentRefreshedStatusPayloadSchema,
  RestartRequestedStatusPayloadSchema,
  ShutdownRequestedStatusPayloadSchema,
  type ServerInfoStatusPayload,
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
import { AgentRpc } from "./agent-rpc.js";
import { ScheduleRpc } from "./schedule-rpc.js";
import { ChatRpc } from "./chat-rpc.js";
import { WorkspaceRpc } from "./workspace-rpc.js";
import { GitRpc } from "./git-rpc.js";
import { TerminalRpc } from "./terminal-rpc.js";
import { ConnectionManager } from "./connection-manager.js";
import type { DaemonTransportFactory, WebSocketFactory } from "./daemon-client-transport.js";
import type { Logger } from "../shared/logger.js";

export type { Logger } from "../shared/logger.js";

export { ConnectionManager } from "./connection-manager.js";
export type { ConnectionRpcHooks } from "./connection-manager.js";

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

export class DaemonClient {
  private readonly connection: ConnectionManager;
  private readonly agentRpc: AgentRpc;
  private readonly scheduleRpc: ScheduleRpc;
  private readonly chatRpc: ChatRpc;
  private readonly workspaceRpc: WorkspaceRpc;
  private readonly gitRpc: GitRpc;
  private readonly terminalRpc: TerminalRpc;

  constructor(config: DaemonClientConfig) {
    this.connection = new ConnectionManager(config);
    this.agentRpc = new AgentRpc(this.connection);
    this.scheduleRpc = new ScheduleRpc(this.connection);
    this.chatRpc = new ChatRpc(this.connection);
    this.workspaceRpc = new WorkspaceRpc(this.connection);
    this.gitRpc = new GitRpc(this.connection);
    this.terminalRpc = new TerminalRpc(this.connection);
    this.connection.setHooks({
      tryHandleBinaryFrame: (rawBytes) => this.terminalRpc.tryHandleBinaryFrame(rawBytes),
      resubscribe: () => {
        this.gitRpc.resubscribe();
        this.terminalRpc.resubscribe();
      },
      onConnectionLost: () => this.terminalRpc.clearSlots(),
      onTerminalStreamExit: (terminalId) => this.terminalRpc.removeSlot(terminalId),
    });
  }

  // ============================================================================
  // Domain Namespaces
  // ============================================================================

  /** Agent lifecycle, interaction, permission and streaming RPCs. */
  get agents(): AgentRpc {
    return this.agentRpc;
  }

  /** Schedule and loop automation RPCs. */
  get schedules(): ScheduleRpc {
    return this.scheduleRpc;
  }

  /** Chat room RPCs. */
  get chat(): ChatRpc {
    return this.chatRpc;
  }

  /** Workspace, project, provider and config RPCs. */
  get workspaces(): WorkspaceRpc {
    return this.workspaceRpc;
  }

  /** Git checkout, stash, worktree and suggestion RPCs. */
  get git(): GitRpc {
    return this.gitRpc;
  }

  /** Terminal, tmux and voice RPCs. */
  get terminal(): TerminalRpc {
    return this.terminalRpc;
  }

  // ============================================================================
  // Connection
  // ============================================================================

  async connect(): Promise<void> {
    return this.connection.connect();
  }

  async close(): Promise<void> {
    return this.connection.close();
  }

  ensureConnected(): void {
    this.connection.ensureConnected();
  }

  getConnectionState(): ConnectionState {
    return this.connection.getConnectionState();
  }

  subscribeConnectionStatus(listener: (status: ConnectionState) => void): () => void {
    return this.connection.subscribeConnectionStatus(listener);
  }

  get isConnected(): boolean {
    return this.connection.isConnected;
  }

  get isConnecting(): boolean {
    return this.connection.isConnecting;
  }

  get lastError(): string | null {
    return this.connection.lastError;
  }

  // ============================================================================
  // Message Subscription
  // ============================================================================

  subscribe(handler: DaemonEventHandler): () => void {
    return this.connection.subscribe(handler);
  }

  subscribeRawMessages(handler: (message: SessionOutboundMessage) => void): () => void {
    return this.connection.subscribeRawMessages(handler);
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
      return this.connection.on(arg1);
    }
    const type = arg1;
    const handler = arg2 as (message: SessionOutboundMessage) => void;
    return this.connection.on(type, handler);
  }

  // ============================================================================
  // Connection Maintenance
  // ============================================================================

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

  getLastServerInfoMessage(): ServerInfoStatusPayload | null {
    return this.connection.getLastServerInfoMessage();
  }

  setReconnectEnabled(enabled: boolean): void {
    this.connection.setReconnectEnabled(enabled);
  }

  /** @internal */
  waitForWithCancel<T>(
    predicate: (msg: SessionOutboundMessage) => T | null,
    timeout = 30000,
    options?: { skipQueue?: boolean },
  ): { promise: Promise<T>; cancel: (error: Error) => void } {
    return this.connection.waitForWithCancel(predicate, timeout, options);
  }

  // ============================================================================
  // Agent RPCs (requestId-correlated)
  // ============================================================================

  /** @deprecated Use `client.agents.fetchAgents` instead. */
  async fetchAgents(options?: FetchAgentsOptions): Promise<FetchAgentsPayload> {
    return this.agentRpc.fetchAgents(options);
  }

  /** @deprecated Use `client.agents.fetchAgentHistory` instead. */
  async fetchAgentHistory(options?: FetchAgentHistoryOptions): Promise<FetchAgentHistoryPayload> {
    return this.agentRpc.fetchAgentHistory(options);
  }

  /** @deprecated Use `client.workspaces.fetchWorkspaces` instead. */
  async fetchWorkspaces(options?: FetchWorkspacesOptions): Promise<FetchWorkspacesPayload> {
    return this.workspaceRpc.fetchWorkspaces(options);
  }

  /** @deprecated Use `client.workspaces.openProject` instead. */
  async openProject(cwd: string, requestId?: string): Promise<OpenProjectPayload> {
    return this.workspaceRpc.openProject(cwd, requestId);
  }

  /** @deprecated Use `client.workspaces.startWorkspaceScript` instead. */
  async startWorkspaceScript(
    workspaceId: string,
    scriptName: string,
    requestId?: string,
  ): Promise<
    Extract<SessionOutboundMessage, { type: "start_workspace_script_response" }>["payload"]
  > {
    return this.workspaceRpc.startWorkspaceScript(workspaceId, scriptName, requestId);
  }

  /** @deprecated Use `client.workspaces.listAvailableEditors` instead. */
  async listAvailableEditors(requestId?: string): Promise<ListAvailableEditorsPayload> {
    return this.workspaceRpc.listAvailableEditors(requestId);
  }

  /** @deprecated Use `client.workspaces.openInEditor` instead. */
  async openInEditor(
    path: string,
    editorId: EditorTargetId,
    requestId?: string,
  ): Promise<OpenInEditorPayload> {
    return this.workspaceRpc.openInEditor(path, editorId, requestId);
  }

  /** @deprecated Use `client.workspaces.archiveWorkspace` instead. */
  async archiveWorkspace(
    workspaceId: string,
    requestId?: string,
  ): Promise<ArchiveWorkspacePayload> {
    return this.workspaceRpc.archiveWorkspace(workspaceId, requestId);
  }

  /** @deprecated Use `client.workspaces.removeProject` instead. */
  async removeProject(
    workspaceIds: string[],
    requestId?: string,
  ): Promise<RemoveProjectPayload> {
    return this.workspaceRpc.removeProject(workspaceIds, requestId);
  }

  /** @deprecated Use `client.workspaces.fetchWorkspaceSetupStatus` instead. */
  async fetchWorkspaceSetupStatus(
    workspaceId: string,
    requestId?: string,
  ): Promise<WorkspaceSetupStatusPayload> {
    return this.workspaceRpc.fetchWorkspaceSetupStatus(workspaceId, requestId);
  }

  /** @deprecated Use `client.agents.fetchAgent` instead. */
  async fetchAgent(agentId: string, requestId?: string): Promise<FetchAgentResult | null> {
    return this.agentRpc.fetchAgent(agentId, requestId);
  }

  // ============================================================================
  // Agent Lifecycle
  // ============================================================================

  /** @deprecated Use `client.agents.createAgent` instead. */
  async createAgent(options: CreateAgentRequestOptions): Promise<AgentSnapshotPayload> {
    return this.agentRpc.createAgent(options);
  }

  /** @deprecated Use `client.agents.deleteAgent` instead. */
  async deleteAgent(agentId: string): Promise<void> {
    return this.agentRpc.deleteAgent(agentId);
  }

  /** @deprecated Use `client.agents.archiveAgent` instead. */
  async archiveAgent(agentId: string): Promise<{ archivedAt: string }> {
    return this.agentRpc.archiveAgent(agentId);
  }

  /** @deprecated Use `client.agents.updateAgent` instead. */
  async updateAgent(
    agentId: string,
    updates: { name?: string; labels?: Record<string, string> },
  ): Promise<void> {
    return this.agentRpc.updateAgent(agentId, updates);
  }

  /** @deprecated Use `client.agents.resumeAgent` instead. */
  async resumeAgent(
    handle: AgentPersistenceHandle,
    overrides?: Partial<AgentSessionConfig>,
  ): Promise<AgentSnapshotPayload> {
    return this.agentRpc.resumeAgent(handle, overrides);
  }

  /** @deprecated Use `client.agents.refreshAgent` instead. */
  async refreshAgent(agentId: string, requestId?: string): Promise<AgentRefreshedStatusPayload> {
    return this.agentRpc.refreshAgent(agentId, requestId);
  }

  /** @deprecated Use `client.agents.fetchAgentTimeline` instead. */
  async fetchAgentTimeline(
    agentId: string,
    options: FetchAgentTimelineOptions = {},
  ): Promise<FetchAgentTimelinePayload> {
    return this.agentRpc.fetchAgentTimeline(agentId, options);
  }

  // ============================================================================
  // Agent Interaction
  // ============================================================================

  /** @deprecated Use `client.agents.sendAgentMessage` instead. */
  async sendAgentMessage(
    agentId: string,
    text: string,
    options?: SendMessageOptions,
  ): Promise<void> {
    return this.agentRpc.sendAgentMessage(agentId, text, options);
  }

  /** @deprecated Use `client.agents.sendMessage` instead. */
  async sendMessage(agentId: string, text: string, options?: SendMessageOptions): Promise<void> {
    return this.agentRpc.sendMessage(agentId, text, options);
  }

  /** @deprecated Use `client.agents.cancelAgent` instead. */
  async cancelAgent(agentId: string): Promise<void> {
    return this.agentRpc.cancelAgent(agentId);
  }

  /** @deprecated Use `client.agents.setAgentMode` instead. */
  async setAgentMode(agentId: string, modeId: string): Promise<void> {
    return this.agentRpc.setAgentMode(agentId, modeId);
  }

  /** @deprecated Use `client.agents.setAgentModel` instead. */
  async setAgentModel(agentId: string, modelId: string | null): Promise<void> {
    return this.agentRpc.setAgentModel(agentId, modelId);
  }

  /** @deprecated Use `client.agents.setAgentFeature` instead. */
  async setAgentFeature(agentId: string, featureId: string, value: unknown): Promise<void> {
    return this.agentRpc.setAgentFeature(agentId, featureId, value);
  }

  /** @deprecated Use `client.agents.setAgentThinkingOption` instead. */
  async setAgentThinkingOption(agentId: string, thinkingOptionId: string | null): Promise<void> {
    return this.agentRpc.setAgentThinkingOption(agentId, thinkingOptionId);
  }

  /** @deprecated Use `client.agents.restartServer` instead. */
  async restartServer(reason?: string, requestId?: string): Promise<RestartRequestedStatusPayload> {
    return this.agentRpc.restartServer(reason, requestId);
  }

  /** @deprecated Use `client.agents.shutdownServer` instead. */
  async shutdownServer(requestId?: string): Promise<ShutdownRequestedStatusPayload> {
    return this.agentRpc.shutdownServer(requestId);
  }

  // ============================================================================
  // Audio / Voice
  // ============================================================================

  /** @deprecated Use `client.terminal.setVoiceMode` instead. */
  async setVoiceMode(enabled: boolean, agentId?: string): Promise<SetVoiceModePayload> {
    return this.terminalRpc.setVoiceMode(enabled, agentId);
  }

  /** @deprecated Use `client.terminal.sendVoiceAudioChunk` instead. */
  async sendVoiceAudioChunk(audio: string, format: string, isLast = false): Promise<void> {
    return this.terminalRpc.sendVoiceAudioChunk(audio, format, isLast);
  }

  /** @deprecated Use `client.terminal.startDictationStream` instead. */
  async startDictationStream(dictationId: string, format: string): Promise<void> {
    return this.terminalRpc.startDictationStream(dictationId, format);
  }

  /** @deprecated Use `client.terminal.sendDictationStreamChunk` instead. */
  sendDictationStreamChunk(dictationId: string, seq: number, audio: string, format: string): void {
    return this.terminalRpc.sendDictationStreamChunk(dictationId, seq, audio, format);
  }

  /** @deprecated Use `client.terminal.finishDictationStream` instead. */
  async finishDictationStream(
    dictationId: string,
    finalSeq: number,
  ): Promise<{ dictationId: string; text: string }> {
    return this.terminalRpc.finishDictationStream(dictationId, finalSeq);
  }

  /** @deprecated Use `client.terminal.cancelDictationStream` instead. */
  cancelDictationStream(dictationId: string): void {
    return this.terminalRpc.cancelDictationStream(dictationId);
  }

  /** @deprecated Use `client.terminal.abortRequest` instead. */
  async abortRequest(): Promise<void> {
    return this.terminalRpc.abortRequest();
  }

  /** @deprecated Use `client.terminal.audioPlayed` instead. */
  async audioPlayed(id: string): Promise<void> {
    return this.terminalRpc.audioPlayed(id);
  }

  // ============================================================================
  // Git Operations
  // ============================================================================

  /** @deprecated Use `client.git.getCheckoutStatus` instead. */
  async getCheckoutStatus(
    cwd: string,
    options?: { requestId?: string },
  ): Promise<CheckoutStatusPayload> {
    return this.gitRpc.getCheckoutStatus(cwd, options);
  }

  /** @deprecated Use `client.git.getCheckoutDiff` instead. */
  async getCheckoutDiff(
    cwd: string,
    compare: { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean },
    requestId?: string,
  ): Promise<CheckoutDiffPayload> {
    return this.gitRpc.getCheckoutDiff(cwd, compare, requestId);
  }

  /** @deprecated Use `client.git.subscribeCheckoutDiff` instead. */
  async subscribeCheckoutDiff(
    cwd: string,
    compare: { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean },
    options?: { subscriptionId?: string; requestId?: string },
  ): Promise<SubscribeCheckoutDiffPayload> {
    return this.gitRpc.subscribeCheckoutDiff(cwd, compare, options);
  }

  /** @deprecated Use `client.git.unsubscribeCheckoutDiff` instead. */
  unsubscribeCheckoutDiff(subscriptionId: string): void {
    return this.gitRpc.unsubscribeCheckoutDiff(subscriptionId);
  }

  /** @deprecated Use `client.git.checkoutCommit` instead. */
  async checkoutCommit(
    cwd: string,
    input: { message?: string; addAll?: boolean },
    requestId?: string,
  ): Promise<CheckoutCommitPayload> {
    return this.gitRpc.checkoutCommit(cwd, input, requestId);
  }

  /** @deprecated Use `client.git.checkoutMerge` instead. */
  async checkoutMerge(
    cwd: string,
    input: { baseRef?: string; strategy?: "merge" | "squash"; requireCleanTarget?: boolean },
    requestId?: string,
  ): Promise<CheckoutMergePayload> {
    return this.gitRpc.checkoutMerge(cwd, input, requestId);
  }

  /** @deprecated Use `client.git.checkoutMergeFromBase` instead. */
  async checkoutMergeFromBase(
    cwd: string,
    input: { baseRef?: string; requireCleanTarget?: boolean },
    requestId?: string,
  ): Promise<CheckoutMergeFromBasePayload> {
    return this.gitRpc.checkoutMergeFromBase(cwd, input, requestId);
  }

  /** @deprecated Use `client.git.checkoutPull` instead. */
  async checkoutPull(cwd: string, requestId?: string): Promise<CheckoutPullPayload> {
    return this.gitRpc.checkoutPull(cwd, requestId);
  }

  /** @deprecated Use `client.git.checkoutPush` instead. */
  async checkoutPush(cwd: string, requestId?: string): Promise<CheckoutPushPayload> {
    return this.gitRpc.checkoutPush(cwd, requestId);
  }

  /** @deprecated Use `client.git.checkoutPrCreate` instead. */
  async checkoutPrCreate(
    cwd: string,
    input: { title?: string; body?: string; baseRef?: string },
    requestId?: string,
  ): Promise<CheckoutPrCreatePayload> {
    return this.gitRpc.checkoutPrCreate(cwd, input, requestId);
  }

  /** @deprecated Use `client.git.checkoutPrStatus` instead. */
  async checkoutPrStatus(cwd: string, requestId?: string): Promise<CheckoutPrStatusPayload> {
    return this.gitRpc.checkoutPrStatus(cwd, requestId);
  }

  /** @deprecated Use `client.git.pullRequestTimeline` instead. */
  async pullRequestTimeline(
    input: { cwd: string; prNumber: number; repoOwner: string; repoName: string },
    requestId?: string,
  ): Promise<PullRequestTimelinePayload> {
    return this.gitRpc.pullRequestTimeline(input, requestId);
  }

  /** @deprecated Use `client.git.checkoutSwitchBranch` instead. */
  async checkoutSwitchBranch(
    cwd: string,
    branch: string,
    requestId?: string,
  ): Promise<CheckoutSwitchBranchPayload> {
    return this.gitRpc.checkoutSwitchBranch(cwd, branch, requestId);
  }

  /** @deprecated Use `client.git.stashSave` instead. */
  async stashSave(
    cwd: string,
    options?: { branch?: string },
    requestId?: string,
  ): Promise<StashSavePayload> {
    return this.gitRpc.stashSave(cwd, options, requestId);
  }

  /** @deprecated Use `client.git.stashPop` instead. */
  async stashPop(cwd: string, stashIndex: number, requestId?: string): Promise<StashPopPayload> {
    return this.gitRpc.stashPop(cwd, stashIndex, requestId);
  }

  /** @deprecated Use `client.git.stashList` instead. */
  async stashList(
    cwd: string,
    options?: { soloOnly?: boolean },
    requestId?: string,
  ): Promise<StashListPayload> {
    return this.gitRpc.stashList(cwd, options, requestId);
  }

  /** @deprecated Use `client.git.getSoloWorktreeList` instead. */
  async getSoloWorktreeList(
    input: { cwd?: string; repoRoot?: string },
    requestId?: string,
  ): Promise<SoloWorktreeListPayload> {
    return this.gitRpc.getSoloWorktreeList(input, requestId);
  }

  /** @deprecated Use `client.git.archiveSoloWorktree` instead. */
  async archiveSoloWorktree(
    input: { worktreePath?: string; repoRoot?: string; branchName?: string },
    requestId?: string,
  ): Promise<SoloWorktreeArchivePayload> {
    return this.gitRpc.archiveSoloWorktree(input, requestId);
  }

  /** @deprecated Use `client.git.createSoloWorktree` instead. */
  async createSoloWorktree(
    input: CreateSoloWorktreeInput,
    requestId?: string,
  ): Promise<CreateSoloWorktreePayload> {
    return this.gitRpc.createSoloWorktree(input, requestId);
  }

  /** @deprecated Use `client.git.validateBranch` instead. */
  async validateBranch(
    options: { cwd: string; branchName: string },
    requestId?: string,
  ): Promise<ValidateBranchPayload> {
    return this.gitRpc.validateBranch(options, requestId);
  }

  /** @deprecated Use `client.git.getBranchSuggestions` instead. */
  async getBranchSuggestions(
    options: { cwd: string; query?: string; limit?: number },
    requestId?: string,
  ): Promise<BranchSuggestionsPayload> {
    return this.gitRpc.getBranchSuggestions(options, requestId);
  }

  /** @deprecated Use `client.git.searchGitHub` instead. */
  async searchGitHub(
    options: { cwd: string; query: string; limit?: number; kinds?: GitHubSearchRequest["kinds"] },
    requestId?: string,
  ): Promise<GitHubSearchPayload> {
    return this.gitRpc.searchGitHub(options, requestId);
  }

  /** @deprecated Use `client.git.getDirectorySuggestions` instead. */
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

  /** @deprecated Use `client.workspaces.exploreFileSystem` instead. */
  async exploreFileSystem(
    cwd: string,
    path: string,
    mode: "list" | "file" = "list",
    requestId?: string,
  ): Promise<FileExplorerPayload> {
    return this.workspaceRpc.exploreFileSystem(cwd, path, mode, requestId);
  }

  /** @deprecated Use `client.workspaces.requestDownloadToken` instead. */
  async requestDownloadToken(
    cwd: string,
    path: string,
    requestId?: string,
  ): Promise<FileDownloadTokenPayload> {
    return this.workspaceRpc.requestDownloadToken(cwd, path, requestId);
  }

  /** @deprecated Use `client.workspaces.requestProjectIcon` instead. */
  async requestProjectIcon(
    cwd: string,
    requestId?: string,
  ): Promise<ProjectIconResponse["payload"]> {
    return this.workspaceRpc.requestProjectIcon(cwd, requestId);
  }

  // ============================================================================
  // Provider Models / Commands
  // ============================================================================

  /** @deprecated Use `client.workspaces.listProviderModels` instead. */
  async listProviderModels(
    provider: AgentProvider,
    options?: { cwd?: string; requestId?: string },
  ): Promise<ListProviderModelsPayload> {
    return this.workspaceRpc.listProviderModels(provider, options);
  }

  /** @deprecated Use `client.workspaces.listProviderModes` instead. */
  async listProviderModes(
    provider: AgentProvider,
    options?: { cwd?: string; requestId?: string },
  ): Promise<ListProviderModesPayload> {
    return this.workspaceRpc.listProviderModes(provider, options);
  }

  /** @deprecated Use `client.workspaces.listProviderFeatures` instead. */
  async listProviderFeatures(
    draftConfig: ListCommandsDraftConfig,
    options?: { requestId?: string },
  ): Promise<ListProviderFeaturesPayload> {
    return this.workspaceRpc.listProviderFeatures(draftConfig, options);
  }

  /** @deprecated Use `client.workspaces.listAvailableProviders` instead. */
  async listAvailableProviders(options?: {
    requestId?: string;
  }): Promise<ListAvailableProvidersPayload> {
    return this.workspaceRpc.listAvailableProviders(options);
  }

  /** @deprecated Use `client.workspaces.getProvidersSnapshot` instead. */
  async getProvidersSnapshot(options?: {
    cwd?: string;
    requestId?: string;
  }): Promise<GetProvidersSnapshotPayload> {
    return this.workspaceRpc.getProvidersSnapshot(options);
  }

  /** @deprecated Use `client.workspaces.getDaemonConfig` instead. */
  async getDaemonConfig(
    requestId?: string,
  ): Promise<{ requestId: string; config: MutableDaemonConfig }> {
    return this.workspaceRpc.getDaemonConfig(requestId);
  }

  /** @deprecated Use `client.workspaces.patchDaemonConfig` instead. */
  async patchDaemonConfig(
    config: MutableDaemonConfigPatch,
    requestId?: string,
  ): Promise<{ requestId: string; config: MutableDaemonConfig }> {
    return this.workspaceRpc.patchDaemonConfig(config, requestId);
  }

  /** @deprecated Use `client.workspaces.readProjectConfig` instead. */
  async readProjectConfig(repoRoot: string, requestId?: string): Promise<ReadProjectConfigPayload> {
    return this.workspaceRpc.readProjectConfig(repoRoot, requestId);
  }

  /** @deprecated Use `client.workspaces.writeProjectConfig` instead. */
  async writeProjectConfig(input: WriteProjectConfigInput): Promise<WriteProjectConfigPayload> {
    return this.workspaceRpc.writeProjectConfig(input);
  }

  /** @deprecated Use `client.workspaces.refreshProvidersSnapshot` instead. */
  async refreshProvidersSnapshot(options?: {
    cwd?: string;
    providers?: AgentProvider[];
    requestId?: string;
  }): Promise<RefreshProvidersSnapshotPayload> {
    return this.workspaceRpc.refreshProvidersSnapshot(options);
  }

  /** @deprecated Use `client.workspaces.getProviderDiagnostic` instead. */
  async getProviderDiagnostic(
    provider: AgentProvider,
    options?: { requestId?: string },
  ): Promise<ProviderDiagnosticPayload> {
    return this.workspaceRpc.getProviderDiagnostic(provider, options);
  }

  /** @deprecated Use `client.workspaces.listCommands` instead. */
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

  /** @deprecated Use `client.agents.respondToPermission` instead. */
  async respondToPermission(
    agentId: string,
    requestId: string,
    response: AgentPermissionResponse,
  ): Promise<void> {
    return this.agentRpc.respondToPermission(agentId, requestId, response);
  }

  /** @deprecated Use `client.agents.respondToPermissionAndWait` instead. */
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

  /** @deprecated Use `client.agents.waitForAgentUpsert` instead. */
  async waitForAgentUpsert(
    agentId: string,
    predicate: (snapshot: AgentSnapshotPayload) => boolean,
    timeout = 60000,
  ): Promise<AgentSnapshotPayload> {
    return this.agentRpc.waitForAgentUpsert(agentId, predicate, timeout);
  }

  /** @deprecated Use `client.agents.waitForFinish` instead. */
  async waitForFinish(agentId: string, timeout = 60000): Promise<WaitForFinishResult> {
    return this.agentRpc.waitForFinish(agentId, timeout);
  }

  // ============================================================================
  // Terminals
  // ============================================================================

  /** @deprecated Use `client.terminal.subscribeTerminals` instead. */
  subscribeTerminals(input: { cwd: string }): void {
    return this.terminalRpc.subscribeTerminals(input);
  }

  /** @deprecated Use `client.terminal.unsubscribeTerminals` instead. */
  unsubscribeTerminals(input: { cwd: string }): void {
    return this.terminalRpc.unsubscribeTerminals(input);
  }

  /** @deprecated Use `client.terminal.listTerminals` instead. */
  async listTerminals(cwd?: string, requestId?: string): Promise<ListTerminalsPayload> {
    return this.terminalRpc.listTerminals(cwd, requestId);
  }

  /** @deprecated Use `client.terminal.createTerminal` instead. */
  async createTerminal(
    cwd: string,
    name?: string,
    requestId?: string,
    options?: { agentId?: string; command?: string; args?: string[] },
  ): Promise<CreateTerminalPayload> {
    return this.terminalRpc.createTerminal(cwd, name, requestId, options);
  }

  /** @deprecated Use `client.terminal.subscribeTerminal` instead. */
  async subscribeTerminal(
    terminalId: string,
    requestId?: string,
  ): Promise<SubscribeTerminalPayload> {
    return this.terminalRpc.subscribeTerminal(terminalId, requestId);
  }

  /** @deprecated Use `client.terminal.unsubscribeTerminal` instead. */
  unsubscribeTerminal(terminalId: string): void {
    return this.terminalRpc.unsubscribeTerminal(terminalId);
  }

  /** @deprecated Use `client.terminal.sendTerminalInput` instead. */
  sendTerminalInput(terminalId: string, message: TerminalInput["message"]): void {
    return this.terminalRpc.sendTerminalInput(terminalId, message);
  }

  /** @deprecated Use `client.terminal.killTerminal` instead. */
  async killTerminal(terminalId: string, requestId?: string): Promise<KillTerminalPayload> {
    return this.terminalRpc.killTerminal(terminalId, requestId);
  }

  /** @deprecated Use `client.terminal.closeItems` instead. */
  async closeItems(
    input: { agentIds?: string[]; terminalIds?: string[] },
    requestId?: string,
  ): Promise<CloseItemsPayload> {
    return this.terminalRpc.closeItems(input, requestId);
  }

  /** @deprecated Use `client.terminal.captureTerminal` instead. */
  async captureTerminal(
    terminalId: string,
    options?: { start?: number; end?: number; stripAnsi?: boolean },
    requestId?: string,
  ): Promise<CaptureTerminalPayload> {
    return this.terminalRpc.captureTerminal(terminalId, options, requestId);
  }

  /** @deprecated Use `client.chat.createChatRoom` instead. */
  async createChatRoom(options: CreateChatRoomOptions): Promise<ChatCreatePayload> {
    return this.chatRpc.createChatRoom(options);
  }

  /** @deprecated Use `client.chat.listChatRooms` instead. */
  async listChatRooms(requestId?: string): Promise<ChatListPayload> {
    return this.chatRpc.listChatRooms(requestId);
  }

  /** @deprecated Use `client.chat.inspectChatRoom` instead. */
  async inspectChatRoom(options: InspectChatRoomOptions): Promise<ChatInspectPayload> {
    return this.chatRpc.inspectChatRoom(options);
  }

  /** @deprecated Use `client.chat.deleteChatRoom` instead. */
  async deleteChatRoom(options: DeleteChatRoomOptions): Promise<ChatDeletePayload> {
    return this.chatRpc.deleteChatRoom(options);
  }

  /** @deprecated Use `client.chat.postChatMessage` instead. */
  async postChatMessage(options: PostChatMessageOptions): Promise<ChatPostPayload> {
    return this.chatRpc.postChatMessage(options);
  }

  /** @deprecated Use `client.chat.readChatMessages` instead. */
  async readChatMessages(options: ReadChatMessagesOptions): Promise<ChatReadPayload> {
    return this.chatRpc.readChatMessages(options);
  }

  /** @deprecated Use `client.chat.waitForChatMessages` instead. */
  async waitForChatMessages(options: WaitForChatMessagesOptions): Promise<ChatWaitPayload> {
    return this.chatRpc.waitForChatMessages(options);
  }

  /** @deprecated Use `client.schedules.scheduleCreate` instead. */
  async scheduleCreate(options: CreateScheduleOptions): Promise<ScheduleCreatePayload> {
    return this.scheduleRpc.scheduleCreate(options);
  }

  /** @deprecated Use `client.schedules.scheduleList` instead. */
  async scheduleList(requestId?: string): Promise<ScheduleListPayload> {
    return this.scheduleRpc.scheduleList(requestId);
  }

  /** @deprecated Use `client.schedules.scheduleInspect` instead. */
  async scheduleInspect(options: InspectScheduleOptions): Promise<ScheduleInspectPayload> {
    return this.scheduleRpc.scheduleInspect(options);
  }

  /** @deprecated Use `client.schedules.scheduleLogs` instead. */
  async scheduleLogs(options: InspectScheduleOptions): Promise<ScheduleLogsPayload> {
    return this.scheduleRpc.scheduleLogs(options);
  }

  /** @deprecated Use `client.schedules.schedulePause` instead. */
  async schedulePause(options: InspectScheduleOptions): Promise<SchedulePausePayload> {
    return this.scheduleRpc.schedulePause(options);
  }

  /** @deprecated Use `client.schedules.scheduleResume` instead. */
  async scheduleResume(options: InspectScheduleOptions): Promise<ScheduleResumePayload> {
    return this.scheduleRpc.scheduleResume(options);
  }

  /** @deprecated Use `client.schedules.scheduleDelete` instead. */
  async scheduleDelete(options: InspectScheduleOptions): Promise<ScheduleDeletePayload> {
    return this.scheduleRpc.scheduleDelete(options);
  }

  /** @deprecated Use `client.schedules.scheduleUpdate` instead. */
  async scheduleUpdate(options: UpdateScheduleOptions): Promise<ScheduleUpdatePayload> {
    return this.scheduleRpc.scheduleUpdate(options);
  }

  /** @deprecated Use `client.schedules.scheduleAssist` instead. */
  async scheduleAssist(options: ScheduleAssistOptions): Promise<ScheduleAssistPayload> {
    return this.scheduleRpc.scheduleAssist(options);
  }

  /** @deprecated Use `client.terminal.tmuxListAgents` instead. */
  async tmuxListAgents(requestId?: string): Promise<TmuxListAgentsPayload> {
    return this.terminalRpc.tmuxListAgents(requestId);
  }

  /** @deprecated Use `client.terminal.tmuxCapturePane` instead. */
  async tmuxCapturePane(
    paneId: string,
    startLine?: number,
    lastContentHash?: string,
    cols?: number,
    requestId?: string,
  ): Promise<TmuxCapturePanePayload> {
    return this.terminalRpc.tmuxCapturePane(paneId, startLine, lastContentHash, cols, requestId);
  }

  /** @deprecated Use `client.terminal.tmuxSendKeys` instead. */
  async tmuxSendKeys(paneId: string, keys: string, sendEnter?: boolean, requestId?: string): Promise<TmuxSendKeysPayload> {
    return this.terminalRpc.tmuxSendKeys(paneId, keys, sendEnter, requestId);
  }

  /** @deprecated Use `client.terminal.tmuxStatusLine` instead. */
  async tmuxStatusLine(sessionId: string, requestId?: string): Promise<TmuxStatusLinePayload> {
    return this.terminalRpc.tmuxStatusLine(sessionId, requestId);
  }

  /** @deprecated Use `client.terminal.tmuxNewSession` instead. */
  async tmuxNewSession(name: string, options?: { workingDir?: string; command?: string }, requestId?: string): Promise<TmuxNewSessionPayload> {
    return this.terminalRpc.tmuxNewSession(name, options, requestId);
  }

  /** @deprecated Use `client.terminal.tmuxKillSession` instead. */
  async tmuxKillSession(sessionName: string, requestId?: string): Promise<TmuxKillSessionPayload> {
    return this.terminalRpc.tmuxKillSession(sessionName, requestId);
  }

  /** @deprecated Use `client.terminal.tmuxDeleteCommandHistory` instead. */
  async tmuxDeleteCommandHistory(launchCmd: string, requestId?: string): Promise<TmuxDeleteCommandHistoryPayload> {
    return this.terminalRpc.tmuxDeleteCommandHistory(launchCmd, requestId);
  }

  /** @deprecated Use `client.schedules.loopRun` instead. */
  async loopRun(options: RunLoopOptions): Promise<LoopRunPayload> {
    return this.scheduleRpc.loopRun(options);
  }

  /** @deprecated Use `client.schedules.loopList` instead. */
  async loopList(requestId?: string): Promise<LoopListPayload> {
    return this.scheduleRpc.loopList(requestId);
  }

  /** @deprecated Use `client.schedules.loopInspect` instead. */
  async loopInspect(options: string | InspectLoopOptions): Promise<LoopInspectPayload> {
    return this.scheduleRpc.loopInspect(options);
  }

  /** @deprecated Use `client.schedules.loopLogs` instead. */
  async loopLogs(options: string | LoopLogsOptions, afterSeq?: number): Promise<LoopLogsPayload> {
    return this.scheduleRpc.loopLogs(options, afterSeq);
  }

  /** @deprecated Use `client.schedules.loopStop` instead. */
  async loopStop(options: string | StopLoopOptions): Promise<LoopStopPayload> {
    return this.scheduleRpc.loopStop(options);
  }

  /** @deprecated Use `client.schedules.loopUpdate` instead. */
  async loopUpdate(options: UpdateLoopOptions): Promise<LoopUpdatePayload> {
    return this.scheduleRpc.loopUpdate(options);
  }

  /** @deprecated Use `client.schedules.loopDelete` instead. */
  async loopDelete(options: string | DeleteLoopOptions): Promise<LoopDeletePayload> {
    return this.scheduleRpc.loopDelete(options);
  }

  /** @deprecated Use `client.schedules.loopTemplateList` instead. */
  async loopTemplateList(requestId?: string): Promise<LoopTemplateListPayload> {
    return this.scheduleRpc.loopTemplateList(requestId);
  }

  /** @deprecated Use `client.schedules.loopTemplateGet` instead. */
  async loopTemplateGet(options: string | GetLoopTemplateOptions): Promise<LoopTemplateGetPayload> {
    return this.scheduleRpc.loopTemplateGet(options);
  }

  /** @deprecated Use `client.schedules.loopTemplateDelete` instead. */
  async loopTemplateDelete(options: string | DeleteLoopTemplateOptions): Promise<LoopTemplateDeletePayload> {
    return this.scheduleRpc.loopTemplateDelete(options);
  }

  /** @deprecated Use `client.terminal.onTerminalStreamEvent` instead. */
  onTerminalStreamEvent(handler: (event: TerminalStreamEvent) => void): () => void {
    return this.terminalRpc.onTerminalStreamEvent(handler);
  }

  /** @deprecated Use `client.terminal.waitForTerminalStreamEvent` instead. */
  async waitForTerminalStreamEvent(
    predicate: (event: TerminalStreamEvent) => boolean,
    timeout = 5000,
  ): Promise<TerminalStreamEvent> {
    return this.terminalRpc.waitForTerminalStreamEvent(predicate, timeout);
  }
}
