package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentCommandStoreMerge(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentCommandStore(dir)

	// First merge: add two entries.
	store.Merge([]AgentCommandEntry{
		{AgentName: "claude", LaunchCmd: "claude"},
		{AgentName: "qodercli", LaunchCmd: "qodercli --permission-mode=bypass_permissions"},
	})
	entries := store.Entries()
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].AgentName != "claude" {
		t.Errorf("entries[0].AgentName = %q, want %q", entries[0].AgentName, "claude")
	}
	if entries[0].LastSeen == "" {
		t.Error("entries[0].LastSeen should be set")
	}

	// Second merge: duplicate + new entry.
	store.Merge([]AgentCommandEntry{
		{AgentName: "claude", LaunchCmd: "claude"},               // duplicate
		{AgentName: "claude", LaunchCmd: "claude --dangerously-skip-permission"}, // new
	})
	entries = store.Entries()
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	// Verify persistence: load from disk.
	store2 := NewAgentCommandStore(dir)
	entries2 := store2.Entries()
	if len(entries2) != 3 {
		t.Fatalf("reloaded: got %d entries, want 3", len(entries2))
	}
}

func TestAgentCommandStoreEmptyLaunchCmd(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentCommandStore(dir)

	// Empty launchCmd should be skipped.
	store.Merge([]AgentCommandEntry{
		{AgentName: "pi", LaunchCmd: ""},
		{AgentName: "kimi", LaunchCmd: "kimi --yolo"},
	})
	entries := store.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].LaunchCmd != "kimi --yolo" {
		t.Errorf("entries[0].LaunchCmd = %q, want %q", entries[0].LaunchCmd, "kimi --yolo")
	}
}

func TestAgentCommandStoreNoFile(t *testing.T) {
	dir := t.TempDir()
	store := NewAgentCommandStore(dir)
	entries := store.Entries()
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestAgentCommandStoreCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-commands.json")
	os.WriteFile(path, []byte("not json"), 0644)

	store := NewAgentCommandStore(dir)
	entries := store.Entries()
	if len(entries) != 0 {
		t.Fatalf("corrupt file: got %d entries, want 0", len(entries))
	}
}
