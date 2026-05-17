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

	// Auto-subscribe to the new terminal's output
	s.subscribeTerminalOutput(proc)

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
}

func (s *Session) handleSubscribeTerminals(m *protocol.SubscribeTerminalsRequest) {
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
	s.subscribeTerminalOutput(proc)
}

func (s *Session) handleUnsubscribeTerminal(m *protocol.UnsubscribeTerminalRequest) {
	s.slotMu.Lock()
	slot, ok := s.terminalToSlot[m.TerminalID]
	if ok {
		delete(s.terminalToSlot, m.TerminalID)
		delete(s.slotToTerminal, slot)
	}
	s.slotMu.Unlock()
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
		proc.WriteInput(data)
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
	proc.WriteInput([]byte(cmd + "\n"))

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
			RequestID:  m.RequestID,
			ScriptName: m.ScriptName,
			Hostname:   hostname,
			Port:       port,
			ProxyURL:   proxyURL,
			TerminalID: proc.ID,
			Error:      nil,
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
	proc.WriteInput(payload)
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
	proc.Resize(uint16(resize.Rows), uint16(resize.Cols))
}

func (s *Session) subscribeTerminalOutput(proc *terminal.TerminalProcess) {
	s.slotMu.Lock()
	// Check if already subscribed
	if _, ok := s.terminalToSlot[proc.ID]; ok {
		s.slotMu.Unlock()
		return
	}

	slot := s.nextSlot
	s.nextSlot++
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
}

