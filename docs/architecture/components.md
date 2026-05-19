# 组件详细说明

## 1. App (客户端应用)

**目录**: `app/`

**技术栈**: React Native + Expo

**职责**:
- 提供用户界面（Web、iOS、Android）
- 通过 App-Bridge 与 Daemon 通信
- 管理用户会话和工作区

**关键目录**:
- `src/screens/` - 页面组件
- `src/components/` - 可复用组件
- `src/app/` - Expo Router 路由

## 2. App-Bridge (客户端通信库)

**目录**: `app-bridge/`

**技术栈**: TypeScript

**职责**:
- 封装 WebSocket 通信细节
- 支持直接连接和 Relay 连接
- 实现端到端加密 (E2EE)

**关键模块**:

### 2.1 Client (客户端)

**目录**: `src/client/`

| 文件 | 职责 |
|------|------|
| `daemon-client.ts` | 主客户端，管理连接状态 |
| `daemon-client-websocket-transport.ts` | WebSocket 传输实现 |
| `daemon-client-relay-e2ee-transport.ts` | Relay E2EE 传输 |
| `daemon-client-transport-types.ts` | 传输层类型定义 |
| `daemon-client-transport-utils.ts` | 传输工具函数 |

### 2.2 Relay (中继)

**目录**: `src/relay/`

| 文件 | 职责 |
|------|------|
| `e2ee.ts` | 端到端加密实现 |
| `encrypted-channel.ts` | 加密通道 |
| `crypto.ts` | 加密工具 |
| `base64.ts` | Base64 编码工具 |

### 2.3 Server (服务端)

**目录**: `src/server/`

| 目录 | 职责 |
|------|------|
| `agent/` | Agent 管理 |
| `chat/` | 聊天功能 |
| `loop/` | 主循环 |
| `schedule/` | 调度器 |

## 3. Daemon (守护进程)

**目录**: `daemon/`

**技术栈**: Go

**职责**:
- 核心服务，管理所有业务逻辑
- WebSocket 服务器
- Agent 生命周期管理
- 工作区和项目管理

**架构**:

```
daemon/
├── main.go              # 入口
└── internal/
    ├── agent/           # Agent 管理
    ├── config/          # 配置
    ├── metrics/         # 指标
    ├── pidlock/         # PID 锁
    ├── push/            # 推送通知
    ├── relayclient/     # Relay 客户端
    ├── server/          # WebSocket 服务器
    ├── terminal/        # 终端管理
    ├── workspace/       # 工作区管理
    └── wsconn/          # WebSocket 连接抽象
```

### 3.1 Server (WebSocket 服务器)

**目录**: `internal/server/`

核心文件:
- `daemon.go` - Daemon 主结构，服务编排
- `session.go` - 会话管理
- `session_agent.go` - Agent 会话
- `session_terminal.go` - 终端会话
- `handler_registry.go` - 处理器注册表

### 3.2 Relay Client (Relay 客户端)

**目录**: `internal/relayclient/`

核心文件:
- `client.go` - Relay 客户端实现
- `e2ee.go` - 端到端加密
- `e2ee_test.go` - E2EE 测试

功能:
- 维护控制连接（Control Connection）
- 管理数据连接（Data Connection）
- 自动重连
- 心跳保活

### 3.3 Agent Manager (Agent 管理器)

**目录**: `internal/agent/`

功能:
- Agent 生命周期管理
- Provider 注册和发现
- 模型配置

### 3.4 Workspace (工作区)

**目录**: `internal/workspace/`

功能:
- 工作区管理
- Git 集成
- 脚本执行

## 4. Relay (中继服务器)

**目录**: `relay-go/`

**技术栈**: Go

**职责**:
- WebSocket 连接中继
- 会话管理
- 消息缓冲
- NAT 穿透支持

**架构**:

```
relay-go/
├── cmd/relay/
│   └── main.go          # 入口
└── internal/
    ├── config/          # 配置
    ├── metrics/         # 指标
    └── relay/           # 核心实现
        ├── server.go    # HTTP/WebSocket 服务器
        ├── session.go   # 会话管理
        ├── session_manager.go # 会话管理器
        ├── control.go   # 控制连接逻辑
        └── buffer.go    # 消息缓冲
```

### 4.1 Server

**文件**: `internal/relay/server.go`

功能:
- HTTP 服务器
- WebSocket 升级
- 健康检查端点 (`/health`)
- 指标端点 (`/metrics`)

### 4.2 Session

**文件**: `internal/relay/session.go`

功能:
- 会话状态管理
- 消息路由
- 连接配对

## 5. CLI (命令行工具)

**目录**: `cli/`

**技术栈**: Go

**职责**:
- 命令行交互
- 会话管理
- 配置管理

## 6. Protocol (协议定义)

**目录**: `protocol/`

**技术栈**: Go

**职责**:
- 定义共享协议常量
- 消息结构
- 类型定义

**核心文件**:
- `protocol.go` - 协议常量
- `message.go` - 消息类型
- `message_*.go` - 各类消息定义

## 组件交互

```
┌─────────┐     ┌─────────────┐     ┌─────────┐
│   App   │◄───►│ App-Bridge  │◄───►│  Relay  │
│         │     │             │     │         │
└─────────┘     └─────────────┘     └────┬────┘
                                         │
                                    ┌────┴────┐
                                    │  Daemon │
                                    └─────────┘
```

## 数据流向

1. **用户操作** → App
2. **App** → App-Bridge (封装消息)
3. **App-Bridge** → Relay (可选，公网模式)
4. **Relay** → Daemon (转发消息)
5. **Daemon** → 业务处理 → 返回结果
6. **结果** → Relay → App-Bridge → App
