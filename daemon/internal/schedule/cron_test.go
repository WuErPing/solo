package schedule

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestNextRunAt_Cron(t *testing.T) {
	cases := []struct {
		name       string
		expression string
		now        time.Time
		wantAfter  time.Time
		wantBefore time.Time
	}{
		{
			name:       "every day at 9am",
			expression: "0 9 * * *",
			now:        time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 2, 8, 59, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 2, 9, 0, 1, 0, time.UTC),
		},
		{
			name:       "every day at 9am after 9am wraps to next day",
			expression: "0 9 * * *",
			now:        time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 3, 8, 59, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 3, 9, 0, 1, 0, time.UTC),
		},
		{
			name:       "every 5 minutes",
			expression: "*/5 * * * *",
			now:        time.Date(2026, 6, 2, 12, 3, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 2, 12, 4, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 2, 12, 5, 1, 0, time.UTC),
		},
		{
			name:       "specific day of month",
			expression: "0 0 15 * *",
			now:        time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 14, 23, 59, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 15, 0, 0, 1, 0, time.UTC),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cadence := protocol.ScheduleCadence{
				Type:       "cron",
				Expression: c.expression,
			}
			got := NextRunAt(cadence, c.now)
			if got == nil {
				t.Fatal("expected non-nil next run time")
			}
			if got.Before(c.wantAfter) || got.After(c.wantBefore) {
				t.Errorf("next run %v not in expected range [%v, %v]", got, c.wantAfter, c.wantBefore)
			}
		})
	}
}

func TestNextRunAt_Every(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	cadence := protocol.ScheduleCadence{
		Type:    "every",
		EveryMs: 3600000,
	}

	got := NextRunAt(cadence, now)
	if got == nil {
		t.Fatal("expected non-nil next run time")
	}

	expected := now.Add(1 * time.Hour)
	if !got.Equal(expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestNextRunAt_EverySmallInterval(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	cadence := protocol.ScheduleCadence{
		Type:    "every",
		EveryMs: 60000,
	}

	got := NextRunAt(cadence, now)
	if got == nil {
		t.Fatal("expected non-nil next run time")
	}

	expected := now.Add(1 * time.Minute)
	if !got.Equal(expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestNextRunAt_InvalidCronExpression(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	cadence := protocol.ScheduleCadence{
		Type:       "cron",
		Expression: "not a cron expression",
	}

	got := NextRunAt(cadence, now)
	if got != nil {
		t.Errorf("expected nil for invalid cron, got %v", got)
	}
}

func TestNextRunAt_UnknownCadenceType(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	cadence := protocol.ScheduleCadence{
		Type: "unknown",
	}

	got := NextRunAt(cadence, now)
	if got != nil {
		t.Errorf("expected nil for unknown cadence type, got %v", got)
	}
}

func TestNextRunAt_CronWithTimezone(t *testing.T) {
	// The expression is already in UTC (frontend converts local→UTC on save).
	// NextRunAt evaluates it in UTC regardless of the timezone field.
	// The timezone field is validated but does NOT shift the cron evaluation.
	cases := []struct {
		name       string
		expression string
		timezone   string
		now        time.Time
		wantAfter  time.Time
		wantBefore time.Time
	}{
		{
			name:       "UTC expression evaluated as-is, not shifted by timezone",
			expression: "0 9 * * *",
			timezone:   "Asia/Shanghai",
			// now = 08:00 UTC, next run = 09:00 UTC (timezone field does NOT shift)
			now:        time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 2, 8, 59, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 2, 9, 0, 1, 0, time.UTC),
		},
		{
			name:       "after 9am wraps to next day",
			expression: "0 9 * * *",
			timezone:   "Asia/Shanghai",
			now:        time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 3, 8, 59, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 3, 9, 0, 1, 0, time.UTC),
		},
		{
			name:       "empty timezone defaults to UTC",
			expression: "0 9 * * *",
			timezone:   "",
			now:        time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 2, 8, 59, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 2, 9, 0, 1, 0, time.UTC),
		},
		{
			name:       "16:43 UTC for 00:43 Shanghai schedule",
			expression: "43 16 * * *",
			timezone:   "Asia/Shanghai",
			now:        time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
			wantAfter:  time.Date(2026, 6, 3, 16, 42, 59, 0, time.UTC),
			wantBefore: time.Date(2026, 6, 3, 16, 43, 1, 0, time.UTC),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cadence := protocol.ScheduleCadence{
				Type:       "cron",
				Expression: c.expression,
				Timezone:   c.timezone,
			}
			got := NextRunAt(cadence, c.now)
			if got == nil {
				t.Fatal("expected non-nil next run time")
			}
			if got.Before(c.wantAfter) || got.After(c.wantBefore) {
				t.Errorf("next run %v not in expected range [%v, %v]", got, c.wantAfter, c.wantBefore)
			}
			// Verify result is in UTC
			if got.Location() != time.UTC {
				t.Errorf("expected UTC location, got %v", got.Location())
			}
		})
	}

}

func TestNextRunAt_CronWithInvalidTimezone(t *testing.T) {
	now := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)

	cadence := protocol.ScheduleCadence{
		Type:       "cron",
		Expression: "0 9 * * *",
		Timezone:   "Invalid/Timezone",
	}

	got := NextRunAt(cadence, now)
	if got != nil {
		t.Errorf("expected nil for invalid timezone, got %v", got)
	}
}

func TestNextRunAt_EveryIgnoresTimezone(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	cadence := protocol.ScheduleCadence{
		Type:     "every",
		EveryMs:  3600000,
		Timezone: "Asia/Shanghai",
	}

	got := NextRunAt(cadence, now)
	if got == nil {
		t.Fatal("expected non-nil next run time")
	}

	expected := now.Add(1 * time.Hour)
	if !got.Equal(expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}
