import { SessionInboundMessageSchema } from "../shared/messages.js";
import type {
  EditorTargetId,
  MutableDaemonConfig,
  MutableDaemonConfigPatch,
  ProjectIconResponse,
  SessionOutboundMessage,
} from "../shared/messages.js";
import type { AgentProvider, AgentSessionConfig } from "../server/agent/agent-sdk-types.js";
import type {
  FetchWorkspacesOptions,
  WriteProjectConfigInput,
} from "./daemon-client.js";
import type { ConnectionManager } from "./connection-manager.js";

type FetchWorkspacesPayload = Extract<SessionOutboundMessage, { type: "fetch_workspaces_response" }>["payload"];
type OpenProjectPayload = Extract<SessionOutboundMessage, { type: "open_project_response" }>["payload"];
type StartWorkspaceScriptPayload = Extract<SessionOutboundMessage, { type: "start_workspace_script_response" }>["payload"];
type ListAvailableEditorsPayload = Extract<SessionOutboundMessage, { type: "list_available_editors_response" }>["payload"];
type OpenInEditorPayload = Extract<SessionOutboundMessage, { type: "open_in_editor_response" }>["payload"];
type ArchiveWorkspacePayload = Extract<SessionOutboundMessage, { type: "archive_workspace_response" }>["payload"];
type RemoveProjectPayload = Extract<SessionOutboundMessage, { type: "remove_project_response" }>["payload"];
type WorkspaceSetupStatusPayload = Extract<SessionOutboundMessage, { type: "workspace_setup_status_response" }>["payload"];
type FileExplorerPayload = Extract<SessionOutboundMessage, { type: "file_explorer_response" }>["payload"];
type FileDownloadTokenPayload = Extract<SessionOutboundMessage, { type: "file_download_token_response" }>["payload"];
type ProjectIconPayload = ProjectIconResponse["payload"];
type ListProviderModelsPayload = Extract<SessionOutboundMessage, { type: "list_provider_models_response" }>["payload"];
type ListProviderModesPayload = Extract<SessionOutboundMessage, { type: "list_provider_modes_response" }>["payload"];
type ListProviderFeaturesPayload = Extract<SessionOutboundMessage, { type: "list_provider_features_response" }>["payload"];
type ListAvailableProvidersPayload = Extract<SessionOutboundMessage, { type: "list_available_providers_response" }>["payload"];
type GetProvidersSnapshotPayload = Extract<SessionOutboundMessage, { type: "get_providers_snapshot_response" }>["payload"];
type RefreshProvidersSnapshotPayload = Extract<SessionOutboundMessage, { type: "refresh_providers_snapshot_response" }>["payload"];
type ProviderDiagnosticPayload = Extract<SessionOutboundMessage, { type: "provider_diagnostic_response" }>["payload"];
type ReadProjectConfigPayload = Extract<SessionOutboundMessage, { type: "read_project_config_response" }>["payload"];
type WriteProjectConfigPayload = Extract<SessionOutboundMessage, { type: "write_project_config_response" }>["payload"];
type ListCommandsPayload = Extract<SessionOutboundMessage, { type: "list_commands_response" }>["payload"];

type ListCommandsDraftConfig = Pick<
  AgentSessionConfig,
  "provider" | "cwd" | "modeId" | "model" | "thinkingOptionId" | "featureValues"
>;
interface ListCommandsOptions {
  requestId?: string;
  draftConfig?: ListCommandsDraftConfig;
}

export class WorkspaceRpc {
  constructor(private readonly client: ConnectionManager) {}

  // ============================================================================
  // Workspace / Project
  // ============================================================================

  async fetchWorkspaces(options?: FetchWorkspacesOptions): Promise<FetchWorkspacesPayload> {
    const resolvedRequestId = this.client.createRequestId(options?.requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "fetch_workspaces_request",
      requestId: resolvedRequestId,
      ...(options?.filter ? { filter: options.filter } : {}),
      ...(options?.sort ? { sort: options.sort } : {}),
      ...(options?.page ? { page: options.page } : {}),
      ...(options?.subscribe ? { subscribe: options.subscribe } : {}),
    });
    return this.client.sendRequest({
      requestId: resolvedRequestId,
      message,
      timeout: 10000,
      options: { skipQueue: true },
      select: (msg) => {
        if (msg.type !== "fetch_workspaces_response") {
          return null;
        }
        if (msg.payload.requestId !== resolvedRequestId) {
          return null;
        }
        return msg.payload;
      },
    });
  }

  async openProject(cwd: string, requestId?: string): Promise<OpenProjectPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "open_project_request",
        cwd,
      },
      responseType: "open_project_response",
      timeout: 10000,
    });
  }

  async startWorkspaceScript(
    workspaceId: string,
    scriptName: string,
    requestId?: string,
  ): Promise<StartWorkspaceScriptPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "start_workspace_script_request",
        workspaceId,
        scriptName,
      },
      responseType: "start_workspace_script_response",
      timeout: 10000,
    });
  }

  async listAvailableEditors(requestId?: string): Promise<ListAvailableEditorsPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "list_available_editors_request",
      },
      responseType: "list_available_editors_response",
      timeout: 10000,
    });
  }

  async openInEditor(
    path: string,
    editorId: EditorTargetId,
    requestId?: string,
  ): Promise<OpenInEditorPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "open_in_editor_request",
        path,
        editorId,
      },
      responseType: "open_in_editor_response",
      timeout: 10000,
    });
  }

  async archiveWorkspace(
    workspaceId: string,
    requestId?: string,
  ): Promise<ArchiveWorkspacePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "archive_workspace_request",
        workspaceId,
      },
      responseType: "archive_workspace_response",
      timeout: 10000,
    });
  }

  async removeProject(
    workspaceIds: string[],
    requestId?: string,
  ): Promise<RemoveProjectPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "remove_project_request",
        workspaceIds,
      },
      responseType: "remove_project_response",
      timeout: 15000,
    });
  }

  async fetchWorkspaceSetupStatus(
    workspaceId: string,
    requestId?: string,
  ): Promise<WorkspaceSetupStatusPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "workspace_setup_status_request",
        workspaceId,
      },
      responseType: "workspace_setup_status_response",
      timeout: 10000,
    });
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
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "file_explorer_request",
        cwd,
        path,
        mode,
      },
      responseType: "file_explorer_response",
      timeout: 10000,
    });
  }

  async requestDownloadToken(
    cwd: string,
    path: string,
    requestId?: string,
  ): Promise<FileDownloadTokenPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "file_download_token_request",
        cwd,
        path,
      },
      responseType: "file_download_token_response",
      timeout: 10000,
    });
  }

  async requestProjectIcon(
    cwd: string,
    requestId?: string,
  ): Promise<ProjectIconPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "project_icon_request",
        cwd,
      },
      responseType: "project_icon_response",
      timeout: 10000,
    });
  }

  // ============================================================================
  // Provider / Config
  // ============================================================================

  async listProviderModels(
    provider: AgentProvider,
    options?: { cwd?: string; requestId?: string },
  ): Promise<ListProviderModelsPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options?.requestId,
      message: {
        type: "list_provider_models_request",
        provider,
        cwd: options?.cwd,
      },
      responseType: "list_provider_models_response",
      // Provider SDK cold starts (especially model discovery) can exceed 30s.
      timeout: 45000,
    });
  }

  async listProviderModes(
    provider: AgentProvider,
    options?: { cwd?: string; requestId?: string },
  ): Promise<ListProviderModesPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options?.requestId,
      message: {
        type: "list_provider_modes_request",
        provider,
        cwd: options?.cwd,
      },
      responseType: "list_provider_modes_response",
      timeout: 45000,
    });
  }

  async listProviderFeatures(
    draftConfig: ListCommandsDraftConfig,
    options?: { requestId?: string },
  ): Promise<ListProviderFeaturesPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options?.requestId,
      message: {
        type: "list_provider_features_request",
        draftConfig,
      },
      responseType: "list_provider_features_response",
      timeout: 45000,
    });
  }

  async listAvailableProviders(options?: {
    requestId?: string;
  }): Promise<ListAvailableProvidersPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options?.requestId,
      message: {
        type: "list_available_providers_request",
      },
      responseType: "list_available_providers_response",
      timeout: 30000,
    });
  }

  async getProvidersSnapshot(options?: {
    cwd?: string;
    requestId?: string;
  }): Promise<GetProvidersSnapshotPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options?.requestId,
      message: {
        type: "get_providers_snapshot_request",
        cwd: options?.cwd,
      },
      responseType: "get_providers_snapshot_response",
      timeout: 10000,
    });
  }

  async refreshProvidersSnapshot(options?: {
    cwd?: string;
    providers?: AgentProvider[];
    requestId?: string;
  }): Promise<RefreshProvidersSnapshotPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options?.requestId,
      message: {
        type: "refresh_providers_snapshot_request",
        cwd: options?.cwd,
        providers: options?.providers,
      },
      responseType: "refresh_providers_snapshot_response",
      timeout: 60000,
    });
  }

  async getProviderDiagnostic(
    provider: AgentProvider,
    options?: { requestId?: string },
  ): Promise<ProviderDiagnosticPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options?.requestId,
      message: {
        type: "provider_diagnostic_request",
        provider,
      },
      responseType: "provider_diagnostic_response",
      timeout: 30000,
    });
  }

  async getDaemonConfig(
    requestId?: string,
  ): Promise<{ requestId: string; config: MutableDaemonConfig }> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "get_daemon_config_request",
      },
      responseType: "get_daemon_config_response",
      timeout: 10000,
    });
  }

  async patchDaemonConfig(
    config: MutableDaemonConfigPatch,
    requestId?: string,
  ): Promise<{ requestId: string; config: MutableDaemonConfig }> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "set_daemon_config_request",
        config,
      },
      responseType: "set_daemon_config_response",
      timeout: 10000,
    });
  }

  async readProjectConfig(repoRoot: string, requestId?: string): Promise<ReadProjectConfigPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "read_project_config_request",
        repoRoot,
      },
      responseType: "read_project_config_response",
      timeout: 10000,
    });
  }

  async writeProjectConfig(input: WriteProjectConfigInput): Promise<WriteProjectConfigPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: input.requestId,
      message: {
        type: "write_project_config_request",
        repoRoot: input.repoRoot,
        config: input.config,
        expectedRevision: input.expectedRevision,
      },
      responseType: "write_project_config_response",
      timeout: 10000,
    });
  }

  async listCommands(agentId: string, requestId?: string): Promise<ListCommandsPayload>;
  async listCommands(agentId: string, options?: ListCommandsOptions): Promise<ListCommandsPayload>;
  async listCommands(
    agentId: string,
    requestIdOrOptions?: string | ListCommandsOptions,
  ): Promise<ListCommandsPayload> {
    const requestId =
      typeof requestIdOrOptions === "string" ? requestIdOrOptions : requestIdOrOptions?.requestId;
    const draftConfig =
      typeof requestIdOrOptions === "string" ? undefined : requestIdOrOptions?.draftConfig;

    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "list_commands_request",
        agentId,
        ...(draftConfig ? { draftConfig } : {}),
      },
      responseType: "list_commands_response",
      timeout: 30000,
    });
  }
}
