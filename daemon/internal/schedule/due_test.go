package schedule

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func makeSchedule(store *Store, cadence protocol.ScheduleCadence) *protocol.StoredSchedule {
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: cadence,
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})
	return sched
}

func TestDueSchedules_ReturnsActiveSchedulesPastNextRun(t *testing.T) {
	store := NewStore()
	sched := makeSchedule(store, protocol.ScheduleCadence{Type: "every", EveryMs: 1})

	time.Sleep(5 * time.Millisecond)

	due := store.DueSchedules(time.Now().UTC())
	if len(due) != 1 {
		t.Fatalf("expected 1 due schedule, got %d", len(due))
	}
	if due[0].ID != sched.ID {
		t.Errorf("due schedule ID mismatch: got %q, want %q", due[0].ID, sched.ID)
	}
}

func TestDueSchedules_ExcludesFutureSchedules(t *testing.T) {
	store := NewStore()
	makeSchedule(store, protocol.ScheduleCadence{Type: "every", EveryMs: 3600000})

	due := store.DueSchedules(time.Now().UTC())
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules, got %d", len(due))
	}
}

func TestDueSchedules_ExcludesPausedSchedules(t *testing.T) {
	store := NewStore()
	sched := makeSchedule(store, protocol.ScheduleCadence{Type: "every", EveryMs: 1})
	time.Sleep(5 * time.Millisecond)
	store.Pause(sched.ID)

	due := store.DueSchedules(time.Now().UTC())
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules (paused), got %d", len(due))
	}
}

func TestDueSchedules_ExcludesCompletedSchedules(t *testing.T) {
	store := NewStore()
	maxRuns := 1
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 1},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
		MaxRuns: &maxRuns,
	})
	time.Sleep(5 * time.Millisecond)
	store.RecordRun(sched.ID, RunResult{})

	due := store.DueSchedules(time.Now().UTC())
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules (completed), got %d", len(due))
	}
}

func TestDueSchedules_ExcludesNilNextRunAt(t *testing.T) {
	store := NewStore()
	sched := makeSchedule(store, protocol.ScheduleCadence{Type: "every", EveryMs: 1})
	time.Sleep(5 * time.Millisecond)

	store.mu.Lock()
	store.schedules[sched.ID].NextRunAt = nil
	store.mu.Unlock()

	due := store.DueSchedules(time.Now().UTC())
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules (nil nextRunAt), got %d", len(due))
	}
}

func TestDueSchedules_ReturnsCopy(t *testing.T) {
	store := NewStore()
	makeSchedule(store, protocol.ScheduleCadence{Type: "every", EveryMs: 1})
	time.Sleep(5 * time.Millisecond)

	due := store.DueSchedules(time.Now().UTC())
	due[0].Prompt = "modified"

	original, _ := store.Get(due[0].ID)
	if original.Prompt == "modified" {
		t.Error("DueSchedules should return copies, not references")
	}
}

func TestDueSchedules_MultipleDue(t *testing.T) {
	store := NewStore()
	makeSchedule(store, protocol.ScheduleCadence{Type: "every", EveryMs: 1})
	makeSchedule(store, protocol.ScheduleCadence{Type: "every", EveryMs: 1})

	time.Sleep(5 * time.Millisecond)

	due := store.DueSchedules(time.Now().UTC())
	if len(due) != 2 {
		t.Fatalf("expected 2 due schedules, got %d", len(due))
	}
}
