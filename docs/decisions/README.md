# Architecture Decision Records

ADRs capture significant design decisions that shape the codebase. Each record documents the context, alternatives considered, consequences, and any tech debt accepted.

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [ADR-001](adr-001-shared-agent-template-for-loop-and-schedule.md) | Shared Agent Template for Loop and Schedule | Accepted | 2026-06-29 |

## Conventions

- **Numbering**: sequential, zero-padded to 3 digits (`adr-001`, `adr-002`, …).
- **Filename**: `adr-NNN-<kebab-case-title>.md`.
- **Template**: use [`adr-template.md`](adr-template.md) for new records.
- **Statuses**: Proposed → Accepted → Deprecated / Superseded (by ADR-NNN).
- **Tech debt**: any compromise MUST reference a [`../debt/`](../debt/README.md) entry with a repayment window.
- **Immutability**: accepted ADRs are not edited except to update status or add implementation notes. New context → new ADR that supersedes.
