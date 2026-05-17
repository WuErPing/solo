package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
	"github.com/google/uuid"
)

// StoredAgentRecord is the on-disk JSON representation of an agent.
type StoredAgentRecord struct {
	ID                 string                           `json:"id"`
	Provider           string                           `json:"provider"`
	Cwd                string                           `json:"cwd"`
	CreatedAt          string                           `json:"createdAt"`
	UpdatedAt          string                           `json:"updatedAt"`
	LastActivityAt     string                           `json:"lastActivityAt,omitempty"`
	LastUserMessageAt  *string                          `json:"lastUserMessageAt,omitempty"`
	Title              *string                          `json:"title,omitempty"`
	Labels             map[string]string                `json:"labels"`
	LastStatus         string                           `json:"lastStatus"`
	LastModeID         *string                          `json:"lastModeId,omitempty"`
	Config             *SerializableConfig              `json:"config,omitempty"`
	RuntimeInfo        *StoredRuntimeInfo               `json:"runtimeInfo,omitempty"`
	Features           []protocol.AgentFeature          `json:"features,omitempty"`
	Persistence        *protocol.AgentPersistenceHandle `json:"persistence,omitempty"`
	LastError          *string                          `json:"lastError,omitempty"`
	RequiresAttention  bool                             `json:"requiresAttention,omitempty"`
	AttentionReason    *string                          `json:"attentionReason,omitempty"`
	AttentionTimestamp *string                          `json:"attentionTimestamp,omitempty"`
	Internal           bool                             `json:"internal,omitempty"`
	ArchivedAt         *string                          `json:"archivedAt,omitempty"`
}

type SerializableConfig struct {
	Title            *string                             `json:"title,omitempty"`
	ModeID           *string                             `json:"modeId,omitempty"`
	Model            *string                             `json:"model,omitempty"`
	ThinkingOptionID *string                             `json:"thinkingOptionId,omitempty"`
	FeatureValues    map[string]interface{}              `json:"featureValues,omitempty"`
	Extra            map[string]interface{}              `json:"extra,omitempty"`
	SystemPrompt     string                              `json:"systemPrompt,omitempty"`
	McpServers       map[string]protocol.McpServerConfig `json:"mcpServers,omitempty"`
}

type StoredRuntimeInfo struct {
	Provider         string                 `json:"provider"`
	SessionID        *string                `json:"sessionId"`
	Model            *string                `json:"model,omitempty"`
	ThinkingOptionID *string                `json:"thinkingOptionId,omitempty"`
	ModeID           *string                `json:"modeId,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

// AgentStorage persists agent records as JSON files on disk.
type AgentStorage struct {
	mu       sync.RWMutex
	cache    map[string]*StoredAgentRecord
	pathByID map[string]string
	deleting map[string]bool
	loaded   bool
	baseDir  string
	logger   *slog.Logger
}

// NewAgentStorage creates a new AgentStorage.
func NewAgentStorage(baseDir string, logger *slog.Logger) *AgentStorage {
	return &AgentStorage{
		cache:    make(map[string]*StoredAgentRecord),
		pathByID: make(map[string]string),
		deleting: make(map[string]bool),
		baseDir:  baseDir,
		logger:   logger.With("component", "agent-storage"),
	}
}

// Initialize scans disk for existing agent records.
func (s *AgentStorage) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return nil
	}

	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return fmt.Errorf("cannot create agents dir: %w", err)
	}

	records, err := s.scanDisk()
	if err != nil {
		return fmt.Errorf("scan disk: %w", err)
	}

	for _, r := range records {
		s.cache[r.ID] = r
		s.pathByID[r.ID] = s.recordPath(r)
	}

	s.loaded = true
	s.logger.Info("initialized", "agents", len(s.cache))
	return nil
}

// List returns all stored agent records.
func (s *AgentStorage) List() []*StoredAgentRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*StoredAgentRecord, 0, len(s.cache))
	for _, r := range s.cache {
		result = append(result, r)
	}
	return result
}

// Get returns a single agent record by ID.
func (s *AgentStorage) Get(agentID string) *StoredAgentRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[agentID]
}

// Upsert writes or updates an agent record.
func (s *AgentStorage) Upsert(record *StoredAgentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upsertLocked(record)
}

// upsertLocked is the internal implementation; callers must hold s.mu.Lock().
func (s *AgentStorage) upsertLocked(record *StoredAgentRecord) error {
	if s.deleting[record.ID] {
		return nil
	}

	targetPath := s.recordPath(record)
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal agent %s: %w", record.ID, err)
	}

	if err := writeFileAtomic(targetPath, data); err != nil {
		return fmt.Errorf("write agent %s: %w", record.ID, err)
	}

	// Remove old file if path changed
	if oldPath, ok := s.pathByID[record.ID]; ok && oldPath != targetPath {
		os.Remove(oldPath)
	}

	s.cache[record.ID] = record
	s.pathByID[record.ID] = targetPath
	return nil
}

// BeginDelete marks an agent for deletion, preventing future upserts.
func (s *AgentStorage) BeginDelete(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleting[agentID] = true
}

// Remove deletes an agent from disk and cache.
func (s *AgentStorage) Remove(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, ok := s.pathByID[agentID]
	if ok {
		os.Remove(path)
	}
	delete(s.cache, agentID)
	delete(s.pathByID, agentID)
	return nil
}

// ApplySnapshot projects a ManagedAgent into a StoredAgentRecord and upserts it.
func (s *AgentStorage) ApplySnapshot(agent *ManagedAgent, opts ...SnapshotOption) error {
	options := &snapshotOptions{}
	for _, o := range opts {
		o(options)
	}

	// Read the existing record under a read lock (fast map lookup).
	s.mu.RLock()
	existing := s.cache[agent.ID]
	s.mu.RUnlock()

	// Build the record outside the storage lock. toStoredAgentRecord acquires
	// agent.mu.RLock() internally, so lock order is always agent.mu → s.mu,
	// matching ClearAgentAttention (which also does agent.mu → s.mu).
	record := toStoredAgentRecord(agent, existing, options)

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upsertLocked(record)
}

// recordPath returns the file path for a stored agent record.
func (s *AgentStorage) recordPath(r *StoredAgentRecord) string {
	projectDir := projectDirNameFromCwd(r.Cwd)
	if projectDir == "" {
		projectDir = "root"
	}
	return filepath.Join(s.baseDir, projectDir, r.ID+".json")
}

// scanDisk reads all agent JSON files from the base directory.
func (s *AgentStorage) scanDisk() ([]*StoredAgentRecord, error) {
	var records []*StoredAgentRecord

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(s.baseDir, entry.Name())
			subRecords, err := s.scanDir(subPath)
			if err != nil {
				s.logger.Warn("error scanning subdir", "dir", subPath, "error", err)
				continue
			}
			records = append(records, subRecords...)
		} else if strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(s.baseDir, entry.Name())
			r, err := s.readRecord(filePath)
			if err != nil {
				s.logger.Warn("error reading agent file", "path", filePath, "error", err)
				continue
			}
			records = append(records, r)
		}
	}

	return records, nil
}

func (s *AgentStorage) scanDir(dir string) ([]*StoredAgentRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var records []*StoredAgentRecord
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(dir, entry.Name())
			r, err := s.readRecord(filePath)
			if err != nil {
				s.logger.Warn("error reading agent file", "path", filePath, "error", err)
				continue
			}
			records = append(records, r)
		}
	}
	return records, nil
}

func (s *AgentStorage) readRecord(filePath string) (*StoredAgentRecord, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var r StoredAgentRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// --- Helper functions ---

func projectDirNameFromCwd(cwd string) string {
	if cwd == "" {
		return "root"
	}
	// Sanitize: replace path separators and problematic chars with -
	result := strings.ReplaceAll(cwd, "/", "-")
	result = strings.ReplaceAll(result, "\\", "-")
	result = strings.ReplaceAll(result, ":", "-")
	result = strings.Trim(result, "-")
	if result == "" {
		return "root"
	}
	return result
}

func writeFileAtomic(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tempName := fmt.Sprintf(".agent.tmp-%d-%s", os.Getpid(), uuid.New().String()[:8])
	tempPath := filepath.Join(dir, tempName)
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempPath, targetPath)
}

// --- Snapshot projection ---

type snapshotOptions struct {
	title    *string
	internal *bool
}

type SnapshotOption func(*snapshotOptions)

func WithTitle(title string) SnapshotOption {
	return func(o *snapshotOptions) { o.title = &title }
}

func WithInternal(internal bool) SnapshotOption {
	return func(o *snapshotOptions) { o.internal = &internal }
}

func toStoredAgentRecord(agent *ManagedAgent, existing *StoredAgentRecord, opts *snapshotOptions) *StoredAgentRecord {
	now := time.Now().UTC().Format(time.RFC3339)

	agent.mu.RLock()
	defer agent.mu.RUnlock()

	// Determine title
	var title *string
	if opts.title != nil {
		title = opts.title
	} else if existing != nil {
		title = existing.Title
	}

	// Determine createdAt
	createdAt := now
	if existing != nil && existing.CreatedAt != "" {
		createdAt = existing.CreatedAt
	}

	// Determine internal
	internal := agent.Internal
	if opts.internal != nil {
		internal = *opts.internal
	}

	// Determine archivedAt (prefer agent value, fallback to existing)
	var archivedAt *string
	if agent.ArchivedAt != nil {
		archivedAt = agent.ArchivedAt
	} else if existing != nil {
		archivedAt = existing.ArchivedAt
	}

	// Last status
	lastStatus := string(agent.Lifecycle)

	// Attention
	requiresAttention := agent.Attention.Requires
	var attentionReason *string
	var attentionTimestamp *string
	if agent.Attention.Requires {
		reason := agent.Attention.Reason
		attentionReason = &reason
		ts := agent.Attention.Timestamp.Format(time.RFC3339)
		attentionTimestamp = &ts
	}

	// Config
	var config *SerializableConfig
	if agent.Config != nil {
		config = &SerializableConfig{
			Title:            agent.Config.Title,
			ModeID:           agent.Config.ModeID,
			Model:            agent.Config.Model,
			ThinkingOptionID: agent.Config.ThinkingOptionID,
			FeatureValues:    agent.Config.FeatureValues,
			Extra:            agent.Config.Extra,
			SystemPrompt:     agent.Config.SystemPrompt,
			McpServers:       agent.Config.McpServers,
		}
	}

	// Runtime info
	var runtimeInfo *StoredRuntimeInfo
	if agent.RuntimeInfo != nil {
		runtimeInfo = &StoredRuntimeInfo{
			Provider:         agent.RuntimeInfo.Provider,
			SessionID:        agent.RuntimeInfo.SessionID,
			Model:            agent.RuntimeInfo.Model,
			ThinkingOptionID: agent.RuntimeInfo.ThinkingOptionID,
			ModeID:           agent.RuntimeInfo.ModeID,
			Extra:            agent.RuntimeInfo.Extra,
		}
	}

	// Persistence
	var persistence *protocol.AgentPersistenceHandle
	if agent.Persistence != nil {
		p := *agent.Persistence
		persistence = &p
	}

	// Last user message at
	var lastUserMessageAt *string
	if agent.LastUserMessageAt != nil {
		s := agent.LastUserMessageAt.Format(time.RFC3339)
		lastUserMessageAt = &s
	}

	// Last mode ID
	lastModeID := agent.CurrentModeID
	if lastModeID == nil && agent.Config != nil {
		lastModeID = agent.Config.ModeID
	}

	return &StoredAgentRecord{
		ID:                 agent.ID,
		Provider:           agent.Provider,
		Cwd:                agent.Cwd,
		CreatedAt:          createdAt,
		UpdatedAt:          now,
		LastActivityAt:     now,
		LastUserMessageAt:  lastUserMessageAt,
		Title:              title,
		Labels:             agent.Labels,
		LastStatus:         lastStatus,
		LastModeID:         lastModeID,
		Config:             config,
		RuntimeInfo:        runtimeInfo,
		Features:           agent.Features,
		Persistence:        persistence,
		LastError:          agent.LastError,
		RequiresAttention:  requiresAttention,
		AttentionReason:    attentionReason,
		AttentionTimestamp: attentionTimestamp,
		Internal:           internal,
		ArchivedAt:         archivedAt,
	}
}
