package protocol

// Process exit-code contract between the daemon and its supervisor.
//
// The supervisor (solo-supervisor) spawns the daemon as a child process and
// decides whether to respawn it based on the child's exit code:
//
//   - 0: clean shutdown (shutdown_server_request or OS signal) — do NOT respawn.
//   - ExitCodeRestartRequested: restart_server_request was accepted — respawn
//     immediately.
//   - any other code or signal death: crash — respawn with backoff.
//
// Both sides must use these constants; do not hardcode the numbers.
const (
	// ExitCodeClean is a normal, intentional shutdown. The supervisor must
	// not respawn the daemon after this exit code.
	ExitCodeClean = 0

	// ExitCodeRestartRequested tells the supervisor the daemon handled a
	// restart_server_request and exited intentionally; the supervisor
	// respawns it immediately (no crash backoff).
	ExitCodeRestartRequested = 42
)
