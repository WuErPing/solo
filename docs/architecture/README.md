# Solo System Architecture

> **Back to**: [Documentation Index](../README.md)

## In This Directory

- [Network Architecture](network-architecture.md) — Nginx → Relay → Daemon paths, E2EE, Pairing Link
- [Components](components.md) — App, App-Bridge, Daemon, Relay, CLI, Protocol specs
- [Data Flow](data-flow.md) — WS message flows, session lifecycle, heartbeat
- [Timeline Design](timeline-design.md) — Head/Tail stream model, seq gate, bootstrap policy, catch-up
- [Session Memory Persistence](session-memory-persistence.md) — Turn recording hooks, TurnRecorder interface, file layout, migration path
- [Deployment](deployment.md) — Systemd, Docker, Nginx config, env vars

## Related

- [Provider Integration Research](../providers/) — Kimi Wire vs ACP, Cursor-Agent plan
- [Technical Analysis](../analysis/) — Host status check probe cycle deep-dive

## 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         客户端层                                  │
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
│                      网络传输层                                   │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Nginx (可选)                          │   │
│  └─────────────────────────┬───────────────────────────────┘   │
│                            │                                    │
│  ┌─────────────────────────▼───────────────────────────────┐   │
│  │              Relay Server (中继服务器)                    │   │
│  └─────────────────────────┬───────────────────────────────┘   │
└────────────────────────────┼────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                      服务层                                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Daemon (守护进程)                           │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## 核心组件

| 组件 | 目录 | 语言 | 职责 |
|------|------|------|------|
| **App** | `app/` | TypeScript/React Native | 用户界面 |
| **App-Bridge** | `app-bridge/` | TypeScript | 客户端通信库 |
| **Daemon** | `daemon/` | Go | 核心服务 |
| **Relay** | `relay-go/` | Go | 连接中继 |
| **CLI** | `cli/` | Go | 命令行工具 |
| **Protocol** | `protocol/` | Go | 协议定义 |

## 快速链接

- [网络架构详解](network-architecture.md)
- [查看组件详情](components.md)
- [了解数据流](data-flow.md)
- [部署指南](deployment.md)
