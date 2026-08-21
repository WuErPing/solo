# ADR-005: Supervisor Crash Fallback to a Working Daemon Build

|              |                                                               |
|--------------|---------------------------------------------------------------|
| **Status**   | Accepted                                                      |
| **Date**     | 2026-08-21                                                    |
| **Author**   | Solo Agent                                                    |
| **Scope**    | `supervisor/`, `app/`                                         |
| **Related**  | [ADR-003](adr-003-supervisor-exit-code-restart-contract.md), [ADR-004](adr-004-daemon-version-switching.md), [Daemon Supervision](../architecture/daemon-supervision.md) |

---

## 1. Context

ADR-004 lets users switch the running daemon to any local build from the app.
The remaining failure mode: a freshly-built daemon that **won't start** (bad
refactor, broken config parse, …). ADR-003's crash breaker made this fatal —
after 5 fast crashes the supervisor exited 1, the host went permanently
offline, and the user had to SSH in to recover. That defeats the purpose of
app-driven version switching, whose main use case is exactly this recovery.

## 2. Problem Statement

The app can only talk to a *running* daemon. When the pointed-to build
crash-loops, someone other than the daemon must pick a working binary — and
only the supervisor is still alive.

## 3. Decision

When the crash breaker trips (`maxFastCrashes` exceeded), the supervisor falls
back instead of exiting:

1. Blacklist the crashed build (versioned builds by basename, the default
   binary separately) for the supervisor's lifetime.
2. Spawn the newest versioned build in `$SoloHome/versions/` that hasn't
   failed, and **rewrite the `current` pointer to it** (atomic tmp+rename,
   same convention as the daemon's switch handler).
3. If no versioned build is left, remove the pointer so the next spawn uses
   the default binary (ADR-003 resolution chain).
4. Only when every candidate has failed does the supervisor exit 1.

Attempts are bounded (number of versioned builds + 1), so this cannot loop.

**Why rewrite the pointer:** the pointer is the single source of truth for
"which build runs". An in-memory-only fallback would silently re-run the
broken build on the next restart (exit 42 respawn reads the pointer again),
and the app would show `currentVersion` pointing at a build that isn't
running. Rewriting keeps the file, the app UI, and reality in sync.

App-driven switches are unaffected: the blacklist only constrains *automatic*
fallback selection, and a user can still explicitly switch back to a build
that crashed before (e.g. after fixing it).

## 4. Alternatives Considered

- **App-only recovery** (no supervisor fallback): impossible — a daemon that
  won't start leaves nothing for the app to talk to.
- **Fallback without pointer rewrite**: rejected (state drift, see above).
- **Exponential give-up only (status quo)**: forces manual SSH recovery for
  the exact scenario the feature exists for.

## 5. Consequences

- A broken build costs at most `maxFastCrashes` fast crashes per candidate
  before a working build serves again; the app reconnects on its own.
- The broken build's filename stays in `~/.solo/versions/` (with the supervisor
  log recording the fallback); cleanup is manual.
- The supervisor gains a small versions-dir scanner (duplicated from the
  daemon by design — module boundaries only allow importing `protocol/`).
