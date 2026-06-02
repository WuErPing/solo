package schedule

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

type Runner interface {
	Run(sched protocol.StoredSchedule) RunResult
}

type Executor struct {
	store    *Store
	runner   Runner
	interval time.Duration
	logger   *slog.Logger
	wg       sync.WaitGroup
}

func NewExecutor(store *Store, runner Runner, interval time.Duration, logger *slog.Logger) *Executor {
	return &Executor{
		store:    store,
		runner:   runner,
		interval: interval,
		logger:   logger.With("component", "schedule-executor"),
	}
}

func (e *Executor) Start(ctx context.Context) {
	e.logger.Info("schedule executor starting", "interval", e.interval)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				e.logger.Info("schedule executor stopping")
				return
			case <-ticker.C:
				e.tick()
			}
		}
	}()
}

func (e *Executor) Wait() {
	e.wg.Wait()
}

func (e *Executor) tick() {
	now := time.Now().UTC()
	due := e.store.DueSchedules(now)
	if len(due) > 0 {
		e.logger.Info("found due schedules", "count", len(due), "now", now)
	}
	for _, sched := range due {
		e.logger.Info("running schedule", "id", sched.ID, "name", sched.Name, "prompt", sched.Prompt)
		result := e.runner.Run(sched)
		e.logger.Info("schedule run complete", "id", sched.ID, "error", result.Error, "hasOutput", result.Output != nil)
		if _, err := e.store.RecordRun(sched.ID, result); err != nil {
			e.logger.Error("failed to record run", "id", sched.ID, "error", err)
		}
	}
}
