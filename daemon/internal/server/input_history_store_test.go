package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestInputHistoryStoreRecord(t *testing.T) {
	dir := t.TempDir()
	store := NewInputHistoryStore(dir)

	store.Record("continue")
	store.Record("fix the lint error")
	store.Record("continue")

	entries := store.Entries()
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// Sorted by count descending.
	if entries[0].Text != "continue" || entries[0].Count != 2 {
		t.Errorf("entries[0] = %+v, want text=%q count=2", entries[0], "continue")
	}
	if entries[1].Text != "fix the lint error" || entries[1].Count != 1 {
		t.Errorf("entries[1] = %+v, want text=%q count=1", entries[1], "fix the lint error")
	}
	if entries[0].LastUsed == "" {
		t.Error("entries[0].LastUsed should be set")
	}

	// Verify persistence: load from disk.
	store2 := NewInputHistoryStore(dir)
	entries2 := store2.Entries()
	if len(entries2) != 2 || entries2[0].Text != "continue" || entries2[0].Count != 2 {
		t.Fatalf("reloaded: got %+v, want continue with count=2 first", entries2)
	}
}

func TestInputHistoryStoreEmptyText(t *testing.T) {
	dir := t.TempDir()
	store := NewInputHistoryStore(dir)

	store.Record("")
	if entries := store.Entries(); len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestInputHistoryStoreNoFile(t *testing.T) {
	dir := t.TempDir()
	store := NewInputHistoryStore(dir)
	if entries := store.Entries(); len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestInputHistoryStoreCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux-input-history.json")
	os.WriteFile(path, []byte("not json"), 0644)

	store := NewInputHistoryStore(dir)
	if entries := store.Entries(); len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestInputHistoryStoreEviction(t *testing.T) {
	dir := t.TempDir()
	store := NewInputHistoryStore(dir)

	// One frequent entry plus enough one-off entries to exceed the cap.
	store.Record("frequent")
	for i := 0; i < 5; i++ {
		store.Record("frequent")
	}
	for i := 0; i < maxInputHistoryEntries; i++ {
		store.Record(fmt.Sprintf("one-off-%d", i))
	}

	entries := store.Entries()
	if len(entries) != maxInputHistoryEntries {
		t.Fatalf("got %d entries, want capped at %d", len(entries), maxInputHistoryEntries)
	}
	if entries[0].Text != "frequent" {
		t.Errorf("entries[0].Text = %q, want %q (highest count survives eviction)", entries[0].Text, "frequent")
	}
}
