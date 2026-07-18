package schedule

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

var assistTestNow = time.Date(2026, 7, 17, 22, 50, 0, 0, time.UTC)

func createAssistTestSchedule(t *testing.T, store *Store, name string) string {
	t.Helper()
	sched, err := store.Create(protocol.ScheduleCreateRequest{
		Name:    name,
		Prompt:  "test prompt",
		Cadence: protocol.ScheduleCadence{Type: "every", EveryMs: 3600000},
		Target:  protocol.ScheduleTarget{Type: "agent", AgentID: "a"},
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	return sched.ID
}

const validCreateJSON = `{"kind":"proposal","op":"create","name":"Nightly test summary","prompt":"Summarize the nightly test runs","cadence":{"type":"cron","expression":"0 9 * * 1-5"},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/work"}},"summary":"Create nightly summary"}`

func TestParseAssistIntent(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		setupStore func(t *testing.T) *Store
		agents     []AgentInfo
		wantErr    string // substring of error; empty means success
		wantRefErr bool   // error must be an *assistRefError
		wantCands  []string
		check      func(t *testing.T, intent *assistIntent)
	}{
		{
			name:    "fenced json",
			raw:     "```json\n{\"kind\":\"clarify\",\"message\":\"which schedule do you mean?\"}\n```",
			wantErr: "",
			check: func(t *testing.T, intent *assistIntent) {
				if intent.Kind != "clarify" || intent.Message != "which schedule do you mean?" {
					t.Errorf("unexpected intent: %+v", intent)
				}
			},
		},
		{
			name:    "raw json create",
			raw:     validCreateJSON,
			wantErr: "",
			check: func(t *testing.T, intent *assistIntent) {
				if intent.Kind != "proposal" || intent.Op != "create" {
					t.Fatalf("unexpected intent: %+v", intent)
				}
				if intent.Prompt != "Summarize the nightly test runs" {
					t.Errorf("prompt = %q", intent.Prompt)
				}
				if intent.Cadence == nil || intent.Cadence.Expression != "0 9 * * 1-5" {
					t.Errorf("cadence = %+v", intent.Cadence)
				}
				if intent.Target == nil || intent.Target.Type != "new-agent" {
					t.Errorf("target = %+v", intent.Target)
				}
			},
		},
		{
			name:    "prose wrapped json",
			raw:     "Sure! Here is the proposal:\n" + validCreateJSON + "\nLet me know if that works.",
			wantErr: "",
		},
		{
			name:    "answer kind",
			raw:     `{"kind":"answer","message":"3 schedules run today"}`,
			wantErr: "",
		},
		{
			name:    "truncated json",
			raw:     `{"kind":"proposal","op":"create","prompt":"x"`,
			wantErr: "truncated",
		},
		{
			name:    "no json at all",
			raw:     "I don't understand the question.",
			wantErr: "no JSON",
		},
		{
			name:    "invalid json inside fence",
			raw:     "```json\n{not valid}\n```",
			wantErr: "invalid JSON",
		},
		{
			name:    "unknown kind",
			raw:     `{"kind":"mutate"}`,
			wantErr: "kind",
		},
		{
			name:    "create missing prompt",
			raw:     `{"kind":"proposal","op":"create","cadence":{"type":"every","everyMs":3600000},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"summary":"s"}`,
			wantErr: "prompt",
		},
		{
			name:    "create missing cadence",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"summary":"s"}`,
			wantErr: "cadence",
		},
		{
			name:    "create missing target",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"summary":"s"}`,
			wantErr: "target",
		},
		{
			name:    "create missing summary",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}}}`,
			wantErr: "summary",
		},
		{
			name: "update missing scheduleId",
			raw:  `{"kind":"proposal","op":"update","prompt":"new prompt","summary":"s"}`,
			setupStore: func(t *testing.T) *Store {
				store := NewStore()
				createAssistTestSchedule(t, store, "Nightly")
				return store
			},
			wantErr: "scheduleId",
		},
		{
			name: "update with no changed fields",
			raw:  `{"kind":"proposal","op":"update","scheduleId":"%SID%","summary":"s"}`,
			setupStore: func(t *testing.T) *Store {
				store := NewStore()
				createAssistTestSchedule(t, store, "Nightly")
				return store
			},
			wantErr: "changed field",
		},
		{
			name: "pause missing scheduleId",
			raw:  `{"kind":"proposal","op":"pause","summary":"s"}`,
			setupStore: func(_ *testing.T) *Store {
				return NewStore()
			},
			wantErr: "scheduleId",
		},
		{
			name:    "clarify missing message",
			raw:     `{"kind":"clarify"}`,
			wantErr: "message",
		},
		{
			name:    "bad cron expression",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"cron","expression":"not a cron"},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"summary":"s"}`,
			wantErr: "cron",
		},
		{
			name:    "every interval below minimum",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":30000},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"summary":"s"}`,
			wantErr: "60000",
		},
		{
			name:    "unknown cadence type",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"weekly"},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"summary":"s"}`,
			wantErr: "cadence",
		},
		{
			name:    "prompt too long",
			raw:     `{"kind":"proposal","op":"create","prompt":"` + strings.Repeat("p", 4001) + `","cadence":{"type":"every","everyMs":3600000},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"summary":"s"}`,
			wantErr: "4000",
		},
		{
			name: "unknown schedule id",
			raw:  `{"kind":"proposal","op":"pause","scheduleId":"does-not-exist","summary":"s"}`,
			setupStore: func(t *testing.T) *Store {
				store := NewStore()
				createAssistTestSchedule(t, store, "Nightly")
				return store
			},
			wantErr:    "does-not-exist",
			wantRefErr: true,
		},
		{
			name: "unknown schedule with fuzzy candidates",
			raw:  `{"kind":"proposal","op":"pause","scheduleId":"backup","summary":"s"}`,
			setupStore: func(t *testing.T) *Store {
				store := NewStore()
				createAssistTestSchedule(t, store, "DB backup")
				createAssistTestSchedule(t, store, "Files backup")
				return store
			},
			wantErr:    "backup",
			wantRefErr: true,
			wantCands:  []string{"DB backup", "Files backup"},
		},
		{
			name:    "past expiresAt",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"expiresAt":"2020-01-01T00:00:00Z","summary":"s"}`,
			wantErr: "expiresAt",
		},
		{
			name:    "malformed expiresAt",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"expiresAt":"next friday","summary":"s"}`,
			wantErr: "expiresAt",
		},
		{
			name:    "maxRuns zero",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"new-agent","config":{"provider":"claude","cwd":"/w"}},"maxRuns":0,"summary":"s"}`,
			wantErr: "maxRuns",
		},
		{
			name:       "unknown agent target",
			raw:        `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"agent","agentId":"ghost"},"summary":"s"}`,
			agents:     []AgentInfo{{ID: "a1", Title: "backend"}},
			wantErr:    "ghost",
			wantRefErr: true,
		},
		{
			name:    "invalid target shape",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"new-agent"},"summary":"s"}`,
			wantErr: "target",
		},
		{
			name: "pause by exact id",
			setupStore: func(t *testing.T) *Store {
				store := NewStore()
				createAssistTestSchedule(t, store, "Nightly")
				return store
			},
			raw:     `{"kind":"proposal","op":"pause","scheduleId":"%SID%","name":"Nightly","summary":"pause it"}`,
			wantErr: "",
			check: func(t *testing.T, intent *assistIntent) {
				if intent.Op != "pause" {
					t.Errorf("op = %q", intent.Op)
				}
			},
		},
		{
			name: "schedule ref resolved from name",
			setupStore: func(t *testing.T) *Store {
				store := NewStore()
				createAssistTestSchedule(t, store, "Nightly")
				return store
			},
			raw:     `{"kind":"proposal","op":"pause","scheduleId":"nightly","summary":"pause it"}`,
			wantErr: "",
			check: func(t *testing.T, intent *assistIntent) {
				if intent.ScheduleID == "nightly" {
					t.Error("expected scheduleId to be rewritten to the resolved id")
				}
			},
		},
		{
			name: "ambiguous schedule name",
			setupStore: func(t *testing.T) *Store {
				store := NewStore()
				createAssistTestSchedule(t, store, "Backup")
				createAssistTestSchedule(t, store, "backup")
				return store
			},
			raw:        `{"kind":"proposal","op":"delete","scheduleId":"backup","summary":"delete it"}`,
			wantErr:    "backup",
			wantRefErr: true,
			wantCands:  []string{"Backup", "backup"},
		},
		{
			name:    "agent target resolved by exact id",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"agent","agentId":"a1"},"summary":"s"}`,
			agents:  []AgentInfo{{ID: "a1", Title: "backend"}},
			wantErr: "",
		},
		{
			name:    "agent target resolved from title",
			raw:     `{"kind":"proposal","op":"create","prompt":"do things","cadence":{"type":"every","everyMs":3600000},"target":{"type":"agent","agentId":"Backend"},"summary":"s"}`,
			agents:  []AgentInfo{{ID: "a1", Title: "backend"}},
			wantErr: "",
			check: func(t *testing.T, intent *assistIntent) {
				if intent.Target.AgentID != "a1" {
					t.Errorf("expected agentId rewritten to a1, got %q", intent.Target.AgentID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var store *Store
			var sid string
			if tt.setupStore != nil {
				store = tt.setupStore(t)
				if list := store.List(); len(list) > 0 {
					sid = list[0].ID
				}
			} else {
				store = NewStore()
			}
			raw := strings.ReplaceAll(tt.raw, "%SID%", sid)

			intent, err := parseAssistIntent(raw, store, tt.agents, assistTestNow)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("parseAssistIntent() unexpected error: %v", err)
				}
				if tt.check != nil {
					tt.check(t, intent)
				}
				return
			}

			if err == nil {
				t.Fatalf("parseAssistIntent() expected error containing %q, got nil (intent %+v)", tt.wantErr, intent)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
			if tt.wantRefErr {
				var refErr *assistRefError
				if !errors.As(err, &refErr) {
					t.Fatalf("expected *assistRefError, got %T: %v", err, err)
				}
				if tt.wantCands != nil {
					if len(refErr.candidates) != len(tt.wantCands) {
						t.Fatalf("candidates = %v, want %v", refErr.candidates, tt.wantCands)
					}
					for i, c := range tt.wantCands {
						if refErr.candidates[i] != c {
							t.Errorf("candidates[%d] = %q, want %q", i, refErr.candidates[i], c)
						}
					}
				}
			}
		})
	}
}
