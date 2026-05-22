---
name: solo-dev-base
description: Base development context for the Solo AI coding assistant platform. Provides architecture overview, tech stack, build commands, CI/CD reference, directory map, and development conventions. Use at the start of any Solo development task — feature work, bug fixes, provider integration, infrastructure changes, or code review.
---

# Solo Dev Base

## Overview

Solo is a local-first AI coding assistant platform with a Go daemon, a cross-platform React Native/Expo app, a WebSocket relay, and a CLI. The system supports direct local connections and remote relay connections with end-to-end encryption (E2EE). It currently ships 4 built-in AI providers (Claude, Kimi, OpenCode, Codex) with Kimi integrated via JSON-RPC 2.0 Wire mode.

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
│   │   ├── screens/     # Screen components
│   │   ├── hooks/       # Custom hooks (~95 files)
│   │   ├── stores/      # Zustand stores (~33 files)
│   │   ├── contexts/    # React contexts (~20 files)
│   │   ├── utils/       # Utilities (~156 files)
│   │   ├── desktop/     # Desktop-specific modules
│   │   └── terminal/    # Terminal emulation (xterm)
│   └── e2e/             # Playwright E2E tests
├── app-bridge/          # TypeScript communication library
│   └── src/
│       ├── client/      # DaemonClient, transports (WS, Relay E2EE)
│       ├── relay/       # E2EE crypto (X25519 + XSalsa20-Poly1305)
│       ├── server/      # Agent, chat, loop, schedule modules
│       └── shared/      # Connection offer types, protocol constants
├── daemon/              # Go core service
│   └── internal/
│       ├── server/      # WebSocket server, session management
│       ├── agent/       # Agent lifecycle, provider registry
│       ├── workspace/   # Workspace & project management
│       ├── terminal/    # PTY terminal management
│       ├── relayclient/ # Relay client + E2EE
│       ├── push/        # Expo push notifications
│       └── config/      # JSON config (~/.solo/config.json)
├── relay-go/            # Go WebSocket relay server
│   └── internal/relay/  # Server, session, control, buffer
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
| **CI** | GitHub Actions, golangci-lint v2, ESLint |
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

File: `.github/workflows/ci.yml`

| Job | Trigger | Steps |
|-----|---------|-------|
| `go` | push/PR to main | For each module (protocol, cli, daemon, relay-go): `go mod verify` → `go build` → `go test -short -race` → `golangci-lint v2.10` |
| `js` | push/PR to main | `npm ci` → lint (app, app-bridge, highlight) → typecheck highlight → test highlight → optional typecheck app/app-bridge |

## Key Network Facts

| Port | Service | Bind Address | Access |
|------|---------|-------------|--------|
| 443 | Nginx (SSL) | 0.0.0.0 | Public |
| 8081 | Relay WS | 127.0.0.1 | Local only (via Nginx) |
| 17612 | Daemon WS | 127.0.0.1 | Local only |
| 19000 | Expo dev | 0.0.0.0 | Dev only |

- **Production relay endpoint**: `solo.up2ai.top:443` (NEVER use raw IP:8081)
- **Pairing Link**: `https://app.solo.sh/#offer={base64url(ConnectionOfferV2)}`
- **Config file**: `~/.solo/config.json`
- **Daemon keypair**: `~/.solo/daemon-keypair.json`

## Currently Implemented Providers

| Provider | Mode | Backend | Status |
|----------|------|---------|--------|
| Claude | Print (`--print --output-format stream-json`) | Go | ✅ Full |
| Kimi | Wire (`kimi --wire`, JSON-RPC 2.0 stdio) | Go | ✅ Full (758 LOC, 23 tests) |
| OpenCode | SSE (`/global/event`) | Go | ✅ Full |
| Codex | Print (OpenAI) | — | ⚠️ Definition only, no backend |
| Mock | Test | Go | ✅ Test only |

**Removed**: Copilot, Pi (removed from builtin registry).
**Planned**: Cursor-Agent (Print mode). See `docs/providers/`.

## Documentation Index

Full docs live in `docs/`. Read `docs/README.md` for the structured index.

| Category | Path | Use When |
|----------|------|----------|
| Architecture | `docs/architecture/` | Designing features, understanding data flow |
| Product | `docs/product/` | Checking feature coverage, UI component inventory |
| Providers | `docs/providers/` | Adding new AI providers |
| Analysis | `docs/analysis/` | Deep-dives into specific subsystems |

## Development Conventions

1. **Go modules**: Each Go component (daemon, cli, relay-go, protocol) has its own `go.mod`. Use `go.work` at repo root for local development.
2. **npm workspaces**: `app/`, `app-bridge/`, `packages/highlight/` are npm workspaces.
3. **Testing**: Go tests use `-short -race` flags. JS tests use Vitest (app) and Jest (packages).
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
