# Solo System Architecture

> **Back to**: [Documentation Index](../README.md)

## In This Directory

- [Network Architecture](network-architecture.md) — Nginx → Relay → Daemon paths, E2EE, Pairing Link
- [Components](components.md) — App, App-Bridge, Daemon, Relay, CLI, Protocol specs
- [Data Flow](data-flow.md) — WS message flows, session lifecycle, heartbeat
- [Timeline Design](timeline-design.md) — Head/Tail stream model, seq gate, bootstrap policy, catch-up
- [Session Memory Persistence](session-memory-persistence.md) — Turn recording hooks, TurnRecorder interface, file layout, migration path
- [Agent Stall Detection](agent-stall-detection.md) — Inactivity & repetition detection, grace-period fix, operational tuning
- [Push Notifications](push-notifications.md) — Push notification architecture and delivery flow
- [Deployment](deployment.md) — Systemd, Docker, Nginx config, env vars

See also [`../product/session-memory-spec.md`](../product/session-memory-spec.md) for the Phase 1 implementation spec (M1–M6 shipped 2026-05-28).

## Related

- [Provider Integration Research](../providers/) — Kimi Wire vs ACP, Cursor-Agent plan
- [Technical Analysis](../analysis/) — Host status check probe cycle deep-dive

## System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Web App    │  │  Mobile App  │  │    CLI       │         │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘         │
└─────────┼─────────────────┼─────────────────┼──────────────────┘
          │                 │                 │
          └─────────────────┴─────────────────┘
                            │
                    ┌───────▼───────┐
                    │   App-Bridge   │
                    └───────┬───────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                      Network Transport Layer                    │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Nginx (optional)                      │   │
│  └─────────────────────────┬───────────────────────────────┘   │
│                            │                                    │
│  ┌─────────────────────────▼───────────────────────────────┐   │
│  │              Relay Server                               │   │
│  └─────────────────────────┬───────────────────────────────┘   │
└────────────────────────────┼────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                      Service Layer                              │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Daemon                                     │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

| Component | Directory | Language | Responsibility |
|-----------|-----------|----------|----------------|
| **App** | `app/` | TypeScript/React Native | User Interface |
| **App-Bridge** | `app-bridge/` | TypeScript | Client Communication Library |
| **Daemon** | `daemon/` | Go | Core Service |
| **Relay** | `relay-go/` | Go | Connection Relay |
| **CLI** | `cli/` | Go | Command Line Tool |
| **Protocol** | `protocol/` | Go | Protocol Definitions |
| **Highlight** | `packages/highlight/` | TypeScript | Syntax Highlighting Library |

## Quick Links

- [Network Architecture Details](network-architecture.md)
- [View Component Details](components.md)
- [Learn about Data Flow](data-flow.md)
- [Deployment Guide](deployment.md)
