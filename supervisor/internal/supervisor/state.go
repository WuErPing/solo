package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// The supervisor persists its spawn-loop health to
// $SoloHome/supervisor-state.json after every state transition. The daemon
// relays it inside list_daemon_versions responses so the app can show why a
// host is unreachable (backoff, fallback, exhausted). Keep the field set in
// sync with protocol.SupervisorState.

const (
	stateFileName = "supervisor-state.json"
	// supervisorStateSchemaVersion bumps when the on-disk shape changes.
	supervisorStateSchemaVersion = 1

	stateRunning   = "running"
	stateBackoff   = "backoff"
	stateFallback  = "fallback"
	stateExhausted = "exhausted"
	stateStopped   = "stopped"
)

// SupervisorState is the on-disk image; json tags match protocol.SupervisorState.
type SupervisorState struct {
	SchemaVersion      int     `json:"schemaVersion"`
	State              string  `json:"state"`
	Pid                int     `json:"pid"`
	SpawnedBinary      string  `json:"spawnedBinary"`
	PointerVersion     *string `json:"pointerVersion,omitempty"`
	ConsecutiveCrashes int     `json:"consecutiveCrashes"`
	BackoffMs          *int64  `json:"backoffMs,omitempty"`
	LastExitCode       *int    `json:"lastExitCode,omitempty"`
	UpdatedAtMs        int64   `json:"updatedAtMs"`
	LastEvent          string  `json:"lastEvent"`
}

// recordState writes the state file. Called only from the Run goroutine, so
// no locking is needed. Write failures are logged but never change control
// flow — the state file is diagnostics, not authority.
func (s *Supervisor) recordState(state, event string, crashes int, backoff time.Duration, exitCode *int) {
	if err := os.MkdirAll(s.cfg.SoloHome, 0755); err != nil {
		s.cfg.Logger.Warn("cannot create solo home for state file", "error", err)
		return
	}
	st := SupervisorState{
		SchemaVersion:      supervisorStateSchemaVersion,
		State:              state,
		Pid:                s.lastChildPID,
		SpawnedBinary:      filepath.Base(s.lastChildBinary),
		ConsecutiveCrashes: crashes,
		UpdatedAtMs:        time.Now().UnixMilli(),
		LastEvent:          event,
	}
	if pointer := s.readVersionPointer(); pointer != "" {
		st.PointerVersion = &pointer
	}
	if backoff > 0 {
		ms := backoff.Milliseconds()
		st.BackoffMs = &ms
	}
	st.LastExitCode = exitCode

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		s.cfg.Logger.Warn("cannot encode supervisor state", "error", err)
		return
	}
	tmp, err := os.CreateTemp(s.cfg.SoloHome, ".supervisor-state-*")
	if err != nil {
		s.cfg.Logger.Warn("cannot create supervisor state file", "error", err)
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		s.cfg.Logger.Warn("cannot write supervisor state file", "error", err)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		s.cfg.Logger.Warn("cannot write supervisor state file", "error", err)
		return
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		_ = os.Remove(tmpName)
		s.cfg.Logger.Warn("cannot chmod supervisor state file", "error", err)
		return
	}
	if err := os.Rename(tmpName, filepath.Join(s.cfg.SoloHome, stateFileName)); err != nil {
		_ = os.Remove(tmpName)
		s.cfg.Logger.Warn("cannot rename supervisor state file", "error", err)
	}
}
