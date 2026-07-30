# Analysis Documents

This directory contains deep-dive technical analyses, reviews, and decision records for the Solo project. Dated documents record a point-in-time assessment; reference documents describe ongoing structural concerns.

> **Graduation rule**: Analysis is a staging area, not a destination. When an analysis produces a decision, it graduates to `decisions/` (ADR) or `architecture/`. When it produces test/verification findings, those go to `verification/reports/`. The analysis file gets a "Superseded by" or "Graduated to" link.

## Dated Analyses (newest first)

| Date | Document | Status | Summary |
|------|----------|--------|---------|
| 2026-07-29 | [Tmux Pane Rendering Decision](tmux-pane-rendering-decision.md) | Decided & Implemented | Client-side xterm.js rendering chosen & implemented; tmux control-mode alternative deferred. Supersedes the earlier tmux-pane first-principles analyses. |
| 2026-07-29 | [Tmux Project Matcher](tmux-project-matcher.md) | Implemented | Spec + plan for matching tmux panes to projects (sidebar badge). |
| 2026-07-28 | [Tmux Keybar Layout Analysis](tmux-keybar-layout-analysis-2026-07-28.md) | Analysis Complete | Post-implementation UX audit of the three-layer `TmuxKeyBar`; Primary Row overflow risk + 5 medium/low findings. |
| 2026-07-25 | [Security Deep Analysis](security-deep-analysis-2026-07-25.md) | Analysis | Threat model + 9 High / 14 Medium / 12 Low findings and remediation roadmap. |
| 2026-07-24 | [Tmux Discovery & Refresh](tmux-discovery-refresh-analysis-2026-07-24.md) | Fixes Implemented & Verified | Source of truth for agent discovery (4-layer) + adaptive refresh; includes the 2026-07-29 refresh-lag tuning addendum. |
| 2026-07-24 | [App Performance Analysis](app-performance-analysis-2026-07-24.md) | Analysis Complete | RN/Expo rendering, state, WS flow, terminal, polling, memory audit; findings resolved. |
| 2026-07-27 | [Test Quality Audit](../verification/reports/test-quality-audit-2026-07.md) | Complete | Unit-test quality issues (timing coupling, empty assertions, infra duplication) + prioritized fix tracking. *(moved to verification/reports/)* |
| 2026-06-20 | [Solo Roadmap Architecture Mapping](solo-roadmap-architecture-mapping.md) | Analysis Complete | Maps Solo features to 2026 roadmap pillars; layered architecture + phased plan. |
| 2026-06-19 | [Dead Code Analysis](dead-code-analysis-2026-06-19.md) | Analysis Complete | ~118 findings / ~2940 LOC; phased removal plan. |
| 2026-06-18 | [Architecture First-Principles Review](architecture-first-principles-review-2026-06-18.md) | Complete | First-principles evaluation of major decisions; long-term risk identification. |
| 2026-06-16 | [Test Coverage](../verification/reports/test-coverage.md) | Complete | Unified coverage report: Go backend + App frontend + E2E + CI/Codecov. *(moved to verification/reports/)* |
| 2026-06-15 | [Tmux Agent Misidentification (kimi → kimi-code)](tmux-agent-misidentification-kimi-code-2026-06-15.md) | Analysis Complete | `kimi --yolo` misidentified as `kimi-code`: setproctitle pollution + cascade failure + fixes. |
| 2026-06-12 | [Architecture Review](architecture-review-2026-06-12/) | Complete | 4+1 views, maturity scoring, ATAM evaluation, recommendations (4-file review). |

## Reference / Undated

| Document | Status | Summary |
|----------|--------|---------|
| [Go Provider Type Erasure](go-provider-type-erasure-analysis.md) | Analysis Complete (P1) | `interface{}` / `map[string]interface{}` growth diagnosis + phased D→B remediation. |
| [Agent/Provider Status Unification](agent-provider-status-unification.md) | Proposal | OCP-based unification of AgentLifecycleStatus / ProviderStatus across layers. |
| [App Agent Status Analysis](app-agent-status-analysis.md) | Complete | App agent lifecycle states and Copy button display logic. |
| [App-Bridge Schedule Module](app-bridge-schedule-module.md) | Reference | Schedule module type contract, RPC schema, domain models. |
| [Create Schedule Flow](create-schedule-flow.md) | Reference | End-to-end schedule creation flow with timezone-aware cron. |
| [Host Status Check](host-status-check.md) | Analysis | Probe cycle (2-30 s), adaptive switching, state-machine conflict, grace-period fix. |
| [Coding Agent Hooks Comparison](coding-agent-hooks-comparison.md) | Reference | Hook systems across 7 coding agents + Solo design suggestions. |
| [Tmux Transport Disposed Race](tmux-transport-disposed-race.md) | Fixed | `Transport not connected (status: disposed)` root cause + `withLiveTmuxClient` retry. |
| [iTerm2 Agent Observation](iterm2-agent-observation.md) | Analysis Complete | iTerm2 agent detection observation + CLI discovery (see `demo/`). |

## Related Documentation

### Architecture
- [Agent Stall Detection](../architecture/agent-stall-detection.md)
- [Components](../architecture/components.md)
- [Data Flow](../architecture/data-flow.md)
- [Session Memory Persistence](../architecture/session-memory-persistence.md)
- [Timeline Design](../architecture/timeline-design.md)
- [Tmux Pane Content Loading](../architecture/tmux-pane-content-loading.md)
- [Push Notifications](../architecture/push-notifications.md)

### Product
- [Features](../product/features.md)
- [2026 Roadmap](../product/roadmap-2026.md)
- [Loop Schedule Spec](../product/prd/loop-schedule-spec.md)

### Providers
- [Kimi Cursor Integration](../providers/kimi-cursor-integration.md)
- [Kimi Wire vs ACP](../providers/kimi-wire-vs-acp.md)

---

## Creating New Analyses

When creating a new analysis document:

1. **Use a descriptive filename:** `<topic>-analysis.md` or `<topic>-analysis-YYYY-MM-DD.md` for dated work.
2. **Include metadata:** Date, Status, Priority.
3. **Follow the template below.**
4. **Add a row** to the appropriate table above.

### Template

```markdown
# <Topic> Analysis

**Date:** YYYY-MM-DD
**Status:** Analysis Complete | In Progress | Draft
**Priority:** High | Medium | Low

## Executive Summary
Brief summary of the analysis...

## Current State
Description of the current state...

## Analysis
Detailed analysis...

## Recommendations
List of recommendations...

## Implementation Plan
Phased implementation plan...

## Related Files
Links to relevant files...
```
