package loop

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestStore_Create(t *testing.T) {
	t.Parallel()

	s := NewStore()
	name := "test-loop"
	prompt := "do something"
	cwd := "/tmp"
	provider := "mock"
	model := "test-model"
	archive := true
	sleepMs := 500
	maxIterations := 5

	req := protocol.LoopRunRequest{
		Prompt:        prompt,
		Cwd:           cwd,
		Provider:      &provider,
		Model:         &model,
		Archive:       &archive,
		Name:          &name,
		SleepMs:       &sleepMs,
		MaxIterations: &maxIterations,
	}

	record, err := s.Create(req, func() (string, error) { return "fallback", nil })
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if record.ID == "" {
		t.Error("expected non-empty ID")
	}
	if record.Prompt != prompt {
		t.Errorf("prompt = %q, want %q", record.Prompt, prompt)
	}
	if record.Provider != provider {
		t.Errorf("provider = %q, want %q", record.Provider, provider)
	}
	if record.Model == nil || *record.Model != model {
		t.Errorf("model = %v, want %q", record.Model, model)
	}
	if record.Archive != archive {
		t.Errorf("archive = %v, want %v", record.Archive, archive)
	}
	if record.SleepMs != sleepMs {
		t.Errorf("sleepMs = %d, want %d", record.SleepMs, sleepMs)
	}
	if record.MaxIterations == nil || *record.MaxIterations != maxIterations {
		t.Errorf("maxIterations = %v, want %d", record.MaxIterations, maxIterations)
	}
	if record.Status != string(StatusRunning) {
		t.Errorf("status = %q, want %q", record.Status, StatusRunning)
	}
	if len(record.Logs) != 1 || record.Logs[0].Text != "Loop started" {
		t.Errorf("expected initial log entry, got %v", record.Logs)
	}
	if record.NextLogSeq != 2 {
		t.Errorf("nextLogSeq = %d, want 2", record.NextLogSeq)
	}
}

func TestStore_Create_Defaults(t *testing.T) {
	t.Parallel()

	s := NewStore()
	req := protocol.LoopRunRequest{
		Prompt: "do something",
	}

	record, err := s.Create(req, func() (string, error) { return "mock", nil })
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if record.Provider != "mock" {
		t.Errorf("provider = %q, want mock", record.Provider)
	}
	if record.MaxIterations == nil || *record.MaxIterations != 10 {
		t.Errorf("maxIterations = %v, want 10", record.MaxIterations)
	}
	if record.SleepMs != 1000 {
		t.Errorf("sleepMs = %d, want 1000", record.SleepMs)
	}
}

func TestStore_Create_UsesDefaultProvider(t *testing.T) {
	t.Parallel()

	s := NewStore()
	req := protocol.LoopRunRequest{Prompt: "do something"}

	called := false
	record, err := s.Create(req, func() (string, error) {
		called = true
		return "fallback", nil
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !called {
		t.Error("expected defaultProvider to be called")
	}
	if record.Provider != "fallback" {
		t.Errorf("provider = %q, want fallback", record.Provider)
	}
}

func TestStore_Create_Validation(t *testing.T) {
	t.Parallel()

	s := NewStore()
	_, err := s.Create(protocol.LoopRunRequest{}, func() (string, error) { return "mock", nil })
	if err == nil {
		t.Error("expected error for empty prompt")
	}
}

func TestStore_List(t *testing.T) {
	t.Parallel()

	s := NewStore()
	first, _ := s.Create(protocol.LoopRunRequest{Prompt: "first"}, func() (string, error) { return "mock", nil })
	second, _ := s.Create(protocol.LoopRunRequest{Prompt: "second"}, func() (string, error) { return "mock", nil })

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}

	ids := map[string]bool{first.ID: true, second.ID: true}
	for _, item := range list {
		if !ids[item.ID] {
			t.Errorf("unexpected item id %s", item.ID)
		}
		delete(ids, item.ID)
	}
	if len(ids) != 0 {
		t.Errorf("missing items: %v", ids)
	}
}

func TestStore_Get(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "get me"}, func() (string, error) { return "mock", nil })

	got, ok := s.Get(created.ID)
	if !ok {
		t.Fatal("expected record to be found")
	}
	if got.ID != created.ID {
		t.Errorf("id = %q, want %q", got.ID, created.ID)
	}

	// Mutating the returned copy should not affect the store.
	got.Prompt = "mutated"
	fresh, _ := s.Get(created.ID)
	if fresh.Prompt != "get me" {
		t.Error("store record was mutated through returned copy")
	}

	_, ok = s.Get("missing")
	if ok {
		t.Error("expected missing record to not be found")
	}
}

func TestStore_Update(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "update me"}, func() (string, error) { return "mock", nil })

	newName := "updated-name"
	archive := true
	updated, err := s.Update(created.ID, UpdateInput{Name: &newName, Archive: &archive})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name == nil || *updated.Name != newName {
		t.Errorf("name = %v, want %q", updated.Name, newName)
	}
	if !updated.Archive {
		t.Error("expected archive to be true")
	}

	_, err = s.Update("missing", UpdateInput{Name: &newName})
	if err == nil {
		t.Error("expected error for missing loop")
	}
}

func TestStore_Delete(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "delete me"}, func() (string, error) { return "mock", nil })

	err := s.Delete(created.ID)
	if err == nil {
		t.Error("expected error deleting running loop")
	}

	_, err = s.Stop(created.ID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	err = s.Delete(created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, ok := s.Get(created.ID)
	if ok {
		t.Error("expected record to be deleted")
	}

	err = s.Delete("missing")
	if err == nil {
		t.Error("expected error for missing loop")
	}
}

func TestStore_Stop(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "stop me"}, func() (string, error) { return "mock", nil })

	stopped, err := s.Stop(created.ID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if stopped.Status != string(StatusStopped) {
		t.Errorf("status = %q, want %q", stopped.Status, StatusStopped)
	}
	if stopped.StopRequestedAt == nil {
		t.Error("expected StopRequestedAt to be set")
	}

	_, err = s.Stop(created.ID)
	if err == nil {
		t.Error("expected error stopping non-running loop")
	}

	_, err = s.Stop("missing")
	if err == nil {
		t.Error("expected error for missing loop")
	}
}

func TestStore_AppendLog(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "log me"}, func() (string, error) { return "mock", nil })

	iter := 1
	entry := protocol.LoopLogEntry{
		Timestamp: nowISO(),
		Iteration: &iter,
		Source:    "loop",
		Level:     "info",
		Text:      "hello",
	}
	if err := s.AppendLog(created.ID, entry); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	got, _ := s.Get(created.ID)
	if len(got.Logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(got.Logs))
	}
	if got.Logs[1].Text != "hello" {
		t.Errorf("log text = %q, want hello", got.Logs[1].Text)
	}
	if got.Logs[1].Seq != 2 {
		t.Errorf("log seq = %d, want 2", got.Logs[1].Seq)
	}
}

func TestStore_AppendIteration(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "iterate me"}, func() (string, error) { return "mock", nil })

	agentID := "agent-1"
	iter := protocol.LoopIterationRecord{
		Index:         1,
		WorkerAgentID: &agentID,
		Status:        string(StatusRunning),
	}
	if err := s.AppendIteration(created.ID, iter); err != nil {
		t.Fatalf("AppendIteration failed: %v", err)
	}

	got, _ := s.Get(created.ID)
	if len(got.Iterations) != 1 {
		t.Fatalf("expected 1 iteration, got %d", len(got.Iterations))
	}
	if got.ActiveIteration == nil || *got.ActiveIteration != 1 {
		t.Errorf("activeIteration = %v, want 1", got.ActiveIteration)
	}
	if got.ActiveWorkerAgentID == nil || *got.ActiveWorkerAgentID != agentID {
		t.Errorf("activeWorkerAgentID = %v, want %q", got.ActiveWorkerAgentID, agentID)
	}
}

func TestStore_SetStatus(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "finish me"}, func() (string, error) { return "mock", nil })

	agentID := "agent-1"
	iter := protocol.LoopIterationRecord{
		Index:         1,
		WorkerAgentID: &agentID,
		Status:        string(StatusRunning),
	}
	_ = s.AppendIteration(created.ID, iter)

	reason := "all good"
	updated, err := s.SetStatus(created.ID, StatusSucceeded, &reason)
	if err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}
	if updated.Status != string(StatusSucceeded) {
		t.Errorf("status = %q, want %q", updated.Status, StatusSucceeded)
	}
	if updated.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if updated.ActiveIteration != nil {
		t.Error("expected ActiveIteration to be cleared")
	}
	if updated.ActiveWorkerAgentID != nil {
		t.Error("expected ActiveWorkerAgentID to be cleared")
	}
}

func TestStore_Persistence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "loops.json")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	s := NewStore(WithDataPath(path), WithLogger(logger))
	created, _ := s.Create(protocol.LoopRunRequest{Prompt: "persist me"}, func() (string, error) { return "mock", nil })

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persistence file to exist: %v", err)
	}

	s2 := NewStore(WithDataPath(path), WithLogger(logger))
	got, ok := s2.Get(created.ID)
	if !ok {
		t.Fatal("expected record to be loaded from disk")
	}
	if got.Prompt != "persist me" {
		t.Errorf("prompt = %q, want persist me", got.Prompt)
	}
}
