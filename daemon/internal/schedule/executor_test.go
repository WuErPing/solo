package schedule

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

type fakeRunner struct {
	mu      sync.Mutex
	calls   []protocol.StoredSchedule
	results map[string]RunResult
}

func (r *fakeRunner) Run(sched protocol.StoredSchedule) RunResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, sched)
	if result, ok := r.results[sched.ID]; ok {
		return result
	}
	return RunResult{}
}

func (r *fakeRunner) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func TestExecutor_RunsDueSchedules(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test prompt",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 1},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	time.Sleep(5 * time.Millisecond)

	runner := &fakeRunner{}
	executor := NewExecutor(store, runner, 10*time.Millisecond, testLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	executor.Start(ctx)
	<-ctx.Done()
	executor.Wait()

	if runner.callCount() == 0 {
		t.Fatal("expected runner to be called at least once")
	}

	updated, ok := store.Get(sched.ID)
	if !ok {
		t.Fatal("schedule not found")
	}
	if len(updated.Runs) == 0 {
		t.Fatal("expected at least 1 run recorded")
	}
	if updated.Runs[0].Status != "succeeded" {
		t.Errorf("run status: got %q, want %q", updated.Runs[0].Status, "succeeded")
	}
}

func TestExecutor_RecordsFailure(t *testing.T) {
	store := NewStore()
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test prompt",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 1},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	time.Sleep(5 * time.Millisecond)

	errMsg := "agent crashed"
	runner := &fakeRunner{
		results: map[string]RunResult{
			sched.ID: {Error: &errMsg},
		},
	}
	executor := NewExecutor(store, runner, 10*time.Millisecond, testLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	executor.Start(ctx)
	<-ctx.Done()
	executor.Wait()

	updated, _ := store.Get(sched.ID)
	if len(updated.Runs) == 0 {
		t.Fatal("expected run to be recorded")
	}
	if updated.Runs[0].Status != "failed" {
		t.Errorf("run status: got %q, want %q", updated.Runs[0].Status, "failed")
	}
	if updated.Runs[0].Error == nil || *updated.Runs[0].Error != errMsg {
		t.Errorf("run error mismatch")
	}
}

func TestExecutor_StopsOnContextCancel(_ *testing.T) {
	store := NewStore()
	runner := &fakeRunner{}
	executor := NewExecutor(store, runner, 10*time.Millisecond, testLogger)

	ctx, cancel := context.WithCancel(context.Background())
	executor.Start(ctx)

	time.Sleep(30 * time.Millisecond)
	cancel()
	executor.Wait()
}

func TestExecutor_SkipsNoDueSchedules(t *testing.T) {
	store := NewStore()
	store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})

	runner := &fakeRunner{}
	executor := NewExecutor(store, runner, 10*time.Millisecond, testLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	executor.Start(ctx)
	<-ctx.Done()
	executor.Wait()

	if runner.callCount() != 0 {
		t.Errorf("expected runner not to be called, got %d calls", runner.callCount())
	}
}

func TestExecutor_RespectsMaxRuns(t *testing.T) {
	store := NewStore()
	maxRuns := 1
	sched, _ := store.Create(protocol.ScheduleCreateRequest{
		Prompt:  "test",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 1},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
		MaxRuns: &maxRuns,
	})

	time.Sleep(5 * time.Millisecond)

	runner := &fakeRunner{}
	executor := NewExecutor(store, runner, 10*time.Millisecond, testLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	executor.Start(ctx)
	<-ctx.Done()
	executor.Wait()

	updated, _ := store.Get(sched.ID)
	if updated.Status != "completed" {
		t.Errorf("status: got %q, want %q", updated.Status, "completed")
	}
	if len(updated.Runs) != 1 {
		t.Errorf("expected exactly 1 run, got %d", len(updated.Runs))
	}
}
