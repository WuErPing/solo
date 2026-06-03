# Solo

📄 [中文版本](README.zh-CN.md)

Solo is an AI coding assistant platform that connects your local development environment with AI providers through a secure, end-to-end encrypted architecture. It consists of a local daemon, a relay server for remote connectivity, a cross-platform mobile/web app, and a CLI tool.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   Web App   │  │ Mobile App  │  │    CLI      │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
└─────────┼────────────────┼────────────────┼────────────────┘
          └────────────────┴────────────────┘
                         │
                ┌────────▼────────┐
                │   App-Bridge    │
                └────────┬────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│                     Network Layer                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Nginx (optional)                        │   │
│  └────────────────────────┬────────────────────────────┘   │
│                           │                                  │
│  ┌────────────────────────▼────────────────────────────┐   │
│  │            Relay Server (signaling relay)            │   │
│  └────────────────────────┬────────────────────────────┘   │
└───────────────────────────┼──────────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────────┐
│                      Service Layer                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Daemon (core service)                   │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Core Components

| Component | Directory | Language | Responsibility |
|-----------|-----------|----------|----------------|
| **App** | [`app/`](app/) | TypeScript / React Native | User interface (iOS, Android, Web) |
| **App-Bridge** | [`app-bridge/`](app-bridge/) | TypeScript | Client-side communication library |
| **Daemon** | [`daemon/`](daemon/) | Go | Core service — manages sessions, agents, and provider connections |
| **Relay** | [`relay-go/`](relay-go/) | Go | Connection relay for remote/mobile access |
| **CLI** | [`cli/`](cli/) | Go | Command-line tool for session and agent management |
| **Protocol** | [`protocol/`](protocol/) | Go | Shared protocol definitions |

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.25 · gorilla/websocket · creack/pty · slog |
| Frontend | Expo 54 · React Native 0.81 · React 19 · TypeScript |
| State Management | Zustand · @tanstack/react-query · React Context |
| Cryptography | X25519 key exchange + XSalsa20-Poly1305 (E2EE) |
| Testing | Vitest · Playwright (E2E) · Go test |
| Deployment | Systemd · Docker · Nginx + Let's Encrypt |
| CI/CD | GitHub Actions · golangci-lint v2 · ESLint |

---

## Quick Start

### Prerequisites

- [Go](https://go.dev/) 1.25+
- [Node.js](https://nodejs.org/) 20+
- [Expo CLI](https://docs.expo.dev/get-started/installation/) (for mobile/web development)

### Build

```bash
# Build all Darwin binaries (daemon, relay, CLI)
make darwin

# Build all Linux binaries
make linux

# Build everything
make all
```

Output binaries are placed in `output/darwin/` and `output/linux/`.

### Development

```bash
# Start daemon + web app together
make dev

# Start only the web app
make dev-web

# Start only the daemon (must build first)
make dev-daemon

# Restart the daemon
make restart

# Stop all dev processes
make stop
```

- **Daemon** listens on `127.0.0.1:17612`
- **Web app** runs on `http://localhost:19000`

### Run Tests

```bash
# Go tests (all modules)
cd protocol && go test -short -race ./...
cd cli && go test -short -race ./...
cd daemon && go test -short -race ./...
cd relay-go && go test -short -race ./...

# JavaScript / TypeScript tests
npm run lint
npm run test --workspaces --if-present
```

---

## Project Structure

```
solo/
├── app/                 # React Native / Expo application
├── app-bridge/          # Client communication library
├── cli/                 # Go CLI tool
├── daemon/              # Go core service
├── docs/                # Architecture & product documentation
├── packages/highlight/  # Shared syntax highlighting package
├── protocol/            # Go protocol definitions
├── relay-go/            # Go relay server
├── Makefile             # Build & development commands
├── go.work              # Go workspace
└── package.json         # Node.js workspace root
```

For detailed documentation, see [`docs/README.md`](docs/README.md).

---

## Supported AI Providers

- **Claude** — print / stream-json mode
- **Kimi** — Wire mode (JSON-RPC 2.0 over stdio)
- **OpenCode** — SSE mode
- **Codex** — definition only
- **Mock** — for testing

See [`docs/providers/`](docs/providers/) for provider integration research and planned additions.

---

## Session Memory

The daemon persists every user/assistant turn of every session to disk as Markdown with YAML frontmatter, giving you a local, queryable transcript of everything Solo has done.

- **Storage**: `~/.solo/memory/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md`
- **Index**: `~/.solo/memory/sessions.jsonl` (one JSONL line per session)
- **Streaming**: an assistant streaming response is coalesced into a **single** `assistant.md` per logical turn — you never end up with dozens of files for one answer.
- **Redaction**: OpenAI / GitHub / Anthropic / AWS tokens and common env-file secrets are replaced with `[redacted:<reason>]` before writing.
- **Isolation**: the recorder runs behind a `SafeBridge` wrapper that recovers from panics and trips a circuit breaker on repeated failures, so a storage problem can never take down the daemon's main session loop.

### Configuration

Session memory is **on by default**. To opt out, add to `~/.solo/config.json`:

```json
{
  "memory": { "enabled": false }
}
```

Other knobs (`backend`, `root`, `retention_days`, `queue_size`, `overflow`, `redact.*`, `safe.failure_threshold`, `safe.failure_cooldown`) are documented in [`docs/product/session-memory-spec.md`](docs/product/session-memory-spec.md).

---

## Tmux Dashboard

The app includes a Tmux Dashboard that automatically detects AI agents running in tmux sessions and provides interactive control.

### Agent Detection

Three-layer detection identifies agents even when `pane_current_command` reports a different process name (e.g., `node` for pi):

1. **Command name** — matches `claude`, `pi`, `kimi`, `kimi-cli`, `opencode`, `qoder`, `cursor`
2. **Pane title** — unicode normalization (e.g., `π` → `pi`) with word-boundary matching
3. **Child process inspection** — `pgrep`/`ps` fallback for wrapped launchers

### Features

- **Agent cards** — grouped by agent name, tap to filter
- **Pane content capture** — live terminal view (last 500 lines), auto-refreshes every 5 seconds
- **Interactive control** — send text commands with Enter, or use quick-action buttons:
  - Arrow keys (↑↓←→) for TUI menu navigation
  - Enter, Esc, Tab, Ctrl+C for control
  - Number keys (1–4) for TUI menu selection

### Supported Agents

| Agent | Detection Method |
|-------|-----------------|
| claude | command / title |
| pi | command / title (π unicode) / child process |
| kimi | command / title |
| kimi-cli | command / title |
| opencode | command / title |
| qoder | command |
| cursor | command |

---

## Security

- **End-to-End Encryption**: All communication between client and daemon is encrypted using X25519 key exchange + XSalsa20-Poly1305.
- **Pairing Link**: Secure pairing via `https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}`.

---

## CI/CD

The project uses GitHub Actions (`.github/workflows/ci.yml`) with the following jobs:

| Job | Steps |
|-----|-------|
| **Go** (matrix: protocol, cli, daemon, relay-go) | `go mod verify` → `go build` → `go test -short -race` → `golangci-lint` |
| **JS** | `npm ci` → lint → typecheck → test |

---

## Documentation

- [Architecture Overview](docs/architecture/README.md)
- [Component Specifications](docs/architecture/components.md)
- [Data Flow & Session Lifecycle](docs/architecture/data-flow.md)
- [Network Architecture](docs/architecture/network-architecture.md)
- [Deployment Guide](docs/architecture/deployment.md)
- [Product Features](docs/product/features.md)
- [Session Memory Spec](docs/product/session-memory-spec.md)

---

## License

[Add your license here]
