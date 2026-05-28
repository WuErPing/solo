# Solo — Documentation Index

> **Purpose**: Persistent context base for Solo development, CI/CD, and architecture decisions.
> **Last updated**: 2026-05-25

---

## Directory Structure

```
docs/
├── README.md                              ← You are here (master index)
├── architecture/                          ← System architecture & design
│   ├── README.md                          # Architecture overview & diagrams
│   ├── components.md                      # Component specifications
│   ├── data-flow.md                       # Message flows & session lifecycle
│   ├── deployment.md                      # Deployment, Nginx, Systemd, Docker
│   ├── network-architecture.md            # Network paths, E2EE, Pairing Link
│   ├── session-memory-persistence.md      # Session turn recording & memory layer design
│   └── timeline-design.md                 # Head/Tail model, seq gate, deduplication
├── product/                               ← Product feature analysis
│   ├── features.md                        # Full product feature analysis
│   └── ui-features.md                     # App UI screens, components, hooks
├── providers/                             ← AI provider integration research
│   ├── kimi-wire-vs-acp.md               # Kimi Wire vs ACP protocol comparison
│   └── kimi-cursor-integration.md         # Kimi & Cursor-Agent integration plan
└── analysis/                              ← Deep-dive technical analysis
    ├── host-status-check.md               # Host probe cycle & status machine
    └── test-suite-analysis.md             # Full test suite inventory, CI gaps, coverage report
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
| [Session Memory Persistence](architecture/session-memory-persistence.md) | Design | Dev | Hook points, TurnRecorder interface, file layout, migration path to DB / memory middleware |
| [Deployment](architecture/deployment.md) | Runbook | Infra / CI | Systemd, Docker, Nginx config, env vars, monitoring, troubleshooting |

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
| [UI Features](product/ui-features.md) | Analysis | Screen map, component catalogue, contexts, hooks, stores, feature checklist |

**Current completion**: ~80-87 %. Main gaps: GitHub integration, voice (TTS/STT), Cursor-Agent / ACP providers.

---

## 3 · Providers

AI provider integration research and implementation plans.

| Document | Type | Summary |
|----------|------|---------|
| [Kimi Wire vs ACP](providers/kimi-wire-vs-acp.md) | Comparison | Wire mode recommended for Solo (full Kimi feature set, stdio-only) |
| [Kimi & Cursor-Agent Integration](providers/kimi-cursor-integration.md) | Implementation plan | Wire mode for Kimi; Print mode for Cursor-Agent; backend Go registration |

**Currently implemented providers**: Claude (print/stream-json), Kimi (Wire mode, JSON-RPC 2.0 stdio), OpenCode (SSE), Codex (definition only), Mock (test).

**Removed**: Copilot, Pi. **Planned**: Cursor-Agent (Print mode).

---

## 4 · Technical Analysis

Deep dives into specific subsystems.

| Document | Type | Summary |
|----------|------|---------|
| [Host Status Check](analysis/host-status-check.md) | Analysis | Probe cycle (2-30 s), adaptive switching, state machine conflict, grace-period fix |
| [Test Suite Analysis](analysis/test-suite-analysis.md) | Analysis | Test inventory (~366 app unit, ~80 Go, 22 E2E), CI coverage, Codecov integration |

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
