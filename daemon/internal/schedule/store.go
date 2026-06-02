package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

type StoreOption func(*Store)

func WithDataPath(path string) StoreOption {
	return func(s *Store) {
		s.dataPath = path
	}
}

type Store struct {
	mu        sync.RWMutex
	schedules map[string]*protocol.StoredSchedule
	dataPath  string
}

func NewStore(opts ...StoreOption) *Store {
	s := &Store{
		schedules: make(map[string]*protocol.StoredSchedule),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.dataPath != "" {
		s.load()
	}
	return s
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func ptr[T any](v T) *T {
	return &v
}

func computeNextRun(cadence protocol.ScheduleCadence) *string {
	next := NextRunAt(cadence, time.Now().UTC())
	if next == nil {
		return nil
	}
	s := next.Format(time.RFC3339)
	return &s
}

func (st *Store) save() error {
	if st.dataPath == "" {
		return nil
	}

	dir := filepath.Dir(st.dataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	st.mu.RLock()
	data := make(map[string]*protocol.StoredSchedule, len(st.schedules))
	for k, v := range st.schedules {
		data[k] = v
	}
	st.mu.RUnlock()

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schedules: %w", err)
	}

	if err := os.WriteFile(st.dataPath, b, 0644); err != nil {
		return fmt.Errorf("write schedules file: %w", err)
	}
	return nil
}

func (st *Store) load() error {
	if st.dataPath == "" {
		return nil
	}

	b, err := os.ReadFile(st.dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read schedules file: %w", err)
	}

	var data map[string]*protocol.StoredSchedule
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("unmarshal schedules: %w", err)
	}

	st.schedules = data
	st.fixupNextRunAt()
	return nil
}

// fixupNextRunAt recomputes nextRunAt for active schedules on load.
// This self-heals stale values from a previous buggy NextRunAt computation.
func (st *Store) fixupNextRunAt() {
	dirty := false
	for _, s := range st.schedules {
		if s.Status != "active" {
			continue
		}
		next := computeNextRun(s.Cadence)
		if next == nil && s.NextRunAt == nil {
			continue
		}
		if next != nil && s.NextRunAt != nil && *next == *s.NextRunAt {
			continue
		}
		s.NextRunAt = next
		dirty = true
	}
	if dirty {
		_ = st.save()
	}
}

func (st *Store) Create(input protocol.ScheduleCreateRequest) (*protocol.StoredSchedule, error) {
	if input.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if input.Cadence.Type != "cron" && input.Cadence.Type != "every" {
		return nil, fmt.Errorf("invalid cadence type: %s", input.Cadence.Type)
	}
	if input.Cadence.Type == "every" && input.Cadence.EveryMs <= 0 {
		return nil, fmt.Errorf("everyMs must be positive")
	}
	if input.Cadence.Type == "cron" && input.Cadence.Expression == "" {
		return nil, fmt.Errorf("cron expression is required")
	}

	now := nowISO()
	schedule := &protocol.StoredSchedule{
		ID:        generateID(),
		Prompt:    input.Prompt,
		Cadence:   input.Cadence,
		Target:    input.Target,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
		NextRunAt: computeNextRun(input.Cadence),
		Runs:      []protocol.ScheduleRun{},
	}
	if input.Name != "" {
		schedule.Name = &input.Name
	}
	if input.MaxRuns != nil && *input.MaxRuns > 0 {
		schedule.MaxRuns = input.MaxRuns
	}
	if input.ExpiresAt != "" {
		schedule.ExpiresAt = &input.ExpiresAt
	}

	st.mu.Lock()
	st.schedules[schedule.ID] = schedule
	st.mu.Unlock()

	if err := st.save(); err != nil {
		return nil, err
	}

	return schedule, nil
}

func (st *Store) List() []protocol.ScheduleSummary {
	st.mu.RLock()
	defer st.mu.RUnlock()

	result := make([]protocol.ScheduleSummary, 0, len(st.schedules))
	for _, s := range st.schedules {
		result = append(result, toSummary(s))
	}
	return result
}

func (st *Store) Get(id string) (*protocol.StoredSchedule, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	s, ok := st.schedules[id]
	if !ok {
		return nil, false
	}
	schedCopy := *s
	schedCopy.Runs = make([]protocol.ScheduleRun, len(s.Runs))
	copy(schedCopy.Runs, s.Runs)
	return &schedCopy, true
}

func (st *Store) Pause(id string) (*protocol.StoredSchedule, error) {
	st.mu.Lock()

	s, ok := st.schedules[id]
	if !ok {
		st.mu.Unlock()
		return nil, fmt.Errorf("schedule not found")
	}
	if s.Status == "paused" {
		st.mu.Unlock()
		return nil, fmt.Errorf("schedule already paused")
	}
	s.Status = "paused"
	now := nowISO()
	s.PausedAt = &now
	s.UpdatedAt = now
	st.mu.Unlock()

	if err := st.save(); err != nil {
		return nil, err
	}

	return s, nil
}

func (st *Store) Resume(id string) (*protocol.StoredSchedule, error) {
	st.mu.Lock()

	s, ok := st.schedules[id]
	if !ok {
		st.mu.Unlock()
		return nil, fmt.Errorf("schedule not found")
	}
	if s.Status == "active" {
		st.mu.Unlock()
		return nil, fmt.Errorf("schedule already active")
	}
	s.Status = "active"
	s.NextRunAt = computeNextRun(s.Cadence)
	s.UpdatedAt = nowISO()
	st.mu.Unlock()

	if err := st.save(); err != nil {
		return nil, err
	}

	return s, nil
}

func (st *Store) Update(input protocol.ScheduleUpdateRequest) (*protocol.StoredSchedule, error) {
	if input.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if input.Cadence.Type != "cron" && input.Cadence.Type != "every" {
		return nil, fmt.Errorf("invalid cadence type: %s", input.Cadence.Type)
	}
	if input.Cadence.Type == "every" && input.Cadence.EveryMs <= 0 {
		return nil, fmt.Errorf("everyMs must be positive")
	}
	if input.Cadence.Type == "cron" && input.Cadence.Expression == "" {
		return nil, fmt.Errorf("cron expression is required")
	}

	st.mu.Lock()
	s, ok := st.schedules[input.ScheduleID]
	if !ok {
		st.mu.Unlock()
		return nil, fmt.Errorf("schedule not found")
	}

	s.Prompt = input.Prompt
	s.Cadence = input.Cadence
	s.Target = input.Target
	s.UpdatedAt = nowISO()
	s.NextRunAt = computeNextRun(input.Cadence)
	if input.Name != "" {
		s.Name = &input.Name
	} else {
		s.Name = nil
	}
	if input.MaxRuns != nil && *input.MaxRuns > 0 {
		s.MaxRuns = input.MaxRuns
	} else {
		s.MaxRuns = nil
	}
	if input.ExpiresAt != "" {
		s.ExpiresAt = &input.ExpiresAt
	} else {
		s.ExpiresAt = nil
	}
	st.mu.Unlock()

	if err := st.save(); err != nil {
		return nil, err
	}

	return s, nil
}

func (st *Store) Delete(id string) error {
	st.mu.Lock()
	if _, ok := st.schedules[id]; !ok {
		st.mu.Unlock()
		return fmt.Errorf("schedule not found")
	}
	delete(st.schedules, id)
	st.mu.Unlock()

	if err := st.save(); err != nil {
		return err
	}

	return nil
}

func toSummary(s *protocol.StoredSchedule) protocol.ScheduleSummary {
	return protocol.ScheduleSummary{
		ID:        s.ID,
		Name:      s.Name,
		Prompt:    s.Prompt,
		Cadence:   s.Cadence,
		Target:    s.Target,
		Status:    s.Status,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		NextRunAt: s.NextRunAt,
		LastRunAt: s.LastRunAt,
		PausedAt:  s.PausedAt,
		ExpiresAt: s.ExpiresAt,
		MaxRuns:   s.MaxRuns,
	}
}
