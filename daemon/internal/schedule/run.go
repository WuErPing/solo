package schedule

import (
	"fmt"
	"time"

	"github.com/WuErPing/solo/protocol"
)

type RunResult struct {
	AgentID *string
	Output  *string
	Error   *string
}

func (st *Store) RecordRun(scheduleID string, result RunResult) (*protocol.StoredSchedule, error) {
	st.mu.Lock()

	s, ok := st.schedules[scheduleID]
	if !ok {
		st.mu.Unlock()
		return nil, fmt.Errorf("schedule not found")
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	status := "succeeded"
	if result.Error != nil {
		status = "failed"
	}

	scheduledFor := nowStr
	if s.NextRunAt != nil && *s.NextRunAt != "" {
		scheduledFor = *s.NextRunAt
	}

	run := protocol.ScheduleRun{
		ID:           generateID(),
		ScheduledFor: scheduledFor,
		StartedAt:    nowStr,
		EndedAt:      &nowStr,
		Status:       status,
		AgentID:      result.AgentID,
		Output:       result.Output,
		Error:        result.Error,
	}

	s.Runs = append(s.Runs, run)
	s.LastRunAt = &nowStr
	s.UpdatedAt = nowStr

	next := NextRunAt(s.Cadence, now)
	if next != nil {
		nextStr := next.Format(time.RFC3339)
		s.NextRunAt = &nextStr
	} else {
		s.NextRunAt = nil
	}

	if s.MaxRuns != nil && len(s.Runs) >= *s.MaxRuns {
		s.Status = "completed"
	}

	st.mu.Unlock()

	if err := st.save(); err != nil {
		return nil, err
	}

	loaded, _ := st.Get(scheduleID)
	return loaded, nil
}
