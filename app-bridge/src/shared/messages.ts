// ============================================================================
// messages.ts — Barrel re-export + session/WS union assembly + helpers
//
// Domain schemas live in per-domain files:
//   messages-base.ts      — WS envelope, infra, daemon config
//   messages-agent.ts     — Agent lifecycle, providers, voice/dictation
//   messages-workspace.ts — Workspaces, editors, file explorer, worktrees
//   messages-git.ts       — Checkout, stash, branch, GitHub search
//   messages-terminal.ts  — Terminal lifecycle and streaming
//
// This file re-exports all domain schemas (preserving the public API),
// assembles the discriminated unions that span domains, and provides
// WS-level message helpers.
// ============================================================================

export * from "./messages-base.js";
export * from "./messages-agent.js";
export * from "./messages-workspace.js";
export * from "./messages-git.js";
export * from "./messages-terminal.js";

import { z } from "zod";
import {
  ChatCreateRequestSchema,
  ChatListRequestSchema,
  ChatInspectRequestSchema,
  ChatDeleteRequestSchema,
  ChatPostRequestSchema,
  ChatReadRequestSchema,
  ChatWaitRequestSchema,
  ChatCreateResponseSchema,
  ChatListResponseSchema,
  ChatInspectResponseSchema,
  ChatDeleteResponseSchema,
  ChatPostResponseSchema,
  ChatReadResponseSchema,
  ChatWaitResponseSchema,
} from "../server/chat/chat-rpc-schemas.js";
import {
  ScheduleCreateRequestSchema,
  ScheduleListRequestSchema,
  ScheduleInspectRequestSchema,
  ScheduleLogsRequestSchema,
  SchedulePauseRequestSchema,
  ScheduleResumeRequestSchema,
  ScheduleDeleteRequestSchema,
  ScheduleUpdateRequestSchema,
  ScheduleAssistRequestSchema,
  ScheduleCreateResponseSchema,
  ScheduleListResponseSchema,
  ScheduleInspectResponseSchema,
  ScheduleLogsResponseSchema,
  SchedulePauseResponseSchema,
  ScheduleResumeResponseSchema,
  ScheduleDeleteResponseSchema,
  ScheduleUpdateResponseSchema,
  ScheduleAssistResponseSchema,
} from "../server/schedule/rpc-schemas.js";
import {
  TmuxListAgentsRequestSchema,
  TmuxListAgentsResponseSchema,
  TmuxCapturePaneRequestSchema,
  TmuxCapturePaneResponseSchema,
  TmuxSendKeysRequestSchema,
  TmuxSendKeysResponseSchema,
  TmuxGetThemeRequestSchema,
  TmuxGetThemeResponseSchema,
  TmuxStatusLineRequestSchema,
  TmuxStatusLineResponseSchema,
  TmuxNewSessionRequestSchema,
  TmuxNewSessionResponseSchema,
  TmuxKillSessionRequestSchema,
  TmuxKillSessionResponseSchema,
  TmuxDeleteCommandHistoryRequestSchema,
  TmuxDeleteCommandHistoryResponseSchema,
} from "../server/tmux/rpc-schemas.js";
import {
  LoopRunRequestSchema,
  LoopListRequestSchema,
  LoopInspectRequestSchema,
  LoopLogsRequestSchema,
  LoopStopRequestSchema,
  LoopUpdateRequestSchema,
  LoopDeleteRequestSchema,
  LoopTemplateListRequestSchema,
  LoopTemplateListResponseSchema,
  LoopTemplateGetRequestSchema,
  LoopTemplateGetResponseSchema,
  LoopTemplateDeleteRequestSchema,
  LoopTemplateDeleteResponseSchema,
  LoopRunResponseSchema,
  LoopListResponseSchema,
  LoopInspectResponseSchema,
  LoopLogsResponseSchema,
  LoopStopResponseSchema,
  LoopUpdateResponseSchema,
  LoopDeleteResponseSchema,
} from "../server/loop/rpc-schemas.js";

// Domain schema imports for union assembly
import {
  // base
  StatusMessageSchema,
  PongMessageSchema,
  RpcErrorMessageSchema,
  ActivityLogMessageSchema,
  AssistantChunkMessageSchema,
  ArtifactMessageSchema,
  ServerInfoStatusPayloadSchema,
  RestartRequestedStatusPayloadSchema,
  ShutdownRequestedStatusPayloadSchema,
  DaemonConfigChangedStatusPayloadSchema,
  GetDaemonConfigRequestMessageSchema,
  SetDaemonConfigRequestMessageSchema,
  ReadProjectConfigRequestMessageSchema,
  WriteProjectConfigRequestMessageSchema,
  RestartServerRequestMessageSchema,
  ShutdownServerRequestMessageSchema,
  ClientHeartbeatMessageSchema,
  PingMessageSchema,
  ListCommandsRequestSchema,
  RegisterPushTokenMessageSchema,
  GetDaemonConfigResponseMessageSchema,
  SetDaemonConfigResponseMessageSchema,
  ReadProjectConfigResponseMessageSchema,
  WriteProjectConfigResponseMessageSchema,
  ListCommandsResponseSchema,
  WSPingMessageSchema,
  WSPongMessageSchema,
  WSHelloMessageSchema,
  WSRecordingStateMessageSchema,
  type ServerInfoStatusPayload,
} from "./messages-base.js";
import {
  // agent
  VoiceAudioChunkMessageSchema,
  AbortRequestMessageSchema,
  AudioPlayedMessageSchema,
  AgentCreatedStatusPayloadSchema,
  AgentCreateFailedStatusPayloadSchema,
  AgentResumedStatusPayloadSchema,
  AgentRefreshedStatusPayloadSchema,
  FetchAgentsRequestMessageSchema,
  FetchAgentHistoryRequestMessageSchema,
  FetchAgentRequestMessageSchema,
  DeleteAgentRequestMessageSchema,
  ArchiveAgentRequestMessageSchema,
  CloseItemsRequestMessageSchema,
  UpdateAgentRequestMessageSchema,
  SetVoiceModeMessageSchema,
  SendAgentMessageRequestSchema,
  WaitForFinishRequestSchema,
  DictationStreamStartMessageSchema,
  DictationStreamChunkMessageSchema,
  DictationStreamFinishMessageSchema,
  DictationStreamCancelMessageSchema,
  CreateAgentRequestMessageSchema,
  ListProviderModelsRequestMessageSchema,
  ListProviderModesRequestMessageSchema,
  ListProviderFeaturesRequestMessageSchema,
  ListAvailableProvidersRequestMessageSchema,
  GetProvidersSnapshotRequestMessageSchema,
  RefreshProvidersSnapshotRequestMessageSchema,
  ProviderDiagnosticRequestMessageSchema,
  ResumeAgentRequestMessageSchema,
  RefreshAgentRequestMessageSchema,
  CancelAgentRequestMessageSchema,
  FetchAgentTimelineRequestMessageSchema,
  SetAgentModeRequestMessageSchema,
  SetAgentModelRequestMessageSchema,
  SetAgentThinkingRequestMessageSchema,
  SetAgentFeatureRequestMessageSchema,
  AgentPermissionResponseMessageSchema,
  ClearAgentAttentionMessageSchema,
  AudioOutputMessageSchema,
  TranscriptionResultMessageSchema,
  VoiceInputStateMessageSchema,
  DictationStreamAckMessageSchema,
  DictationStreamFinishAcceptedMessageSchema,
  DictationStreamPartialMessageSchema,
  DictationStreamFinalMessageSchema,
  DictationStreamErrorMessageSchema,
  AgentUpdateMessageSchema,
  AgentStreamMessageSchema,
  AgentStatusMessageSchema,
  FetchAgentsResponseMessageSchema,
  FetchAgentHistoryResponseMessageSchema,
  FetchAgentResponseMessageSchema,
  FetchAgentTimelineResponseMessageSchema,
  CancelAgentResponseMessageSchema,
  ClearAgentAttentionResponseMessageSchema,
  SendAgentMessageResponseMessageSchema,
  SetVoiceModeResponseMessageSchema,
  SetAgentModeResponseMessageSchema,
  SetAgentModelResponseMessageSchema,
  SetAgentThinkingResponseMessageSchema,
  SetAgentFeatureResponseMessageSchema,
  UpdateAgentResponseMessageSchema,
  WaitForFinishResponseMessageSchema,
  AgentPermissionRequestMessageSchema,
  AgentPermissionResolvedMessageSchema,
  AgentDeletedMessageSchema,
  AgentArchivedMessageSchema,
  CloseItemsResponseSchema,
  ListProviderModelsResponseMessageSchema,
  ListProviderModesResponseMessageSchema,
  ListProviderFeaturesResponseMessageSchema,
  ListAvailableProvidersResponseSchema,
  GetProvidersSnapshotResponseMessageSchema,
  ProvidersSnapshotUpdateMessageSchema,
  RefreshProvidersSnapshotResponseMessageSchema,
  ProviderDiagnosticResponseMessageSchema,
} from "./messages-agent.js";
import {
  // workspace
  FetchWorkspacesRequestMessageSchema,
  DirectorySuggestionsRequestSchema,
  SoloWorktreeListRequestSchema,
  SoloWorktreeArchiveRequestSchema,
  CreateSoloWorktreeRequestSchema,
  WorkspaceSetupStatusRequestSchema,
  ListAvailableEditorsRequestSchema,
  OpenInEditorRequestSchema,
  OpenProjectRequestSchema,
  ArchiveWorkspaceRequestSchema,
  RemoveProjectRequestSchema,
  FileExplorerRequestSchema,
  ProjectIconRequestSchema,
  FileDownloadTokenRequestSchema,
  StartWorkspaceScriptRequestSchema,
  FetchWorkspacesResponseMessageSchema,
  WorkspaceUpdateMessageSchema,
  ScriptStatusUpdateMessageSchema,
  WorkspaceSetupProgressMessageSchema,
  WorkspaceSetupStatusResponseMessageSchema,
  OpenProjectResponseMessageSchema,
  StartWorkspaceScriptResponseMessageSchema,
  ListAvailableEditorsResponseMessageSchema,
  OpenInEditorResponseMessageSchema,
  ArchiveWorkspaceResponseMessageSchema,
  RemoveProjectResponseMessageSchema,
  DirectorySuggestionsResponseSchema,
  SoloWorktreeListResponseSchema,
  SoloWorktreeArchiveResponseSchema,
  CreateSoloWorktreeResponseSchema,
  FileExplorerResponseSchema,
  ProjectIconResponseSchema,
  FileDownloadTokenResponseSchema,
} from "./messages-workspace.js";
import {
  // git
  CheckoutStatusRequestSchema,
  SubscribeCheckoutDiffRequestSchema,
  UnsubscribeCheckoutDiffRequestSchema,
  CheckoutCommitRequestSchema,
  CheckoutMergeRequestSchema,
  CheckoutMergeFromBaseRequestSchema,
  CheckoutPullRequestSchema,
  CheckoutPushRequestSchema,
  CheckoutPrCreateRequestSchema,
  CheckoutPrStatusRequestSchema,
  PullRequestTimelineRequestSchema,
  CheckoutSwitchBranchRequestSchema,
  StashSaveRequestSchema,
  StashPopRequestSchema,
  StashListRequestSchema,
  ValidateBranchRequestSchema,
  BranchSuggestionsRequestSchema,
  GitHubSearchRequestSchema,
  CheckoutStatusResponseSchema,
  CheckoutStatusUpdateSchema,
  SubscribeCheckoutDiffResponseSchema,
  CheckoutDiffUpdateSchema,
  CheckoutCommitResponseSchema,
  CheckoutMergeResponseSchema,
  CheckoutMergeFromBaseResponseSchema,
  CheckoutPullResponseSchema,
  CheckoutPushResponseSchema,
  CheckoutPrCreateResponseSchema,
  CheckoutPrStatusResponseSchema,
  PullRequestTimelineResponseSchema,
  CheckoutSwitchBranchResponseSchema,
  StashSaveResponseSchema,
  StashPopResponseSchema,
  StashListResponseSchema,
  ValidateBranchResponseSchema,
  BranchSuggestionsResponseSchema,
  GitHubSearchResponseSchema,
} from "./messages-git.js";
import {
  // terminal
  ListTerminalsRequestSchema,
  SubscribeTerminalsRequestSchema,
  UnsubscribeTerminalsRequestSchema,
  CreateTerminalRequestSchema,
  SubscribeTerminalRequestSchema,
  UnsubscribeTerminalRequestSchema,
  TerminalInputSchema,
  KillTerminalRequestSchema,
  CaptureTerminalRequestSchema,
  ListTerminalsResponseSchema,
  TerminalsChangedSchema,
  CreateTerminalResponseSchema,
  SubscribeTerminalResponseSchema,
  KillTerminalResponseSchema,
  CaptureTerminalResponseSchema,
  TerminalStreamExitSchema,
} from "./messages-terminal.js";

// ============================================================================
// Session Inbound Messages (Session receives these)
// ============================================================================

export const SessionInboundMessageSchema = z.discriminatedUnion("type", [
  VoiceAudioChunkMessageSchema,
  AbortRequestMessageSchema,
  AudioPlayedMessageSchema,
  FetchAgentsRequestMessageSchema,
  FetchAgentHistoryRequestMessageSchema,
  FetchWorkspacesRequestMessageSchema,
  FetchAgentRequestMessageSchema,
  DeleteAgentRequestMessageSchema,
  ArchiveAgentRequestMessageSchema,
  CloseItemsRequestMessageSchema,
  UpdateAgentRequestMessageSchema,
  SetVoiceModeMessageSchema,
  SendAgentMessageRequestSchema,
  WaitForFinishRequestSchema,
  GetDaemonConfigRequestMessageSchema,
  SetDaemonConfigRequestMessageSchema,
  ReadProjectConfigRequestMessageSchema,
  WriteProjectConfigRequestMessageSchema,
  DictationStreamStartMessageSchema,
  DictationStreamChunkMessageSchema,
  DictationStreamFinishMessageSchema,
  DictationStreamCancelMessageSchema,
  CreateAgentRequestMessageSchema,
  ListProviderModelsRequestMessageSchema,
  ListProviderModesRequestMessageSchema,
  ListProviderFeaturesRequestMessageSchema,
  ListAvailableProvidersRequestMessageSchema,
  GetProvidersSnapshotRequestMessageSchema,
  RefreshProvidersSnapshotRequestMessageSchema,
  ProviderDiagnosticRequestMessageSchema,
  ResumeAgentRequestMessageSchema,
  RefreshAgentRequestMessageSchema,
  CancelAgentRequestMessageSchema,
  ShutdownServerRequestMessageSchema,
  RestartServerRequestMessageSchema,
  FetchAgentTimelineRequestMessageSchema,
  SetAgentModeRequestMessageSchema,
  SetAgentModelRequestMessageSchema,
  SetAgentThinkingRequestMessageSchema,
  SetAgentFeatureRequestMessageSchema,
  AgentPermissionResponseMessageSchema,
  CheckoutStatusRequestSchema,
  SubscribeCheckoutDiffRequestSchema,
  UnsubscribeCheckoutDiffRequestSchema,
  CheckoutCommitRequestSchema,
  CheckoutMergeRequestSchema,
  CheckoutMergeFromBaseRequestSchema,
  CheckoutPullRequestSchema,
  CheckoutPushRequestSchema,
  CheckoutPrCreateRequestSchema,
  CheckoutPrStatusRequestSchema,
  PullRequestTimelineRequestSchema,
  CheckoutSwitchBranchRequestSchema,
  StashSaveRequestSchema,
  StashPopRequestSchema,
  StashListRequestSchema,
  ValidateBranchRequestSchema,
  BranchSuggestionsRequestSchema,
  GitHubSearchRequestSchema,
  DirectorySuggestionsRequestSchema,
  SoloWorktreeListRequestSchema,
  SoloWorktreeArchiveRequestSchema,
  CreateSoloWorktreeRequestSchema,
  WorkspaceSetupStatusRequestSchema,
  ListAvailableEditorsRequestSchema,
  OpenInEditorRequestSchema,
  OpenProjectRequestSchema,
  ArchiveWorkspaceRequestSchema,
  RemoveProjectRequestSchema,
  FileExplorerRequestSchema,
  ProjectIconRequestSchema,
  FileDownloadTokenRequestSchema,
  ClearAgentAttentionMessageSchema,
  ClientHeartbeatMessageSchema,
  PingMessageSchema,
  ListCommandsRequestSchema,
  RegisterPushTokenMessageSchema,
  ListTerminalsRequestSchema,
  SubscribeTerminalsRequestSchema,
  UnsubscribeTerminalsRequestSchema,
  CreateTerminalRequestSchema,
  StartWorkspaceScriptRequestSchema,
  SubscribeTerminalRequestSchema,
  UnsubscribeTerminalRequestSchema,
  TerminalInputSchema,
  KillTerminalRequestSchema,
  CaptureTerminalRequestSchema,
  ChatCreateRequestSchema,
  ChatListRequestSchema,
  ChatInspectRequestSchema,
  ChatDeleteRequestSchema,
  ChatPostRequestSchema,
  ChatReadRequestSchema,
  ChatWaitRequestSchema,
  ScheduleCreateRequestSchema,
  ScheduleListRequestSchema,
  ScheduleInspectRequestSchema,
  ScheduleLogsRequestSchema,
  SchedulePauseRequestSchema,
  ScheduleResumeRequestSchema,
  ScheduleDeleteRequestSchema,
  ScheduleUpdateRequestSchema,
  ScheduleAssistRequestSchema,
  TmuxListAgentsRequestSchema,
  TmuxCapturePaneRequestSchema,
  TmuxSendKeysRequestSchema,
  TmuxNewSessionRequestSchema,
  TmuxKillSessionRequestSchema,
  TmuxDeleteCommandHistoryRequestSchema,
  TmuxGetThemeRequestSchema,
  TmuxStatusLineRequestSchema,
  LoopRunRequestSchema,
  LoopListRequestSchema,
  LoopInspectRequestSchema,
  LoopLogsRequestSchema,
  LoopStopRequestSchema,
  LoopUpdateRequestSchema,
  LoopDeleteRequestSchema,
  LoopTemplateListRequestSchema,
  LoopTemplateGetRequestSchema,
  LoopTemplateDeleteRequestSchema,
]);

export type SessionInboundMessage = z.infer<typeof SessionInboundMessageSchema>;

// ============================================================================
// Session Outbound Messages (Session emits these)
// ============================================================================

export const KnownStatusPayloadSchema = z.discriminatedUnion("status", [
  AgentCreatedStatusPayloadSchema,
  AgentCreateFailedStatusPayloadSchema,
  AgentResumedStatusPayloadSchema,
  AgentRefreshedStatusPayloadSchema,
  ShutdownRequestedStatusPayloadSchema,
  RestartRequestedStatusPayloadSchema,
  DaemonConfigChangedStatusPayloadSchema,
]);

export type KnownStatusPayload = z.infer<typeof KnownStatusPayloadSchema>;

export type SessionOutboundMessage =
  | z.infer<typeof ActivityLogMessageSchema>
  | z.infer<typeof AssistantChunkMessageSchema>
  | z.infer<typeof AudioOutputMessageSchema>
  | z.infer<typeof TranscriptionResultMessageSchema>
  | z.infer<typeof VoiceInputStateMessageSchema>
  | z.infer<typeof DictationStreamAckMessageSchema>
  | z.infer<typeof DictationStreamFinishAcceptedMessageSchema>
  | z.infer<typeof DictationStreamPartialMessageSchema>
  | z.infer<typeof DictationStreamFinalMessageSchema>
  | z.infer<typeof DictationStreamErrorMessageSchema>
  | z.infer<typeof StatusMessageSchema>
  | z.infer<typeof PongMessageSchema>
  | z.infer<typeof RpcErrorMessageSchema>
  | z.infer<typeof ArtifactMessageSchema>
  | z.infer<typeof AgentUpdateMessageSchema>
  | z.infer<typeof WorkspaceUpdateMessageSchema>
  | z.infer<typeof ScriptStatusUpdateMessageSchema>
  | z.infer<typeof WorkspaceSetupProgressMessageSchema>
  | z.infer<typeof WorkspaceSetupStatusResponseMessageSchema>
  | z.infer<typeof AgentStreamMessageSchema>
  | z.infer<typeof AgentStatusMessageSchema>
  | z.infer<typeof FetchAgentsResponseMessageSchema>
  | z.infer<typeof FetchAgentHistoryResponseMessageSchema>
  | z.infer<typeof FetchWorkspacesResponseMessageSchema>
  | z.infer<typeof OpenProjectResponseMessageSchema>
  | z.infer<typeof StartWorkspaceScriptResponseMessageSchema>
  | z.infer<typeof ListAvailableEditorsResponseMessageSchema>
  | z.infer<typeof OpenInEditorResponseMessageSchema>
  | z.infer<typeof ArchiveWorkspaceResponseMessageSchema>
  | z.infer<typeof RemoveProjectResponseMessageSchema>
  | z.infer<typeof FetchAgentResponseMessageSchema>
  | z.infer<typeof FetchAgentTimelineResponseMessageSchema>
  | z.infer<typeof CancelAgentResponseMessageSchema>
  | z.infer<typeof ClearAgentAttentionResponseMessageSchema>
  | z.infer<typeof SendAgentMessageResponseMessageSchema>
  | z.infer<typeof SetVoiceModeResponseMessageSchema>
  | z.infer<typeof GetDaemonConfigResponseMessageSchema>
  | z.infer<typeof SetDaemonConfigResponseMessageSchema>
  | z.infer<typeof ReadProjectConfigResponseMessageSchema>
  | z.infer<typeof WriteProjectConfigResponseMessageSchema>
  | z.infer<typeof SetAgentModeResponseMessageSchema>
  | z.infer<typeof SetAgentModelResponseMessageSchema>
  | z.infer<typeof SetAgentThinkingResponseMessageSchema>
  | z.infer<typeof SetAgentFeatureResponseMessageSchema>
  | z.infer<typeof UpdateAgentResponseMessageSchema>
  | z.infer<typeof WaitForFinishResponseMessageSchema>
  | z.infer<typeof AgentPermissionRequestMessageSchema>
  | z.infer<typeof AgentPermissionResolvedMessageSchema>
  | z.infer<typeof AgentDeletedMessageSchema>
  | z.infer<typeof AgentArchivedMessageSchema>
  | z.infer<typeof CloseItemsResponseSchema>
  | z.infer<typeof CheckoutStatusResponseSchema>
  | z.infer<typeof CheckoutStatusUpdateSchema>
  | z.infer<typeof SubscribeCheckoutDiffResponseSchema>
  | z.infer<typeof CheckoutDiffUpdateSchema>
  | z.infer<typeof CheckoutCommitResponseSchema>
  | z.infer<typeof CheckoutMergeResponseSchema>
  | z.infer<typeof CheckoutMergeFromBaseResponseSchema>
  | z.infer<typeof CheckoutPullResponseSchema>
  | z.infer<typeof CheckoutPushResponseSchema>
  | z.infer<typeof CheckoutPrCreateResponseSchema>
  | z.infer<typeof CheckoutPrStatusResponseSchema>
  | z.infer<typeof PullRequestTimelineResponseSchema>
  | z.infer<typeof CheckoutSwitchBranchResponseSchema>
  | z.infer<typeof StashSaveResponseSchema>
  | z.infer<typeof StashPopResponseSchema>
  | z.infer<typeof StashListResponseSchema>
  | z.infer<typeof ValidateBranchResponseSchema>
  | z.infer<typeof BranchSuggestionsResponseSchema>
  | z.infer<typeof GitHubSearchResponseSchema>
  | z.infer<typeof DirectorySuggestionsResponseSchema>
  | z.infer<typeof SoloWorktreeListResponseSchema>
  | z.infer<typeof SoloWorktreeArchiveResponseSchema>
  | z.infer<typeof CreateSoloWorktreeResponseSchema>
  | z.infer<typeof FileExplorerResponseSchema>
  | z.infer<typeof ProjectIconResponseSchema>
  | z.infer<typeof FileDownloadTokenResponseSchema>
  | z.infer<typeof ListProviderModelsResponseMessageSchema>
  | z.infer<typeof ListProviderModesResponseMessageSchema>
  | z.infer<typeof ListProviderFeaturesResponseMessageSchema>
  | z.infer<typeof ListAvailableProvidersResponseSchema>
  | z.infer<typeof GetProvidersSnapshotResponseMessageSchema>
  | z.infer<typeof ProvidersSnapshotUpdateMessageSchema>
  | z.infer<typeof RefreshProvidersSnapshotResponseMessageSchema>
  | z.infer<typeof ProviderDiagnosticResponseMessageSchema>
  | z.infer<typeof ListCommandsResponseSchema>
  | z.infer<typeof ListTerminalsResponseSchema>
  | z.infer<typeof TerminalsChangedSchema>
  | z.infer<typeof CreateTerminalResponseSchema>
  | z.infer<typeof SubscribeTerminalResponseSchema>
  | z.infer<typeof KillTerminalResponseSchema>
  | z.infer<typeof CaptureTerminalResponseSchema>
  | z.infer<typeof TerminalStreamExitSchema>
  | z.infer<typeof ChatCreateResponseSchema>
  | z.infer<typeof ChatListResponseSchema>
  | z.infer<typeof ChatInspectResponseSchema>
  | z.infer<typeof ChatDeleteResponseSchema>
  | z.infer<typeof ChatPostResponseSchema>
  | z.infer<typeof ChatReadResponseSchema>
  | z.infer<typeof ChatWaitResponseSchema>
  | z.infer<typeof ScheduleCreateResponseSchema>
  | z.infer<typeof ScheduleListResponseSchema>
  | z.infer<typeof ScheduleInspectResponseSchema>
  | z.infer<typeof ScheduleLogsResponseSchema>
  | z.infer<typeof SchedulePauseResponseSchema>
  | z.infer<typeof ScheduleResumeResponseSchema>
  | z.infer<typeof ScheduleDeleteResponseSchema>
  | z.infer<typeof ScheduleUpdateResponseSchema>
  | z.infer<typeof ScheduleAssistResponseSchema>
  | z.infer<typeof TmuxListAgentsResponseSchema>
  | z.infer<typeof TmuxCapturePaneResponseSchema>
  | z.infer<typeof TmuxSendKeysResponseSchema>
  | z.infer<typeof TmuxNewSessionResponseSchema>
  | z.infer<typeof TmuxKillSessionResponseSchema>
  | z.infer<typeof TmuxDeleteCommandHistoryResponseSchema>
  | z.infer<typeof TmuxGetThemeResponseSchema>
  | z.infer<typeof TmuxStatusLineResponseSchema>
  | z.infer<typeof LoopRunResponseSchema>
  | z.infer<typeof LoopListResponseSchema>
  | z.infer<typeof LoopInspectResponseSchema>
  | z.infer<typeof LoopLogsResponseSchema>
  | z.infer<typeof LoopStopResponseSchema>
  | z.infer<typeof LoopUpdateResponseSchema>
  | z.infer<typeof LoopDeleteResponseSchema>
  | z.infer<typeof LoopTemplateListResponseSchema>
  | z.infer<typeof LoopTemplateGetResponseSchema>
  | z.infer<typeof LoopTemplateDeleteResponseSchema>;

export const SessionOutboundMessageSchema = z.discriminatedUnion("type", [
  ActivityLogMessageSchema,
  AssistantChunkMessageSchema,
  AudioOutputMessageSchema,
  TranscriptionResultMessageSchema,
  VoiceInputStateMessageSchema,
  DictationStreamAckMessageSchema,
  DictationStreamFinishAcceptedMessageSchema,
  DictationStreamPartialMessageSchema,
  DictationStreamFinalMessageSchema,
  DictationStreamErrorMessageSchema,
  StatusMessageSchema,
  PongMessageSchema,
  RpcErrorMessageSchema,
  ArtifactMessageSchema,
  AgentUpdateMessageSchema,
  WorkspaceUpdateMessageSchema,
  ScriptStatusUpdateMessageSchema,
  WorkspaceSetupProgressMessageSchema,
  WorkspaceSetupStatusResponseMessageSchema,
  AgentStreamMessageSchema,
  AgentStatusMessageSchema,
  FetchAgentsResponseMessageSchema,
  FetchAgentHistoryResponseMessageSchema,
  FetchWorkspacesResponseMessageSchema,
  OpenProjectResponseMessageSchema,
  StartWorkspaceScriptResponseMessageSchema,
  ListAvailableEditorsResponseMessageSchema,
  OpenInEditorResponseMessageSchema,
  ArchiveWorkspaceResponseMessageSchema,
  RemoveProjectResponseMessageSchema,
  FetchAgentResponseMessageSchema,
  FetchAgentTimelineResponseMessageSchema,
  CancelAgentResponseMessageSchema,
  ClearAgentAttentionResponseMessageSchema,
  SendAgentMessageResponseMessageSchema,
  SetVoiceModeResponseMessageSchema,
  GetDaemonConfigResponseMessageSchema,
  SetDaemonConfigResponseMessageSchema,
  ReadProjectConfigResponseMessageSchema,
  WriteProjectConfigResponseMessageSchema,
  SetAgentModeResponseMessageSchema,
  SetAgentModelResponseMessageSchema,
  SetAgentThinkingResponseMessageSchema,
  SetAgentFeatureResponseMessageSchema,
  UpdateAgentResponseMessageSchema,
  WaitForFinishResponseMessageSchema,
  AgentPermissionRequestMessageSchema,
  AgentPermissionResolvedMessageSchema,
  AgentDeletedMessageSchema,
  AgentArchivedMessageSchema,
  CloseItemsResponseSchema,
  CheckoutStatusResponseSchema,
  CheckoutStatusUpdateSchema,
  SubscribeCheckoutDiffResponseSchema,
  CheckoutDiffUpdateSchema,
  CheckoutCommitResponseSchema,
  CheckoutMergeResponseSchema,
  CheckoutMergeFromBaseResponseSchema,
  CheckoutPullResponseSchema,
  CheckoutPushResponseSchema,
  CheckoutPrCreateResponseSchema,
  CheckoutPrStatusResponseSchema,
  PullRequestTimelineResponseSchema,
  CheckoutSwitchBranchResponseSchema,
  StashSaveResponseSchema,
  StashPopResponseSchema,
  StashListResponseSchema,
  ValidateBranchResponseSchema,
  BranchSuggestionsResponseSchema,
  GitHubSearchResponseSchema,
  DirectorySuggestionsResponseSchema,
  SoloWorktreeListResponseSchema,
  SoloWorktreeArchiveResponseSchema,
  CreateSoloWorktreeResponseSchema,
  FileExplorerResponseSchema,
  ProjectIconResponseSchema,
  FileDownloadTokenResponseSchema,
  ListProviderModelsResponseMessageSchema,
  ListProviderModesResponseMessageSchema,
  ListProviderFeaturesResponseMessageSchema,
  ListAvailableProvidersResponseSchema,
  GetProvidersSnapshotResponseMessageSchema,
  ProvidersSnapshotUpdateMessageSchema,
  RefreshProvidersSnapshotResponseMessageSchema,
  ProviderDiagnosticResponseMessageSchema,
  ListCommandsResponseSchema,
  ListTerminalsResponseSchema,
  TerminalsChangedSchema,
  CreateTerminalResponseSchema,
  SubscribeTerminalResponseSchema,
  KillTerminalResponseSchema,
  CaptureTerminalResponseSchema,
  TerminalStreamExitSchema,
  ChatCreateResponseSchema,
  ChatListResponseSchema,
  ChatInspectResponseSchema,
  ChatDeleteResponseSchema,
  ChatPostResponseSchema,
  ChatReadResponseSchema,
  ChatWaitResponseSchema,
  ScheduleCreateResponseSchema,
  ScheduleListResponseSchema,
  ScheduleInspectResponseSchema,
  ScheduleLogsResponseSchema,
  SchedulePauseResponseSchema,
  ScheduleResumeResponseSchema,
  ScheduleDeleteResponseSchema,
  ScheduleUpdateResponseSchema,
  ScheduleAssistResponseSchema,
  TmuxListAgentsResponseSchema,
  TmuxCapturePaneResponseSchema,
  TmuxSendKeysResponseSchema,
  TmuxNewSessionResponseSchema,
  TmuxKillSessionResponseSchema,
  TmuxDeleteCommandHistoryResponseSchema,
  TmuxGetThemeResponseSchema,
  TmuxStatusLineResponseSchema,
  LoopRunResponseSchema,
  LoopListResponseSchema,
  LoopInspectResponseSchema,
  LoopLogsResponseSchema,
  LoopStopResponseSchema,
  LoopUpdateResponseSchema,
  LoopDeleteResponseSchema,
  LoopTemplateListResponseSchema,
  LoopTemplateGetResponseSchema,
  LoopTemplateDeleteResponseSchema,
]) as z.ZodType<SessionOutboundMessage, z.ZodTypeDef, unknown>;

// ============================================================================
// WebSocket Level Messages (wraps session messages)
// ============================================================================

// Wrapped session message
export const WSSessionInboundSchema = z.object({
  type: z.literal("session"),
  message: SessionInboundMessageSchema,
});

export type WSSessionOutboundMessage = {
  type: "session";
  message: SessionOutboundMessage;
};

export const WSSessionOutboundSchema = z.object({
  type: z.literal("session"),
  message: SessionOutboundMessageSchema,
});

// Complete WebSocket message schemas
export const WSInboundMessageSchema = z.discriminatedUnion("type", [
  WSPingMessageSchema,
  WSHelloMessageSchema,
  WSRecordingStateMessageSchema,
  WSSessionInboundSchema,
]);

export type WSOutboundMessage =
  | z.infer<typeof WSPongMessageSchema>
  | WSSessionOutboundMessage;

export const WSOutboundMessageSchema = z.discriminatedUnion("type", [
  WSPongMessageSchema,
  WSSessionOutboundSchema,
]) as z.ZodType<WSOutboundMessage, z.ZodTypeDef, unknown>;

export type WSInboundMessage = z.infer<typeof WSInboundMessageSchema>;

// ============================================================================
// Helper functions for message conversion
// ============================================================================

/**
 * Extract session message from WebSocket message
 * Returns null if message should be handled at WS level only
 */
export function extractSessionMessage(wsMsg: WSInboundMessage): SessionInboundMessage | null {
  if (wsMsg.type === "session") {
    return wsMsg.message;
  }
  // Ping and recording_state are WS-level only
  return null;
}

/**
 * Wrap session message in WebSocket envelope
 */
export function wrapSessionMessage(sessionMsg: SessionOutboundMessage): WSOutboundMessage {
  return {
    type: "session",
    message: sessionMsg,
  };
}

export function parseServerInfoStatusPayload(payload: unknown): ServerInfoStatusPayload | null {
  const parsed = ServerInfoStatusPayloadSchema.safeParse(payload);
  if (!parsed.success) {
    return null;
  }
  return parsed.data;
}

// ---------------------------------------------------------------------------
// Type re-exports for chat/schedule/tmux/loop RPC schemas
// (schemas are imported from their owning modules; types are surfaced here
//  for backward compatibility with the original messages.ts public API)
// ---------------------------------------------------------------------------

export type ChatCreateResponse = z.infer<typeof ChatCreateResponseSchema>;
export type ChatListResponse = z.infer<typeof ChatListResponseSchema>;
export type ChatInspectResponse = z.infer<typeof ChatInspectResponseSchema>;
export type ChatDeleteResponse = z.infer<typeof ChatDeleteResponseSchema>;
export type ChatPostResponse = z.infer<typeof ChatPostResponseSchema>;
export type ChatReadResponse = z.infer<typeof ChatReadResponseSchema>;
export type ChatWaitResponse = z.infer<typeof ChatWaitResponseSchema>;
export type ScheduleCreateResponse = z.infer<typeof ScheduleCreateResponseSchema>;
export type ScheduleListResponse = z.infer<typeof ScheduleListResponseSchema>;
export type ScheduleInspectResponse = z.infer<typeof ScheduleInspectResponseSchema>;
export type ScheduleLogsResponse = z.infer<typeof ScheduleLogsResponseSchema>;
export type SchedulePauseResponse = z.infer<typeof SchedulePauseResponseSchema>;
export type ScheduleResumeResponse = z.infer<typeof ScheduleResumeResponseSchema>;
export type ScheduleDeleteResponse = z.infer<typeof ScheduleDeleteResponseSchema>;
export type ScheduleUpdateResponse = z.infer<typeof ScheduleUpdateResponseSchema>;
export type ScheduleAssistResponse = z.infer<typeof ScheduleAssistResponseSchema>;
export type LoopRunResponse = z.infer<typeof LoopRunResponseSchema>;
export type LoopListResponse = z.infer<typeof LoopListResponseSchema>;
export type LoopInspectResponse = z.infer<typeof LoopInspectResponseSchema>;
export type LoopLogsResponse = z.infer<typeof LoopLogsResponseSchema>;
export type LoopStopResponse = z.infer<typeof LoopStopResponseSchema>;
export type LoopUpdateResponse = z.infer<typeof LoopUpdateResponseSchema>;
export type LoopDeleteResponse = z.infer<typeof LoopDeleteResponseSchema>;
export type LoopTemplateListRequest = z.infer<typeof LoopTemplateListRequestSchema>;
export type LoopTemplateGetRequest = z.infer<typeof LoopTemplateGetRequestSchema>;
export type LoopTemplateDeleteRequest = z.infer<typeof LoopTemplateDeleteRequestSchema>;
export type LoopTemplateListResponse = z.infer<typeof LoopTemplateListResponseSchema>;
export type LoopTemplateGetResponse = z.infer<typeof LoopTemplateGetResponseSchema>;
export type LoopTemplateDeleteResponse = z.infer<typeof LoopTemplateDeleteResponseSchema>;
export type ChatCreateRequest = z.infer<typeof ChatCreateRequestSchema>;
export type ChatListRequest = z.infer<typeof ChatListRequestSchema>;
export type ChatInspectRequest = z.infer<typeof ChatInspectRequestSchema>;
export type ChatDeleteRequest = z.infer<typeof ChatDeleteRequestSchema>;
export type ChatPostRequest = z.infer<typeof ChatPostRequestSchema>;
export type ChatReadRequest = z.infer<typeof ChatReadRequestSchema>;
export type ChatWaitRequest = z.infer<typeof ChatWaitRequestSchema>;
export type ScheduleCreateRequest = z.infer<typeof ScheduleCreateRequestSchema>;
export type ScheduleListRequest = z.infer<typeof ScheduleListRequestSchema>;
export type ScheduleInspectRequest = z.infer<typeof ScheduleInspectRequestSchema>;
export type ScheduleLogsRequest = z.infer<typeof ScheduleLogsRequestSchema>;
export type SchedulePauseRequest = z.infer<typeof SchedulePauseRequestSchema>;
export type ScheduleResumeRequest = z.infer<typeof ScheduleResumeRequestSchema>;
export type ScheduleDeleteRequest = z.infer<typeof ScheduleDeleteRequestSchema>;
export type ScheduleUpdateRequest = z.infer<typeof ScheduleUpdateRequestSchema>;
export type ScheduleAssistRequest = z.infer<typeof ScheduleAssistRequestSchema>;
export type LoopRunRequest = z.infer<typeof LoopRunRequestSchema>;
export type LoopListRequest = z.infer<typeof LoopListRequestSchema>;
export type LoopInspectRequest = z.infer<typeof LoopInspectRequestSchema>;
export type LoopLogsRequest = z.infer<typeof LoopLogsRequestSchema>;
export type LoopStopRequest = z.infer<typeof LoopStopRequestSchema>;
export type LoopUpdateRequest = z.infer<typeof LoopUpdateRequestSchema>;
export type LoopDeleteRequest = z.infer<typeof LoopDeleteRequestSchema>;
