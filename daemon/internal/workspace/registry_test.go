package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testRecord struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	ArchivedAt *time.Time `json:"archivedAt,omitempty"`
}

func (r testRecord) GetID() string { return r.ID }

func (r *testRecord) SetArchivedAt(t *time.Time) { r.ArchivedAt = t }

func TestFileBackedRegistry_InitializeEmpty(t *testing.T) {
	dir := t.TempDir()
	r := NewFileBackedRegistry[*testRecord](filepath.Join(dir, "registry.json"))

	if err := r.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if len(r.List()) != 0 {
		t.Errorf("expected empty list, got %d", len(r.List()))
	}
}

func TestFileBackedRegistry_InitializeFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")
	data := []byte(`[{"id":"r1","name":"one"},{"id":"r2","name":"two"}]`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	r := NewFileBackedRegistry[*testRecord](path)
	if err := r.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 records, got %d", len(list))
	}

	rec, ok := r.Get("r1")
	if !ok || rec.Name != "one" {
		t.Errorf("expected record r1 with name one, got %+v", rec)
	}
}

func TestFileBackedRegistry_UpsertAndGet(t *testing.T) {
	dir := t.TempDir()
	r := NewFileBackedRegistry[*testRecord](filepath.Join(dir, "registry.json"))
	_ = r.Initialize()

	rec := &testRecord{ID: "r1", Name: "alpha"}
	if err := r.Upsert(rec); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, ok := r.Get("r1")
	if !ok || got.Name != "alpha" {
		t.Errorf("expected alpha, got %+v", got)
	}

	// Update existing
	rec.Name = "beta"
	if err := r.Upsert(rec); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	got, _ = r.Get("r1")
	if got.Name != "beta" {
		t.Errorf("expected beta after update, got %s", got.Name)
	}
}

func TestFileBackedRegistry_Remove(t *testing.T) {
	dir := t.TempDir()
	r := NewFileBackedRegistry[*testRecord](filepath.Join(dir, "registry.json"))
	_ = r.Initialize()

	_ = r.Upsert(&testRecord{ID: "r1", Name: "alpha"})
	if err := r.Remove("r1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, ok := r.Get("r1"); ok {
		t.Error("expected record to be removed")
	}

	if err := r.Remove("missing"); err == nil {
		t.Error("expected error removing missing record")
	}
}

func TestFileBackedRegistry_Archive(t *testing.T) {
	dir := t.TempDir()
	r := NewFileBackedRegistry[*testRecord](filepath.Join(dir, "registry.json"))
	_ = r.Initialize()

	now := time.Now()
	_ = r.Upsert(&testRecord{ID: "r1", Name: "alpha"})
	if err := r.Archive("r1", now); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	got, _ := r.Get("r1")
	if got.ArchivedAt == nil {
		t.Error("expected ArchivedAt to be set")
	}

	// Archive is idempotent: archiving a missing record succeeds silently.
	if err := r.Archive("missing", now); err != nil {
		t.Errorf("expected nil error archiving missing record, got: %v", err)
	}

	// Double-archive also succeeds (idempotent).
	if err := r.Archive("r1", now); err != nil {
		t.Errorf("expected nil error on double-archive, got: %v", err)
	}
}

func TestFileBackedRegistry_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")
	r1 := NewFileBackedRegistry[*testRecord](path)
	_ = r1.Initialize()
	_ = r1.Upsert(&testRecord{ID: "r1", Name: "persisted"})

	// Re-create registry from same path
	r2 := NewFileBackedRegistry[*testRecord](path)
	if err := r2.Initialize(); err != nil {
		t.Fatalf("Initialize second: %v", err)
	}

	got, ok := r2.Get("r1")
	if !ok || got.Name != "persisted" {
		t.Errorf("expected persisted record after reload, got %+v", got)
	}
}

func TestFileBackedRegistry_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	r := NewFileBackedRegistry[*testRecord](filepath.Join(dir, "registry.json"))
	_ = r.Initialize()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			id := string(rune('a' + n))
			_ = r.Upsert(&testRecord{ID: id, Name: "concurrent"})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if len(r.List()) != 10 {
		t.Errorf("expected 10 records after concurrent upserts, got %d", len(r.List()))
	}
}
