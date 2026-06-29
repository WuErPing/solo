# Solo — Documentation Index

> **Purpose**: Persistent context base for Solo development, CI/CD, and architecture decisions.
> **Last updated**: 2026-06-28

---

## Directory Structure

```
docs/
├── README.md                              ← You are here (master index)
├── architecture/                          ← System architecture & design
│   ├── README.md                          # Architecture overview & diagrams
│   ├── agent-stall-detection.md           # Agent stuck-loop detection & grace fix
│   ├── components.md                      # Component specifications
│   ├── data-flow.md                       # Message flows & session lifecycle
│   ├── deployment.md                      # Deployment, Nginx, Systemd, Docker
│   ├── network-architecture.md            # Network paths, E2EE, Pairing Link
│   ├── network-data-state-architecture.md # Network + data + state synthesis
│   ├── push-notifications.md              # Push notification architecture
│   ├── session-memory-persistence.md      # Session turn recording & memory layer design
│   ├── solo-system-architecture.png       # System architecture diagram (PNG)
│   ├── solo-system-architecture.svg       # System architecture diagram (SVG)
│   ├── solo-system-architecture-detailed.png  # Detailed architecture diagram (PNG)
│   ├── solo-system-architecture-detailed.svg  # Detailed architecture diagram (SVG)
│   ├── timeline-design.md                 # Head/Tail model, seq gate, deduplication
│   └── tmux-pane-content-loading.md       # Tmux agent detection, pane capture, polling, key injection
├── decisions/                             ← Architecture Decision Records (ADRs)
│   └── adr-001-shared-agent-template-for-loop-and-schedule.md  # Shared AgentTemplate for Loop & Schedule
├── product/                               ← Product feature analysis
│   ├── agent-profile-switch-export-design.md # Provider Hub / CC-Switch migration design
│   ├── agent-send-presets-design.md       # Agent send button presets design
│   ├── feature-directions-2026.md         # Feature direction analysis with industry benchmark
│   ├── features.md                        # Full product feature analysis + UI component catalogue
│   ├── loop-schedule-deep-dive.md         # Loop Schedule deep-dive analysis
│   ├── loop-schedule-design.md            # LLM-driven Loop Schedule design
│   ├── loop-schedule-spec.md              # Loop Schedule implementation spec
│   ├── roadmap-2026.md                    # 2026 product/technical roadmap
│   └── session-memory-spec.md             # Session memory Phase-1 implementation spec
├── providers/                             ← AI provider integration research
│   ├── kimi-wire-vs-acp.md               # Kimi Wire vs ACP protocol comparison
│   └── kimi-cursor-integration.md         # Cursor-Agent integration plan (Kimi: done)
└── analysis/                              ← Deep-dive technical analysis
    ├── agent-provider-status-unification.md # Agent/provider status unification design
    ├── app-agent-status-analysis.md         # App agent status & Copy button logic
    ├── app-bridge-schedule-module.md        # Schedule module type contract & RPC schema
    ├── architecture-first-principles-review-2026-06-18.md # First-principles architecture review
    ├── architecture-review-2026-06-12/     # Architecture review (4+1 views, maturity, recommendations)
    ├── create-schedule-flow.md              # End-to-end schedule creation flow
    ├── dead-code-analysis-2026-06-19.md    # Dead code analysis
    ├── demo/                              # Demo code (iterm2-agent-detection)
    ├── first-turn-completion-signal-loss-2026-06-20.md # First turn completion signal loss analysis
    ├── go-provider-type-erasure-analysis.md # interface{} growth diagnosis + remediation
    ├── host-status-check.md                 # Host probe cycle & status machine
    ├── iterm2-agent-observation.md          # iTerm2 agent detection observation
    ├── lint-capability-plan.md              # Lint tooling roadmap (Phase 1 complete)
    ├── makefile-benchmark.md                # Makefile sequential vs parallel CI benchmark
    ├── opencode-cross-device-sync-fix.md   # Cross-client sync bug fix record
    ├── plan-tmux-project-matcher.md        # Tmux project matcher plan
    ├── README.md                            # Analysis directory index
    ├── solo-roadmap-architecture-mapping.md # Roadmap to architecture mapping
    ├── spec-tmux-project-matcher.md        # Tmux project matcher spec
    ├── test-coverage.md                     # 统一测试覆盖率报告 (Go/App/E2E/CI)
    ├── tmux-agent-misidentification-kimi-code-2026-06-15.md # kimi --yolo → kimi-code: 八层防御级联失败深度分析
    ├── tmux-pane-analysis.md               # Tmux pane subsystem: jitter fix + performance + rendering optimization
    ├── tmux-pane-client-emulator-first-principles.md # Client terminal emulator first-principles analysis
    ├── tmux-pane-first-principles-2026-06-20.md # Tmux pane first-principles analysis
    └── tmux-transport-disposed-race.md      # Tmux Transport disposed race analysis
```

---

## 1 · Architecture

System design, component contracts, and runtime behaviour.

| Document | Type | Audience | Summary |
|----------|------|----------|---------|
| [Architecture Overview](architecture/README.md) | Reference | All | Layer diagram, component table, quick links |
| [Components](architecture/components.md) | Reference | Dev | App · App-Bridge · Daemon · Relay · CLI · Protocol |
| [Data Flow](architecture/data-flow.md) | Reference | Dev | WS message flow, E2EE handshake, session lifecycle, heartbeat |
| [Network Architecture](architecture/network-architecture.md) | Deep-dive | Dev / Infra | Nginx → Relay → Daemon paths, port ACL, Pairing Link protocol |
| [Network · Data · State Architecture](architecture/network-data-state-architecture.md) | Synthesis | Dev / Architect | End-to-end tie-up of network paths, data stores (Timeline + Memory), and state sync (Seq Gate / Head-Tail / cursor) |
| [Session Memory Persistence](architecture/session-memory-persistence.md) | Design | Dev | Hook points, TurnRecorder interface, file layout, migration path to DB / memory middleware |
| [Agent Stall Detection](architecture/agent-stall-detection.md) | Design | Dev | Inactivity & repetition detection, grace-period tightening, operational tuning |
| [Deployment](architecture/deployment.md) | Runbook | Infra / CI | Systemd, Docker, Nginx config, env vars, monitoring, troubleshooting |
| [Tmux Pane Content Loading](architecture/tmux-pane-content-loading.md) | Design | Dev | Tmux agent detection, pane capture with ANSI rendering, lazy history loading, keystroke injection, terminal themes |
| [System Architecture Diagram](architecture/solo-system-architecture.svg) | Diagram | All | Visual system architecture (SVG) — [PNG version](architecture/solo-system-architecture.png) |
| [Detailed Architecture Diagram](architecture/solo-system-architecture-detailed.svg) | Diagram | All | Detailed component view (SVG) — [PNG version](architecture/solo-system-architecture-detailed.png) |

**Key facts (always-on context)**:
- Daemon listens `127.0.0.1:17612`; Relay listens `127.0.0.1:8081` (behind Nginx :443)
- Production relay endpoint: `solo.up2ai.top:443` (never use raw IP:8081)
- E2EE: X25519 key exchange + XSalsa20-Poly1305
- Pairing Link format: `https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}`

---

## 2 · Architecture Decision Records

Formal records of significant architectural decisions, including context, alternatives considered, and consequences.

| Document | Status | Summary |
|----------|--------|---------|
| [ADR-001: Shared Agent Template for Loop and Schedule](decisions/adr-001-shared-agent-template-for-loop-and-schedule.md) | Accepted | Introduce `AgentTemplate` as a shared user-facing preset for Loop and Schedule, distinct from runtime `AgentSessionConfig` |

---

## 3 · Product

Feature inventory and UI/UX analysis.

| Document | Type | Summary |
|----------|------|---------|
| [2026 Product/Technical Roadmap](product/roadmap-2026.md) | Roadmap | Unified roadmap: vision, three product pillars, quarterly plan, KPIs, risks |
| [Product Features](product/features.md) | Analysis | Full feature tree + UI component catalogue + hooks/stores reference |
| [Feature Directions 2026](product/feature-directions-2026.md) | Analysis | Original feature direction analysis with industry benchmark |
| [Provider Hub / CC-Switch Migration Design](product/agent-profile-switch-export-design.md) | Design | Migrate farion1231/cc-switch into Solo: Provider Hub, Local API Proxy, MCP/Skills/Prompts management, and multi-agent config exporter |
| [Loop Schedule Design](product/loop-schedule-design.md) | Design | LLM-driven Loop Schedule: evolve schedule into autonomous iteration loops using existing providers |
| [Loop Schedule Deep Dive](product/loop-schedule-deep-dive.md) | Analysis | Deep-dive into Loop Controller, Step Executor, state machine, persistence, security, and observability |
| [Loop Schedule Implementation Spec](product/loop-schedule-spec.md) | Spec | Implementation-ready spec for merging Loop into Schedule: protocol changes, daemon modules, step executors, migration plan |
| [Agent Send Presets Design](product/agent-send-presets-design.md) | Design | Agent-specific tmux send button presets |
| [Session Memory Spec](product/session-memory-spec.md) | Spec | Phase-1 implementation spec: TurnRecorder interface, FileTurnRecorder, hooks, redaction, tests |

**Current completion**: ~85-90 %. Main gaps: Chat system (multi-agent), Cursor-Agent / ACP providers.

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

Deep dives into specific subsystems.

| Document | Type | Summary |
|----------|------|---------|
| [Architecture Review (2026-06-12)](analysis/architecture-review-2026-06-12/) | Review | 4+1 views, maturity scoring, ATAM evaluation, improvement recommendations |
| [Architecture First-Principles Review (2026-06-18)](analysis/architecture-first-principles-review-2026-06-18.md) | Review | First-principles evaluation of all architectural decisions, long-term risk identification |
| [Solo Roadmap Architecture Mapping (2026-06-20)](analysis/solo-roadmap-architecture-mapping.md) | Design | Maps existing Solo features to 2026 roadmap pillars; layered architecture and phased implementation plan |
| [Agent/Provider Status Unification](analysis/agent-provider-status-unification.md) | Design | OCP-based proposal to unify AgentLifecycleStatus, ProviderStatus across layers |
| [App Agent Status Analysis](analysis/app-agent-status-analysis.md) | Analysis | App agent lifecycle states and Copy button display logic |
| [App-Bridge Schedule Module](analysis/app-bridge-schedule-module.md) | Analysis | Schedule module type contract, RPC schema, and domain models |
| [Create Schedule Flow](analysis/create-schedule-flow.md) | Analysis | End-to-end schedule creation flow with timezone-aware cron scheduling |
| [Dead Code Analysis (2026-06-19)](analysis/dead-code-analysis-2026-06-19.md) | Analysis | Dead code identification and analysis across the codebase |
| [First Turn Completion Signal Loss (2026-06-20)](analysis/first-turn-completion-signal-loss-2026-06-20.md) | Analysis | Root cause analysis of first-turn completion signal loss |
| [Go Provider Type Erasure](analysis/go-provider-type-erasure-analysis.md) | Analysis | `interface{}` / `map[string]interface{}` growth diagnosis, remediation strategies |
| [Host Status Check](analysis/host-status-check.md) | Analysis | Probe cycle (2-30 s), adaptive switching, state machine conflict, grace-period fix |
| [OpenCode Cross-Device Sync Fix](analysis/opencode-cross-device-sync-fix.md) | Fix record | Root cause and fix for cross-client sync issues |
| [Tmux Project Matcher Plan](analysis/plan-tmux-project-matcher.md) | Design | Plan for matching tmux sessions to projects |
| [Tmux Project Matcher Spec](analysis/spec-tmux-project-matcher.md) | Spec | Implementation spec for tmux project matcher |
| [Test Coverage](analysis/test-coverage.md) | Report | 统一测试覆盖率报告: Go 后端 + App 前端 + E2E + CI/Codecov 集成 |
| [Tmux Agent Misidentification (kimi → kimi-code)](analysis/tmux-agent-misidentification-kimi-code-2026-06-15.md) | Analysis | `kimi --yolo` 被误识为 `kimi-code`: Bun setproctitle 污染源 + 八层防御级联失败 + 修复 |
| [Tmux Pane Analysis](analysis/tmux-pane-analysis.md) | Analysis | Jitter 根因与修复 + 4 层架构瓶颈 + xterm.js 迁移方案 |
| [Makefile Benchmark](analysis/makefile-benchmark.md) | Benchmark | Sequential vs parallel CI targets: test-go, lint, typecheck timing |
| [Tmux Pane Client Emulator First Principles](analysis/tmux-pane-client-emulator-first-principles.md) | Analysis | 第一性原理分析：客户端 terminal emulator 路径 + 两阶段实施方案 |
| [Tmux Pane First Principles (2026-06-20)](analysis/tmux-pane-first-principles-2026-06-20.md) | Analysis | First-principles analysis of tmux pane subsystem |
| [Tmux Transport Disposed Race](analysis/tmux-transport-disposed-race.md) | Analysis | `Transport not connected (status: disposed)` root cause: probe-cycle switch vs. in-flight tmux RPC |

---

## 6 · Build & CI/CD Quick Reference

> Full commands live in `Makefile`, `.github/workflows/ci.yml`, and `.github/workflows/e2e-nightly.yml`.

### Build targets

| Target | Command | Output |
|--------|---------|--------|
| Darwin binaries | `make darwin` | `output/darwin/{solo,solo-relay,solo-cli}` |
| Linux binaries | `make linux` | `output/linux/{solo,solo-relay,solo-cli}` |
| Dev (daemon + web) | `make dev` | daemon :17612 + Expo :19000 |
| Deploy relay | `make deploy-solo-relay` | scp + systemctl restart |

### CI pipeline

| Workflow | Job | Steps |
|----------|-----|-------|
| `.github/workflows/ci.yml` | `go` (matrix: protocol, cli, daemon, relay-go) | `go mod verify` → `go build -v ./...` → `go test -short -race -coverprofile=coverage.out` → upload coverage (Codecov + artifact, 14 days) → `golangci-lint v2.10` (`--timeout=5m`) |
| `.github/workflows/ci.yml` | `js` | `npm ci` → lint app / app-bridge / highlight → typecheck all three → test highlight → test app (unit, **1663 tests**) → test app-bridge (**32 tests**) → upload coverage (Codecov + artifacts) |
| `.github/workflows/e2e-nightly.yml` | `e2e-nightly` | daily 02:00 UTC + manual; Playwright E2E (38 specs) with daemon/relay/Metro globalSetup; failure artifacts retained 7 days |

### Tech stack summary

| Layer | Stack |
|-------|-------|
| Backend | Go 1.25 · gorilla/websocket · creack/pty · slog |
| Frontend | Expo 54 · React Native 0.81 · React 19 · TypeScript |
| State | Zustand · @tanstack/react-query · React Context |
| Crypto | X25519 + XSalsa20-Poly1305 (E2EE) |
| Deploy | Systemd · Docker · Nginx + Let's Encrypt |
| CI | GitHub Actions · golangci-lint v2 · ESLint |

---

## 7 · How to Use These Docs

1. **Starting a feature** → read the relevant Architecture doc first, then check Product for existing coverage.
2. **Making or revisiting an architectural decision** → check `decisions/` for ADRs that record the context, alternatives, and consequences.
3. **Adding a provider** → read `providers/` docs for protocol decisions, then `architecture/components.md` § Daemon.
3. **Debugging connectivity** → `architecture/network-architecture.md` (port ACL, common misconfig) + `architecture/deployment.md` (troubleshooting).
4. **CI/CD changes** → check § 5 above + `Makefile` + `.github/workflows/ci.yml`.
5. **Agent/context boot** → the `solo-dev-base` skill (`.agents/skills/solo-dev-base/SKILL.md`) loads key facts from this index automatically.
