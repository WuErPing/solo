# DEBT-001: Session-Memory Config Opt-Out Not Wired

|              |                                                              |
|--------------|--------------------------------------------------------------|
| **Status**   | Resolved (2026-09-23)                                        |
| **Introduced** | 2026-06-01 (`002d53a`, session-memory Phase 1)             |
| **ADR**      | — (source: [session-memory-persistence.md](../architecture/session-memory-persistence.md)) |
| **Repayment Target** | v0.13.0 (实际偿还于 v0.15.0 之后，2026-09-23)          |
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

Repaid 2026-09-23: `PersistedConfig` gained a top-level `Memory *MemoryConfig`
field (`json:"memory,omitempty"`) and `applyPersistedConfig` maps it onto
`cfg.Memory`, so `"memory": {"enabled": false}` in `~/.solo/config.json` now
reaches the existing `IsEnabled()` gate in `daemon.go` and the recorder is not
built. `Save()` preserves a user-written memory block instead of silently
dropping it. Tests: `TestLoad_PersistedConfig_MemoryOptOut`,
`TestLoad_PersistedConfig_MemoryFields`,
`TestLoad_PersistedConfig_MemoryDefaultEnabled`,
`TestSave_PreservesMemoryBlock` (daemon/internal/config).
