package server

// ExitRequest describes a daemon-initiated process exit requested through the
// WS protocol (restart_server_request / shutdown_server_request). main() owns
// the actual os.Exit; the daemon only records the intent.
type ExitRequest struct {
	Code   int
	Reason string
}

// RequestExit records a daemon-initiated exit request. Only the first request
// is kept; subsequent ones are dropped because a shutdown is already pending.
func (d *Daemon) RequestExit(code int, reason string) {
	select {
	case d.exitCh <- ExitRequest{Code: code, Reason: reason}:
	default:
		d.logger.Warn("exit request dropped: shutdown already pending", "code", code, "reason", reason)
	}
}

// ExitRequested returns the channel on which daemon-initiated exit requests
// arrive. main() should select on it alongside the OS signal channel and exit
// with the request's code after a graceful Stop.
func (d *Daemon) ExitRequested() <-chan ExitRequest {
	return d.exitCh
}
