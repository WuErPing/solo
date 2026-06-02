package schedule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "schedules.json")

	store1 := NewStore(WithDataPath(dataPath))
	input := protocol.ScheduleCreateRequest{
		Prompt: "test prompt",
		Cadence: protocol.ScheduleCadence{
			Type:    "every",
			EveryMs: 3600000,
		},
		Target: protocol.ScheduleTarget{
			Type:    "agent",
			AgentID: "agent-123",
		},
	}

	sched, err := store1.Create(input)
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Fatal("expected data file to exist after create")
	}

	store2 := NewStore(WithDataPath(dataPath))

	loaded, ok := store2.Get(sched.ID)
	if !ok {
		t.Fatal("expected schedule to be loaded from file")
	}
	if loaded.Prompt != sched.Prompt {
		t.Errorf("prompt mismatch: got %q, want %q", loaded.Prompt, sched.Prompt)
	}
	if loaded.Status != sched.Status {
		t.Errorf("status mismatch: got %q, want %q", loaded.Status, sched.Status)
	}

	list := store2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 schedule in list, got %d", len(list))
	}

	paused, err := store2.Pause(sched.ID)
	if err != nil {
		t.Fatalf("pause schedule: %v", err)
	}
	if paused.Status != "paused" {
		t.Errorf("expected paused status, got %q", paused.Status)
	}

	store3 := NewStore(WithDataPath(dataPath))
	loaded3, ok := store3.Get(sched.ID)
	if !ok {
		t.Fatal("expected schedule after pause")
	}
	if loaded3.Status != "paused" {
		t.Errorf("pause not persisted: got %q, want %q", loaded3.Status, "paused")
	}

	resumed, err := store3.Resume(sched.ID)
	if err != nil {
		t.Fatalf("resume schedule: %v", err)
	}
	if resumed.Status != "active" {
		t.Errorf("expected active status, got %q", resumed.Status)
	}

	store4 := NewStore(WithDataPath(dataPath))
	loaded4, ok := store4.Get(sched.ID)
	if !ok {
		t.Fatal("expected schedule after resume")
	}
	if loaded4.Status != "active" {
		t.Errorf("resume not persisted: got %q, want %q", loaded4.Status, "active")
	}

	if err := store4.Delete(sched.ID); err != nil {
		t.Fatalf("delete schedule: %v", err)
	}

	store5 := NewStore(WithDataPath(dataPath))
	_, ok = store5.Get(sched.ID)
	if ok {
		t.Fatal("expected schedule to be deleted")
	}
	if len(store5.List()) != 0 {
		t.Fatalf("expected 0 schedules after delete, got %d", len(store5.List()))
	}
}

func TestStore_NoDataPath(t *testing.T) {
	store := NewStore()
	input := protocol.ScheduleCreateRequest{
		Prompt: "test prompt",
		Cadence: protocol.ScheduleCadence{
			Type:    "every",
			EveryMs: 3600000,
		},
		Target: protocol.ScheduleTarget{
			Type:    "agent",
			AgentID: "agent-123",
		},
	}

	sched, err := store.Create(input)
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	loaded, ok := store.Get(sched.ID)
	if !ok {
		t.Fatal("expected schedule in memory")
	}
	if loaded.Prompt != input.Prompt {
		t.Errorf("prompt mismatch: got %q, want %q", loaded.Prompt, input.Prompt)
	}
}

func TestStore_LoadCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "schedules.json")

	if err := os.WriteFile(dataPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	store := NewStore(WithDataPath(dataPath))
	if len(store.List()) != 0 {
		t.Fatalf("expected empty store after loading corrupted file, got %d", len(store.List()))
	}
}

func TestStore_LoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "nonexistent.json")

	store := NewStore(WithDataPath(dataPath))
	if len(store.List()) != 0 {
		t.Fatalf("expected empty store, got %d", len(store.List()))
	}
}

func TestStore_CreateValidation(t *testing.T) {
	store := NewStore()

	cases := []struct {
		name  string
		input protocol.ScheduleCreateRequest
		want  string
	}{
		{
			name:  "empty prompt",
			input: protocol.ScheduleCreateRequest{Prompt: ""},
			want:  "prompt is required",
		},
		{
			name: "invalid cadence type",
			input: protocol.ScheduleCreateRequest{
				Prompt:  "test",
				Cadence: protocol.ScheduleCadence{Type: "invalid"},
			},
			want: "invalid cadence type: invalid",
		},
		{
			name: "everyMs zero",
			input: protocol.ScheduleCreateRequest{
				Prompt:  "test",
				Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 0},
			},
			want: "everyMs must be positive",
		},
		{
			name: "empty cron expression",
			input: protocol.ScheduleCreateRequest{
				Prompt:  "test",
				Cadence: protocol.ScheduleCadence{Type: "cron", Expression: ""},
			},
			want: "cron expression is required",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := store.Create(c.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != c.want {
				t.Errorf("error message: got %q, want %q", err.Error(), c.want)
			}
		})
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := NewStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestStore_PauseNotFound(t *testing.T) {
	store := NewStore()
	_, err := store.Pause("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "schedule not found" {
		t.Errorf("got %q, want %q", err.Error(), "schedule not found")
	}
}

func TestStore_PauseAlreadyPaused(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt: "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target: protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})
	store.Pause(sched.ID)
	_, err := store.Pause(sched.ID)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "schedule already paused" {
		t.Errorf("got %q, want %q", err.Error(), "schedule already paused")
	}
}

func TestStore_ResumeNotFound(t *testing.T) {
	store := NewStore()
	_, err := store.Resume("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "schedule not found" {
		t.Errorf("got %q, want %q", err.Error(), "schedule not found")
	}
}

func TestStore_ResumeAlreadyActive(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt: "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target: protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})
	_, err := store.Resume(sched.ID)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "schedule already active" {
		t.Errorf("got %q, want %q", err.Error(), "schedule already active")
	}
}

func TestStore_DeleteNotFound(t *testing.T) {
	store := NewStore()
	err := store.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "schedule not found" {
		t.Errorf("got %q, want %q", err.Error(), "schedule not found")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt: "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target: protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			store.Get(sched.ID)
			store.List()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStore_CreateWithOptionalFields(t *testing.T) {
	store := NewStore()
	name := "my schedule"
	maxRuns := 5
	expiresAt := "2026-12-31T23:59:59Z"
	sched, err := store.Create(protocol.ScheduleCreateRequest{
		Prompt:    "test",
		Name:      name,
		Cadence:   protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:    protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
		MaxRuns:   &maxRuns,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sched.Name == nil || *sched.Name != name {
		t.Errorf("name mismatch")
	}
	if sched.MaxRuns == nil || *sched.MaxRuns != maxRuns {
		t.Errorf("maxRuns mismatch")
	}
	if sched.ExpiresAt == nil || *sched.ExpiresAt != expiresAt {
		t.Errorf("expiresAt mismatch")
	}
}

func TestStore_ListReturnsCopy(t *testing.T) {
	store := NewStore()
	store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	list1 := store.List()
	if len(list1) != 1 {
		t.Fatal("expected 1 schedule")
	}

	list1[0].Prompt = "modified"

	list2 := store.List()
	if list2[0].Prompt == "modified" {
		t.Fatal("List should return copies, not references")
	}
}

func TestStore_GetReturnsCopy(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	loaded1, _ := store.Get(sched.ID)
	loaded1.Prompt = "modified"
	loaded1.Runs = append(loaded1.Runs, protocol.ScheduleRun{ID: "run-1"})

	loaded2, _ := store.Get(sched.ID)
	if loaded2.Prompt == "modified" {
		t.Error("Get should return copy of Prompt")
	}
	if len(loaded2.Runs) != 0 {
		t.Error("Get should return copy of Runs")
	}
}

func TestStore_FileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "schedules.json")

	store := NewStore(WithDataPath(dataPath))
	store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test prompt",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	b, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(data))
	}
}

func TestStore_MultipleSchedules(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "schedules.json")

	store1 := NewStore(WithDataPath(dataPath))
	for i := 0; i < 3; i++ {
		store1.Create(protocol.ScheduleCreateRequest{
			Prompt:  fmt.Sprintf("prompt %d", i),
			Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000 * (i + 1)},
			Target:  protocol.ScheduleTarget{Type: "agent", AgentID: fmt.Sprintf("agent-%d", i)},
		})
	}

	store2 := NewStore(WithDataPath(dataPath))
	list := store2.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 schedules, got %d", len(list))
	}
}

func TestStore_ResumeRecomputesNextRun(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	store.Pause(sched.ID)
	before := time.Now()
	resumed, _ := store.Resume(sched.ID)
	after := time.Now()

	if resumed.NextRunAt == nil {
		t.Fatal("expected nextRunAt to be set")
	}

	nextRun, _ := time.Parse(time.RFC3339, *resumed.NextRunAt)
	expectedMin := before.Add(59 * time.Minute)
	expectedMax := after.Add(61 * time.Minute)
	if nextRun.Before(expectedMin) || nextRun.After(expectedMax) {
		t.Errorf("nextRunAt out of range: %v", nextRun)
	}
}

func TestStore_ConcurrentCreate(t *testing.T) {
	store := NewStore()
	done := make(chan string, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			sched, err := store.Create(protocol.ScheduleCreateRequest{
				Prompt:  fmt.Sprintf("prompt %d", n),
				Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
				Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
			})
			if err != nil {
				done <- ""
				return
			}
			done <- sched.ID
		}(i)
	}

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id := <-done
		if id == "" {
			t.Fatal("create failed")
		}
		if ids[id] {
			t.Fatal("duplicate ID generated")
		}
		ids[id] = true
	}

	if len(store.List()) != 10 {
		t.Fatalf("expected 10 schedules, got %d", len(store.List()))
	}
}
