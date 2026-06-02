package schedule

import (
	"time"

	"github.com/WuErPing/solo/protocol"
)

func (st *Store) DueSchedules(now time.Time) []protocol.StoredSchedule {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var result []protocol.StoredSchedule
	for _, s := range st.schedules {
		if s.Status != "active" {
			continue
		}
		if s.NextRunAt == nil {
			continue
		}
		nextRun, err := time.Parse(time.RFC3339, *s.NextRunAt)
		if err != nil {
			continue
		}
		if !nextRun.After(now) {
			schedCopy := *s
			schedCopy.Runs = make([]protocol.ScheduleRun, len(s.Runs))
			copy(schedCopy.Runs, s.Runs)
			result = append(result, schedCopy)
		}
	}
	return result
}
