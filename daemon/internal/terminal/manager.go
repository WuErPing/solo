package terminal

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/WuErPing/solo/protocol"
)

type TerminalsChangedEvent struct {
	Cwd       string
	Terminals []*TerminalProcess
	Kind      string // "added", "removed", "title_change"
}

type TerminalsChangedFunc func(event TerminalsChangedEvent)

type TerminalManager struct {
	mu        sync.RWMutex
	terminals map[string]*TerminalProcess   // by ID
	byCwd     map[string][]*TerminalProcess // by CWD

	subscribers map[uint64]TerminalsChangedFunc
	nextSubID   uint64
	logger      *slog.Logger
}

func NewTerminalManager(logger *slog.Logger) *TerminalManager {
	return &TerminalManager{
		terminals:   make(map[string]*TerminalProcess),
		byCwd:       make(map[string][]*TerminalProcess),
		subscribers: make(map[uint64]TerminalsChangedFunc),
		logger:      logger,
	}
}

func (m *TerminalManager) CreateTerminal(cwd, name, command string, args []string, rows, cols uint16) (*TerminalProcess, error) {
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}

	id := uuid.New().String()[:8]
	if name == "" {
		name = "Terminal"
	}

	proc := NewTerminalProcess(id, name, cwd, command, args, rows, cols, m.logger)

	if err := proc.Start(); err != nil {
		return nil, fmt.Errorf("start terminal: %w", err)
	}

	m.mu.Lock()
	m.terminals[id] = proc
	m.byCwd[cwd] = append(m.byCwd[cwd], proc)
	m.mu.Unlock()

	proc.OnExit(func(ExitInfo) {
		m.mu.Lock()
		delete(m.terminals, id)
		list := m.byCwd[cwd]
		for i, t := range list {
			if t.ID == id {
				m.byCwd[cwd] = append(list[:i], list[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
		m.emitChanged(TerminalsChangedEvent{Cwd: cwd, Kind: "removed"})
	})

	m.emitChanged(TerminalsChangedEvent{Cwd: cwd, Kind: "added"})

	return proc, nil
}

func (m *TerminalManager) KillTerminal(id string) error {
	m.mu.Lock()
	proc, ok := m.terminals[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("terminal %s not found", id)
	}
	proc.Kill()
	return nil
}

func (m *TerminalManager) KillTerminalAndWait(id string) error {
	m.mu.Lock()
	proc, ok := m.terminals[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("terminal %s not found", id)
	}
	proc.Close()
	return nil
}

func (m *TerminalManager) ListTerminals(cwd string) []protocol.TerminalInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cwd == "" {
		var result []protocol.TerminalInfo
		for _, proc := range m.terminals {
			info := procToInfo(proc)
			result = append(result, info)
		}
		return result
	}

	list := m.byCwd[cwd]
	result := make([]protocol.TerminalInfo, 0, len(list))
	for _, proc := range list {
		result = append(result, procToInfo(proc))
	}
	return result
}

func (m *TerminalManager) GetTerminal(id string) *TerminalProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.terminals[id]
}

func (m *TerminalManager) SubscribeTerminalsChanged(fn TerminalsChangedFunc) func() {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextSubID
	m.nextSubID++
	m.subscribers[id] = fn
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.subscribers, id)
	}
}

func (m *TerminalManager) KillAll() {
	m.mu.Lock()
	procs := make([]*TerminalProcess, 0, len(m.terminals))
	for _, proc := range m.terminals {
		procs = append(procs, proc)
	}
	m.mu.Unlock()

	for _, proc := range procs {
		proc.Close()
	}
}

func (m *TerminalManager) emitChanged(event TerminalsChangedEvent) {
	m.mu.RLock()
	subs := make(map[uint64]TerminalsChangedFunc, len(m.subscribers))
	for k, v := range m.subscribers {
		subs[k] = v
	}
	m.mu.RUnlock()

	for _, fn := range subs {
		fn(event)
	}
}

func procToInfo(proc *TerminalProcess) protocol.TerminalInfo {
	return protocol.TerminalInfo{
		ID:    proc.ID,
		Name:  proc.Name,
		Cwd:   proc.Cwd,
		Title: proc.Title,
	}
}
