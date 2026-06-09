# Analysis Documents

This directory contains analysis documents for the Solo project.

## Recent Analyses

### 2026-06-09: Android Tmux Pane Rendering Optimization

**Status:** Analysis Complete
**Priority:** High (UX)

**Summary:**
- Deep analysis of why Android tmux pane output diverges from host tmux experience
- Comparison of snapshot polling vs cell-based VT stream rendering models
- Evaluation of 4 optimization strategies (ANSI Text Enhancement, xterm.js Migration, PTY Stream, Daemon Cell Grid Diff)
- Recommended phased approach: xterm.js migration first (Phase 1), tmux Control Mode second (Phase 2)

**Key Findings:**
1. Current architecture uses `tmux capture-pane` (snapshot) vs host tmux's incremental cell-grid rendering
2. Box Drawing / Braille characters stripped due to width mismatch between host terminal and mobile display
3. No cursor rendering, no `wcwidth` Unicode width calculation
4. Workspace terminal already has mature xterm.js + WebGL infrastructure ready for reuse
5. xterm.js migration is highest ROI: medium effort, near-native quality, low maintenance

**Document:** [tmux-pane-rendering-optimization.md](tmux-pane-rendering-optimization.md)

---

### 2026-06-07: Tmux Pane Refresh Jitter Analysis

**Status:** Analysis Complete
**Priority:** Medium (UX)

**Summary:**
- Root cause analysis of tmux pane viewer jitter on refresh
- Comparison with host tmux cell-based rendering model
- Box drawing character handling and width adaptation gap
- 6 proposed solutions ranked by priority (P0–P4)

**Key Findings:**
1. Full content replacement on 5s poll causes complete React tree re-render
2. `scrollToEnd({ animated: true })` fires on every content change, including unchanged content
3. Background color dynamically derived from content causes visual flash
4. Box drawing characters stripped because no width adaptation exists
5. Host tmux uses cell-based incremental rendering; app uses full-string replacement

**Document:** [tmux-pane-jitter-analysis.md](tmux-pane-jitter-analysis.md)

---

### 2026-06-07: Go Provider Type Erasure Analysis

**Status:** Analysis Complete
**Priority:** P1 (Structural Risk)

**Summary:**
- Diagnoses Go-side `interface{}` / `map[string]interface{}` growth (~25-30%/cycle)
- Compares 5 remediation strategies (Gradual, Tagged Union, Code Gen, Boundary Isolation, Linter)
- Recommends phased D → B approach: Boundary Isolation first, then Tagged Union

**Key Findings:**
1. Root cause: `protocol.AgentStreamPayload.Event` acts as a type sink
2. OpenCode provider is the largest contributor (~120 instances)
3. Boundary Isolation is the most feasible structural fix (~45h, 1-2 weeks)

**Document:** [go-provider-type-erasure-analysis.md](go-provider-type-erasure-analysis.md)

---

### 2026-06-03: iTerm2 Agent Observation Analysis

**Status:** Analysis Complete
**Priority:** High

**Summary:**
- iOS build status analysis
- Agent observation architecture design
- iTerm2 integration solutions

**Key Findings:**
1. iOS build missing: release build skill, CI workflow, EAS profile, secrets
2. Agent observation requires iTerm2 integration
3. Recommended solution: iTerm2 Python API + Daemon Bridge

**Document:** [iterm2-agent-observation.md](iterm2-agent-observation.md)

---

### 2026-06-03: App Agent Status Analysis

**Status:** Complete
**Priority:** Medium

**Summary:**
- Analysis of agent status display in the app
- Recommendations for improving status visibility

**Document:** [app-agent-status-analysis.md](app-agent-status-analysis.md)

---

### 2026-06-03: Session Timeline E2E Gaps

**Status:** Complete
**Priority:** High

**Summary:**
- Analysis of session timeline end-to-end gaps
- Recommendations for improving timeline functionality

**Document:** [session-timeline-e2e-gaps.md](session-timeline-e2e-gaps.md)

---

## Analysis Categories

### Architecture
- [Agent Stall Detection](../architecture/agent-stall-detection.md)
- [Components](../architecture/components.md)
- [Data Flow](../architecture/data-flow.md)
- [Network Architecture](../architecture/network-architecture.md)
- [Session Memory Persistence](../architecture/session-memory-persistence.md)
- [Timeline Design](../architecture/timeline-design.md)

### Product
- [Features](../product/features.md)
- [UI Features](../product/ui-features.md)
- [Product Analysis](../product/product-analysis.md)
- [Session Memory Spec](../product/session-memory-spec.md)

### Providers
- [Kimi Cursor Integration](../providers/kimi-cursor-integration.md)
- [Kimi Wire vs ACP](../providers/kimi-wire-vs-acp.md)

---

## Creating New Analyses

When creating a new analysis document:

1. **Use descriptive filename:** `analysis-<topic>.md` or `<topic>-analysis.md`
2. **Include metadata:** Date, Status, Priority
3. **Follow structure:**
   - Executive Summary
   - Current State
   - Analysis
   - Recommendations
   - Implementation Plan
   - Related Files
4. **Update this README** with a link to the new document

---

## Analysis Template

```markdown
# <Topic> Analysis

**Date:** YYYY-MM-DD
**Status:** Analysis Complete | In Progress | Draft
**Priority:** High | Medium | Low

---

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
