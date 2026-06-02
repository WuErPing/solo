package schedule

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestStore_RecordRun_Success(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	agentID := "agent-run-1"
	output := "task completed"
	result := RunResult{
		AgentID: &agentID,
		Output:  &output,
		Error:   nil,
	}

	updated, err := store.RecordRun(sched.ID, result)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	if len(updated.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(updated.Runs))
	}

	run := updated.Runs[0]
	if run.Status != "succeeded" {
		t.Errorf("run status: got %q, want %q", run.Status, "succeeded")
	}
	if run.AgentID == nil || *run.AgentID != agentID {
		t.Errorf("run agentId mismatch")
	}
	if run.Output == nil || *run.Output != output {
		t.Errorf("run output mismatch")
	}
	if run.Error != nil {
		t.Errorf("expected no error, got %v", run.Error)
	}
	if run.StartedAt == "" || run.EndedAt == nil || *run.EndedAt == "" {
		t.Error("expected startedAt and endedAt to be set")
	}
	if updated.LastRunAt == nil {
		t.Error("expected LastRunAt to be set")
	}
	if updated.NextRunAt == nil {
		t.Error("expected NextRunAt to be recomputed")
	}
}

func TestStore_RecordRun_Failure(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	errMsg := "timeout exceeded"
	result := RunResult{
		Error: &errMsg,
	}

	updated, err := store.RecordRun(sched.ID, result)
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	run := updated.Runs[0]
	if run.Status != "failed" {
		t.Errorf("run status: got %q, want %q", run.Status, "failed")
	}
	if run.Error == nil || *run.Error != errMsg {
		t.Errorf("run error mismatch")
	}
}

func TestStore_RecordRun_NotFound(t *testing.T) {
	store := NewStore()
	_, err := store.RecordRun("nonexistent", RunResult{})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "schedule not found" {
		t.Errorf("got %q, want %q", err.Error(), "schedule not found")
	}
}

func TestStore_RecordRun_MaxRunsReached(t *testing.T) {
	store := NewStore()
	maxRuns := 1
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
		MaxRuns: &maxRuns,
	})

	updated, err := store.RecordRun(sched.ID, RunResult{})
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	if updated.Status != "completed" {
		t.Errorf("status: got %q, want %q", updated.Status, "completed")
	}
}

func TestStore_RecordRun_Persists(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := tmpDir + "/schedules.json"

	store1 := NewStore(WithDataPath(dataPath))
	sched, _ := store1.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	store1.RecordRun(sched.ID, RunResult{})

	store2 := NewStore(WithDataPath(dataPath))
	loaded, ok := store2.Get(sched.ID)
	if !ok {
		t.Fatal("schedule not found after reload")
	}
	if len(loaded.Runs) != 1 {
		t.Fatalf("expected 1 run after reload, got %d", len(loaded.Runs))
	}
	if loaded.LastRunAt == nil {
		t.Error("expected LastRunAt to persist")
	}
}

func TestStore_RecordRun_MultipleRuns(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	store.RecordRun(sched.ID, RunResult{})
	time.Sleep(time.Millisecond)
	store.RecordRun(sched.ID, RunResult{})

	updated, _ := store.Get(sched.ID)
	if len(updated.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(updated.Runs))
	}
}
