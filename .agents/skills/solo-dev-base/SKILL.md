---
name: solo-dev-base
description: Base development context for the Solo AI coding assistant platform. Provides architecture overview, tech stack, build commands, CI/CD reference, directory map, and development conventions. Use at the start of any Solo development task — feature work, bug fixes, provider integration, infrastructure changes, or code review.
version: "2026-09-23"
tags:
  - solo
  - architecture
  - development
  - onboarding
  - context
---

# Solo Dev Base

## Overview

Solo is a local-first AI coding assistant platform with a Go daemon, a cross-platform React Native/Expo app, a WebSocket relay, a CLI, and a daemon supervisor. The system supports direct local connections and remote relay connections with end-to-end encryption (E2EE). It currently ships 5 built-in AI providers (Claude, Codex, Kimi, OpenCode, Pi) plus a development-only Mock provider, with Kimi integrated via JSON-RPC 2.0 Wire mode.

## When to Use

- Starting any development task on the Solo codebase
- Need architecture context before implementing a feature
- Looking up build commands, CI pipeline, or directory structure
- Adding a new AI provider
- Debugging connectivity or session issues
- Reviewing code changes

## Architecture at a Glance

```
┌─────────────────────────────────────────────────┐
│              Client Layer                        │
│  Web App · Mobile App · CLI                      │
└──────────────────┬──────────────────────────────┘
                   │ App-Bridge (TypeScript)
                   │ WebSocket (+ E2EE via Relay)
┌──────────────────▼──────────────────────────────┐
│              Network Layer                       │
│  Nginx (:443 SSL) → Relay (:8081 localhost)      │
└──────────────────┬──────────────────────────────┘
                   │ WebSocket
┌──────────────────▼──────────────────────────────┐
│              Service Layer (user machine)         │
│  Daemon (:17612) — Agent · Workspace · Terminal  │
└─────────────────────────────────────────────────┘
```

## Repository Map

```
solo/
├── app/                 # React Native / Expo frontend
│   ├── src/
│   │   ├── app/         # Expo Router (file-system routing)
│   │   ├── components/  # Reusable UI components (~160 files)
│   │   ├── screens/     # Screen components (settings sections, tmux-dashboard, schedules)
│   │   ├── hooks/       # Custom hooks (~140 files)
│   │   ├── stores/      # Zustand stores (~40 files)
│   │   ├── contexts/    # React contexts (~20 files)
│   │   ├── utils/       # Utilities (~186 files)
│   │   ├── constants/   # App constants (agent slash commands)
│   │   ├── styles/      # Theme and terminal theme presets
│   │   ├── desktop/     # Desktop-specific modules
│   │   └── terminal/    # Terminal emulation (xterm)
│   └── e2e/             # Playwright E2E tests
├── app-bridge/          # TypeScript communication library
│   └── src/
│       ├── client/      # DaemonClient, transports (WS, Relay E2EE), per-domain RPC modules
│       ├── relay/       # E2EE crypto (X25519 + XSalsa20-Poly1305)
│       ├── server/      # Agent, chat, loop, schedule, tmux, usage, version modules
│       └── shared/      # Connection offer types, protocol constants
├── daemon/              # Go core service
│   └── internal/
│       ├── server/      # WebSocket server, session management, tmux handlers + pane watcher
│       ├── agent/       # Agent lifecycle, provider registry, providers/{claude,codex,kimi,opencode,pi}, TurnGuard, typed errors
│       ├── workspace/   # Workspace & project management
│       ├── terminal/    # PTY terminal management
│       ├── relayclient/ # Relay client + E2EE
│       ├── push/        # Expo push notifications
│       ├── memory/      # Session memory: TurnRecorder, bridge, filebackend, redact
│       ├── memorysetup/ # Wires MemoryConfig → recorder+redactor+bridge for the daemon
│       ├── schedule/    # Cron-based schedule automation (executor, store, NL assistant)
│       ├── loop/        # Loop engine (template/instance model, worker+verifier)
│       ├── llm/         # Minimal OpenAI-compatible completion client (schedule assistant parse)
│       └── config/      # JSON config (~/.solo/config.json), incl. MemoryConfig
├── usage/               # Go usage/quota service (solo-usage binary)
├── relay-go/            # Go WebSocket relay server
│   └── internal/relay/  # Server, session, control, buffer, metrics
├── supervisor/          # Go daemon supervisor (solo-supervisor)
│   └── internal/supervisor/  # Spawn/respawn loop, backoff, PID/log ownership
├── cli/                 # Go CLI tool
│   └── cmd/             # daemon, agent, provider subcommands
├── protocol/            # Shared Go protocol definitions
│   ├── protocol.go      # Constants (WSProtocolVersion, endpoints)
│   ├── process_contract.go  # Daemon↔supervisor exit-code contract (42=restart)
│   └── message*.go      # Message type definitions
└── packages/highlight/  # Syntax highlighting package
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.25, gorilla/websocket, creack/pty, slog, BurntSushi/toml |
| **Frontend** | Expo 57, React Native 0.86, React 19.2, TypeScript |
| **State** | Zustand, @tanstack/react-query, React Context |
| **Styling** | Unistyles (dynamic theming) |
| **Terminal** | @xterm/xterm v6 |
| **Crypto** | X25519 key exchange + XSalsa20-Poly1305 (E2EE) |
| **CI** | GitHub Actions, golangci-lint v2, ESLint, Codecov |
| **Deploy** | Systemd, Docker, Nginx + Let's Encrypt |

## Build & Dev Commands

```bash
# Build all Darwin binaries
make darwin
# → output/darwin/{solo, solo-relay, solo-cli, solo-usage, solo-supervisor}

# Build Linux binaries
make linux
# → output/linux/{solo, solo-relay, solo-cli, solo-supervisor}

# Build + publish into ~/.solo/versions/ and restart under solo-supervisor
make restart

# Local dev (daemon + web app)
make dev
# daemon on :17612, Expo web on :19000

# Just web dev
make dev-web

# Just daemon
make dev-daemon

# Deploy relay to production
make deploy-solo-relay

# Stop all dev processes
make stop
```

## CI Pipeline

**`.github/workflows/ci.yml`** (push/PR to `main`/`master`):

| Job | Steps |
|-----|-------|
| `go` | For each module (protocol, cli, daemon, relay-go, supervisor, usage): `go mod verify` → `go build -v ./...` → `go test -short -race -coverprofile=coverage.out` → upload coverage (Codecov + artifact, 14 days) → `golangci-lint v2.10` (`--timeout=5m`) |
| `arch-boundaries` | `scripts/check-arch-boundaries.sh` — enforces the Go module boundaries from `.agents/rules/architecture.md` (protocol is dependency-free; daemon/cli/relay-go may import protocol only). Also runs locally via `make lint` |
| `js` | `npm ci` → lint app / app-bridge / highlight → typecheck all three → test highlight → **test app (unit, ~2100 tests)** → **test app-bridge (~200 tests)** → upload coverage (Codecov + artifacts, 14 days) |

**`.github/workflows/semantic-check.yml`** (PR label `semantic-check` + manual):

| Job | Steps |
|-----|-------|
| `adr-consistency` | `scripts/semantic-verify/check-adr-consistency.mjs` — an LLM evaluator reviews the PR diff against every accepted ADR in `docs/decisions/` and posts an advisory PR comment (never blocks merging). Needs `secrets.LLM_API_KEY`; optional `vars.LLM_BASE_URL` / `vars.LLM_MODEL` |

**`.github/workflows/e2e-nightly.yml`** (daily 02:00 UTC + manual):

| Job | Steps |
|-----|-------|
| `e2e` | Install dependencies → Playwright browsers → build workspace deps → run E2E (44 specs); failure artifacts retained 7 days |

**Coverage**: JS via Vitest v8 → lcov → Codecov (app ~36 % stmt, app-bridge ~89 % stmt). Go via `-coverprofile=coverage.out` → Codecov. `codecov.yml` gates **patch coverage ≥ 70 %** on PRs (hard check); whole-project coverage stays informational.

## Key Network Facts

| Port | Service | Bind Address | Access |
|------|---------|-------------|--------|
| 443 | Nginx (SSL) | 0.0.0.0 | Public |
| 8081 | Relay WS | 127.0.0.1 | Local only (via Nginx) |
| 17612 | Daemon WS | 127.0.0.1 | Local only |
| 19000 | Expo dev | 0.0.0.0 | Dev only |

- **Production relay endpoint**: `solo.up2ai.top:443` (NEVER use raw IP:8081)
- **Pairing Link**: `https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}`
- **Config file**: `~/.solo/config.json`
- **Daemon keypair**: `~/.solo/daemon-keypair.json`

## Currently Implemented Providers

| Provider | Mode | Backend | Status |
|----------|------|---------|--------|
| Claude | Print (`--print --output-format stream-json`) | Go | ✅ Full |
| Kimi | Wire (`kimi --wire`, JSON-RPC 2.0 stdio) | Go | ✅ Full |
| OpenCode | SSE (`/global/event`) | Go | ✅ Full |
| Pi | Minimal terminal harness | Go | ✅ Full |
| Codex | Print (`codex exec --json`, session resume) | Go | ✅ Full |
| Mock | Test | Go | ✅ Dev-only (`SOLO_ENABLE_MOCK_PROVIDER=1`) |

**Removed**: Copilot.
**Planned**: Cursor-Agent (Print mode). See `docs/providers/`.

## Recent Architecture Changes

1. **Session memory Phase 1** (2026-05-29): Turns (user + assistant) are persisted as Markdown + YAML frontmatter under `~/.solo/memory/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md`, indexed by `~/.solo/memory/sessions.jsonl`. New `daemon/internal/memory` module (`TurnRecorder` interface, `FileTurnRecorder` async writer, `Redactor` stack, `Bridge` for seq/parent chain + streaming-chunk accumulation, `SafeBridge` panic/circuit-breaker wrapper); `memorysetup` wires it from `config.MemoryConfig`; server hooks on `handleSendAgentMessage`/`sendAgentStream`. On by default (opt-out via `"memory": {"enabled": false}`). ~465 tests across memory/bridge/filebackend/redact/memorysetup/config/server. See `docs/architecture/session-memory-persistence.md`.
2. **MessageID propagation** (2026-05-25): All providers now attach a unique `MessageID` to `user_message` events, enabling backend timeline deduplication across multiple concurrent sessions.
3. **Timeline deduplication** (2026-05-25): `InMemoryTimelineStore.Append()` compares the last row by type-specific equality (`MessageID` → `Text` → `CallID+Status`) to prevent duplicate entries when N sessions emit the same event.
4. **Multi-client sync test** (2026-05-25): Added `daemon/internal/server/multi_client_sync_test.go` (180 LOC) verifying concurrent session handling correctness.
5. **Mermaid preview** (2026-05-24): Markdown file panes now render Mermaid diagrams inline.
6. **App-bridge test suite** (2026-05-24): 3 test files covering base64, crypto, and path-utils (32 tests, ~300 ms).
7. **CI overhaul** (2026-05-24): App unit tests (1617 tests) and app-bridge tests now run on every PR; nightly E2E workflow; Codecov integration.
8. **Schedule automation** (2026-06-02): New `daemon/internal/schedule/` module with cron/interval cadences, timezone-aware input, UTC evaluation, and JSON persistence; App schedule dashboard and per-host schedule screens; app-bridge schedule RPC module. See `docs/analysis/create-schedule-flow.md` and `docs/analysis/app-bridge-schedule-module.md`.
9. **Tmux subsystem** (2026-06-03 ~ 2026-06-12): Tmux Dashboard (`screens/tmux-dashboard/`) and full-screen Tmux Pane Screen with ANSI rendering, lazy history loading (200→5000 lines), agent detection (3-layer), slash-command filtering, terminal theme sync, and status-line aggregation. See `docs/architecture/tmux-pane-content-loading.md`.
10. **Agent stall detection** (2026-05-30): `daemon/internal/agent/stall_monitor.go` detects stuck/repeating agents and tightens grace periods. See `docs/architecture/agent-stall-detection.md`.
11. **TurnGuard & typed provider errors** (2026-06-09): `daemon/internal/agent/base/turn_guard.go` prevents inconsistent provider turn transitions; `daemon/internal/agent/errors.go` introduces typed sentinel errors.
12. **Type-erasure convergence** (2026-06-07 ~ 2026-06-08): Typed stream events and tool-call structs (`protocol/stream_event.go`, `protocol/tool_call_detail.go`) replace broad `interface{}`/`map[string]interface{}` usage in provider pipelines. See `docs/analysis/go-provider-type-erasure-analysis.md`.
13. **OpenCode cross-device sync fix** (2026-06-09): SSE heartbeat and corrected event ordering resolve cross-client timeline duplication for OpenCode sessions.
14. **Terminal themes** (2026-06-12, revised later): Terminal theme presets are now `system` / `dark` / `light` / `bash` / `auto` (`app/src/styles/terminal-themes.ts`).
15. **Loop templates & instances** (v0.10.0): `daemon/internal/loop/` engine with template/instance model (`templateID` grouping), shared `AgentTemplate` for loop+schedule agents (ADR-001), worker+verifier feedback loop; app template hooks and instance detail screen.
16. **Schedule assistant & LLM config** (v0.10.0): NL schedule parse via `daemon/internal/schedule/assistant*.go` + `daemon/internal/llm/` client (`schedule/assist` RPC, proposal-only safety); LLM provider configuration backend + settings UI. See `docs/architecture/schedule-assistant.md`.
17. **Tmux push refresh** (v0.11.0): server-level `TmuxPaneWatcher` (~500ms poll) broadcasts `tmux/pane_changed`; app refetches the active pane immediately and repaints only changed lines (`diffSnapshots`).
18. **Supervisor & version switching** (v0.12.0): `solo-supervisor` watchdog with exit-code restart contract (42=restart, ADR-003), `~/.solo/versions/` + `current` pointer version switching from the host page (ADR-004), crash-breaker fallback to the last working build (ADR-005); spawn-loop health persisted to `supervisor-state.json` and relayed via `list_daemon_versions` (v0.13.0). See `docs/architecture/daemon-supervision.md`.
19. **Codex provider** (v0.12.0 era): full Go backend at `daemon/internal/agent/providers/codex/` (`codex exec --json`, native session resume via `exec resume`).
20. **LaTeX math rendering** (v0.15.0): markdown preview renders `$$…$$` display and `$…$` inline math via MathJax tex-svg drawn with react-native-svg (no WebView); KaTeX assets bundled offline (`npm run generate:katex-assets`).
21. **Task Groups prototype removed** (2026-09-23): the `/task-groups` mock-data route and screens were deleted; `docs/design/multi-agent-collaboration.md` remains a proposal for the real feature.

## Documentation Index

Full docs live in `docs/`. Read `docs/README.md` for the structured index.

| Category | Path | Use When |
|----------|------|----------|
| Architecture | `docs/architecture/` | Designing features, understanding data flow |
| Product | `docs/product/` | Checking feature coverage, UI component inventory |
| Decisions (ADR) | `docs/decisions/` | Making or reviewing significant design decisions |
| Providers | `docs/providers/` | Adding new AI providers |
| Release | `docs/release/` | Cutting a release, build/deploy per module |
| Verification | `docs/verification/` | Test strategy, coverage gates |
| Analysis | `docs/analysis/` | Deep-dives into specific subsystems |
| Project Rules | `.agents/rules/` | Go/TS conventions, testing, security, architecture boundaries (indexed from `CLAUDE.md`) |

## Development Conventions

1. **Go modules**: Each Go component (daemon, cli, relay-go, protocol) has its own `go.mod`. Use `go.work` at repo root for local development.
2. **npm workspaces**: `app/`, `app-bridge/`, `packages/highlight/` are npm workspaces.
3. **Testing**: Go tests use `-short -race` flags. JS tests use Vitest (app, app-bridge) and Jest (packages).
4. **Linting**: Go uses `golangci-lint v2` with `.golangci.yml`. JS uses ESLint with per-workspace configs.
5. **Commit style**: Conventional commits preferred.
6. **E2E tests**: Playwright tests in `app/e2e/`, Maestro flows in `app/maestro/`.

## The Process

```
UNDERSTAND ──→ LOCATE ──→ IMPLEMENT ──→ VERIFY
     │             │           │            │
     ▼             ▼           ▼            ▼
  Read docs/   Find files   Make changes  Run tests
  for context  in repo map  + build       + lint
```

1. **Understand**: Read the relevant doc from `docs/README.md` before coding.
2. **Locate**: Use the repository map above to find the right module/directory.
3. **Implement**: Make changes following the conventions above.
4. **Verify**: Run `go test -short -race ./...` (Go) or `npx expo lint` (JS) before committing.
