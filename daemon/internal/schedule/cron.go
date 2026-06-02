package schedule

import (
	"time"

	"github.com/WuErPing/solo/protocol"
	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func NextRunAt(cadence protocol.ScheduleCadence, now time.Time) *time.Time {
	switch cadence.Type {
	case "every":
		if cadence.EveryMs <= 0 {
			return nil
		}
		next := now.Add(time.Duration(cadence.EveryMs) * time.Millisecond)
		return &next
	case "cron":
		sched, err := cronParser.Parse(cadence.Expression)
		if err != nil {
			return nil
		}
		next := sched.Next(now)
		if next.IsZero() {
			return nil
		}
		return &next
	default:
		return nil
	}
}
