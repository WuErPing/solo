# DEBT-001: Session-Memory Config Opt-Out Not Wired

|              |                                                              |
|--------------|--------------------------------------------------------------|
| **Status**   | Active                                                       |
| **Introduced** | 2026-06-01 (`002d53a`, session-memory Phase 1)             |
| **ADR**      | — (source: [session-memory-persistence.md](../architecture/session-memory-persistence.md)) |
| **Repayment Target** | v0.13.0                                          |
| **Owner**    | —                                                            |

## What

The session-memory feature documents and comments an opt-out via
`config.json`: `"memory": {"enabled": false}` (see
`daemon/internal/config/memoryconfig.go:12` and
`docs/architecture/components.md` §3.5). In reality `PersistedConfig`
(`daemon/internal/config/config.go:55`) only has `daemon` and `app` keys, so
`Load()` silently ignores a top-level `memory` block — the feature is always
on (zero-value `MemoryConfig` runs it via `ApplyDefaults`).

## Why It Was Accepted

Discovered during the 2026-08-21 docs sweep, after the fact. Phase 1 shipped
the feature default-on with the opt-out described but never wired into the
persisted-config loader.

## Impact

- Users who set `"memory": {"enabled": false}` get no error and no effect —
  turns keep being persisted under `~/.solo/memory/`. Silent no-op configs
  violate POLA and are a privacy-relevant surprise.
- Docs and code comments describe behavior that does not exist.

## Repayment Plan

1. Add `Memory *MemoryConfig` to `PersistedConfig` and map it in
   `applyPersistedConfig` (explicit `enabled: false` → `cfg.Memory.Enabled =
   &false` before memory wiring runs). ~30 lines including tests.
2. Add a config-load test: `memory.enabled=false` in config.json disables the
   recorder (no files written under `~/.solo/memory/`).
3. Update `memoryconfig.go` comment if semantics change; keep docs as-is once
   the behavior matches them.

## Resolution (fill when done)
