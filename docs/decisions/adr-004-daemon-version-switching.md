# ADR-004: Daemon Version Switching via Versions Directory and Pointer File

|              |                                                               |
|--------------|---------------------------------------------------------------|
| **Status**   | Accepted                                                      |
| **Date**     | 2026-08-20                                                    |
| **Author**   | Solo Agent                                                    |
| **Scope**    | `protocol/`, `daemon/`, `supervisor/`, `app-bridge/`, `app/`  |
| **Related**  | [ADR-003](adr-003-supervisor-exit-code-restart-contract.md), [Daemon Supervision](../architecture/daemon-supervision.md) |

---

## 1. Context

Hosts accumulate multiple daemon builds over time (release binaries, dev
builds). Users want to switch the running daemon to the latest runnable build
from the app's host page — manually, without SSH-ing into the host. ADR-003
established the supervisor and its exit-code restart contract: the daemon can
ask to be restarted (exit 42) and the supervisor respawns it.

What was missing: a way to tell the supervisor *which* binary to spawn, and a
protocol/UI for listing and selecting builds.

## 2. Problem Statement

1. **Binary selection must survive the daemon's death.** The daemon is the
   only process the app can talk to, but the decision of what to run next must
   be read by the supervisor *after* the daemon exits. A file is the natural
   channel.
2. **"Latest" is not lexicographic.** Version strings like
   `v0.8.0-dev-20260820123953-dirty` do not sort reliably. File mtime reflects
   "newest build placed on the host" with zero parsing.
3. **The wire format should be self-describing.** Filenames double as version
   labels (`solo-<version>`), so no version flag needs to be added to the
   daemon binary (it parses no CLI flags today).

## 3. Decision

1. **Versions directory convention**: `$SoloHome/versions/` holds executable
   daemon binaries named `solo-*` (filename = version label). A pointer file
   `$SoloHome/versions/current` contains the basename of the build to run.
2. **Supervisor resolves the binary per spawn** (no longer once at startup):
   a valid pointer (basename only, existing executable regular file) wins;
   anything invalid falls back to the ADR-003 resolution chain
   (`SOLO_DAEMON_BINARY` → sibling `solo` → PATH) with a warning.
3. **New protocol messages**:
   - `list_daemon_versions_request` → response with `runningVersion`
     (ldflags version of the live daemon), `currentVersion` (pointer target),
     and `versions[]` (executable `solo-*` files, newest mtime first).
     Works supervised or not; a missing dir is an empty list, not an error.
   - `switch_daemon_version_request { version? }` — omitted/`"latest"` =
     newest mtime. Supervised only (`NOT_SUPERVISED` otherwise). The daemon
     atomically writes the pointer (tmp + rename), replies
     `daemon_version_switch_requested`, then exits 42; the supervisor respawns
     it on the new build.
4. **No-op case**: when the pointer already targets the requested build and
   the running daemon is that build (filename `solo-<version>` matches the
   ldflags version), the daemon replies without restarting.
5. **App UI**: a "Daemon version" card in the host page Operations section
   shows running + latest local build and offers "Switch to latest", reusing
   the restart confirmation and disconnect→reconnect wait.

## 4. Alternatives Considered

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| A. Pointer file + per-spawn resolution | Switch survives daemon death; no supervisor protocol needed; tiny diff | Two components must agree on the dir convention | Chosen |
| B. Supervisor control socket for switch command | Explicit RPC to supervisor | New auth surface; unreachable via relay/E2EE; contradicts ADR-003 | Rejected |
| C. Daemon swaps binary and re-execs itself | No pointer file | Self-replace is fragile (running binary locked on some platforms); loses supervision during the swap | Rejected |
| D. Semver parsing for "latest" | Precise ordering | Dev/dirty suffixes break parsing; mtime already matches user intent | Rejected |

## 5. Consequences

### Positive

- Version switching reuses the entire restart machinery (protocol, supervisor
  respawn, app reconnect wait) — the only new moving part is the pointer file.
- Fallback chain means a broken/missing pointer never prevents the daemon
  from starting.
- Listing works on any daemon; only switching requires supervision.

### Negative / Risks

- Two writers of the convention (daemon writes pointer, supervisor reads it);
  the constants are documented in both places and in the architecture doc.
- "Latest by mtime" can surprise users who copy an old build last (documented).
- Populating `~/.solo/versions/` is manual/CI for now (no download feature).

## 6. Tech Debt / Repayment Window

| Debt Item | Debt ID | Repayment Target |
|-----------|---------|------------------|
| No remote download into the versions dir | — | When an update server exists |
| CLI `daemon versions/switch` parity missing | — | On demand (protocol is ready) |
| Version-picker UI (explicit `version` is protocol-supported; app only offers "latest") | — | On demand |

## 7. Acceptance Criteria

- `list_daemon_versions_request` returns running/current/versions (newest
  first; non-executable and non-`solo-*` files excluded; missing dir = empty).
- Switch to latest writes `versions/current` atomically, replies
  `daemon_version_switch_requested`, daemon exits 42, supervisor respawns the
  pointed-to binary (visible in supervisor log and changed child PID).
- Tampered pointer (missing target, path traversal) → supervisor falls back
  to the default binary with a warning.
- Unsupervised daemon refuses switch with `NOT_SUPERVISED` and keeps running.
- App host page shows running/latest and switches on confirm.

## 8. References

- `protocol/message_version.go`
- `daemon/internal/server/session_version.go`
- `supervisor/internal/supervisor/supervisor.go` (`resolveChildBinary`)
- `app-bridge/src/client/version-rpc.ts`
- [Daemon Supervision — Version switching](../architecture/daemon-supervision.md)
