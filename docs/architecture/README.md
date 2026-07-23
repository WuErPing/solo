# Solo System Architecture

> **Back to**: [Documentation Index](../README.md)

## In This Directory

- [Network Architecture](network-architecture.md) — Nginx → Relay → Daemon paths, E2EE, Pairing Link
- [Network · Data · State Architecture](network-data-state-architecture.md) — Synthesis of the three layers end-to-end: paths, stores, sync
- [Components](components.md) — App, App-Bridge, Daemon, Relay, CLI, Protocol specs
- [Data Flow](data-flow.md) — WS message flows, session lifecycle, heartbeat
- [Timeline Design](timeline-design.md) — Head/Tail stream model, seq gate, bootstrap policy, catch-up
- [Session Memory Persistence](session-memory-persistence.md) — Turn recording hooks, TurnRecorder interface, file layout, migration path
- [Agent Stall Detection](agent-stall-detection.md) — Inactivity & repetition detection, grace-period fix, operational tuning
- [Tmux Pane Content Loading](tmux-pane-content-loading.md) — Tmux agent detection, pane capture, polling, and key injection flow
- [Push Notifications](push-notifications.md) — Push notification architecture and delivery flow
- [Schedule Assistant](schedule-assistant.md) — NL schedule parse via configured LLM providers, proposal-only safety, confirm path
- [Deployment](deployment.md) — Systemd, Docker, Nginx config, env vars

See also [`../product/session-memory-spec.md`](../product/session-memory-spec.md) for the Phase 1 implementation spec (M1–M6 shipped 2026-05-28).

## Related

- [Provider Integration Research](../providers/) — Kimi Wire vs ACP, Cursor-Agent plan
- [Technical Analysis](../analysis/) — Host status check probe cycle deep-dive

## System Architecture Diagram

**Visual versions**:
- Overview: [SVG](solo-system-architecture.svg) | [PNG](solo-system-architecture.png)
- Detailed: [SVG](solo-system-architecture-detailed.svg) | [PNG](solo-system-architecture-detailed.png)

<details>
<summary>ASCII version (for text-only environments)</summary>

```
┌──────────────────────────────────────────────────────────────────────────┐
│                              Client Layer                                │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐       │
│  │     Web App      │  │   Mobile App     │  │       CLI        │       │
│  │  (Expo Router)   │  │  (iOS / Android) │  │   (solo-cli)     │       │
│  │                  │  │                  │  │                  │       │
│  │  Screens:        │  │  Screens:        │  │  Commands:       │       │
│  │  · Dashboard     │  │  · Dashboard     │  │  · agent ls/run  │       │
│  │  · Agent Detail  │  │  · Agent Detail  │  │  · daemon start  │       │
│  │  · Sessions      │  │  · Sessions      │  │  · loop ls/run   │       │
│  │  · Schedules     │  │  · Schedules     │  │  · provider ls   │       │
│  │  · Loops         │  │  · Loops         │  │  · onboard       │       │
│  │  · Tmux Dash     │  │  · Tmux Dash     │  │                  │       │
│  │  · Tmux Pane     │  │  · Tmux Pane     │  │                  │       │
│  │  · Projects      │  │  · Projects      │  │                  │       │
│  │  · Workspace     │  │  · Workspace     │  │                  │       │
│  │  · Settings      │  │  · Settings      │  │                  │       │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘       │
│           └──────────────────────┴──────────────────────┘                │
│                                  │                                       │
│                    ┌─────────────▼──────────────┐                        │
│                    │        App-Bridge          │                        │
│                    │  ┌──────────────────────┐  │                        │
│                    │  │    DaemonClient      │  │                        │
│                    │  │  · WebSocket trans.  │  │                        │
│                    │  │  · Relay E2EE trans. │  │                        │
│                    │  │  · Runtime metrics   │  │                        │
│                    │  └──────────────────────┘  │                        │
│                    │  ┌────────────┐ ┌────────┐ │                        │
│                    │  │ Agent RPCs │ │ Tmux   │ │                        │
│                    │  │ Schedule   │ │ RPCs   │ │                        │
│                    │  │ Loop RPCs  │ │ Chat   │ │                        │
│                    │  └────────────┘ └────────┘ │                        │
│                    │  ┌──────────────────────┐  │                        │
│                    │  │   Relay / E2EE       │  │                        │
│                    │  │  · X25519 + XSalsa  │  │                        │
│                    │  │  · EncryptedChannel  │  │                        │
│                    │  └──────────────────────┘  │                        │
│                    └─────────────┬──────────────┘                        │
│                                  │                                       │
┌──────────────────────────────────▼───────────────────────────────────────┐
│                           Network Layer                                  │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                      Nginx (optional)                            │   │
│  │               TLS termination · reverse proxy                    │   │
│  └───────────────────────────────┬──────────────────────────────────┘   │
│                                  │                                       │
│  ┌───────────────────────────────▼──────────────────────────────────┐   │
│  │                    Relay Server (relay-go)                       │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐             │   │
│  │  │    Control    │ │     Data     │ │    Session   │             │   │
│  │  │   Channel     │ │   Channel    │ │   Manager    │             │   │
│  │  └──────────────┘ └──────────────┘ └──────────────┘             │   │
│  │  ┌──────────────┐ ┌──────────────┐                              │   │
│  │  │    Crypto     │ │   Metrics    │                              │   │
│  │  │ (X25519+XS)  │ │ (Prometheus) │                              │   │
│  │  └──────────────┘ └──────────────┘                              │   │
│  └───────────────────────────────┬──────────────────────────────────┘   │
└──────────────────────────────────┼───────────────────────────────────────┘
                                   │
┌──────────────────────────────────▼───────────────────────────────────────┐
│                            Service Layer                                 │
│                       Daemon (daemon/internal)                           │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────── HTTP / WebSocket Server ────────────────────┐ │
│  │  ┌────────────────────────────────────────────────────────────┐   │ │
│  │  │  server/daemon.go — service orchestration, handler registry│   │ │
│  │  └────────────────────────────────────────────────────────────┘   │ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌────────────────────────┐   │ │
│  │  │   session/   │ │  terminal/   │ │      workspace/        │   │ │
│  │  │  · agent     │ │  · PTY mgmt  │ │  · ProjectRegistry     │   │ │
│  │  │  · terminal  │ │  · resize    │ │  · WorkspaceRegistry   │   │ │
│  │  │  · tmux      │ │              │ │  · GitService          │   │ │
│  │  │  · schedule  │ │              │ │  · ScriptManager       │   │ │
│  │  │  · loop      │ │              │ │  · FileExplorer        │   │ │
│  │  │  · workspace │ │              │ │  · ScriptProxy         │   │ │
│  │  │  · send      │ │              │ │                        │   │ │
│  │  │  · multi-    │ │              │ │                        │   │ │
│  │  │    socket    │ │              │ │                        │   │ │
│  │  └──────────────┘ └──────────────┘ └────────────────────────┘   │ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐             │ │
│  │  │  attention/  │ │  sendqueue/  │ │  activity/   │             │ │
│  │  │  policy +   │ │  async msg   │ │  tracker     │             │ │
│  │  │  broadcast  │ │  buffering   │ │  heartbeat   │             │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘             │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                                                       │
│  ┌──────────────────── agent/ (Agent Manager) ──────────────────────┐ │
│  │  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌──────┐ ┌──────────┐   │ │
│  │  │ Claude  │ │  Kimi   │ │ OpenCode │ │  Pi  │ │  Codex   │   │ │
│  │  │ (print/ │ │ (Wire/  │ │  (SSE)   │ │(JSON │ │(auto/    │   │ │
│  │  │ stream) │ │ JSONRPC)│ │          │ │stdio)│ │full-acc) │   │ │
│  │  └─────────┘ └─────────┘ └──────────┘ └──────┘ └──────────┘   │ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │ │
│  │  │  ProviderReg │ │  AgentStore  │ │  TurnGuard   │            │ │
│  │  │  discovery   │ │  persistence │ │  dedup guard │            │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘            │ │
│  │  ┌──────────────┐ ┌──────────────┐                             │ │
│  │  │ StallMonitor │ │ CustomModels │                             │ │
│  │  │ stuck detect │ │ user-defined │                             │ │
│  │  └──────────────┘ └──────────────┘                             │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│                                                                       │
│  ┌────────────────── loop/ (Loop Engine) ───────────────────────────┐ │
│  │  ┌──────────────┐ ┌──────────────┐                              │ │
│  │  │   Engine     │ │    Store     │                              │ │
│  │  │  iteration   │ │  persistence │                              │ │
│  │  └──────────────┘ └──────────────┘                              │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                                                       │
│  ┌────────────────── schedule/ (Schedule Engine) ───────────────────┐ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │ │
│  │  │    Store     │ │   Executor   │ │    Runner    │            │ │
│  │  │  cron state  │ │  agent exec  │ │  cron loop   │            │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘            │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                                                       │
│  ┌─────────────── Supporting Services ──────────────────────────────┐ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │ │
│  │  │   memory/    │ │    push/     │ │ relayclient/ │            │ │
│  │  │ TurnRecorder │ │  FCM / APNs  │ │  · Control   │            │ │
│  │  │ filebackend  │ │  web push    │ │  · Data conn │            │ │
│  │  │ redact/      │ │              │ │  · E2EE      │            │ │
│  │  │ bridge/      │ │              │ │  · Keepalive │            │ │
│  │  │ SafeBridge   │ │              │ │  · Reconnect │            │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘            │ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │ │
│  │  │  metrics/    │ │   config/    │ │   pidlock/   │            │ │
│  │  │  Prometheus  │ │ MemoryConfig │ │  single inst │            │ │
│  │  │  /metrics    │ │ CustomModels │ │  guard       │            │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘            │ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │ │
│  │  │  wsconn/     │ │ memorysetup/ │ │    llm/      │            │ │
│  │  │  WS conn     │ │  wiring +    │ │  chat client │            │ │
│  │  │  abstract.   │ │  assembly    │ │  (schedule   │            │ │
│  │  │              │ │              │ │  assistant)  │            │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘            │ │
│  └──────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

</details>

## Core Components

| Component | Directory | Language | Responsibility |
|-----------|-----------|----------|----------------|
| **App** | `app/` | TypeScript / React Native | User interface (iOS, Android, Web) |
| **App-Bridge** | `app-bridge/` | TypeScript | Client-side communication library |
| **Daemon** | `daemon/` | Go | Core service — manages sessions, agents, loops, and provider connections |
| **Relay** | `relay-go/` | Go | Connection relay for remote/mobile access |
| **CLI** | `cli/` | Go | Command-line tool for session and agent management |
| **Protocol** | `protocol/` | Go | Shared protocol definitions |
| **Highlight** | `packages/highlight/` | TypeScript | Syntax highlighting library |
| **LLM Client** | `daemon/internal/llm/` | Go | OpenAI-compatible chat completion client (schedule assistant) |
| **Tmux Subsystem** | `daemon/internal/server/session_tmux.go` | Go | Tmux agent detection, pane capture, key injection |
| **SVG Preview** | `app/src/components/svg-preview*.tsx` | TypeScript | SVG file preview (WebView for mobile, native for web) |

## Quick Links

- [Network Architecture Details](network-architecture.md)
- [View Component Details](components.md)
- [Learn about Data Flow](data-flow.md)
- [Deployment Guide](deployment.md)
