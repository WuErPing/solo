// Package schedule persists and executes scheduled prompts.
package schedule

import (
	"time"

	"github.com/robfig/cron/v3"

	"github.com/WuErPing/solo/protocol"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func loadTimezone(tz string) (*time.Location, error) {
	if tz == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(tz)
}

func NextRunAt(cadence protocol.ScheduleCadence, now time.Time) *time.Time {
	switch cadence.Type {
	case "every":
		if cadence.EveryMs <= 0 {
			return nil
		}
		next := now.Add(time.Duration(cadence.EveryMs) * time.Millisecond)
		return &next
	case "cron":
		// The expression is already in UTC (frontend converts local→UTC on save).
		// Evaluate it in UTC so NextRunAt matches what the user expects.
		if _, err := loadTimezone(cadence.Timezone); err != nil {
			return nil
		}
		sched, err := cronParser.Parse(cadence.Expression)
		if err != nil {
			return nil
		}
		// Force UTC evaluation — do NOT set ss.Location to the user's timezone,
		// because the expression is already UTC.
		if ss, ok := sched.(*cron.SpecSchedule); ok {
			ss.Location = time.UTC
		}
		next := sched.Next(now)
		if next.IsZero() {
			return nil
		}
		utc := next.UTC()
		return &utc
	default:
		return nil
	}
}
