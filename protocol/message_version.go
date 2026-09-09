package protocol

// --- Daemon Version Switching ---
//
// The daemon lists runnable builds from a local versions directory on the
// host ($SoloHome/versions/) and can switch the running build by writing the
// pointer file ($SoloHome/versions/current) and exiting with
// ExitCodeRestartRequested; the supervisor then respawns it on the pointed-to
// binary. See docs/architecture/daemon-supervision.md.

// ListDaemonVersionsRequest asks the daemon to list the builds available in
// the host's versions directory.
// genzod
type ListDaemonVersionsRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
}

func (m *ListDaemonVersionsRequest) MsgType() string { return "list_daemon_versions_request" }

// DaemonVersionInfo describes one runnable build in the versions directory.
// Version is the binary's filename (the display label); MtimeMs decides which
// build is "latest".
// genzod
type DaemonVersionInfo struct {
	Version string `json:"version"`
	MtimeMs int64  `json:"mtimeMs"`
}

// ListDaemonVersionsResponse
// genzod
type ListDaemonVersionsResponse struct {
	Type    string                    `json:"type"`
	Payload ListDaemonVersionsPayload `json:"payload"`
}

// SupervisorState mirrors the supervisor's on-disk state file
// ($SoloHome/supervisor-state.json): the spawn-loop health of the supervisor
// process on the host. State is one of running, backoff, fallback, exhausted
// or stopped. The daemon reads the file and relays it here; keep the field
// set in sync with supervisor/internal/supervisor/state.go.
// genzod
type SupervisorState struct {
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

// ListDaemonVersionsPayload
// genzod
type ListDaemonVersionsPayload struct {
	RequestID      string              `json:"requestId"`
	RunningVersion string              `json:"runningVersion"`
	CurrentVersion *string             `json:"currentVersion,omitempty"`
	Versions       []DaemonVersionInfo `json:"versions"`
	Supervisor     *SupervisorState    `json:"supervisor,omitempty"`
	Error          *string             `json:"error,omitempty"`
}

func (m *ListDaemonVersionsResponse) MsgType() string { return "list_daemon_versions_response" }

// SwitchDaemonVersionRequest asks the daemon to switch to a specific build, or
// to the latest (newest mtime) when Version is nil or "latest". Only honored
// when the daemon runs under the supervisor.
// genzod
type SwitchDaemonVersionRequest struct {
	Type      string  `json:"type"`
	Version   *string `json:"version,omitempty"`
	RequestID string  `json:"requestId"`
}

func (m *SwitchDaemonVersionRequest) MsgType() string { return "switch_daemon_version_request" }

// DaemonVersionSwitchRequestedStatusPayload is sent as a status message
// payload in reply to switch_daemon_version_request, just before the daemon
// exits with ExitCodeRestartRequested so the supervisor respawns it on the
// selected build.
// genzod
type DaemonVersionSwitchRequestedStatusPayload struct {
	Status    string `json:"status"` // always "daemon_version_switch_requested"
	ClientID  string `json:"clientId"`
	Version   string `json:"version"`
	RequestID string `json:"requestId"`
}
