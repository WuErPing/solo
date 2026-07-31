# Solo — Documentation Index

> **Purpose**: Persistent context base for Solo development, CI/CD, and architecture decisions.
> **Organizing principle**: PADD (Product & Architecture Driven Development) — all context as code, co-located with the repo.
> **Last updated**: 2026-07-30

---

## Directory Structure

```
docs/
├── README.md                              ← You are here (master index / SSOT entry point)
├── configuration.md                       # ~/.solo/ config files reference
├── examples/                              ← Config file examples
│   └── config.json.example
│
├── product/                               ← ── Product-side facts ──
│   ├── roadmap-2026.md                    # North-star roadmap (vision, pillars, KPIs)
│   ├── features.md                        # Full feature tree + UI component catalogue
│   ├── agent-send-presets-design.md       # Agent send button presets design
│   ├── prd/                               # Per-feature PRD / specs
│   │   ├── loop-schedule-spec.md          # Loop-as-Schedule unification spec
│   │   └── provider-hub-design.md         # Provider Hub / CC-Switch migration design
│   └── assets/roadmap-2026/               # Roadmap diagrams
│
├── architecture/                          ← ── Architecture-side facts ──
│   ├── README.md                          # Multi-view overview & diagrams
│   ├── components.md                      # Component contracts (App·Bridge·Daemon·Relay·CLI·Protocol)
│   ├── data-flow.md                       # Network topology, WS message flow, E2EE, session lifecycle
│   ├── deployment.md                      # How to deploy (commands, configs, troubleshooting)
│   ├── push-notifications.md              # Expo push token chain
│   ├── schedule-assistant.md              # Chat-based NL schedule assistant
│   ├── session-memory-persistence.md      # Turn recording & memory layer
│   ├── agent-stall-detection.md           # Stuck-loop detection & grace fix
│   ├── timeline-design.md                 # Head/Tail buffers, Seq Gate, deduplication
│   ├── tmux-pane-content-loading.md       # Tmux agent detection, capture, push refresh, diffing
│   └── *.svg / *.png                      # Architecture diagrams
│
├── decisions/                             ← ── ADR (Architecture Decision Records) ──
│   ├── README.md                          # ADR index + conventions
│   ├── adr-template.md                    # Template (includes tech-debt repayment section)
│   └── adr-001-*.md                       # Shared Agent Template for Loop & Schedule
│
├── debt/                                  ← ── Tech Debt Registry (PADD §4) ──
│   └── README.md                          # Registry rules + entry template
│
├── infrastructure/                        ← ── Infra facts (PADD §5, independent layer) ──
│   ├── README.md                          # Layer overview
│   ├── topology.md                        # Physical deployment, servers, DNS, network path
│   └── capacity.md                        # QPS limits, connection limits, scaling ceiling
│
├── verification/                          ← ── Verification Strategy (PADD §6) ──
│   ├── README.md                          # Six-layer verification mapping for Solo
│   ├── test-strategy.md                   # TDD rules, pyramid, commands, coverage goals
│   └── reports/                           # Audit & coverage reports
│       ├── test-coverage.md
│       └── test-quality-audit-2026-07.md
│
├── process/                               ← ── Process facts (PADD §3-4) ──
│   └── review-checklist.md                # Dual-impact review (product + architecture)
│
├── design/                                ← Feature design proposals
│   └── tmux-keybar-ui-redesign.md         # Three-layer keybar (Implemented 2026-07-28)
│
├── providers/                             ← AI provider integration research
│   ├── kimi-wire-vs-acp.md
│   └── kimi-cursor-integration.md
│
├── release/                               ← Release process (build + deploy)
│   ├── README.md                          # Pipeline overview & module index
│   ├── versioning.md                      # SemVer rules, CHANGELOG, tags
│   ├── relay.md / daemon.md / cli.md / mobile-app.md
│
└── analysis/                              ← Deep-dive analyses (staging area)
    ├── README.md                          # Sub-index + graduation rule
    └── *.md                               # Subsystem analyses, reviews
```

---

## 0 · Decisions

Architecture Decision Records capture significant design decisions, including context, alternatives, consequences, and any accepted tech debt.

| Document | Status | Summary |
|----------|--------|---------|
| [ADR-001: Shared Agent Template](decisions/adr-001-shared-agent-template-for-loop-and-schedule.md) | Accepted | Unify Loop and Schedule on `protocol.AgentSessionConfig` (`AgentTemplate`). |

Conventions & template: [`decisions/README.md`](decisions/README.md)

---

## 1 · Tech Debt Registry

All architectural compromises with repayment windows (PADD §4).

→ [`debt/README.md`](debt/README.md)

---

## 2 · Architecture

System design, component contracts, and runtime behaviour.

| Document | Type | Summary |
|----------|------|---------|
| [Architecture Overview](architecture/README.md) | Reference | Layer diagram, component table, quick links |
| [Components](architecture/components.md) | Reference | App · App-Bridge · Daemon · Relay · CLI · Protocol |
| [Data Flow](architecture/data-flow.md) | Reference | Network topology, WS flow, E2EE, Pairing Link, session lifecycle |
| [Timeline Design](architecture/timeline-design.md) | Design | Head/Tail buffers, Seq Gate, bootstrap, batching |
| [Session Memory Persistence](architecture/session-memory-persistence.md) | Design | TurnRecorder, `~/.solo/memory/` layout |
| [Agent Stall Detection](architecture/agent-stall-detection.md) | Design | Inactivity & repetition detection, grace-period |
| [Deployment](architecture/deployment.md) | Runbook | Systemd, Docker, Nginx, ports, troubleshooting |
| [Push Notifications](architecture/push-notifications.md) | Design | Expo push token chain |
| [Tmux Pane Content Loading](architecture/tmux-pane-content-loading.md) | Design | Agent detection, ANSI capture, push refresh, snapshot diffing |
| [Schedule Assistant](architecture/schedule-assistant.md) | Design | NL schedule parse, proposal-only safety, `schedule/assist` RPC |

**Key facts (always-on context)**:
- Daemon: `127.0.0.1:17612`; Relay: `127.0.0.1:8081` (behind Nginx :443)
- Production relay: `solo.up2ai.top:443` (never raw IP:8081)
- E2EE: X25519 + XSalsa20-Poly1305
- Pairing Link: `https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}`

---

## 3 · Infrastructure

Physical environment facts — independent of product, shared across services.

| Document | Summary |
|----------|---------|
| [Topology](infrastructure/topology.md) | Server inventory, DNS, network path, security group constraints |
| [Capacity](infrastructure/capacity.md) | Connection limits, buffer sizes, scaling ceiling, known bottlenecks |

---

## 4 · Product

Roadmap, feature inventory, and per-feature PRD/specs.

| Document | Type | Summary |
|----------|------|---------|
| [2026 Roadmap](product/roadmap-2026.md) | Roadmap | Vision, three pillars, quarterly plan, KPIs, risks |
| [Features](product/features.md) | Analysis | Full feature tree + UI component catalogue |
| [Loop Schedule Spec](product/prd/loop-schedule-spec.md) | PRD/Spec | Loop-as-Schedule unification: protocol, daemon, executors, migration |
| [Provider Hub Design](product/prd/provider-hub-design.md) | PRD/Design | Provider Hub, Local API Proxy, MCP/Skills/Prompts, config exporter |
| [Product Layer Evolution](product/prd/product-layer-evolution.md) | PRD | Absorb Multica abstractions: Skills, Profiles, unified Task, Board, Inbox, Triggers |
| [Agent Send Presets](product/agent-send-presets-design.md) | Design | Agent-specific tmux send button presets |

---

## 5 · Verification

Six-layer verification strategy and reports (PADD §6).

| Document | Summary |
|----------|---------|
| [Verification Strategy](verification/README.md) | Six-layer mapping to Solo tooling |
| [Test Strategy](verification/test-strategy.md) | TDD rules, pyramid, commands, coverage goals |
| [Test Coverage Report](verification/reports/test-coverage.md) | Unified coverage: Go + App + E2E |
| [Test Quality Audit](verification/reports/test-quality-audit-2026-07.md) | Unit-test quality issues + fix tracking |

---

## 6 · Process

Governance artifacts for the PADD dual-driven workflow.

| Document | Summary |
|----------|---------|
| [Dual-Impact Review Checklist](process/review-checklist.md) | Product + architecture + debt + verification review gates |

---

## 7 · Design

Feature design proposals (pre-implementation or recently implemented).

| Document | Status | Summary |
|----------|--------|---------|
| [Tmux Keybar UI Redesign](design/tmux-keybar-ui-redesign.md) | Implemented (2026-07-28) | Three-layer keybar layout |

---

## 8 · Providers

AI provider integration research and implementation plans.

| Document | Summary |
|----------|---------|
| [Kimi Wire vs ACP](providers/kimi-wire-vs-acp.md) | Wire mode recommended (full feature set, stdio-only) |
| [Kimi & Cursor-Agent Integration](providers/kimi-cursor-integration.md) | Wire for Kimi; Print for Cursor-Agent |

**Implemented**: Claude, Kimi (Wire), OpenCode (SSE), Pi, Codex.
**Dev-only**: Mock (`SOLO_ENABLE_MOCK_PROVIDER=1`). **Planned**: Cursor-Agent.

---

## 9 · Technical Analysis

Deep dives — a staging area. Conclusions graduate to `decisions/`, `architecture/`, or `verification/`.

→ [`analysis/README.md`](analysis/README.md) (full sub-index with status and chronology)

---

## 10 · Release

How to cut a release — build and deploy per module.

| Document | Summary |
|----------|---------|
| [Release Process](release/README.md) | Pipeline: version → CHANGELOG → tag → build → deploy → verify |
| [Versioning](release/versioning.md) | Per-module version locations, SemVer, tags |
| [Relay](release/relay.md) | Build + scp/restart, verify, rollback |
| [Daemon](release/daemon.md) | Build + user-systemd deploy |
| [CLI](release/cli.md) | Build + binary distribution |
| [Mobile App](release/mobile-app.md) | EAS profiles, cloud build, store submit |

---

## 11 · Build & CI/CD Quick Reference

### Build targets

| Target | Command | Output |
|--------|---------|--------|
| Darwin binaries | `make darwin` | `output/darwin/{solo,solo-relay,solo-cli,solo-usage}` |
| Linux binaries | `make linux` | `output/linux/{solo,solo-relay,solo-cli}` |
| Dev (daemon + web) | `make dev` | daemon :17612 + Expo :19000 |
| Deploy relay | `make deploy-solo-relay` | scp + systemctl restart |

### CI pipeline

| Workflow | Job | Steps |
|----------|-----|-------|
| `ci.yml` | `go` (matrix) | `go mod verify` → build → test → coverage → golangci-lint v2 |
| `ci.yml` | `js` | npm ci → lint → typecheck → test → coverage |
| `ci.yml` | `arch-boundaries` | `scripts/check-arch-boundaries.sh` |
| `e2e-nightly.yml` | `e2e-nightly` | Playwright (43 specs), daily 02:00 UTC |
| `semantic-check.yml` | `adr-consistency` | Advisory LLM ADR check (never blocks) |

### Tech stack

| Layer | Stack |
|-------|-------|
| Backend | Go 1.25 · gorilla/websocket · creack/pty · slog |
| Frontend | Expo 57 · React Native 0.86 · React 19 · TypeScript |
| State | Zustand · @tanstack/react-query · React Context |
| Crypto | X25519 + XSalsa20-Poly1305 (E2EE) |
| Deploy | Systemd · Docker · Nginx + Let's Encrypt |
| CI | GitHub Actions · golangci-lint v2 · ESLint |

---

## 12 · How to Use These Docs

1. **Starting a feature** → read `architecture/` first, then check `product/prd/` for specs.
2. **Making a decision** → check `decisions/` for existing ADRs; use `adr-template.md` for new ones.
3. **Accepting a compromise** → create a `debt/` entry with repayment window (PADD rule).
4. **Reviewing a change** → use `process/review-checklist.md` (dual-impact: product + architecture).
5. **Understanding capacity** → `infrastructure/` for topology and scaling limits.
6. **Planning verification** → `verification/README.md` for the six-layer model.
7. **Adding a provider** → `providers/` for protocol decisions, then `architecture/components.md`.
8. **Debugging connectivity** → `architecture/data-flow.md` + `architecture/deployment.md`.
9. **Cutting a release** → `release/README.md` + `release/versioning.md`.
10. **Agent/context boot** → the `solo-dev-base` skill loads key facts from this index automatically.
