package workspace

import (
	"path/filepath"
	"time"
)

// WorkspaceKind represents the type of workspace.
type WorkspaceKind string

const (
	WorkspaceKindLocalCheckout WorkspaceKind = "local_checkout"
	WorkspaceKindWorktree      WorkspaceKind = "worktree"
	WorkspaceKindDirectory     WorkspaceKind = "directory"
)

// PersistedWorkspaceRecord represents a workspace (working directory) in the registry.
type PersistedWorkspaceRecord struct {
	WorkspaceID string        `json:"workspaceId"`
	ProjectID   string        `json:"projectId"`
	Cwd         string        `json:"cwd"`
	Kind        WorkspaceKind `json:"kind"`
	DisplayName string        `json:"displayName"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	ArchivedAt  *time.Time    `json:"archivedAt,omitempty"`
}

func (w PersistedWorkspaceRecord) GetID() string { return w.WorkspaceID }

func (w *PersistedWorkspaceRecord) SetArchivedAt(t *time.Time) { w.ArchivedAt = t }

// WorkspaceRegistry manages workspace records.
type WorkspaceRegistry struct {
	*FileBackedRegistry[*PersistedWorkspaceRecord]
}

// NewWorkspaceRegistry creates a new WorkspaceRegistry.
func NewWorkspaceRegistry(baseDir string) *WorkspaceRegistry {
	return &WorkspaceRegistry{
		FileBackedRegistry: NewFileBackedRegistry[*PersistedWorkspaceRecord](
			filepath.Join(baseDir, "projects", "workspaces.json"),
		),
	}
}

// FindByCwd looks up a workspace by its working directory path.
func (r *WorkspaceRegistry) FindByCwd(cwd string) (*PersistedWorkspaceRecord, bool) {
	for _, rec := range r.List() {
		if rec.Cwd == cwd && rec.ArchivedAt == nil {
			return rec, true
		}
	}
	return nil, false
}

// FindByProjectID returns all workspaces for a given project.
func (r *WorkspaceRegistry) FindByProjectID(projectID string) []*PersistedWorkspaceRecord {
	var result []*PersistedWorkspaceRecord
	for _, rec := range r.List() {
		if rec.ProjectID == projectID && rec.ArchivedAt == nil {
			result = append(result, rec)
		}
	}
	return result
}

// UpsertWorkspace creates or updates a workspace record.
func (r *WorkspaceRegistry) UpsertWorkspace(id, projectID, cwd string, kind WorkspaceKind, displayName string) error {
	now := time.Now()
	existing, ok := r.Get(id)
	if ok {
		existing.UpdatedAt = now
		if kind != "" {
			existing.Kind = kind
		}
		if projectID != "" {
			existing.ProjectID = projectID
		}
		if cwd != "" {
			existing.Cwd = cwd
		}
		if displayName != "" {
			existing.DisplayName = displayName
		}
		return r.Upsert(existing)
	}

	return r.Upsert(&PersistedWorkspaceRecord{
		WorkspaceID: id,
		ProjectID:   projectID,
		Cwd:         cwd,
		Kind:        kind,
		DisplayName: displayName,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}
