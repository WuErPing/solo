package loop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// StoreOption configures a Store.
type StoreOption func(*Store)

// WithDataPath sets the JSON persistence path. Empty disables persistence.
func WithDataPath(path string) StoreOption {
	return func(s *Store) {
		s.dataPath = path
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) StoreOption {
	return func(s *Store) {
		s.logger = logger
	}
}

// Store is an in-memory loop record store with optional JSON persistence.
type Store struct {
	mu       sync.RWMutex
	records  map[string]*protocol.LoopRecord
	dataPath string
	logger   *slog.Logger
}

// NewStore creates a new store.
func NewStore(opts ...StoreOption) *Store {
	s := &Store{
		records: make(map[string]*protocol.LoopRecord),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.dataPath != "" {
		if err := s.load(); err != nil && s.logger != nil {
			s.logger.Warn("failed to load loops", "error", err)
		}
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

// Create creates a new loop record and persists it.
func (s *Store) Create(req protocol.LoopRunRequest, defaultProvider func() (string, error)) (*protocol.LoopRecord, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	provider := ""
	if req.AgentTemplate != nil && req.AgentTemplate.Provider != "" {
		provider = req.AgentTemplate.Provider
	} else if req.Provider != nil && *req.Provider != "" {
		provider = *req.Provider
	} else {
		p, err := defaultProvider()
		if err != nil {
			return nil, fmt.Errorf("no provider available: %w", err)
		}
		provider = p
	}

	// Build shared templates for new code while keeping legacy provider/model
	// fields populated for old clients during the deprecation window.
	agentTemplate := req.AgentTemplate
	if agentTemplate == nil {
		agentTemplate = &protocol.AgentTemplate{
			Provider: provider,
			Cwd:      req.Cwd,
			Model:    req.Model,
		}
	} else if agentTemplate.Cwd == "" {
		// Copy so we don't mutate the request value.
		cp := *agentTemplate
		cp.Cwd = req.Cwd
		agentTemplate = &cp
	}

	workerTemplate := req.WorkerAgentTemplate
	if workerTemplate == nil && (req.WorkerProvider != nil || req.WorkerModel != nil) {
		wp := ""
		if req.WorkerProvider != nil {
			wp = *req.WorkerProvider
		}
		workerTemplate = &protocol.AgentTemplate{
			Provider: wp,
			Cwd:      req.Cwd,
			Model:    req.WorkerModel,
		}
	}

	verifierTemplate := req.VerifierAgentTemplate
	if verifierTemplate == nil && (req.VerifierProvider != nil || req.VerifierModel != nil) {
		vp := ""
		if req.VerifierProvider != nil {
			vp = *req.VerifierProvider
		}
		verifierTemplate = &protocol.AgentTemplate{
			Provider: vp,
			Cwd:      req.Cwd,
			Model:    req.VerifierModel,
		}
	}

	maxIterations := 10
	if req.MaxIterations != nil {
		maxIterations = *req.MaxIterations
	}

	sleepMs := 1000
	if req.SleepMs != nil {
		sleepMs = *req.SleepMs
	}

	now := nowISO()
	record := &protocol.LoopRecord{
		ID:                    generateID(),
		Prompt:                req.Prompt,
		Cwd:                   req.Cwd,
		Provider:              provider,
		Model:                 req.Model,
		VerifyPrompt:          req.VerifyPrompt,
		VerifyChecks:          req.VerifyChecks,
		Archive:               req.Archive != nil && *req.Archive,
		SleepMs:               sleepMs,
		MaxIterations:         &maxIterations,
		MaxTimeMs:             req.MaxTimeMs,
		Status:                string(StatusRunning),
		CreatedAt:             now,
		UpdatedAt:             now,
		StartedAt:             now,
		NextLogSeq:            1,
		Iterations:            []protocol.LoopIterationRecord{},
		AgentTemplate:         agentTemplate,
		WorkerAgentTemplate:   workerTemplate,
		VerifierAgentTemplate: verifierTemplate,
		Logs: []protocol.LoopLogEntry{
			{
				Seq:       1,
				Timestamp: now,
				Source:    "loop",
				Level:     "info",
				Text:      "Loop started",
			},
		},
	}
	if req.Name != nil {
		record.Name = req.Name
	}
	record.NextLogSeq = 2

	s.mu.Lock()
	s.records[record.ID] = record
	err := s.saveLocked()
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}

	return copyRecord(record), nil
}

// List returns loop summaries sorted by UpdatedAt descending.
func (s *Store) List() []protocol.LoopListItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]protocol.LoopListItem, 0, len(s.records))
	for _, r := range s.records {
		result = append(result, toListItem(r))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UpdatedAt != result[j].UpdatedAt {
			return result[i].UpdatedAt > result[j].UpdatedAt
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// Get returns a deep copy of a loop record.
func (s *Store) Get(id string) (*protocol.LoopRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.records[id]
	if !ok {
		return nil, false
	}
	return copyRecord(r), true
}

// Update modifies the mutable fields of a loop record.
func (s *Store) Update(id string, input UpdateInput) (*protocol.LoopRecord, error) {
	s.mu.Lock()

	r, ok := s.records[id]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("loop not found")
	}

	hasAnyField := input.Name != nil || input.Archive != nil ||
		input.Prompt != nil || input.Cwd != nil ||
		input.VerifyChecks != nil || input.MaxIterations != nil ||
		input.AgentTemplate != nil || input.WorkerAgentTemplate != nil ||
		input.VerifierAgentTemplate != nil
	if !hasAnyField {
		s.mu.Unlock()
		return nil, fmt.Errorf("no fields to update")
	}

	if input.Name != nil {
		r.Name = input.Name
	}
	if input.Archive != nil {
		r.Archive = *input.Archive
	}
	if input.Prompt != nil {
		r.Prompt = *input.Prompt
	}
	if input.Cwd != nil {
		r.Cwd = *input.Cwd
	}
	if input.VerifyChecks != nil {
		r.VerifyChecks = *input.VerifyChecks
	}
	if input.MaxIterations != nil {
		r.MaxIterations = input.MaxIterations
	}
	if input.AgentTemplate != nil {
		r.AgentTemplate = input.AgentTemplate
		// Keep legacy fields in sync during the deprecation window.
		r.Provider = input.AgentTemplate.Provider
		r.Model = input.AgentTemplate.Model
	}
	if input.WorkerAgentTemplate != nil {
		r.WorkerAgentTemplate = input.WorkerAgentTemplate
		r.WorkerProvider = &input.WorkerAgentTemplate.Provider
		r.WorkerModel = input.WorkerAgentTemplate.Model
	}
	if input.VerifierAgentTemplate != nil {
		r.VerifierAgentTemplate = input.VerifierAgentTemplate
		r.VerifierProvider = &input.VerifierAgentTemplate.Provider
		r.VerifierModel = input.VerifierAgentTemplate.Model
	}
	r.UpdatedAt = nowISO()
	err := s.saveLocked()
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return copyRecord(r), nil
}

// Delete removes a loop record. Running loops cannot be deleted.
func (s *Store) Delete(id string) error {
	s.mu.Lock()

	r, ok := s.records[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("loop not found")
	}
	if r.Status == string(StatusRunning) {
		s.mu.Unlock()
		return fmt.Errorf("cannot delete a running loop")
	}

	delete(s.records, id)
	err := s.saveLocked()
	s.mu.Unlock()

	return err
}

// Stop requests a running loop to stop and marks it as stopped.
func (s *Store) Stop(id string) (*protocol.LoopRecord, error) {
	s.mu.Lock()

	r, ok := s.records[id]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("loop not found")
	}
	if r.Status != string(StatusRunning) {
		s.mu.Unlock()
		return nil, fmt.Errorf("loop is not running")
	}

	now := nowISO()
	r.StopRequestedAt = &now
	r.Status = string(StatusStopped)
	r.UpdatedAt = now
	err := s.saveLocked()
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return copyRecord(r), nil
}

// AppendLog appends a log entry, assigning the next sequence number.
func (s *Store) AppendLog(id string, entry protocol.LoopLogEntry) error {
	s.mu.Lock()

	r, ok := s.records[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("loop not found")
	}
	entry.Seq = r.NextLogSeq
	r.NextLogSeq++
	r.Logs = append(r.Logs, entry)
	r.UpdatedAt = nowISO()
	err := s.saveLocked()
	s.mu.Unlock()

	return err
}

// AppendIteration appends an iteration and updates active pointers.
func (s *Store) AppendIteration(id string, iter protocol.LoopIterationRecord) error {
	s.mu.Lock()

	r, ok := s.records[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("loop not found")
	}
	r.Iterations = append(r.Iterations, iter)
	idx := len(r.Iterations)
	r.ActiveIteration = &idx
	r.ActiveWorkerAgentID = iter.WorkerAgentID
	r.UpdatedAt = nowISO()
	err := s.saveLocked()
	s.mu.Unlock()

	return err
}

// UpdateIteration updates an existing iteration by index.
func (s *Store) UpdateIteration(id string, iter protocol.LoopIterationRecord) error {
	s.mu.Lock()

	r, ok := s.records[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("loop not found")
	}
	found := false
	for i := range r.Iterations {
		if r.Iterations[i].Index == iter.Index {
			r.Iterations[i] = iter
			found = true
			break
		}
	}
	if !found {
		s.mu.Unlock()
		return fmt.Errorf("iteration not found")
	}
	r.UpdatedAt = nowISO()
	err := s.saveLocked()
	s.mu.Unlock()

	return err
}

// SetStatus sets the loop status and clears active iteration pointers when terminal.
func (s *Store) SetStatus(id string, status Status, failureReason *string) (*protocol.LoopRecord, error) {
	s.mu.Lock()

	r, ok := s.records[id]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("loop not found")
	}

	r.Status = string(status)
	now := nowISO()
	r.UpdatedAt = now
	if status == StatusSucceeded || status == StatusFailed || status == StatusStopped {
		r.CompletedAt = &now
		r.ActiveIteration = nil
		r.ActiveWorkerAgentID = nil
		r.ActiveVerifierAgentID = nil
	}
	if failureReason != nil && *failureReason != "" {
		r.Logs = append(r.Logs, protocol.LoopLogEntry{
			Seq:       r.NextLogSeq,
			Timestamp: now,
			Source:    "loop",
			Level:     "error",
			Text:      *failureReason,
		})
		r.NextLogSeq++
	}
	err := s.saveLocked()
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return copyRecord(r), nil
}

// Save persists the store to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// saveLocked persists the store to disk. Caller must hold s.mu.
func (s *Store) saveLocked() error {
	if s.dataPath == "" {
		return nil
	}

	dir := filepath.Dir(s.dataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	data := make(map[string]*protocol.LoopRecord, len(s.records))
	for k, v := range s.records {
		data[k] = v
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal loops: %w", err)
	}

	tmp := s.dataPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write loops file: %w", err)
	}
	if err := os.Rename(tmp, s.dataPath); err != nil {
		return fmt.Errorf("rename loops file: %w", err)
	}
	return nil
}

func (s *Store) load() error {
	if s.dataPath == "" {
		return nil
	}

	b, err := os.ReadFile(s.dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read loops file: %w", err)
	}

	var data map[string]*protocol.LoopRecord
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("unmarshal loops: %w", err)
	}

	s.records = data
	return nil
}

func toListItem(r *protocol.LoopRecord) protocol.LoopListItem {
	provider := r.Provider
	model := r.Model
	if r.AgentTemplate != nil {
		provider = r.AgentTemplate.Provider
		model = r.AgentTemplate.Model
	}
	return protocol.LoopListItem{
		ID:              r.ID,
		Name:            r.Name,
		Status:          r.Status,
		Cwd:             r.Cwd,
		Provider:        provider,
		Model:           model,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		ActiveIteration: r.ActiveIteration,
	}
}

func copyRecord(r *protocol.LoopRecord) *protocol.LoopRecord {
	if r == nil {
		return nil
	}
	c := *r

	if r.AgentTemplate != nil {
		cp := *r.AgentTemplate
		c.AgentTemplate = &cp
	}
	if r.WorkerAgentTemplate != nil {
		cp := *r.WorkerAgentTemplate
		c.WorkerAgentTemplate = &cp
	}
	if r.VerifierAgentTemplate != nil {
		cp := *r.VerifierAgentTemplate
		c.VerifierAgentTemplate = &cp
	}

	if r.Name != nil {
		name := *r.Name
		c.Name = &name
	}
	if r.Model != nil {
		model := *r.Model
		c.Model = &model
	}
	if r.WorkerProvider != nil {
		wp := *r.WorkerProvider
		c.WorkerProvider = &wp
	}
	if r.WorkerModel != nil {
		wm := *r.WorkerModel
		c.WorkerModel = &wm
	}
	if r.VerifierProvider != nil {
		vp := *r.VerifierProvider
		c.VerifierProvider = &vp
	}
	if r.VerifierModel != nil {
		vm := *r.VerifierModel
		c.VerifierModel = &vm
	}
	if r.VerifyPrompt != nil {
		vp := *r.VerifyPrompt
		c.VerifyPrompt = &vp
	}
	if r.MaxIterations != nil {
		mi := *r.MaxIterations
		c.MaxIterations = &mi
	}
	if r.MaxTimeMs != nil {
		mt := *r.MaxTimeMs
		c.MaxTimeMs = &mt
	}
	if r.CompletedAt != nil {
		ca := *r.CompletedAt
		c.CompletedAt = &ca
	}
	if r.StopRequestedAt != nil {
		sa := *r.StopRequestedAt
		c.StopRequestedAt = &sa
	}
	if r.ActiveIteration != nil {
		ai := *r.ActiveIteration
		c.ActiveIteration = &ai
	}
	if r.ActiveWorkerAgentID != nil {
		aw := *r.ActiveWorkerAgentID
		c.ActiveWorkerAgentID = &aw
	}
	if r.ActiveVerifierAgentID != nil {
		av := *r.ActiveVerifierAgentID
		c.ActiveVerifierAgentID = &av
	}

	c.VerifyChecks = make([]string, len(r.VerifyChecks))
	copy(c.VerifyChecks, r.VerifyChecks)

	c.Iterations = make([]protocol.LoopIterationRecord, len(r.Iterations))
	copy(c.Iterations, r.Iterations)
	for i := range c.Iterations {
		c.Iterations[i].VerifyChecks = make([]protocol.LoopVerifyCheckResult, len(r.Iterations[i].VerifyChecks))
		copy(c.Iterations[i].VerifyChecks, r.Iterations[i].VerifyChecks)
		if r.Iterations[i].WorkerAgentID != nil {
			wid := *r.Iterations[i].WorkerAgentID
			c.Iterations[i].WorkerAgentID = &wid
		}
		if r.Iterations[i].WorkerCompletedAt != nil {
			wca := *r.Iterations[i].WorkerCompletedAt
			c.Iterations[i].WorkerCompletedAt = &wca
		}
		if r.Iterations[i].VerifierAgentID != nil {
			vid := *r.Iterations[i].VerifierAgentID
			c.Iterations[i].VerifierAgentID = &vid
		}
		if r.Iterations[i].WorkerOutcome != nil {
			wo := *r.Iterations[i].WorkerOutcome
			c.Iterations[i].WorkerOutcome = &wo
		}
		if r.Iterations[i].FailureReason != nil {
			fr := *r.Iterations[i].FailureReason
			c.Iterations[i].FailureReason = &fr
		}
		if r.Iterations[i].VerifyPrompt != nil {
			vp := *r.Iterations[i].VerifyPrompt
			c.Iterations[i].VerifyPrompt = &vp
		}
	}

	c.Logs = make([]protocol.LoopLogEntry, len(r.Logs))
	copy(c.Logs, r.Logs)
	for i := range c.Logs {
		if r.Logs[i].Iteration != nil {
			it := *r.Logs[i].Iteration
			c.Logs[i].Iteration = &it
		}
	}

	return &c
}
