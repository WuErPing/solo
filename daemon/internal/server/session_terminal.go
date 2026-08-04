package server

import (
	"fmt"
	"os"
	"strings"

	"github.com/WuErPing/solo/daemon/internal/terminal"
	"github.com/WuErPing/solo/daemon/internal/workspace"
	"github.com/WuErPing/solo/protocol"
)

func (s *Session) handleListTerminals(m *protocol.ListTerminalsRequest) {
	cwd := ""
	if m.Cwd != nil {
		cwd = *m.Cwd
	}
	terminals := s.terminalMgr.ListTerminals(cwd)
	if terminals == nil {
		terminals = []protocol.TerminalInfo{}
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.ListTerminalsResponse{
		Type: "list_terminals_response",
		Payload: protocol.ListTerminalsPayload{
			Cwd:       m.Cwd,
			Terminals: terminals,
			RequestID: m.RequestID,
		},
	}))
}

func (s *Session) handleCreateTerminal(m *protocol.CreateTerminalRequest) {
	cwd := m.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	name := ""
	if m.Name != nil {
		name = *m.Name
	}

	command := ""
	if m.Command != nil {
		command = *m.Command
	}

	var args []string
	if m.Args != nil {
		args = m.Args
	}

	proc, err := s.terminalMgr.CreateTerminal(cwd, name, command, args, 24, 80)
	if err != nil {
		errMsg := fmt.Sprintf("create terminal: %v", err)
		s.sendMessage(protocol.NewSessionMessage(&protocol.CreateTerminalResponse{
			Type: "create_terminal_response",
			Payload: protocol.CreateTerminalPayload{
				Terminal:  nil,
				Error:     &errMsg,
				RequestID: m.RequestID,
			},
		}))
		return
	}

	// Auto-subscribe to the new terminal's output. Failure (slots exhausted)
	// does not fail terminal creation — the terminal just streams no output.
	if _, err := s.subscribeTerminalOutput(proc); err != nil {
		s.logger.Warn("auto-subscribe terminal output failed", "terminalId", proc.ID, "error", err)
	}

	info := protocol.TerminalInfo{
		ID:    proc.ID,
		Name:  proc.Name,
		Cwd:   proc.Cwd,
		Title: proc.Title,
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.CreateTerminalResponse{
		Type: "create_terminal_response",
		Payload: protocol.CreateTerminalPayload{
			Terminal:  &info,
			Error:     nil,
			RequestID: m.RequestID,
		},
	}))
}

func (s *Session) handleKillTerminal(m *protocol.KillTerminalRequest) {
	if err := s.terminalMgr.KillTerminalAndWait(m.TerminalID); err != nil {
		s.sendRPCError(m.RequestID, m.MsgType(), fmt.Sprintf("kill terminal: %v", err), nil)
		return
	}
	// Free the output subscription slot so it can be reused.
	s.releaseTerminalSlot(m.TerminalID)
	s.sendMessage(protocol.NewSessionMessage(&protocol.KillTerminalResponse{
		Type: "kill_terminal_response",
		Payload: protocol.KillTerminalPayload{
			TerminalID: m.TerminalID,
			Success:    true,
			RequestID:  m.RequestID,
		},
	}))
}

func (s *Session) handleSubscribeTerminals(_ *protocol.SubscribeTerminalsRequest) {
	unsub := s.terminalMgr.SubscribeTerminalsChanged(func(event terminal.TerminalsChangedEvent) {
		terminals := s.terminalMgr.ListTerminals(event.Cwd)
		if terminals == nil {
			terminals = []protocol.TerminalInfo{}
		}
		s.sendMessage(protocol.NewSessionMessage(&protocol.ListTerminalsResponse{
			Type: "list_terminals_response",
			Payload: protocol.ListTerminalsPayload{
				Cwd:       &event.Cwd,
				Terminals: terminals,
			},
		}))
	})
	s.terminalSubscriptions = append(s.terminalSubscriptions, unsub)
}

func (s *Session) handleUnsubscribeTerminals(_ *protocol.UnsubscribeTerminalsRequest) {
	// Unsubscribe from all terminals_changed subscriptions
	// This is a simplified implementation - in production, track per-subscription unsub funcs
}

func (s *Session) handleSubscribeTerminal(m *protocol.SubscribeTerminalRequest) {
	proc := s.terminalMgr.GetTerminal(m.TerminalID)
	if proc == nil {
		s.sendRPCError(m.RequestID, m.MsgType(), fmt.Sprintf("terminal %s not found", m.TerminalID), nil)
		return
	}
	slot, err := s.subscribeTerminalOutput(proc)
	if err != nil {
		errMsg := err.Error()
		s.sendMessage(protocol.NewSessionMessage(&protocol.SubscribeTerminalResponse{
			Type: "subscribe_terminal_response",
			Payload: protocol.SubscribeTerminalPayload{
				TerminalID: m.TerminalID,
				Slot:       nil,
				Error:      &errMsg,
				RequestID:  m.RequestID,
			},
		}))
		return
	}
	slotInt := int(slot)
	s.sendMessage(protocol.NewSessionMessage(&protocol.SubscribeTerminalResponse{
		Type: "subscribe_terminal_response",
		Payload: protocol.SubscribeTerminalPayload{
			TerminalID: m.TerminalID,
			Slot:       &slotInt,
			Error:      nil,
			RequestID:  m.RequestID,
		},
	}))
}

func (s *Session) handleUnsubscribeTerminal(m *protocol.UnsubscribeTerminalRequest) {
	s.releaseTerminalSlot(m.TerminalID)
}

func (s *Session) handleTerminalInput(m *protocol.TerminalInputMessage) {
	s.slotMu.Lock()
	// Find the terminal by ID from the slot map (reverse lookup)
	var proc *terminal.TerminalProcess
	for id, slot := range s.terminalToSlot {
		if id == m.TerminalID {
			proc = s.slotToTerminal[slot]
			break
		}
	}
	s.slotMu.Unlock()

	if proc == nil {
		// Try direct lookup from terminal manager
		proc = s.terminalMgr.GetTerminal(m.TerminalID)
	}
	if proc == nil {
		return
	}

	data := m.Message
	if len(data) > 0 {
		if err := proc.WriteInput(data); err != nil {
			s.logger.Debug("terminal input write failed", "terminalId", m.TerminalID, "error", err)
		}
	}
}

func (s *Session) handleCaptureTerminal(m *protocol.CaptureTerminalRequest) {
	s.sendRPCError(m.RequestID, m.MsgType(), "capture terminal not supported", nil)
}

func (s *Session) handleStartWorkspaceScript(m *protocol.StartWorkspaceScriptRequest) {
	// Find the workspace
	s.workspacesMu.RLock()
	wsDesc, ok := s.workspaces[m.WorkspaceID]
	s.workspacesMu.RUnlock()
	if !ok {
		s.sendRPCError(m.RequestID, m.MsgType(), "workspace not found", nil)
		return
	}

	// Read project config
	repoRoot := wsDesc.ProjectRootPath
	cfg, err := workspace.ReadProjectConfig(repoRoot)
	if err != nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "read project config: "+err.Error(), nil)
		return
	}
	if cfg == nil || cfg.Scripts == nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "no scripts defined in project config", nil)
		return
	}

	scriptCfg, ok := cfg.Scripts[m.ScriptName]
	if !ok {
		s.sendRPCError(m.RequestID, m.MsgType(), "script not found: "+m.ScriptName, nil)
		return
	}
	if scriptCfg.Command == "" {
		s.sendRPCError(m.RequestID, m.MsgType(), "script has no command: "+m.ScriptName, nil)
		return
	}

	cwd := wsDesc.WorkspaceDirectory
	if cwd == "" {
		cwd = wsDesc.ProjectRootPath
	}

	// Determine port and hostname
	var port int
	var hostname string
	isService := scriptCfg.Type == "service"

	if isService {
		if scriptCfg.Port != nil {
			port = *scriptCfg.Port
		} else {
			port, err = workspace.AllocatePort()
			if err != nil {
				s.sendRPCError(m.RequestID, m.MsgType(), "allocate port: "+err.Error(), nil)
				return
			}
		}

		projectSlug := "project"
		if wsDesc.GitRuntime != nil && wsDesc.GitRuntime.CurrentBranch != nil {
			projectSlug = strings.ToLower(strings.ReplaceAll(wsDesc.ProjectDisplayName, " ", "-"))
		}
		branchName := "main"
		if wsDesc.GitRuntime != nil && wsDesc.GitRuntime.CurrentBranch != nil {
			branchName = *wsDesc.GitRuntime.CurrentBranch
		}
		hostname = workspace.BuildHostname(projectSlug, branchName, m.ScriptName)
	}

	// Create terminal to run the script
	termName := "script:" + m.ScriptName
	proc, err := s.terminalMgr.CreateTerminal(cwd, termName, "", nil, 24, 80)
	if err != nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "create terminal: "+err.Error(), nil)
		return
	}

	// Build the command with environment
	cmd := scriptCfg.Command
	if isService {
		// Prepend PORT env var assignment
		cmd = fmt.Sprintf("PORT=%d %s", port, cmd)
	}

	// Send the command to the terminal
	if err := proc.WriteInput([]byte(cmd + "\n")); err != nil {
		s.logger.Debug("script terminal input write failed", "terminalId", proc.ID, "error", err)
	}

	// Register proxy route if service
	if isService {
		s.scriptProxy.RegisterRoute(hostname, port)
		s.scriptMgr.Register(&workspace.ScriptRuntime{
			WorkspaceID: m.WorkspaceID,
			ScriptName:  m.ScriptName,
			Hostname:    hostname,
			Port:        port,
			TerminalID:  proc.ID,
			Status:      workspace.ScriptStatusRunning,
			ProxyURL:    "http://" + hostname,
		})
	}

	// Send response
	proxyURL := ""
	if isService {
		proxyURL = "http://" + hostname
	}
	s.sendMessage(protocol.NewSessionMessage(&protocol.StartWorkspaceScriptResponse{
		Type: "start_workspace_script_response",
		Payload: protocol.StartWorkspaceScriptResponsePayload{
			RequestID:   m.RequestID,
			WorkspaceID: m.WorkspaceID,
			ScriptName:  m.ScriptName,
			Hostname:    hostname,
			Port:        port,
			ProxyURL:    proxyURL,
			TerminalID:  proc.ID,
			Error:       nil,
		},
	}))
}

func (s *Session) handleTerminalInputBinary(slot byte, payload []byte) {
	s.slotMu.Lock()
	proc := s.slotToTerminal[slot]
	s.slotMu.Unlock()

	if proc == nil {
		return
	}
	if err := proc.WriteInput(payload); err != nil {
		s.logger.Debug("binary terminal input write failed", "error", err)
	}
}

func (s *Session) handleTerminalResizeBinary(slot byte, payload []byte) {
	s.slotMu.Lock()
	proc := s.slotToTerminal[slot]
	s.slotMu.Unlock()

	if proc == nil {
		return
	}

	resize, err := protocol.DecodeTerminalResize(payload)
	if err != nil {
		s.logger.Warn("invalid terminal resize payload", "error", err)
		return
	}
	if err := proc.Resize(uint16(resize.Rows), uint16(resize.Cols)); err != nil {
		s.logger.Debug("terminal resize failed", "terminalId", proc.ID, "error", err)
	}
}

// subscribeTerminalOutput subscribes to the terminal's output stream and
// returns the slot byte used in binary frames. Subscribing an already
// subscribed terminal is idempotent and returns its existing slot. Slots are
// allocated as the lowest free value in [0,255]; when all 256 slots are taken
// an error is returned instead of wrapping around and overwriting an active
// subscriber's slot.
func (s *Session) subscribeTerminalOutput(proc *terminal.TerminalProcess) (byte, error) {
	s.slotMu.Lock()
	// Check if already subscribed
	if slot, ok := s.terminalToSlot[proc.ID]; ok {
		s.slotMu.Unlock()
		return slot, nil
	}

	slot, ok := allocateFreeSlotLocked(s.slotToTerminal)
	if !ok {
		s.slotMu.Unlock()
		return 0, fmt.Errorf("terminal output slots exhausted (max 256 subscriptions)")
	}
	s.slotToTerminal[slot] = proc
	s.terminalToSlot[proc.ID] = slot
	s.slotMu.Unlock()

	coalescer := terminal.NewOutputCoalescer(func(data []byte) {
		s.SendBinaryFrame(protocol.TerminalStreamFrame{
			Opcode:  protocol.TerminalOutput,
			Slot:    slot,
			Payload: data,
		})
	})

	unsub := proc.Subscribe(func(data []byte) {
		coalescer.Add(data)
	})

	s.terminalSubscriptions = append(s.terminalSubscriptions, func() {
		unsub()
		coalescer.Stop()
	})
	return slot, nil
}

// allocateFreeSlotLocked returns the lowest slot in [0,255] not present in
// slotToTerminal. ok is false when all slots are taken.
func allocateFreeSlotLocked(slotToTerminal map[byte]*terminal.TerminalProcess) (slot byte, ok bool) {
	for i := 0; i < 256; i++ {
		candidate := byte(i)
		if _, taken := slotToTerminal[candidate]; !taken {
			return candidate, true
		}
	}
	return 0, false
}

// releaseTerminalSlot frees the output subscription slot held by terminalID,
// making it available for reuse. Safe to call for terminals without a slot.
func (s *Session) releaseTerminalSlot(terminalID string) {
	s.slotMu.Lock()
	defer s.slotMu.Unlock()
	if slot, ok := s.terminalToSlot[terminalID]; ok {
		delete(s.terminalToSlot, terminalID)
		delete(s.slotToTerminal, slot)
	}
}
