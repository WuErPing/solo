# Daemon Supervision

How the `solo-supervisor` process keeps the local daemon alive and enables
app-triggered restarts. Decision records: [ADR-003](../decisions/adr-003-supervisor-exit-code-restart-contract.md),
[ADR-004](../decisions/adr-004-daemon-version-switching.md),
[ADR-005](../decisions/adr-005-supervisor-crash-fallback.md).

## Process model

```
┌────────────┐   restart_server_request (WS, direct or via relay+E2EE)
│    App     │ ──────────────────────────────────────────────┐
└────────────┘                                                ▼
                                                 ┌─────────────────────┐
                                                 │  daemon (solo)       │
                                                 │  replies status/     │
                                                 │  restart_requested,  │
                                                 │  graceful Stop,      │
                                                 │  exit 42             │
                                                 └──────────▲──────────┘
┌──────────────────┐  spawn (SOLO_SUPERVISED=1)              │ exit code
│ solo-supervisor  │ ────────────────────────────────────────┘
│  42 → respawn now          │
│  0  → exit (no respawn)    │  owns: ~/.solo/solo.pid (child PID)
│  *  → backoff + breaker    │        ~/.solo/logs/daemon.log
│                            │        ~/.solo/supervisor-state.json
└──────────────────┘
```

## Exit-code contract

Defined in `protocol/process_contract.go` — the only interface between the two
processes:

| Child exit | Meaning | Supervisor action |
|------------|---------|-------------------|
| `0` | Clean shutdown (`shutdown_server_request`, OS signal) | Exit 0, do not respawn |
| `42` (`ExitCodeRestartRequested`) | Accepted `restart_server_request` | Respawn immediately, reset backoff |
| other / killed by signal | Crash | Respawn with backoff 1s→2s→…→30s cap; reset after 60s stable uptime; exit 1 after 5 consecutive fast crashes |

## Restart flow (end-to-end)

1. App sends `restart_server_request` (settings → host → Operations, or
   `solo daemon restart`). Works over direct local WS and the relay; the
   payload is E2EE-encrypted on the relay path like everything else.
2. The daemon handler (`daemon/internal/server/session_server_control.go`)
   checks `cfg.Supervised`:
   - **false** → replies `rpc_error` with code `NOT_SUPERVISED`, keeps running.
   - **true** → replies `status` / `restart_requested` (the payload the
     app-bridge `restartServer()` waits for), waits 200ms for the send queue to
     flush, then requests process exit.
3. `daemon/main.go` selects on OS signals **and** the daemon's exit-request
   channel; it runs the normal graceful `Stop(ctx)` (10s budget) and exits
   with the requested code.
4. The supervisor sees exit 42 and respawns immediately.
5. Clients reconnect on their own: direct-WS via app-bridge `ConnectionManager`
   backoff; relay sessions are buffered by the relay while the daemon's relay
   client reconnects. The app's `waitForDaemonRestart` polls for the
   disconnect→reconnect cycle.

Shutdown is the same flow with the `shutdown_requested` payload and exit 0 —
the supervisor exits too, so `solo daemon stop` now stops a supervised daemon
for good.

## Responsibilities

- **Supervisor** (`supervisor/`): spawn/respawn, PID file
  (`~/.solo/solo.pid`, child PID — same format `pidlock` uses, so
  `solo daemon status` keeps working), daemon log capture
  (`~/.solo/logs/daemon.log`), signal forwarding (SIGTERM → child, SIGKILL
  after 10s grace), state file (`~/.solo/supervisor-state.json` —
  observability only; the daemon surfaces it via `list_daemon_versions`
  when `SOLO_SUPERVISED=1`).
- **Daemon** (`daemon/`): protocol handlers, graceful stop, exit with the
  contract code. When `SOLO_SUPERVISED=1` it skips its own PID lock — the
  supervisor owns the PID file.
- **CLI** (`cli/`): `solo daemon start` and onboard start the supervisor
  (falling back to a direct daemon start with a warning when
  `solo-supervisor` is missing). `--foreground` runs the daemon directly.
  The supervisor translates `--port/--home/--no-relay/--no-mcp` into env vars
  (`PORT`, `SOLO_HOME`, `SOLO_RELAY_ENABLED`, `SOLO_MCP_ENABLED`) because the
  daemon binary reads no CLI flags.

## Version switching

Hosts can keep multiple runnable daemon builds and switch between them from
the app (ADR-004):

```
~/.solo/versions/
├── solo-v0.7.4                    ← runnable builds; filename = version label
├── solo-v0.8.0-dev-2026082012
└── current                        ← pointer: basename of the build to run
```

- `list_daemon_versions_request` returns `runningVersion` (ldflags version of
  the live daemon), `currentVersion` (pointer target), and the runnable builds
  (executable `solo-*` files, newest mtime first). Works in both modes; a
  missing dir is an empty list.
- `switch_daemon_version_request { version? }` (supervised only): the daemon
  atomically writes the pointer, replies `daemon_version_switch_requested`,
  and exits 42. Because the supervisor **re-resolves the binary on every
  spawn** — pointer first, then the fallback chain — the respawn runs the
  selected build. An invalid pointer (missing target, non-basename) falls back
  to the default binary with a warning, so a bad pointer can never wedge the
  host.
- "Latest" means newest file mtime; `"latest"` or an omitted `version`
  selects it. When the pointer already targets the running build, the daemon
  replies without restarting.
- App: host page → Operations → "Daemon version" card shows running + latest
  local build; the "Switch version" dropdown lists the recent builds (running
  one checked and disabled) and restarts onto the selected build on confirm,
  reusing the restart reconnect wait.

## Crash fallback

A build that won't start must not take the host down for good — the app can
only talk to a running daemon, so recovery has to happen in the supervisor
(ADR-005). When the crash breaker trips:

1. The crashed build is blacklisted for the supervisor's lifetime.
2. The supervisor spawns the newest versioned build that hasn't failed and
   **rewrites `current` to it** (atomic tmp+rename, same convention as the
   daemon's switch handler) — pointer, app UI, and reality stay in sync.
3. With no versioned build left, the pointer is removed and the default
   binary (ADR-003 chain) gets one try.
4. Only when every candidate has failed does the supervisor exit 1.

Attempts are bounded (versioned builds + 1). The blacklist only constrains
automatic fallback — the app can still explicitly switch to a build that
crashed before (e.g. after it was fixed and rebuilt under the same name).

## What the supervisor does not do

- No control socket, no status API — the WS protocol is the only control
  channel.
- No binary self-update.
- No agent-process cleanup: agent subprocesses survive a daemon restart, as
  before (the app's restart copy says "Agents running on it will keep going").
