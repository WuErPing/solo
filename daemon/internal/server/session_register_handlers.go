package server

import (
	"github.com/WuErPing/solo/protocol"
)

// registerHandlers populates the handler registry with all known message types.
// Adding a new message type only requires appending a Register call here —
// the dispatch logic in handleSessionMessage never changes.
func (s *Session) registerHandlers() {
	r := s.handlerRegistry

	// --- Session-level ---
	r.Register("ping", func(s *Session, msg protocol.SessionInboundMessage) {
		s.handlePing(msg.(*protocol.PingMessage))
	})
	r.Register("client_heartbeat", func(s *Session, msg protocol.SessionInboundMessage) {
		m := msg.(*protocol.ClientHeartbeatMessage)
		s.logger.Debug("heartbeat", "deviceType", m.DeviceType, "visible", m.AppVisible)
		if s.activityTracker != nil {
			focusedAgentID := stringPtrValue(m.FocusedAgentID)
			s.activityTracker.UpdateActivity(s.clientID, m.AppVisible, focusedAgentID)
		}
	})

	// --- Agent handlers (session_agent.go) ---
	r.Register("create_agent_request", typeHandler(s.handleCreateAgent))
	r.Register("fetch_agents_request", typeHandler(s.handleFetchAgents))
	r.Register("fetch_agent_request", typeHandler(s.handleFetchAgent))
	r.Register("fetch_agent_timeline_request", typeHandler(s.handleFetchAgentTimeline))
	r.Register("send_agent_message_request", typeHandler(s.handleSendAgentMessage))
	r.Register("cancel_agent_request", typeHandler(s.handleCancelAgent))
	r.Register("delete_agent_request", typeHandler(s.handleDeleteAgent))
	r.Register("archive_agent_request", typeHandler(s.handleArchiveAgent))
	r.Register("resume_agent_request", typeHandler(s.handleResumeAgent))
	r.Register("wait_for_finish_request", typeHandler(s.handleWaitForFinish))
	r.Register("agent_permission_response", typeHandler(s.handleAgentPermissionResponse))
	r.Register("set_agent_mode_request", typeHandler(s.handleSetAgentMode))
	r.Register("set_agent_model_request", typeHandler(s.handleSetAgentModel))
	r.Register("set_agent_thinking_request", typeHandler(s.handleSetAgentThinking))
	r.Register("set_agent_feature_request", typeHandler(s.handleSetAgentFeature))
	r.Register("update_agent_request", typeHandler(s.handleUpdateAgent))
	r.Register("clear_agent_attention", typeHandler(s.handleClearAgentAttention))
	r.Register("refresh_agent_request", typeHandler(s.handleRefreshAgent))
	r.Register("close_items_request", typeHandler(s.handleCloseItems))
	r.Register("list_commands_request", typeHandler(s.handleListCommands))
	r.Register("list_provider_features_request", typeHandler(s.handleListProviderFeatures))
	r.Register("list_available_editors_request", typeHandler(s.handleListAvailableEditors))
	r.Register("fetch_agent_history_request", typeHandler(s.handleFetchAgentHistory))
	r.Register("get_daemon_config_request", typeHandler(s.handleGetDaemonConfig))
	r.Register("set_daemon_config_request", typeHandler(s.handleSetDaemonConfig))
	r.Register("get_providers_snapshot_request", typeHandler(s.handleGetProvidersSnapshot))

	// --- Server control ---
	r.Register("restart_server_request", func(s *Session, msg protocol.SessionInboundMessage) {
		m := msg.(*protocol.RestartServerRequest)
		s.logger.Info("restart requested", "requestId", m.RequestID)
	})
	r.Register("shutdown_server_request", func(s *Session, msg protocol.SessionInboundMessage) {
		m := msg.(*protocol.ShutdownServerRequest)
		s.logger.Info("shutdown requested", "requestId", m.RequestID)
	})

	// --- Push (session.go) ---
	r.Register("register_push_token", typeHandler(s.handleRegisterPushToken))

	// --- Terminal handlers (session_terminal.go) ---
	r.Register("list_terminals_request", typeHandler(s.handleListTerminals))
	r.Register("create_terminal_request", typeHandler(s.handleCreateTerminal))
	r.Register("kill_terminal_request", typeHandler(s.handleKillTerminal))
	r.Register("subscribe_terminals_request", typeHandler(s.handleSubscribeTerminals))
	r.Register("unsubscribe_terminals_request", typeHandler(s.handleUnsubscribeTerminals))
	r.Register("subscribe_terminal_request", typeHandler(s.handleSubscribeTerminal))
	r.Register("unsubscribe_terminal_request", typeHandler(s.handleUnsubscribeTerminal))
	r.Register("terminal_input", typeHandler(s.handleTerminalInput))
	r.Register("capture_terminal_request", typeHandler(s.handleCaptureTerminal))
	r.Register("start_workspace_script_request", typeHandler(s.handleStartWorkspaceScript))

	// --- Workspace/File handlers (session_workspace.go, session_fileexplorer.go) ---
	r.Register("open_project_request", typeHandler(s.handleOpenProject))
	r.Register("fetch_workspaces_request", typeHandler(s.handleFetchWorkspaces))
	r.Register("file_explorer_request", typeHandler(s.handleFileExplorer))
	r.Register("project_icon_request", typeHandler(s.handleProjectIcon))
	r.Register("directory_suggestions_request", typeHandler(s.handleDirectorySuggestions))
	r.Register("create_solo_worktree_request", typeHandler(s.handleCreateSoloWorktree))
	r.Register("workspace_setup_status_request", typeHandler(s.handleWorkspaceSetupStatus))
	r.Register("archive_workspace_request", typeHandler(s.handleArchiveWorkspace))
	r.Register("checkout_pr_status_request", typeHandler(s.handleCheckoutPrStatus))
	r.Register("read_project_config_request", typeHandler(s.handleReadProjectConfig))
	r.Register("write_project_config_request", typeHandler(s.handleWriteProjectConfig))
	r.Register("checkout_status_request", func(s *Session, msg protocol.SessionInboundMessage) {
		m := msg.(*protocol.CheckoutStatusRequest)
		s.sendRPCError(m.RequestID, m.MsgType(), "not implemented", nil)
	})

	// --- Schedule handlers (session_schedule.go) ---
	r.Register("schedule/create", typeHandler(s.handleScheduleCreate))
	r.Register("schedule/list", typeHandler(s.handleScheduleList))
	r.Register("schedule/inspect", typeHandler(s.handleScheduleInspect))
	r.Register("schedule/logs", typeHandler(s.handleScheduleLogs))
	r.Register("schedule/pause", typeHandler(s.handleSchedulePause))
	r.Register("schedule/resume", typeHandler(s.handleScheduleResume))
	r.Register("schedule/delete", typeHandler(s.handleScheduleDelete))
	r.Register("schedule/update", typeHandler(s.handleScheduleUpdate))

	// --- Loop handlers (session_loop.go) ---
	r.Register("loop/run", typeHandler(s.handleLoopRun))
	r.Register("loop/list", typeHandler(s.handleLoopList))
	r.Register("loop/inspect", typeHandler(s.handleLoopInspect))
	r.Register("loop/logs", typeHandler(s.handleLoopLogs))
	r.Register("loop/stop", typeHandler(s.handleLoopStop))
	r.Register("loop/update", typeHandler(s.handleLoopUpdate))
	r.Register("loop/delete", typeHandler(s.handleLoopDelete))

	// --- Tmux handlers (session_tmux.go) ---
	r.Register("tmux/list_agents", typeHandler(s.handleTmuxListAgents))
	r.Register("tmux/capture_pane", typeHandler(s.handleTmuxCapturePane))
	r.Register("tmux/send_keys", typeHandler(s.handleTmuxSendKeys))
	r.Register("tmux/new_session", typeHandler(s.handleTmuxNewSession))
	r.Register("tmux/kill_session", typeHandler(s.handleTmuxKillSession))
	r.Register("tmux/delete_command_history", typeHandler(s.handleTmuxDeleteCommandHistory))
	r.Register("tmux/status_line", typeHandler(s.handleTmuxStatusLine))
}

// typeHandler is a helper that converts a typed handler func into a messageHandler.
// This avoids repeating the type assertion boilerplate for every handler.
func typeHandler[T protocol.SessionInboundMessage](fn func(T)) messageHandler {
	return func(_ *Session, msg protocol.SessionInboundMessage) {
		fn(msg.(T))
	}
}
