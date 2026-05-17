package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Record is the interface that all registry records must implement.
type Record interface {
	GetID() string
}

// FileBackedRegistry is a generic in-memory map backed by a JSON file on disk.
// It uses atomic writes (write temp file then rename) and serializes all write
// operations through a mutex.
type FileBackedRegistry[T Record] struct {
	mu       sync.RWMutex
	records  map[string]T
	filePath string
}

// NewFileBackedRegistry creates a new FileBackedRegistry that persists to the given file path.
func NewFileBackedRegistry[T Record](filePath string) *FileBackedRegistry[T] {
	return &FileBackedRegistry[T]{
		records:  make(map[string]T),
		filePath: filePath,
	}
}

// Initialize loads records from disk. If the file doesn't exist, it starts with an empty registry.
func (r *FileBackedRegistry[T]) Initialize() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read registry file: %w", err)
	}

	var records []T
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("unmarshal registry: %w", err)
	}

	r.records = make(map[string]T, len(records))
	for _, rec := range records {
		r.records[rec.GetID()] = rec
	}
	return nil
}

// List returns all records.
func (r *FileBackedRegistry[T]) List() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]T, 0, len(r.records))
	for _, rec := range r.records {
		result = append(result, rec)
	}
	return result
}

// Get returns a record by ID, or false if not found.
func (r *FileBackedRegistry[T]) Get(id string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.records[id]
	return rec, ok
}

// Upsert inserts or updates a record and persists to disk.
func (r *FileBackedRegistry[T]) Upsert(rec T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records[rec.GetID()] = rec
	return r.persist()
}

// Archive sets the ArchivedAt field on a record (requires the record to implement ArchivableRecord).
func (r *FileBackedRegistry[T]) Archive(id string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.records[id]
	if !ok {
		return fmt.Errorf("record %s not found", id)
	}

	archivable, ok := any(rec).(ArchivableRecord)
	if !ok {
		return fmt.Errorf("record type does not support archival")
	}
	archivable.SetArchivedAt(&now)
	r.records[id] = rec
	return r.persist()
}

// Remove deletes a record from the registry and persists to disk.
func (r *FileBackedRegistry[T]) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.records[id]; !ok {
		return fmt.Errorf("record %s not found", id)
	}
	delete(r.records, id)
	return r.persist()
}

// persist writes the current state to disk atomically (write temp file then rename).
func (r *FileBackedRegistry[T]) persist() error {
	records := make([]T, 0, len(r.records))
	for _, rec := range r.records {
		records = append(records, rec)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	tmpPath := r.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write registry tmp: %w", err)
	}

	if err := os.Rename(tmpPath, r.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename registry: %w", err)
	}

	return nil
}

// ArchivableRecord is an interface for records that support archival.
type ArchivableRecord interface {
	SetArchivedAt(t *time.Time)
}
