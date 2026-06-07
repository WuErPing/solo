# Docs/ Directory Structure Audit

> Date: 2026-06-03 (updated 2026-06-07)
> Scope: `docs/` directory (34 files across 4 subdirectories + demo/)
> Context: Combined with `.agents/skills/solo-dev-base` and `.agents/rules/architecture.md`

---

## Current Structure

```
docs/
├── README.md                    # Master index
├── architecture/                # System architecture (11 files)
│   ├── README.md
│   ├── components.md
│   ├── data-flow.md
│   ├── deployment.md
│   ├── network-architecture.md
│   ├── timeline-design.md
│   ├── session-memory-persistence.md
│   ├── agent-stall-detection.md
│   ├── push-notifications.md
│   ├── solo-system-architecture.png
│   ├── solo-system-architecture.svg
│   └── tmux-pane-content-loading.md
├── product/                     # Product features (4 files)
│   ├── features.md
│   ├── product-analysis.md
│   ├── session-memory-spec.md
│   └── ui-features.md
├── providers/                   # AI provider integration (2 files)
│   ├── kimi-wire-vs-acp.md
│   └── kimi-cursor-integration.md
└── analysis/                    # Technical analysis (13 files + demo/)
    ├── agent-provider-status-unification.md
    ├── app-agent-status-analysis.md
    ├── app-bridge-schedule-module.md
    ├── app-coverage-analysis.md
    ├── app-lint-analysis.md
    ├── create-schedule-flow.md
    ├── demo/                      # Demo code (iterm2-agent-detection)
    ├── go-coverage-report.md
    ├── host-status-check.md
    ├── iterm2-agent-observation.md
    ├── lint-capability-plan.md
    ├── README.md
    ├── session-timeline-e2e-gaps.md
    ├── test-suite-analysis.md
    └── tmux-transport-disposed-race.md
```

---

## Assessment: Structure Rationality

### Strengths

- Clear separation by concern (architecture / product / provider / analysis)
- `README.md` provides a complete index
- Architecture docs map to code modules

### Issues

| Issue | Impact |
|-------|--------|
| `analysis/` accumulation (11 files) | Directory bloat; hard to distinguish time-sensitive vs. persistent docs |
| Some analysis docs are "snapshots" not "designs" | e.g. `go-coverage-report.md`, `app-coverage-analysis.md` go stale quickly as code changes |
| `architecture/` overlaps with `.agents/rules/architecture.md` | Module boundaries, data flow described in two places |
| No `decisions/` or ADR directory | Important design decisions scattered across documents |

---

## Assessment: Document Necessity

| Category | File | Necessity | Reason |
|----------|------|-----------|--------|
| **Architecture** | `components.md` | Low | Duplicates `.agents/rules/architecture.md` and `solo-dev-base` skill |
| **Architecture** | `data-flow.md` | Low | Same; data flow already covered in rules |
| **Architecture** | `network-architecture.md` | High | Contains production-specific config (IP, domain, ports) |
| **Architecture** | `deployment.md` | High | Contains deployment workflow, Systemd config |
| **Architecture** | `timeline-design.md` | High | Core design; hard to infer full intent from code alone |
| **Architecture** | `session-memory-persistence.md` | High | Design doc, paired with implementation spec |
| **Architecture** | `agent-stall-detection.md` | High | Critical mechanism design |
| **Architecture** | `push-notifications.md` | High | Third-party integration architecture |
| **Product** | `features.md` | Medium | Derivable from code, but valuable as product overview |
| **Product** | `product-analysis.md` | Low | One-time analysis, not a persistent document |
| **Product** | `session-memory-spec.md` | High | Implementation spec, development reference |
| **Product** | `ui-features.md` | Low | UI changes fast; doc goes stale quickly |
| **Provider** | `kimi-wire-vs-acp.md` | High | Technology selection decision record |
| **Provider** | `kimi-cursor-integration.md` | Medium | Contains both implemented and planned items; needs cleanup |
| **Analysis** | `go-coverage-report.md` | Very Low | Snapshot; should be generated from CI, not hand-written |
| **Analysis** | `app-coverage-analysis.md` | Very Low | Same |
| **Analysis** | `test-suite-analysis.md` | Medium | Has reference value but goes stale |
| **Analysis** | `app-lint-analysis.md` | Very Low | Snapshot; lint status should be viewed from CI |
| **Analysis** | `lint-capability-plan.md` | Medium | Planning doc; Phase 1 already complete |
| **Analysis** | Other 6 analysis files | Medium | One-time analyses; diminishing reference value |

---

## Recommendations

### 1. Slim Down `analysis/` Directory

**Delete** (snapshot docs; should be obtained from CI/tooling):
- `go-coverage-report.md` → view from Codecov
- `app-coverage-analysis.md` → view from Codecov
- `app-lint-analysis.md` → view from `expo lint` output

**Archive** (one-time analyses; preserve but mark as stale):
- Move to `docs/analysis/archive/` subdirectory
- Or add `> Snapshot from 2026-05-XX. May be outdated.` at top of file

### 2. Eliminate Duplication with `.agents/rules/`

`docs/architecture/components.md` and `docs/architecture/data-flow.md` are already covered by `.agents/rules/architecture.md` and the `solo-dev-base` skill.

**Recommendation:**
- `components.md` → Delete; merge content into `.agents/rules/architecture.md`
- `data-flow.md` → Delete; merge content into `.agents/rules/architecture.md`
- `docs/architecture/README.md` → Keep, but slim down to "detailed design doc index"; no longer repeat module descriptions

### 3. Add `docs/decisions/` Directory (ADR)

Important design decisions should have independent records:
```
docs/decisions/
├── 001-kimi-wire-mode.md        # Migrate from providers/
├── 002-session-memory-storage.md # Extract from session-memory-persistence.md
└── 003-timeline-dedup.md         # Extract from timeline-design.md
```

### 4. Establish Document Lifecycle Rules

| Type | Retention Policy | Example |
|------|-----------------|---------|
| **Design docs** | Long-term; update with code | `timeline-design.md`, `session-memory-persistence.md` |
| **Implementation specs** | Long-term; sync with code | `session-memory-spec.md` |
| **Analysis snapshots** | Archive after 6 months | `*-coverage-report.md`, `*-analysis.md` |
| **Planning docs** | Archive after completion | `lint-capability-plan.md` |

### 5. Target Structure (Simplified)

```
docs/
├── README.md                    # Master index (keep)
├── architecture/                # Slim to core designs
│   ├── network-architecture.md  # Production config
│   ├── deployment.md            # Deployment workflow
│   ├── timeline-design.md       # Core design
│   ├── session-memory-persistence.md  # Core design
│   ├── agent-stall-detection.md # Core design
│   └── push-notifications.md    # Third-party integration
├── decisions/                   # New: ADR
│   └── 001-kimi-wire-mode.md
├── product/                     # Slim
│   ├── features.md              # Keep (product overview)
│   └── session-memory-spec.md   # Keep (implementation spec)
├── providers/                   # Keep
│   ├── kimi-wire-vs-acp.md
│   └── kimi-cursor-integration.md
└── analysis/                    # Major reduction
    └── archive/                 # Archive old analyses
```

---

## Summary

| Dimension | Current | Recommendation |
|-----------|---------|----------------|
| File count | 34 | ~12 (delete/archive 22) |
| Overlap with rules | 2 files | 0 |
| Snapshot docs | 4 | 0 (archive or delete) |
| ADR | None | New directory |

**Core principle: docs/ should store design intent and decision records, not snapshots derivable from code/CI.**
