# Architecture Rules

Rules for maintaining the Solo system architecture and module boundaries.

## Module Boundaries

```
protocol/     ← shared types, zero dependencies, imported by all
    ↑
    ├── daemon/       ← core service, owns agent/workspace/terminal lifecycle
    ├── cli/          ← thin CLI wrapper, talks to daemon via WebSocket
    └── relay-go/     ← stateless WebSocket relay, no business logic
    
app-bridge/   ← TypeScript communication library (daemon client + E2EE)
    ↑
    └── app/          ← React Native frontend, consumes app-bridge
```

- **No circular dependencies between modules.** If `daemon` needs something from `cli`, extract it to `protocol` or a shared internal package.
- **`protocol` is the contract.** All cross-module communication types live here. Changes to `protocol` affect all modules and require coordinated updates.
- **`app-bridge` is the client SDK.** It encapsulates all daemon communication. The `app/` layer should never open raw WebSocket connections.

## Layer Responsibilities

### Daemon (`daemon/`)
- Owns: agent lifecycle, workspace management, terminal sessions, config persistence, session memory
- Exposes: WebSocket API on `127.0.0.1:17612`
- Does NOT: serve HTTP, render UI, manage relay connections (relay client is in `relayclient/`)

### Relay (`relay-go/`)
- Owns: WebSocket session routing between daemon and clients, message buffering
- Exposes: WebSocket API on `127.0.0.1:8081`
- Does NOT: inspect message content, store session state, authenticate users (auth is at Nginx)

### CLI (`cli/`)
- Owns: user-facing command parsing, daemon process management
- Exposes: terminal commands
- Does NOT: implement business logic (delegates to daemon via WebSocket)

### App-Bridge (`app-bridge/`)
- Owns: typed daemon client, E2EE crypto, connection offer encoding
- Exposes: TypeScript API for app consumption
- Does NOT: manage UI state, render components, access native modules directly

### App (`app/`)
- Owns: UI, state management, user interaction
- Exposes: cross-platform application (iOS, Android, Web, Desktop)
- Does NOT: implement crypto, parse raw WebSocket frames, manage daemon processes

## Provider System

- Providers are registered in `daemon/internal/agent/`. Each provider implements a common interface.
- Supported modes: **Print** (subprocess with stdout streaming), **Wire** (persistent subprocess with bidirectional JSON-RPC), **SSE** (HTTP server-sent events).
- Adding a new provider: implement the provider interface, register in the builtin registry, add integration tests.
- Never modify an existing provider's interface to accommodate a new one. If the interface needs extension, use optional capabilities.
- Provider output is untrusted. Always sanitize before displaying or persisting.

## WebSocket Protocol

- Protocol version is defined in `protocol/protocol.go` as `WSProtocolVersion`.
- Breaking protocol changes require a version bump. Clients and servers must negotiate version on connect.
- Message types are defined in `protocol/message*.go`. Add new message types; never modify existing ones in incompatible ways.
- All messages are JSON. Use tagged unions (`type` field) for message discrimination.

## Session Architecture

- A session is the unit of interaction between a client and the daemon.
- Each session has a unique ID (ULID). Sessions are isolated: no shared state between concurrent sessions.
- Timeline events are deduplicated by `MessageID` for user messages, `CallID+Status` for tool calls.
- Session memory persists turns to `~/.solo/memory/YYYY-MM-DD/sessionID/` as markdown with YAML frontmatter.

## Data Flow

```
User Input → App → App-Bridge (E2EE if relay) → Daemon → Provider
                                                         ↓
User Sees  ← App ← App-Bridge (E2EE if relay) ← Daemon ← Provider Output
```

- Data flows in one direction through the pipeline. No feedback loops.
- The daemon is the single source of truth for session state. Clients are views over daemon state.
- E2EE is transparent to the daemon. Encryption/decryption happens at the app-bridge layer on both ends.

## Configuration

- User config: `~/.solo/config.json` (JSON, managed by `daemon/internal/config/`)
- Runtime state: `~/.solo/` directory (keypairs, memory, logs)
- Build-time config: Makefile variables, CI environment variables
- Never read config from environment variables in the daemon; use the config file. CLI may use env vars for one-off overrides.

## Adding New Features

1. Identify which layer the feature belongs to (see Layer Responsibilities).
2. If it involves cross-module communication, define the protocol types in `protocol/` first.
3. Implement backend (Go) before frontend (TypeScript).
4. Add tests at each layer: unit tests for logic, integration tests for boundaries.
5. Update `docs/` for architectural changes. Add an ADR for significant decisions.
