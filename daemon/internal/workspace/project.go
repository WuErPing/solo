package workspace

import (
	"path/filepath"
	"time"
)

// ProjectKind represents the type of project.
type ProjectKind string

const (
	ProjectKindGit    ProjectKind = "git"
	ProjectKindNonGit ProjectKind = "non_git"
)

// PersistedProjectRecord represents a project (git repo or directory) in the registry.
type PersistedProjectRecord struct {
	ProjectID   string     `json:"projectId"`
	RootPath    string     `json:"rootPath"`
	Kind        ProjectKind `json:"kind"`
	DisplayName string     `json:"displayName"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ArchivedAt  *time.Time `json:"archivedAt,omitempty"`
}

func (p PersistedProjectRecord) GetID() string { return p.ProjectID }

func (p *PersistedProjectRecord) SetArchivedAt(t *time.Time) { p.ArchivedAt = t }

// ProjectRegistry manages project records.
type ProjectRegistry struct {
	*FileBackedRegistry[*PersistedProjectRecord]
}

// NewProjectRegistry creates a new ProjectRegistry.
func NewProjectRegistry(baseDir string) *ProjectRegistry {
	return &ProjectRegistry{
		FileBackedRegistry: NewFileBackedRegistry[*PersistedProjectRecord](
			filepath.Join(baseDir, "projects", "projects.json"),
		),
	}
}

// FindByRootPath looks up a project by its root path.
func (r *ProjectRegistry) FindByRootPath(rootPath string) (*PersistedProjectRecord, bool) {
	for _, rec := range r.List() {
		if rec.RootPath == rootPath && rec.ArchivedAt == nil {
			return rec, true
		}
	}
	return nil, false
}

// UpsertProject creates or updates a project record.
func (r *ProjectRegistry) UpsertProject(id, rootPath string, kind ProjectKind, displayName string) error {
	now := time.Now()
	existing, ok := r.Get(id)
	if ok {
		existing.UpdatedAt = now
		if displayName != "" {
			existing.DisplayName = displayName
		}
		return r.Upsert(existing)
	}

	return r.Upsert(&PersistedProjectRecord{
		ProjectID:   id,
		RootPath:    rootPath,
		Kind:        kind,
		DisplayName: displayName,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}
