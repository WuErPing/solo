---
name: solo-dev-base
description: Base development context for the Solo AI coding assistant platform. Provides architecture overview, tech stack, build commands, CI/CD reference, directory map, and development conventions. Use at the start of any Solo development task — feature work, bug fixes, provider integration, infrastructure changes, or code review.
version: "2026-06-13"
tags:
  - solo
  - architecture
  - development
  - onboarding
  - context
---

# Solo Dev Base

## Overview

Solo is a local-first AI coding assistant platform with a Go daemon, a cross-platform React Native/Expo app, a WebSocket relay, and a CLI. The system supports direct local connections and remote relay connections with end-to-end encryption (E2EE). It currently ships 4 built-in AI providers (Claude, Kimi, OpenCode, Pi) plus a development-only Mock provider, with Kimi integrated via JSON-RPC 2.0 Wire mode. Codex has a frontend definition but no backend implementation yet.

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
│   │   ├── components/  # Reusable UI components (~121 files)
│   │   ├── screens/     # Screen components (settings sections, tmux-dashboard, schedules)
│   │   ├── hooks/       # Custom hooks (~95 files)
│   │   ├── stores/      # Zustand stores (~33 files)
│   │   ├── contexts/    # React contexts (~20 files)
│   │   ├── utils/       # Utilities (~156 files)
│   │   ├── constants/   # App constants (agent slash commands)
│   │   ├── styles/      # Theme and terminal theme presets
│   │   ├── desktop/     # Desktop-specific modules
│   │   └── terminal/    # Terminal emulation (xterm)
│   └── e2e/             # Playwright E2E tests
├── app-bridge/          # TypeScript communication library
│   └── src/
│       ├── client/      # DaemonClient, transports (WS, Relay E2EE)
│       ├── relay/       # E2EE crypto (X25519 + XSalsa20-Poly1305)
│       ├── server/      # Agent, chat, loop, schedule, tmux modules
│       └── shared/      # Connection offer types, protocol constants
├── daemon/              # Go core service
│   └── internal/
│       ├── server/      # WebSocket server, session management
│       ├── agent/       # Agent lifecycle, provider registry, TurnGuard, typed errors
│       ├── workspace/   # Workspace & project management
│       ├── terminal/    # PTY terminal management
│       ├── relayclient/ # Relay client + E2EE
│       ├── push/        # Expo push notifications
│       ├── memory/      # Session memory: TurnRecorder, bridge, filebackend, redact
│       ├── memorysetup/ # Wires MemoryConfig → recorder+redactor+bridge for the daemon
│       ├── schedule/    # Cron-based schedule automation (executor, store, runner)
│       └── config/      # JSON config (~/.solo/config.json), incl. MemoryConfig
├── relay-go/            # Go WebSocket relay server
│   └── internal/relay/  # Server, session, control, buffer, metrics
├── cli/                 # Go CLI tool
│   └── cmd/             # daemon, agent, provider subcommands
├── protocol/            # Shared Go protocol definitions
│   ├── protocol.go      # Constants (WSProtocolVersion, endpoints)
│   └── message*.go      # Message type definitions
└── packages/highlight/  # Syntax highlighting package
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.25, gorilla/websocket, creack/pty, slog, BurntSushi/toml |
| **Frontend** | Expo 54, React Native 0.81, React 19, TypeScript |
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
# → output/darwin/{solo, solo-relay, solo-cli}

# Build Linux binaries
make linux
# → output/linux/{solo, solo-relay, solo-cli}

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
| `go` | For each module (protocol, cli, daemon, relay-go): `go mod verify` → `go build -v ./...` → `go test -short -race -coverprofile=coverage.out` → upload coverage (Codecov + artifact, 14 days) → `golangci-lint v2.10` (`--timeout=5m`) |
| `js` | `npm ci` → lint app / app-bridge / highlight → typecheck all three → test highlight → **test app (unit, 1617 tests)** → **test app-bridge (32 tests)** → upload coverage (Codecov + artifacts, 14 days) |

**`.github/workflows/e2e-nightly.yml`** (daily 02:00 UTC + manual):

| Job | Steps |
|-----|-------|
| `e2e` | Install dependencies → Playwright browsers → build workspace deps → run E2E (31 specs); failure artifacts retained 7 days |

**Coverage**: JS via Vitest v8 → lcov → Codecov (app ~36 % stmt, app-bridge ~89 % stmt). Go via `-coverprofile=coverage.out` → Codecov.

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
| Kimi | Wire (`kimi --wire`, JSON-RPC 2.0 stdio) | Go | ✅ Full (~737 LOC, 31 executed tests) |
| OpenCode | SSE (`/global/event`) | Go | ✅ Full |
| Pi | Minimal terminal harness | Go | ✅ Full |
| Mock | Test | Go | ✅ Dev-only (`SOLO_ENABLE_MOCK_PROVIDER=1`) |
| Codex | Print (OpenAI) | — | ⚠️ Definition only, no backend |

**Removed**: Copilot.
**Planned**: Cursor-Agent (Print mode). See `docs/providers/`.

## Recent Architecture Changes

1. **Session memory Phase 1** (2026-05-29): Turns (user + assistant) are persisted as Markdown + YAML frontmatter under `~/.solo/memory/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md`, indexed by `~/.solo/memory/sessions.jsonl`. New `daemon/internal/memory` module (`TurnRecorder` interface, `FileTurnRecorder` async writer, `Redactor` stack, `Bridge` for seq/parent chain + streaming-chunk accumulation, `SafeBridge` panic/circuit-breaker wrapper); `memorysetup` wires it from `config.MemoryConfig`; server hooks on `handleSendAgentMessage`/`sendAgentStream`. On by default (opt-out via `"memory": {"enabled": false}`). ~465 tests across memory/bridge/filebackend/redact/memorysetup/config/server. See `docs/architecture/session-memory-persistence.md` and `docs/product/session-memory-spec.md`.
2. **MessageID propagation** (2026-05-25): All providers now attach a unique `MessageID` to `user_message` events, enabling backend timeline deduplication across multiple concurrent sessions.
3. **Timeline deduplication** (2026-05-25): `InMemoryTimelineStore.Append()` compares the last row by type-specific equality (`MessageID` → `Text` → `CallID+Status`) to prevent duplicate entries when N sessions emit the same event.
4. **Multi-client sync test** (2026-05-25): Added `daemon/internal/server/multi_client_sync_test.go` (180 LOC) verifying concurrent session handling correctness.
5. **Mermaid preview** (2026-05-24): Markdown file panes now render Mermaid diagrams inline.
6. **App-bridge test suite** (2026-05-24): 3 test files covering base64, crypto, and path-utils (32 tests, ~300 ms).
7. **CI overhaul** (2026-05-24): App unit tests (1617 tests) and app-bridge tests now run on every PR; nightly E2E workflow; Codecov integration.
8. **Schedule automation** (2026-06-02): New `daemon/internal/schedule/` module with cron/interval cadences, timezone-aware input, UTC evaluation, and JSON persistence; App schedule dashboard and per-host schedule screens; app-bridge schedule RPC module. See `docs/analysis/create-schedule-flow.md` and `docs/analysis/app-bridge-schedule-module.md`.
9. **Tmux subsystem** (2026-06-03 ~ 2026-06-12): Tmux Dashboard (`screens/tmux-dashboard/`) and full-screen Tmux Pane Screen with ANSI rendering, lazy history loading (200→5000 lines), agent detection (3-layer), slash-command filtering, terminal theme sync, and status-line aggregation. See `docs/architecture/tmux-pane-content-loading.md` and `docs/analysis/tmux-pane-analysis.md`.
10. **Agent stall detection** (2026-05-30): `daemon/internal/agent/stall_monitor.go` detects stuck/repeating agents and tightens grace periods. See `docs/architecture/agent-stall-detection.md`.
11. **TurnGuard & typed provider errors** (2026-06-09): `daemon/internal/agent/base/turn_guard.go` prevents inconsistent provider turn transitions; `daemon/internal/agent/errors.go` introduces typed sentinel errors.
12. **Type-erasure convergence** (2026-06-07 ~ 2026-06-08): Typed stream events and tool-call structs (`protocol/stream_event.go`, `protocol/tool_call_detail.go`) replace broad `interface{}`/`map[string]interface{}` usage in provider pipelines. See `docs/analysis/go-provider-type-erasure-analysis.md`.
13. **OpenCode cross-device sync fix** (2026-06-09): SSE heartbeat and corrected event ordering resolve cross-client timeline duplication for OpenCode sessions. See `docs/analysis/opencode-cross-device-sync-fix.md`.
14. **Terminal theme simplification** (2026-06-12): Terminal theme presets reduced to `system` / `dark` / `light` / `tmux`.

## Documentation Index

Full docs live in `docs/`. Read `docs/README.md` for the structured index.

| Category | Path | Use When |
|----------|------|----------|
| Architecture | `docs/architecture/` | Designing features, understanding data flow |
| Product | `docs/product/` | Checking feature coverage, UI component inventory |
| Providers | `docs/providers/` | Adding new AI providers |
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
