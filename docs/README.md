# Solo — Documentation Index

> **Purpose**: Persistent context base for Solo development, CI/CD, and architecture decisions.
> **Last updated**: 2026-07-29

---

## Directory Structure

```
docs/
├── README.md                              ← You are here (master index)
├── configuration.md                       # ~/.solo/ config files reference (config.json, usage.json, placeholders)
├── examples/                              ← Config file examples
│   └── config.json.example                # Daemon config.json full-field reference
├── architecture/                          ← System architecture & design
│   ├── README.md                          # Architecture overview & diagrams
│   ├── agent-stall-detection.md           # Agent stuck-loop detection & grace fix
│   ├── components.md                      # Component specifications
│   ├── data-flow.md                       # Network topology, message flows, E2EE, session lifecycle
│   ├── deployment.md                      # Deployment, Nginx, Systemd, Docker, ports, troubleshooting
│   ├── push-notifications.md              # Push notification architecture
│   ├── schedule-assistant.md              # Chat-based schedule assistant (architecture + product context)
│   ├── session-memory-persistence.md      # Session turn recording & memory layer (design + acceptance criteria)
│   ├── solo-system-architecture.png       # System architecture diagram (PNG)
│   ├── solo-system-architecture.svg       # System architecture diagram (SVG)
│   ├── solo-system-architecture-detailed.png  # Detailed architecture diagram (PNG)
│   ├── solo-system-architecture-detailed.svg  # Detailed architecture diagram (SVG)
│   ├── timeline-design.md                 # Head/Tail model, seq gate, deduplication
│   └── tmux-pane-content-loading.md       # Tmux agent detection, pane capture, push refresh, key injection
├── decisions/                             ← Architecture Decision Records (ADRs)
│   └── adr-001-shared-agent-template-for-loop-and-schedule.md  # Shared AgentTemplate for Loop & Schedule
├── design/                                ← Feature design proposals
│   └── tmux-keybar-ui-redesign.md         # Tmux keybar three-layer layout (Implemented 2026-07-28)
├── product/                               ← Product feature analysis
│   ├── agent-profile-switch-export-design.md # Provider Hub / CC-Switch migration design
│   ├── agent-send-presets-design.md       # Agent send button presets design
│   ├── features.md                        # Full product feature analysis + UI component catalogue
│   ├── loop-schedule-spec.md              # Loop Schedule implementation spec
│   ├── roadmap-2026.md                    # 2026 product/technical roadmap
│   └── assets/roadmap-2026/               # Roadmap diagrams (architecture, flywheel, pillars, tech-tree)
├── providers/                             ← AI provider integration research
│   ├── kimi-wire-vs-acp.md                # Kimi Wire vs ACP protocol comparison
│   └── kimi-cursor-integration.md         # Cursor-Agent integration plan (Kimi: done)
└── analysis/                              ← Deep-dive technical analysis (see analysis/README.md)
    ├── README.md                          # Analysis directory index
    ├── demo/                              # Demo code (iterm2-agent-detection)
    └── *.md                               # Subsystem analyses, reviews, and decision records
```

---

## 0 · Decisions

Architecture Decision Records (ADRs) capture significant design decisions that shape the codebase, including context, alternatives considered, and consequences.

| Document | Status | Summary |
|----------|--------|---------|
| [ADR-001: Shared Agent Template for Loop and Schedule](decisions/adr-001-shared-agent-template-for-loop-and-schedule.md) | Accepted | Unify Loop and Schedule agent configuration on `protocol.AgentSessionConfig` (the shared `AgentTemplate`) to eliminate duplication and unblock Loop-as-Schedule unification. |

---

## 1 · Architecture

System design, component contracts, and runtime behaviour.

| Document | Type | Audience | Summary |
|----------|------|----------|---------|
| [Architecture Overview](architecture/README.md) | Reference | All | Layer diagram, component table, quick links |
| [Components](architecture/components.md) | Reference | Dev | App · App-Bridge · Daemon · Relay · CLI · Protocol |
| [Data Flow](architecture/data-flow.md) | Reference | Dev / Infra | Network topology (Nginx → Relay → Daemon), WS message flow, E2EE handshake, Pairing Link, session lifecycle, heartbeat |
| [Timeline Design](architecture/timeline-design.md) | Design | Dev | Head/Tail buffers, Seq Gate, bootstrap, batching, idempotent Append |
| [Session Memory Persistence](architecture/session-memory-persistence.md) | Design | Dev | Hook points, TurnRecorder interface, `~/.solo/memory/` layout, acceptance criteria |
| [Agent Stall Detection](architecture/agent-stall-detection.md) | Design | Dev | Inactivity & repetition detection, grace-period tightening, operational tuning |
| [Deployment](architecture/deployment.md) | Runbook | Infra / CI | Systemd, Docker, Nginx config, port mapping, env vars, monitoring, troubleshooting |
| [Push Notifications](architecture/push-notifications.md) | Design | Dev | Expo push token chain, phases, reliability |
| [Tmux Pane Content Loading](architecture/tmux-pane-content-loading.md) | Design | Dev | Tmux agent detection, pane capture with ANSI rendering, push-driven refresh, snapshot diffing, keystroke injection, terminal themes |
| [Schedule Assistant](architecture/schedule-assistant.md) | Design | Dev | Chat-based NL schedule parse via the host's configured LLM Providers, proposal-only safety invariant, confirm path, `schedule/assist` RPC |
| [System Architecture Diagram](architecture/solo-system-architecture.svg) | Diagram | All | Visual system architecture (SVG) — [PNG version](architecture/solo-system-architecture.png) |
| [Detailed Architecture Diagram](architecture/solo-system-architecture-detailed.svg) | Diagram | All | Detailed component view (SVG) — [PNG version](architecture/solo-system-architecture-detailed.png) |

**Key facts (always-on context)**:
- Daemon listens `127.0.0.1:17612`; Relay listens `127.0.0.1:8081` (behind Nginx :443)
- Production relay endpoint: `solo.up2ai.top:443` (never use raw IP:8081)
- E2EE: X25519 key exchange + XSalsa20-Poly1305
- Pairing Link format: `https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}`

---

## 2 · Design

Feature design proposals and UI redesigns.

| Document | Status | Summary |
|----------|--------|---------|
| [Tmux Keybar UI Redesign](design/tmux-keybar-ui-redesign.md) | Implemented (2026-07-28) | Three-layer keybar layout (Contextual Strip / Primary / Expanded) for the tmux pane screen |

---

## 3 · Product

Feature inventory and UI/UX analysis.

| Document | Type | Summary |
|----------|------|---------|
| [2026 Product/Technical Roadmap](product/roadmap-2026.md) | Roadmap | Unified roadmap: vision, three product pillars, quarterly plan, KPIs, risks |
| [Product Features](product/features.md) | Analysis | Full feature tree + UI component catalogue + hooks/stores reference |
| [Provider Hub / CC-Switch Migration Design](product/agent-profile-switch-export-design.md) | Design | Migrate farion1231/cc-switch into Solo: Provider Hub, Local API Proxy, MCP/Skills/Prompts management, and multi-agent config exporter |
| [Loop Schedule Implementation Spec](product/loop-schedule-spec.md) | Spec | Implementation-ready spec for merging Loop into Schedule: protocol changes, daemon modules, step executors, migration plan |
| [Agent Send Presets Design](product/agent-send-presets-design.md) | Design | Agent-specific tmux send button presets |

---

## 4 · Providers

AI provider integration research and implementation plans.

| Document | Type | Summary |
|----------|------|---------|
| [Kimi Wire vs ACP](providers/kimi-wire-vs-acp.md) | Comparison | Wire mode recommended for Solo (full Kimi feature set, stdio-only) |
| [Kimi & Cursor-Agent Integration](providers/kimi-cursor-integration.md) | Implementation plan | Wire mode for Kimi; Print mode for Cursor-Agent; backend Go registration |

**Currently implemented providers**: Claude (print/stream-json), Kimi (Wire mode, JSON-RPC 2.0 stdio), OpenCode (SSE), Pi (minimal terminal harness), Codex (auto/full-access modes).

**Development-only**: Mock (opt-in via `SOLO_ENABLE_MOCK_PROVIDER=1`).

**Removed**: Copilot. **Planned**: Cursor-Agent (Print mode).

---

## 5 · Technical Analysis

Deep dives into specific subsystems. The [analysis/README.md](analysis/README.md) sub-index tracks status and chronology.

| Document | Type | Summary |
|----------|------|---------|
| [Architecture Review (2026-06-12)](analysis/architecture-review-2026-06-12/) | Review | 4+1 views, maturity scoring, ATAM evaluation, improvement recommendations |
| [Architecture First-Principles Review (2026-06-18)](analysis/architecture-first-principles-review-2026-06-18.md) | Review | First-principles evaluation of all architectural decisions, long-term risk identification |
| [Solo Roadmap Architecture Mapping (2026-06-20)](analysis/solo-roadmap-architecture-mapping.md) | Design | Maps existing Solo features to 2026 roadmap pillars; layered architecture and phased plan |
| [Security Deep Analysis (2026-07-25)](analysis/security-deep-analysis-2026-07-25.md) | Analysis | Threat model + High/Medium/Low findings and remediation roadmap |
| [App Performance Analysis (2026-07-24)](analysis/app-performance-analysis-2026-07-24.md) | Analysis | Frontend performance findings and resolutions |
| [Coding Agent Hooks Comparison (2026-07-26)](analysis/coding-agent-hooks-comparison.md) | Reference | Hook systems across 7 coding agents + Solo design suggestions |
| [Agent/Provider Status Unification](analysis/agent-provider-status-unification.md) | Design | OCP-based proposal to unify AgentLifecycleStatus, ProviderStatus across layers |
| [App Agent Status Analysis](analysis/app-agent-status-analysis.md) | Analysis | App agent lifecycle states and Copy button display logic |
| [App-Bridge Schedule Module](analysis/app-bridge-schedule-module.md) | Analysis | Schedule module type contract, RPC schema, and domain models |
| [Create Schedule Flow](analysis/create-schedule-flow.md) | Analysis | End-to-end schedule creation flow with timezone-aware cron scheduling |
| [Dead Code Analysis (2026-06-19)](analysis/dead-code-analysis-2026-06-19.md) | Analysis | Dead code identification and phased removal plan |
| [Go Provider Type Erasure](analysis/go-provider-type-erasure-analysis.md) | Analysis | `interface{}` / `map[string]interface{}` growth diagnosis, remediation strategies |
| [Host Status Check](analysis/host-status-check.md) | Analysis | Probe cycle (2-30 s), adaptive switching, state machine conflict, grace-period fix |
| [Test Coverage](analysis/test-coverage.md) | Report | Unified coverage report: Go backend + App frontend + E2E + CI/Codecov |
| [Test Quality Audit (2026-07)](analysis/test-quality-audit-2026-07.md) | Report | Unit-test quality audit + prioritized fix tracking |
| [Tmux Discovery & Refresh (2026-07-24)](analysis/tmux-discovery-refresh-analysis-2026-07-24.md) | Analysis | Source of truth for agent discovery (4-layer) + adaptive refresh tuning |
| [Tmux Pane Rendering Decision](analysis/tmux-pane-rendering-decision.md) | Decision | Client-side xterm.js rendering chosen & implemented; control-mode alternative deferred |
| [Tmux Project Matcher](analysis/tmux-project-matcher.md) | Spec | Matching tmux panes to projects for the sidebar badge (implemented) |
| [Tmux Keybar Layout Analysis (2026-07-28)](analysis/tmux-keybar-layout-analysis-2026-07-28.md) | Analysis | Post-implementation UX audit of the three-layer keybar |
| [Tmux Agent Misidentification (kimi → kimi-code)](analysis/tmux-agent-misidentification-kimi-code-2026-06-15.md) | Analysis | `kimi --yolo` misidentified as `kimi-code`: root causes + fixes |
| [Tmux Transport Disposed Race](analysis/tmux-transport-disposed-race.md) | Analysis | `Transport not connected (status: disposed)` root cause + retry fix |
| [iTerm2 Agent Observation](analysis/iterm2-agent-observation.md) | Analysis | iTerm2 agent detection observation + CLI discovery |

---

## 6 · Build & CI/CD Quick Reference

> Full commands live in `Makefile`, `.github/workflows/ci.yml`, and `.github/workflows/e2e-nightly.yml`.

### Build targets

| Target | Command | Output |
|--------|---------|--------|
| Darwin binaries | `make darwin` | `output/darwin/{solo,solo-relay,solo-cli,solo-usage}` |
| Linux binaries | `make linux` | `output/linux/{solo,solo-relay,solo-cli}` (excludes solo-usage) |
| Dev (daemon + web) | `make dev` | daemon :17612 + Expo :19000 |
| Deploy relay | `make deploy-solo-relay` | scp + systemctl restart |

### CI pipeline

| Workflow | Job | Steps |
|----------|-----|-------|
| `.github/workflows/ci.yml` | `go` (matrix: protocol, cli, daemon, relay-go, usage) | `go mod verify` → `go build -v ./...` → `go test -short -race -coverprofile=coverage.out` → upload coverage (Codecov + artifact) → `golangci-lint v2` |
| `.github/workflows/ci.yml` | `js` | `npm ci` → lint app / app-bridge / highlight → typecheck → test (app + app-bridge unit tests) → upload coverage (Codecov + artifacts) |
| `.github/workflows/ci.yml` | `arch-boundaries` | `scripts/check-arch-boundaries.sh` — enforce Go module boundaries |
| `.github/workflows/e2e-nightly.yml` | `e2e-nightly` | daily 02:00 UTC + manual; Playwright E2E (43 specs) with daemon/relay/Metro globalSetup; failure artifacts retained 7 days |
| `.github/workflows/semantic-check.yml` | `adr-consistency` | advisory LLM ADR-consistency check on labeled PRs (never blocks) |

### Tech stack summary

| Layer | Stack |
|-------|-------|
| Backend | Go 1.25 · gorilla/websocket · creack/pty · slog |
| Frontend | Expo 57 · React Native 0.86 · React 19 · TypeScript |
| State | Zustand · @tanstack/react-query · React Context |
| Crypto | X25519 + XSalsa20-Poly1305 (E2EE) |
| Deploy | Systemd · Docker · Nginx + Let's Encrypt |
| CI | GitHub Actions · golangci-lint v2 · ESLint |

---

## 7 · How to Use These Docs

1. **Starting a feature** → read the relevant Architecture doc first, then check Product for existing coverage.
2. **Making or revisiting an architectural decision** → check `decisions/` for ADRs that record the context, alternatives, and consequences.
3. **Adding a provider** → read `providers/` docs for protocol decisions, then `architecture/components.md` § Daemon.
4. **Debugging connectivity** → `architecture/data-flow.md` (topology, port ACL, Pairing Link) + `architecture/deployment.md` (troubleshooting).
5. **CI/CD changes** → check § 6 above + `Makefile` + `.github/workflows/ci.yml`.
6. **Agent/context boot** → the `solo-dev-base` skill (`.agents/skills/solo-dev-base/SKILL.md`) loads key facts from this index automatically.
