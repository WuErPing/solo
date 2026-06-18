# Solo Architecture — First-Principles Review

> **Date**: 2026-06-18
> **Back to**: [Documentation Index](../README.md) | [Analysis Index](./README.md)

## Methodology

This review reasons from first principles: what are the fundamental problems Solo solves, and does each architectural decision serve those problems optimally? No assumption that current design is correct — every choice is re-evaluated from the original requirements.

## Core Problems (from first principles)

Solo addresses five essential needs:

1. **Unified lifecycle management** — multiple AI coding agents, each with its own terminal interface
2. **Real-time observation** — streaming agent terminal output to the user
3. **Agent interaction** — sending commands and injecting keystrokes
4. **Cross-device access** — local and remote, with security
5. **Cross-platform UI** — Web, iOS, Android from a single codebase

## Architectural Decisions — Evaluated

### 1. Daemon as the central hub — Reasonable

**First-principle reasoning**: AI agents run as terminal processes on the user's machine. Terminal processes require a local resident service to manage them. A daemon binding `127.0.0.1:17612` is the minimal viable architecture.

**Strengths**:
- Local-first, no network exposure by default — correct security posture
- Manages PTY, tmux, agent lifecycle — all operations requiring user permissions stay local
- Exposes WebSocket for frontend — standard bidirectional real-time transport

**Concern — scope creep**: The daemon currently serves as:
- Agent lifecycle manager
- PTY/terminal session manager
- Tmux interaction layer
- Session memory store
- Cron-based scheduler
- Relay client
- WebSocket server
- Push notification proxy

From first principles, "manage agents" and "schedule cron jobs" are different concerns. If the daemon remains manageable at current scale (it appears to be), premature splitting adds complexity. **Acceptable now, but watch for bloat.**

### 2. Protocol as an independent module — Very reasonable

**First-principle reasoning**: Go and TypeScript communicate over WebSocket and need shared message type definitions. `protocol/` with zero external dependencies is pure data typing.

**Strengths**:
- Minimal dependency transitivity (relay doesn't pull pty/cron)
- Message type changes can be reviewed independently
- Compilation isolation, faster builds

**Note**: Go's `protocol` and TS's `app-bridge/src/shared/messages.ts` are manually mirrored, with no code generation. This means type drift is possible between the two sides. This is a conscious tradeoff — code generation toolchain complexity may not be worth it at current scale.

### 3. App-Bridge communication abstraction — Reasonable with a caveat

**First-principle reasoning**: The frontend needs to communicate with the daemon via two transport modes (direct WS, Relay E2EE). A unified abstraction is necessary.

**Strengths**:
- Pluggable transport (direct vs Relay) — correct abstraction boundary
- Zod schema validation — runtime type safety
- E2EE encryption at the bridge layer — proper concern separation

**Caveat**: `app-bridge` is not declared as a dependency in `app/package.json` — it's imported as a workspace sibling at runtime. This works fine within the monorepo but would break if `@getsolo/app` were published as a standalone package. Acceptable for internal use; fix if external distribution is planned.

### 4. Relay as a pure forwarder with E2EE — Very reasonable

**First-principle reasoning**: Remote access requires an intermediary server. The minimal-trust principle demands the server cannot see plaintext.

**This is the cleanest part of the architecture**:
- Stateless message router, no storage
- E2EE with X25519 + XSalsa20-Poly1305 — standard cryptographic primitives
- Pairing via URL fragment — fragments are never sent to the server
- Relay only does one thing: forward encrypted messages

No business logic, no state, easy to deploy and audit. This is exactly what a relay should be.

### 5. Tmux as the agent observation surface — Pragmatic, with technical debt

**First-principle reasoning**: Many AI agents offer no API, only a terminal interface. To unify management, we must observe and interact with terminals.

**Pragmatism**:
- 3-layer heuristic detection (command name, pane title normalization, child process inspection) — handles real-world messiness
- Pane capture for output, key injection for interaction — no agent cooperation needed

**Technical debt**:
- Heuristic detection is inherently fragile — agent CLI changes can break it
- Tmux dependency means no support for non-tmux environments (e.g., pure Windows)
- But this is likely the only viable approach today — agent ecosystems lack standard APIs

**Conclusion**: Reasonable under current constraints. Should be treated as a transitional approach. Long-term, push for agent API standardization (ACP/Cursor Agent integration is on the roadmap).

### 6. Session memory as Markdown files — Reasonable, with a clear migration path

**First-principle reasoning**: Session data needs persistence. Filesystem is the simplest store.

- Markdown + YAML frontmatter is human-readable and git-trackable
- JSONL index provides fast lookup
- Redaction layer strips secrets before writing — security considered
- Explicitly marked as Phase 1 with a migration path to a database

For personal use (single user, local machine), the filesystem is entirely sufficient. Multi-user collaboration would require migration, but that's a different product phase.

### 7. Frontend technology choices — Reasonable

**React Native + Expo** means one codebase covers Web/iOS/Android. For a developer-led project, this maximizes coverage correctly.

**State management**: Zustand (client state) + TanStack Query (server state) — modern React best practice.

**Observation**: ~121 components, ~95 hooks, ~33 stores — substantial for a project still in development. If growth continues, consider more explicit module boundaries (e.g., grouping by feature domain rather than by type).

## Module Dependency Graph

```
┌─────────────────────────────────────────────────────┐
│                    Frontend (TS)                      │
│                                                       │
│  ┌─────────┐    ┌────────────┐    ┌───────────────┐  │
│  │ highlight│◄───│    app     │───►│  app-bridge   │  │
│  └─────────┘    └────────────┘    └───────┬───────┘  │
└───────────────────────────────────────────┼──────────┘
                                            │ WebSocket
                   ┌────────────────────────┼──────────┐
                   │         Go Backend      │          │
                   │                          ▼          │
                   │  ┌──────────┐   ┌──────────────┐   │
                   │  │ protocol │◄──│    daemon     │   │
                   │  └────┬─────┘   └──────┬───────┘   │
                   │       │                │            │
                   │       ▼                ▼            │
                   │  ┌─────────┐    ┌──────────────┐   │
                   │  │  relay  │    │     cli      │   │
                   │  └─────────┘    └──────────────┘   │
                   └────────────────────────────────────┘
```

## Summary Assessment

| Dimension | Rating | Rationale |
|-----------|--------|-----------|
| **Concern separation** | Good | Clear module boundaries; Relay is especially clean |
| **Security** | Good | E2EE, local binding, redaction layer, no key persistence |
| **Extensibility** | Fair | Provider plugin architecture is good, but daemon scope is broad |
| **Pragmatism** | Excellent | tmux observation, file storage, manual type mirroring — all correct tradeoffs |
| **Cross-language consistency** | Fair | Manual type mirroring risks drift, but avoids toolchain complexity |
| **Frontend architecture** | Good | Modern React practices; component/hook count warrants attention |

## Key Long-Term Risks

1. **Daemon bloat** — Currently acceptable, but needs ongoing review of whether to split out scheduling, memory, or other concerns.
2. **Cross-language type drift** — Manual mirroring is error-prone when message types change frequently. Consider code generation if the message surface grows significantly.
3. **Tmux coupling** — The heuristic-based observation layer is fragile. Prioritize API-based provider integration (ACP) for new providers.
