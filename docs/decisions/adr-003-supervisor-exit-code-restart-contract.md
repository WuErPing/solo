# ADR-003: Supervisor Process with Exit-Code Restart Contract

|              |                                                               |
|--------------|---------------------------------------------------------------|
| **Status**   | Accepted                                                      |
| **Date**     | 2026-08-20                                                    |
| **Author**   | Solo Agent                                                    |
| **Scope**    | `supervisor/` (new), `daemon/`, `cli/`, `protocol/`           |
| **Related**  | [Daemon Supervision](../architecture/daemon-supervision.md)   |

---

## 1. Context

The app has a "Restart daemon" action (settings → host → Operations) that sends
`restart_server_request` over the existing WS protocol. Before this change the
daemon handler was a stub that only logged — a restart was impossible for an
obvious reason: the daemon cannot restart itself, because the process that
should come back is the one that just died. Restart requires a second process
that outlives the daemon.

Scaffolding for this decision already existed: the protocol messages, the
app-bridge client (`restartServer()` waiting for a `restart_requested` status
reply), the app UI (including the disconnect→reconnect wait), the CLI
`solo daemon restart` command, and the daemon's `SOLO_SUPERVISED=1` hook that
skips PID-lock acquisition. The missing pieces were the supervisor process
itself and the daemon-side handler implementation.

## 2. Problem Statement

From first principles:

1. **A process cannot reliably supervise itself.** Any in-process "restart me"
   logic dies with the process it is supposed to bring back. Supervision must
   live in a parent process.
2. **The control channel should not bypass E2EE.** A supervisor-local control
   socket (HTTP/WS on another port) would need its own auth and would be
   unreachable through the relay. The existing WS protocol already reaches the
   daemon from the app over both direct connections and the E2EE relay.
3. **The supervisor must distinguish intent.** "The daemon exited" has three
   meanings — requested restart (respawn now), requested shutdown (stay down),
   and crash (respawn carefully). Exit codes are the simplest unambiguous
   channel between a child and its parent.

## 3. Decision

1. **New Go module `supervisor/`** producing the `solo-supervisor` binary
   (mirrors `relay-go/`; may import `protocol` only). It spawns the daemon as
   a child with `SOLO_SUPERVISED=1`, owns `~/.solo/solo.pid` (child PID) and
   `~/.solo/logs/daemon.log` while supervising.
2. **Exit-code contract** in `protocol/process_contract.go`:
   - `0` — clean shutdown (`shutdown_server_request` or OS signal): supervisor
     does **not** respawn.
   - `42` (`ExitCodeRestartRequested`) — accepted `restart_server_request`:
     supervisor respawns immediately.
   - any other code / signal death — crash: supervisor respawns with backoff
     (1s→30s cap, reset after 60s stable uptime) and gives up after 5
     consecutive fast crashes.
3. **The WS protocol is the only control channel.** The daemon implements
   `restart_server_request` / `shutdown_server_request`: it replies with the
   `restart_requested` / `shutdown_requested` status payload the app-bridge
   client already waits for, then exits with the corresponding code after a
   graceful `Stop()`. No supervisor control socket exists.
4. **Unsupervised daemons refuse restart** with an `rpc_error`
   (`NOT_SUPERVISED`) — nothing would bring them back (dev runs via
   `make run-daemon` must not be killable from the app).
5. **`solo daemon start` (and onboard) start the supervisor**, falling back to
   a direct daemon start with a printed warning when `solo-supervisor` is not
   found. `--foreground` still runs the daemon directly.

## 4. Alternatives Considered

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| A. Separate `supervisor/` module + exit-code contract | Clear boundary; supervisor needs no protocol parsing; works over relay for free | One more binary to build/ship | Chosen |
| B. Supervisor mode inside the `solo` binary (re-exec self) | Single binary | Process roles hidden behind modes; supervisor bugs ship with daemon releases; harder to reason about | Rejected |
| C. Supervisor-local control socket (HTTP/WS) | Direct supervisor status queries | Needs its own auth; unreachable via relay/E2EE; duplicates the existing protocol | Rejected |
| D. OS service managers (launchd/systemd/KeepAlive) | Battle-tested respawn | Platform-specific; requires install privileges; app cannot trigger it through the protocol | Rejected |

## 5. Consequences

### Positive

- App-triggered restart works end-to-end with **zero app/app-bridge changes**;
  the existing clients already speak this protocol.
- Relay-connected apps survive restarts: the relay buffers frames and both
  sides reconnect with backoff.
- Crash resilience comes for free (backoff + circuit breaker), including
  daemon crashes unrelated to restart.
- `solo daemon stop` now actually stops a CLI-started daemon (clean exit 0 →
  supervisor exits).

### Negative / Risks

- Two processes to reason about; `~/.solo/solo.pid` now contains the daemon
  child PID (supervisor-owned), which tooling must keep matching.
- Supervisor's own logs are lost when started detached via the CLI (child logs
  go to `~/.solo/logs/daemon.log` regardless).
- The daemon binary ignores CLI flags; the supervisor translates
  `--port/--home/--no-relay/--no-mcp` into env vars (`PORT`, `SOLO_HOME`,
  `SOLO_RELAY_ENABLED`, `SOLO_MCP_ENABLED`) for the child.

## 6. Tech Debt / Repayment Window

| Debt Item | Debt ID | Repayment Target |
|-----------|---------|------------------|
| Supervisor status (uptime, restart count) not visible in the app; needs a status file or protocol message if wanted | — | On demand |
| Electron desktop daemon lifecycle (`app/src/desktop/`) does not use the supervisor yet (host bridge lives outside this repo) | — | When desktop adopts it |

## 7. Acceptance Criteria

- `solo daemon start` spawns `solo-supervisor`; `~/.solo/solo.pid` holds the
  daemon child PID; `/api/health` becomes healthy.
- `restart_server_request` (direct WS and relay) receives a
  `restart_requested` status reply; the daemon exits 42; the supervisor
  respawns it; health recovers.
- `shutdown_server_request` receives `shutdown_requested`; daemon exits 0;
  supervisor exits; PID file removed.
- An unsupervised daemon replies `NOT_SUPERVISED` and keeps running.
- A crash loop (5 fast crashes) makes the supervisor exit 1 instead of
  fork-bombing.

## 8. References

- `protocol/process_contract.go` — exit-code contract
- `supervisor/internal/supervisor/supervisor.go` — respawn policy
- `daemon/internal/server/session_server_control.go` — WS handlers
- [Daemon Supervision](../architecture/daemon-supervision.md)
