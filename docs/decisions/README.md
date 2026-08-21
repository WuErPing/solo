# Architecture Decision Records

ADRs capture significant design decisions that shape the codebase. Each record documents the context, alternatives considered, consequences, and any tech debt accepted.

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [ADR-001](adr-001-shared-agent-template-for-loop-and-schedule.md) | Shared Agent Template for Loop and Schedule | Accepted | 2026-06-29 |
| [ADR-002](adr-002-product-task-unification.md) | Product + Task Unification | Proposed | 2026-07-31 |
| [ADR-003](adr-003-supervisor-exit-code-restart-contract.md) | Supervisor Exit-Code Restart Contract | Accepted | 2026-08-20 |
| [ADR-004](adr-004-daemon-version-switching.md) | Daemon Version Switching via Versions Directory and Pointer File | Accepted | 2026-08-20 |
| [ADR-005](adr-005-supervisor-crash-fallback.md) | Supervisor Crash Fallback to a Working Daemon Build | Accepted | 2026-08-21 |

## Conventions

- **Numbering**: sequential, zero-padded to 3 digits (`adr-001`, `adr-002`, …).
- **Filename**: `adr-NNN-<kebab-case-title>.md`.
- **Template**: use [`adr-template.md`](adr-template.md) for new records.
- **Statuses**: Proposed → Accepted → Deprecated / Superseded (by ADR-NNN).
- **Tech debt**: any compromise MUST reference a [`../debt/`](../debt/README.md) entry with a repayment window.
- **Immutability**: accepted ADRs are not edited except to update status or add implementation notes. New context → new ADR that supersedes.
