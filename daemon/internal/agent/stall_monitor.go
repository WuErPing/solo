package agent

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// defaultStallCheckInterval is how often the monitor scans running agents.
const defaultStallCheckInterval = 30 * time.Second

// defaultInactivityThreshold is the max time allowed between any stream events
// while an agent is LifecycleRunning. If no events arrive for this long the
// turn is considered stalled.
const defaultInactivityThreshold = 2 * time.Minute

// defaultRepetitionWindow is the number of recent assistant messages to examine.
const defaultRepetitionWindow = 10

// defaultRepetitionThreshold is how many identical messages within the window
// trigger a repetition stall.
const defaultRepetitionThreshold = 6

// StallReason explains why a turn was interrupted.
type StallReason string

const (
	StallReasonInactivity StallReason = "inactivity"
	StallReasonRepetition StallReason = "repetition"
)

// StallMonitor watches running agents and interrupts turns that show no
// meaningful progress (inactivity or repetitive output).
//
// It is safe for concurrent use.
type StallMonitor struct {
	mu       sync.Mutex
	agents   map[string]*stallState
	ticker   *time.Ticker
	stopCh   chan struct{}
	stopOnce sync.Once
	logger   *slog.Logger

	interruptFn func(agentID string) error

	checkInterval       time.Duration
	inactivityThreshold time.Duration
	repetitionWindow    int
	repetitionThreshold int
}

type stallState struct {
	lastEventTime time.Time
	recentTexts   []string // rolling window of normalized assistant texts
}

// NewStallMonitor creates a monitor. All thresholds use package defaults unless
// overridden by the functional options.
func NewStallMonitor(logger *slog.Logger, interruptFn func(agentID string) error, opts ...StallMonitorOption) *StallMonitor {
	m := &StallMonitor{
		agents:              make(map[string]*stallState),
		stopCh:              make(chan struct{}),
		logger:              logger.With("component", "stall-monitor"),
		interruptFn:         interruptFn,
		checkInterval:       defaultStallCheckInterval,
		inactivityThreshold: defaultInactivityThreshold,
		repetitionWindow:    defaultRepetitionWindow,
		repetitionThreshold: defaultRepetitionThreshold,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// StallMonitorOption overrides default thresholds.
type StallMonitorOption func(*StallMonitor)

// WithCheckInterval sets the scan frequency.
func WithCheckInterval(d time.Duration) StallMonitorOption {
	return func(m *StallMonitor) { m.checkInterval = d }
}

// WithInactivityThreshold sets the no-event timeout.
func WithInactivityThreshold(d time.Duration) StallMonitorOption {
	return func(m *StallMonitor) { m.inactivityThreshold = d }
}

// WithRepetitionThreshold sets how many identical messages within the window
// constitute a repetition loop.
func WithRepetitionThreshold(window, threshold int) StallMonitorOption {
	return func(m *StallMonitor) {
		m.repetitionWindow = window
		m.repetitionThreshold = threshold
	}
}

// Start begins periodic scanning. Safe to call multiple times (no-op after first).
func (m *StallMonitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ticker != nil {
		return
	}
	m.ticker = time.NewTicker(m.checkInterval)
	go m.loop()
}

// Stop halts the monitor.
func (m *StallMonitor) Stop() {
	m.mu.Lock()
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}
	m.mu.Unlock()

	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

// RecordEvent should be called for every stream event while the agent is running.
func (m *StallMonitor) RecordEvent(agentID string, event AgentStreamEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.agents[agentID]
	if !ok {
		st = &stallState{}
		m.agents[agentID] = st
	}
	st.lastEventTime = time.Now()

	// Track assistant message text for repetition detection.
	if text, ok := extractAssistantTextFromStreamEvent(event); ok && text != "" {
		norm := normalizeText(text)
		if len(st.recentTexts) >= m.repetitionWindow {
			st.recentTexts = st.recentTexts[1:]
		}
		st.recentTexts = append(st.recentTexts, norm)
	}
}

// RegisterAgent tells the monitor to start tracking an agent.
func (m *StallMonitor) RegisterAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[agentID] = &stallState{lastEventTime: time.Now()}
}

// UnregisterAgent removes tracking state.
func (m *StallMonitor) UnregisterAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agents, agentID)
}

// HasRecentProgress reports whether the agent produced a stream event within
// the inactivity threshold. Callers use this to decide whether a running agent
// deserves a grace-period extension.
func (m *StallMonitor) HasRecentProgress(agentID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.agents[agentID]
	if !ok {
		return false
	}
	return time.Since(st.lastEventTime) <= m.inactivityThreshold
}

func (m *StallMonitor) loop() {
	for {
		m.mu.Lock()
		ticker := m.ticker
		m.mu.Unlock()

		if ticker == nil {
			return
		}

		select {
		case <-ticker.C:
			m.checkAll()
		case <-m.stopCh:
			return
		}
	}
}

func (m *StallMonitor) checkAll() {
	m.mu.Lock()
	// Copy IDs so we can release the lock before calling interrupt.
	ids := make([]string, 0, len(m.agents))
	for id := range m.agents {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		if reason, msg := m.checkAgent(id); reason != "" {
			m.logger.Warn("stall detected, interrupting agent",
				"agentId", id,
				"reason", reason,
				"detail", msg)
			if err := m.interruptFn(id); err != nil {
				m.logger.Warn("stall interrupt failed", "agentId", id, "error", err)
			}
		}
	}
}

// checkAgent returns (StallReason, detail) if the agent is stalled, or ("", "") if healthy.
func (m *StallMonitor) checkAgent(agentID string) (StallReason, string) {
	m.mu.Lock()
	st, ok := m.agents[agentID]
	if !ok {
		m.mu.Unlock()
		return "", ""
	}
	lastEvent := st.lastEventTime
	recent := make([]string, len(st.recentTexts))
	copy(recent, st.recentTexts)
	m.mu.Unlock()

	// 1. Inactivity check
	if time.Since(lastEvent) > m.inactivityThreshold {
		return StallReasonInactivity, fmt.Sprintf("no events for %v", time.Since(lastEvent).Round(time.Second))
	}

	// 2. Repetition check
	if len(recent) >= m.repetitionThreshold {
		freq := mostFrequent(recent)
		if freq >= m.repetitionThreshold {
			return StallReasonRepetition, fmt.Sprintf("%d/%d identical messages", freq, len(recent))
		}
	}

	return "", ""
}

func extractAssistantTextFromStreamEvent(event AgentStreamEvent) (string, bool) {
	switch e := event.Event.(type) {
	case protocol.TimelineStreamEvent:
		if e.Item.Type == "assistant_message" {
			return e.Item.Text, true
		}
	}
	return "", false
}

func normalizeText(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func mostFrequent(items []string) int {
	counts := make(map[string]int, len(items))
	maxCount := 0
	for _, it := range items {
		counts[it]++
		if counts[it] > maxCount {
			maxCount = counts[it]
		}
	}
	return maxCount
}
