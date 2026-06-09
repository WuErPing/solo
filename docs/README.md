# Solo — Documentation Index

> **Purpose**: Persistent context base for Solo development, CI/CD, and architecture decisions.
> **Last updated**: 2026-06-07

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
│   ├── push-notifications.md              # Push notification architecture
│   ├── session-memory-persistence.md      # Session turn recording & memory layer design
│   ├── solo-system-architecture.png       # System architecture diagram (PNG)
│   ├── solo-system-architecture.svg       # System architecture diagram (SVG)
│   ├── timeline-design.md                 # Head/Tail model, seq gate, deduplication
│   └── tmux-pane-content-loading.md       # Tmux agent detection, pane capture, polling, key injection
├── product/                               ← Product feature analysis
│   ├── features.md                        # Full product feature analysis
│   ├── product-analysis.md                # Optimization & feature suggestions
│   ├── session-memory-spec.md             # Session memory Phase-1 implementation spec
│   └── ui-features.md                     # App UI screens, components, hooks
├── providers/                             ← AI provider integration research
│   ├── kimi-wire-vs-acp.md               # Kimi Wire vs ACP protocol comparison
│   └── kimi-cursor-integration.md         # Kimi & Cursor-Agent integration plan
└── analysis/                              ← Deep-dive technical analysis
    ├── agent-provider-status-unification.md # Agent/provider status unification design
    ├── app-agent-status-analysis.md         # App agent status & Copy button logic
    ├── app-bridge-schedule-module.md        # Schedule module type contract & RPC schema
    ├── app-coverage-analysis.md             # App test coverage analysis
    ├── app-lint-analysis.md                 # App lint capability analysis
    ├── create-schedule-flow.md              # End-to-end schedule creation flow
    ├── demo/                              # Demo code (iterm2-agent-detection)
    ├── go-coverage-report.md                # Go backend coverage report
    ├── host-status-check.md                 # Host probe cycle & status machine
    ├── iterm2-agent-observation.md          # iTerm2 agent detection observation
    ├── lint-capability-plan.md              # Lint tooling capability plan
    ├── README.md                            # Analysis directory index
    ├── session-timeline-e2e-gaps.md         # Session timeline E2E test gaps
    ├── test-suite-analysis.md               # Full test suite inventory, CI gaps, coverage report
    └── tmux-transport-disposed-race.md      # Tmux `Transport not connected (status: disposed)` race analysis
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

**Key facts (always-on context)**:
- Daemon listens `127.0.0.1:17612`; Relay listens `127.0.0.1:8081` (behind Nginx :443)
- Production relay endpoint: `solo.up2ai.top:443` (never use raw IP:8081)
- E2EE: X25519 key exchange + XSalsa20-Poly1305
- Pairing Link format: `https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}`

---

## 2 · Product

Feature inventory and UI/UX analysis.

| Document | Type | Summary |
|----------|------|---------|
| [Product Features](product/features.md) | Analysis | Full feature tree: Agent system, session, workspace, push, relay, CLI, tests, CI/CD |
| [Product Analysis](product/product-analysis.md) | Analysis | 10 major feature opportunities and 5 quick wins |
| [Session Memory Spec](product/session-memory-spec.md) | Spec | Phase-1 implementation spec: TurnRecorder interface, FileTurnRecorder, hooks, redaction, tests |
| [UI Features](product/ui-features.md) | Analysis | Screen map, component catalogue, contexts, hooks, stores, feature checklist |

**Current completion**: ~85-90 %. Main gaps: Chat system (multi-agent), Cursor-Agent / ACP providers.

---

## 3 · Providers

AI provider integration research and implementation plans.

| Document | Type | Summary |
|----------|------|---------|
| [Kimi Wire vs ACP](providers/kimi-wire-vs-acp.md) | Comparison | Wire mode recommended for Solo (full Kimi feature set, stdio-only) |
| [Kimi & Cursor-Agent Integration](providers/kimi-cursor-integration.md) | Implementation plan | Wire mode for Kimi; Print mode for Cursor-Agent; backend Go registration |

**Currently implemented providers**: Claude (print/stream-json), Kimi (Wire mode, JSON-RPC 2.0 stdio), OpenCode (SSE), Pi (minimal terminal harness), Mock (test).

**Definition only (no backend)**: Codex.

**Removed**: Copilot. **Planned**: Cursor-Agent (Print mode).

---

## 4 · Technical Analysis

Deep dives into specific subsystems.

| Document | Type | Summary |
|----------|------|---------|
| [Agent/Provider Status Unification](analysis/agent-provider-status-unification.md) | Design | OCP-based proposal to unify AgentLifecycleStatus, ProviderStatus across layers |
| [App Agent Status Analysis](analysis/app-agent-status-analysis.md) | Analysis | App agent lifecycle states and Copy button display logic |
| [App-Bridge Schedule Module](analysis/app-bridge-schedule-module.md) | Analysis | Schedule module type contract, RPC schema, and domain models |
| [App Coverage Analysis](analysis/app-coverage-analysis.md) | Analysis | App test coverage breakdown by module, gaps, and recommendations |
| [App Lint Analysis](analysis/app-lint-analysis.md) | Analysis | App lint tooling analysis, rules, and capability gaps |
| [Create Schedule Flow](analysis/create-schedule-flow.md) | Analysis | End-to-end schedule creation flow with timezone-aware cron scheduling |
| [Go Coverage Report](analysis/go-coverage-report.md) | Report | Go backend coverage by module (protocol, cli, daemon, relay-go) |
| [Host Status Check](analysis/host-status-check.md) | Analysis | Probe cycle (2-30 s), adaptive switching, state machine conflict, grace-period fix |
| [Lint Capability Plan](analysis/lint-capability-plan.md) | Plan | Lint tooling roadmap and capability gap plan |
| [Session Timeline E2E Gaps](analysis/session-timeline-e2e-gaps.md) | Analysis | Session timeline E2E test coverage gaps and remediation |
| [Test Suite Analysis](analysis/test-suite-analysis.md) | Analysis | Test inventory (app unit, Go, E2E), CI coverage, Codecov integration |
| [Tmux Transport Disposed Race](analysis/tmux-transport-disposed-race.md) | Analysis | `Transport not connected (status: disposed)` root cause: probe-cycle switch vs. in-flight tmux RPC |

---

## 5 · Build & CI/CD Quick Reference

> Full commands live in `Makefile` and `.github/workflows/ci.yml`.

### Build targets

| Target | Command | Output |
|--------|---------|--------|
| Darwin binaries | `make darwin` | `output/darwin/{solo,solo-relay,solo-cli}` |
| Linux binaries | `make linux` | `output/linux/{solo,solo-relay,solo-cli}` |
| Dev (daemon + web) | `make dev` | daemon :17612 + Expo :19000 |
| Deploy relay | `make deploy-solo-relay` | scp + systemctl restart |

### CI pipeline (`.github/workflows/ci.yml`)

| Job | Steps |
|-----|-------|
| `go` (matrix: protocol, cli, daemon, relay-go) | `go mod verify` → `go build` → `go test -short -race` → `golangci-lint v2.10` |
| `js` | `npm ci` → lint (app, app-bridge, highlight) → typecheck (optional) → test highlight |

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

## 6 · How to Use These Docs

1. **Starting a feature** → read the relevant Architecture doc first, then check Product for existing coverage.
2. **Adding a provider** → read `providers/` docs for protocol decisions, then `architecture/components.md` § Daemon.
3. **Debugging connectivity** → `architecture/network-architecture.md` (port ACL, common misconfig) + `architecture/deployment.md` (troubleshooting).
4. **CI/CD changes** → check § 5 above + `Makefile` + `.github/workflows/ci.yml`.
5. **Agent/context boot** → the `solo-dev-base` skill (`.agents/skills/solo-dev-base/SKILL.md`) loads key facts from this index automatically.
