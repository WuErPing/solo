package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/WuErPing/solo/protocol"
)

const DefaultTimelineFetchLimit = 200

// TimelineItem is an alias to the protocol-level definition so the event
// pipeline can reference it without duplicating the type.
type TimelineItem = protocol.TimelineItem

// TodoItem is an alias to the protocol-level definition.
type TodoItem = protocol.TodoItem

// TimelineRow is a timeline item with sequence number and timestamp.
type TimelineRow struct {
	Seq       int          `json:"seq"`
	Timestamp string       `json:"timestamp"`
	Item      TimelineItem `json:"item"`
}

// TimelineState holds the in-memory state for a single agent's timeline.
type TimelineState struct {
	Epoch   string        `json:"epoch"`
	Rows    []TimelineRow `json:"rows"`
	NextSeq int           `json:"nextSeq"`
}

// InMemoryTimelineStore manages timelines for all agents in memory.
type InMemoryTimelineStore struct {
	mu      sync.RWMutex
	states  map[string]*TimelineState
	waiters map[string][]chan struct{}
}

// NewInMemoryTimelineStore creates a new timeline store.
func NewInMemoryTimelineStore() *InMemoryTimelineStore {
	return &InMemoryTimelineStore{
		states:  make(map[string]*TimelineState),
		waiters: make(map[string][]chan struct{}),
	}
}

// Initialize creates a timeline state for an agent.
func (s *InMemoryTimelineStore) Initialize(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.states[agentID]; !ok {
		s.states[agentID] = &TimelineState{
			Epoch:   uuid.New().String(),
			Rows:    []TimelineRow{},
			NextSeq: 0,
		}
	}
}

// Has returns whether a timeline exists for the given agent.
func (s *InMemoryTimelineStore) Has(agentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.states[agentID]
	return ok
}

// Delete removes a timeline for an agent.
func (s *InMemoryTimelineStore) Delete(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, agentID)
}

// Append adds a timeline item and returns the new row.
// It is idempotent: when multiple sessions process the same stream event,
// only the first call creates a row; subsequent calls with an identical
// item return the existing row.
func (s *InMemoryTimelineStore) Append(agentID string, item TimelineItem) TimelineRow {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.getOrCreateStateLocked(agentID)

	// Deduplicate against the most recent row.
	// Multiple sessions may process the same stream event and call Append
	// in quick succession. The second (and subsequent) calls find the row
	// that the first session just appended and return it instead of creating
	// a duplicate.
	if len(state.Rows) > 0 {
		last := state.Rows[len(state.Rows)-1]
		if timelineItemsEqual(last.Item, item) {
			return last
		}
	}

	return s.appendLocked(state, agentID, item)
}

// getOrCreateStateLocked returns the timeline state for agentID, creating one
// if necessary. Caller must hold s.mu.
func (s *InMemoryTimelineStore) getOrCreateStateLocked(agentID string) *TimelineState {
	state, ok := s.states[agentID]
	if !ok {
		state = &TimelineState{
			Epoch:   uuid.New().String(),
			Rows:    []TimelineRow{},
			NextSeq: 0,
		}
		s.states[agentID] = state
	}
	return state
}

// appendLocked appends item to state and notifies waiters. Caller must hold s.mu.
func (s *InMemoryTimelineStore) appendLocked(state *TimelineState, agentID string, item TimelineItem) TimelineRow {
	row := TimelineRow{
		Seq:       state.NextSeq,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Item:      item,
	}
	state.Rows = append(state.Rows, row)
	state.NextSeq++

	// Notify waiters for assistant_message
	if item.Type == "assistant_message" {
		if chs, ok := s.waiters[agentID]; ok {
			for _, ch := range chs {
				close(ch)
			}
			delete(s.waiters, agentID)
		}
	}

	return row
}

// Fetch retrieves timeline rows with cursor-based pagination.
func (s *InMemoryTimelineStore) Fetch(agentID string, direction string, cursor *protocol.AgentTimelineCursor, limit int) *TimelineFetchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[agentID]
	if !ok {
		return &TimelineFetchResult{
			Epoch:     "",
			Direction: direction,
			Reset:     true,
			Rows:      []TimelineRow{},
		}
	}

	// Default limit
	if limit <= 0 {
		limit = DefaultTimelineFetchLimit
	}

	// Handle epoch mismatch
	if cursor != nil && cursor.Epoch != state.Epoch {
		return s.fetchAll(state, true, true, direction)
	}

	// Handle gap detection for "after" direction
	if direction == "after" && cursor != nil && len(state.Rows) > 0 {
		minSeq := state.Rows[0].Seq
		if cursor.Seq < minSeq-1 {
			return s.fetchAll(state, true, false, direction)
		}
	}

	switch direction {
	case "tail":
		return s.fetchTail(state, limit)
	case "after":
		return s.fetchAfter(state, cursor, limit)
	case "before":
		return s.fetchBefore(state, cursor, limit)
	default:
		return s.fetchTail(state, limit)
	}
}

func (s *InMemoryTimelineStore) fetchAll(state *TimelineState, reset, staleCursor bool, direction string) *TimelineFetchResult {
	rows := state.Rows
	window := s.getWindow(state)
	return &TimelineFetchResult{
		Epoch:       state.Epoch,
		Direction:   direction,
		Reset:       reset,
		StaleCursor: staleCursor,
		Window:      window,
		HasOlder:    false,
		HasNewer:    false,
		Rows:        rows,
	}
}

func (s *InMemoryTimelineStore) fetchTail(state *TimelineState, limit int) *TimelineFetchResult {
	rows := state.Rows
	window := s.getWindow(state)
	hasOlder := len(rows) > limit
	if hasOlder {
		rows = rows[len(rows)-limit:]
	}
	return &TimelineFetchResult{
		Epoch:     state.Epoch,
		Direction: "tail",
		Window:    window,
		HasOlder:  hasOlder,
		HasNewer:  false,
		Rows:      rows,
	}
}

func (s *InMemoryTimelineStore) fetchAfter(state *TimelineState, cursor *protocol.AgentTimelineCursor, limit int) *TimelineFetchResult {
	var filtered []TimelineRow
	for _, r := range state.Rows {
		if r.Seq > cursor.Seq {
			filtered = append(filtered, r)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	window := s.getWindow(state)
	hasNewer := len(filtered) > 0 && len(state.Rows) > 0 && filtered[len(filtered)-1].Seq < state.Rows[len(state.Rows)-1].Seq
	return &TimelineFetchResult{
		Epoch:     state.Epoch,
		Direction: "after",
		Window:    window,
		HasOlder:  false,
		HasNewer:  hasNewer,
		Rows:      filtered,
	}
}

func (s *InMemoryTimelineStore) fetchBefore(state *TimelineState, cursor *protocol.AgentTimelineCursor, limit int) *TimelineFetchResult {
	var filtered []TimelineRow
	for _, r := range state.Rows {
		if cursor == nil || r.Seq < cursor.Seq {
			filtered = append(filtered, r)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	window := s.getWindow(state)
	hasOlder := len(filtered) > 0 && len(state.Rows) > 0 && filtered[0].Seq > state.Rows[0].Seq
	return &TimelineFetchResult{
		Epoch:     state.Epoch,
		Direction: "before",
		Window:    window,
		HasOlder:  hasOlder,
		HasNewer:  false,
		Rows:      filtered,
	}
}

func (s *InMemoryTimelineStore) getWindow(state *TimelineState) TimelineWindow {
	if len(state.Rows) == 0 {
		return TimelineWindow{MinSeq: 0, MaxSeq: 0, NextSeq: state.NextSeq}
	}
	return TimelineWindow{
		MinSeq:  state.Rows[0].Seq,
		MaxSeq:  state.Rows[len(state.Rows)-1].Seq,
		NextSeq: state.NextSeq,
	}
}

// getLastAssistantMessageLocked returns the last assistant message assuming the lock is held.
func (s *InMemoryTimelineStore) getLastAssistantMessageLocked(agentID string) *string {
	state, ok := s.states[agentID]
	if !ok || len(state.Rows) == 0 {
		return nil
	}

	var parts []string
	for i := len(state.Rows) - 1; i >= 0; i-- {
		if state.Rows[i].Item.Type == "assistant_message" {
			parts = append(parts, state.Rows[i].Item.Text)
		} else {
			break
		}
	}
	if len(parts) == 0 {
		return nil
	}
	// Reverse and join
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	result := strings.Join(parts, "")
	return &result
}

// GetLastAssistantMessage returns the text of the most recent assistant message.
func (s *InMemoryTimelineStore) GetLastAssistantMessage(agentID string) *string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getLastAssistantMessageLocked(agentID)
}

// WaitForAssistantMessage waits up to timeout for an assistant message to be appended.
// If one already exists, it returns immediately.
func (s *InMemoryTimelineStore) WaitForAssistantMessage(agentID string, timeout time.Duration) *string {
	// Fast path: already available
	s.mu.RLock()
	msg := s.getLastAssistantMessageLocked(agentID)
	s.mu.RUnlock()
	if msg != nil {
		return msg
	}

	// Slow path: register waiter
	ch := make(chan struct{})
	s.mu.Lock()
	s.waiters[agentID] = append(s.waiters[agentID], ch)
	s.mu.Unlock()

	select {
	case <-ch:
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.getLastAssistantMessageLocked(agentID)
	case <-time.After(timeout):
		// Remove ourselves from waiters
		s.mu.Lock()
		if chs, ok := s.waiters[agentID]; ok {
			for i, c := range chs {
				if c == ch {
					s.waiters[agentID] = append(chs[:i], chs[i+1:]...)
					break
				}
			}
			if len(s.waiters[agentID]) == 0 {
				delete(s.waiters, agentID)
			}
		}
		s.mu.Unlock()
		return nil
	}
}

// AppendFromHistory appends a history item to the timeline without notifying waiters.
// item may be a TimelineItem or a map[string]interface{} from StreamHistory.
// It scans all existing rows to avoid inserting duplicates of items already
// added by live events (which can be interleaved with history entries).
func (s *InMemoryTimelineStore) AppendFromHistory(agentID string, item interface{}) {
	var ti TimelineItem
	switch v := item.(type) {
	case TimelineItem:
		ti = v
	case map[string]interface{}:
		ti = timelineItemFromMap(v)
	default:
		return
	}
	if ti.Type == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.getOrCreateStateLocked(agentID)
	for i := len(state.Rows) - 1; i >= 0; i-- {
		if timelineItemsEqual(state.Rows[i].Item, ti) {
			return
		}
	}

	s.appendLocked(state, agentID, ti)
}

// timelineItemFromMap converts a map[string]interface{} to a TimelineItem.
func timelineItemFromMap(m map[string]interface{}) TimelineItem {
	ti := TimelineItem{}
	if t, ok := m["type"].(string); ok {
		ti.Type = t
	}
	if text, ok := m["text"].(string); ok {
		ti.Text = text
	}
	if mid, ok := m["messageId"].(string); ok {
		ti.MessageID = mid
	}
	if cid, ok := m["callId"].(string); ok {
		ti.CallID = cid
	}
	if name, ok := m["name"].(string); ok {
		ti.Name = name
	}
	if status, ok := m["status"].(string); ok {
		ti.Status = status
	}
	if detail, ok := m["detail"]; ok && detail != nil {
		if data, err := json.Marshal(detail); err == nil {
			var wrapper protocol.ToolCallDetailWrapper
			if err := json.Unmarshal(data, &wrapper); err == nil {
				ti.Detail = wrapper.Detail
			}
		}
	}
	if errVal, ok := m["error"]; ok && errVal != nil {
		if data, err := json.Marshal(errVal); err == nil {
			var te protocol.ToolError
			if err := json.Unmarshal(data, &te); err == nil {
				ti.Error = &te
			}
		}
	}
	return ti
}

func (s *InMemoryTimelineStore) GetEpoch(agentID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if state, ok := s.states[agentID]; ok {
		return state.Epoch
	}
	return ""
}

// TimelineFetchResult is the result of a timeline fetch.
type TimelineFetchResult struct {
	Epoch       string         `json:"epoch"`
	Direction   string         `json:"direction"`
	Reset       bool           `json:"reset"`
	StaleCursor bool           `json:"staleCursor"`
	Gap         bool           `json:"gap"`
	Window      TimelineWindow `json:"window"`
	HasOlder    bool           `json:"hasOlder"`
	HasNewer    bool           `json:"hasNewer"`
	Rows        []TimelineRow  `json:"rows"`
}

type TimelineWindow struct {
	MinSeq  int `json:"minSeq"`
	MaxSeq  int `json:"maxSeq"`
	NextSeq int `json:"nextSeq"`
}

// ToProtocolCursor converts a TimelineRow to a protocol cursor.
func (r *TimelineRow) ToProtocolCursor(epoch string) protocol.AgentTimelineCursor {
	return protocol.AgentTimelineCursor{
		Epoch: epoch,
		Seq:   r.Seq,
	}
}

// timelineItemsEqual reports whether two timeline items are identical for
// deduplication purposes. It is called inside Append to prevent duplicate rows
// when multiple sessions process the same stream event.
func timelineItemsEqual(a, b TimelineItem) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case "user_message":
		if a.MessageID != "" && b.MessageID != "" {
			return a.MessageID == b.MessageID
		}
		return a.Text == b.Text
	case "assistant_message", "reasoning":
		return a.Text == b.Text
	case "tool_call":
		return a.CallID == b.CallID && a.Status == b.Status
	case "todo":
		if len(a.TodoItems) != len(b.TodoItems) {
			return false
		}
		for i := range a.TodoItems {
			if a.TodoItems[i].Text != b.TodoItems[i].Text || a.TodoItems[i].Completed != b.TodoItems[i].Completed {
				return false
			}
		}
		return true
	case "error":
		return a.Message == b.Message
	case "compaction":
		return a.CompactionStatus == b.CompactionStatus && a.Trigger == b.Trigger
	default:
		return false
	}
}

// FormatSeqRange returns a string like "5-8" for a range of sequence numbers.
func FormatSeqRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}
