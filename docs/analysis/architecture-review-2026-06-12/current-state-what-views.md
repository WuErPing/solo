# 架构评审报告 — Solo（现状概览、设计要点与 4+1 视图）

## 2. 现状概览 (Current State)

### 技术栈

| 层 | 技术 | 版本 |
|----|------|------|
| Backend | Go | 1.25 |
| WebSocket | gorilla/websocket | latest |
| PTY | creack/pty | latest |
| Logging | slog (stdlib) | Go 1.25 |
| Frontend | React Native + Expo | 0.81 / SDK 54 |
| State | Zustand | latest |
| Data Fetch | @tanstack/react-query | latest |
| Styling | Unistyles | latest |
| Crypto | tweetnacl (X25519 + XSalsa20-Poly1305) | latest |
| Validation | Zod | latest |
| CI | GitHub Actions | — |

### 架构拓扑

```
┌─────────────────────────────────────────────────────────┐
│                    Presentation Layer                     │
│  Expo Router (app/src/app/)                              │
│  Components (136 files, 45K LOC)                         │
│  Screens (42 files, 16K LOC)                             │
└──────────────┬──────────────────────────────────────────┘
               │ hooks + contexts
┌──────────────▼──────────────────────────────────────────┐
│                    State Management                       │
│  Zustand Stores (17 stores, 7.6K LOC)                    │
│  React Contexts (12 files, 3.4K LOC)                     │
│  TanStack Query                                          │
└──────────────┬──────────────────────────────────────────┘
               │ @server/* imports
┌──────────────▼──────────────────────────────────────────┐
│                    Bridge Layer (app-bridge)               │
│  DaemonClient (4.4K LOC) — RPC, events, reconnect        │
│  E2EE Channel (607 LOC) — X25519 + XSalsa20              │
│  Zod Schemas (3.8K LOC) — wire protocol types             │
└──────────────┬──────────────────────────────────────────┘
               │ WebSocket (direct or relay)
┌──────────────▼──────────────────────────────────────────┐
│                    Network Layer                           │
│  Nginx (:443) → Relay (:8081) or Direct (:17612)         │
└──────────────┬──────────────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────────────┐
│                    Service Layer (Go daemon)               │
│  server/ (6.5K LOC) — WS handlers, session mgmt          │
│  agent/ (9.8K LOC) — provider abstraction, lifecycle      │
│  workspace/ (1.6K LOC) — domain models, git service       │
│  memory/ (1.4K LOC) — session persistence                 │
│  schedule/ (587 LOC) — cron automation                    │
│  terminal/ (484 LOC) — PTY management                     │
└─────────────────────────────────────────────────────────┘
```

### 数据流（Agent 消息发送）

```
User Input (app)
  → composer.tsx → useSendMessage hook
    → session-context.tsx.sendAgentMessage()
      → DaemonClient.sendMessage() [app-bridge]
        → WebSocket frame (JSON)
          → [optional: Relay E2EE encrypt]
            → daemon server/session.go.handleSendAgentMessage()
              → agent/ManagedAgent.SendTurn()
                → provider (claude/opencode/kimi) subprocess stdin
                  → provider stdout stream
                    → agent/base/EventPump.Translate()
                      → InMemoryTimelineStore.Append()
                        → Session.broadcastEvent()
                          → WebSocket frame (JSON)
                            → [optional: Relay E2EE decrypt]
                              → DaemonClient event handler
                                → session-context.tsx event reducer
                                  → Zustand session store mutation
                                    → React re-render
```

### 部署形态

- **本地开发**：daemon + Expo dev server，直连 WebSocket
- **远程访问**：daemon → Relay（Nginx SSL 终止）→ 客户端
- **生产部署**：Systemd 管理 daemon，Docker 管理 relay

### 测试现状

| 模块 | 测试类型 | 测试行数 | 覆盖率 |
|------|---------|---------|--------|
| daemon | Go table-driven | ~13,500 | 高 |
| protocol | Go unit | ~1,466 | 高 |
| relay-go | Go unit | ~1,800 | 高 |
| app | Vitest unit | ~25,000 | ~36% stmt |
| app-bridge | Vitest unit | ~800 | ~89% stmt |
| E2E | Playwright (nightly) | 22 specs | — |

---

## 5. What：设计要点与细节

### 5.1 运行时与资源

**Go daemon**：
- **计算模式**：IO 密集（WebSocket I/O + subprocess stdout + 文件持久化）
- **内存**：InMemoryTimelineStore 无上限（⚠️ 长时间运行可能增长）；AgentManager 按 Agent 数量线性增长
- **存储**：JSON 文件（Agent 配置、Workspace 注册表、Schedule Store），无数据库依赖
- **连接池**：WebSocket 连接有 grace period（90s），支持多 socket per session

**React Native app**：
- **内存**：Zustand stores 驻留内存，session store 有 identity-preserving 更新和 coalescer 防止级联渲染
- **持久化**：AsyncStorage 用于 panel/draft/sidebar 配置，有版本化迁移链

### 5.2 网络与基础设施

- **本地直连**：daemon (:17612) ↔ app，延迟 < 1ms（同机 loopback）
- **Relay 链路**：app → Nginx (:443) → Relay (:8081) → daemon (:17612)，增加 ~2-5ms
- **E2EE 握手**：Curve25519 ECDH 一次性密钥交换，后续 XSalsa20-Poly1305 加密
- **重连**：DaemonClient 支持指数退避重连（可配置 base/max delay）
- **序列化**：JSON over WebSocket text frame（Agent 流事件）；Binary frame（终端流）

### 5.3 应用架构

#### Go 后端：Clean Architecture 合规性

**依赖方向**（✅ 符合依赖规则）：
```
protocol (leaf, zero imports)
    ↑
agent/ (domain, imports only protocol)
    ↑
server/ (application, imports agent + protocol)
    ↑
daemon/main (composition root, imports everything)
```

**接口所有权**（18/18 消费者定义）：

| 接口 | 定义位置 | 实现位置 |
|------|---------|---------|
| `AgentClient` | agent/manager.go | agent/provider_*.go |
| `AgentSession` | agent/manager.go | agent/provider_*.go |
| `TimelineAppender` | agent/manager.go | server/ |
| `WSConn` | wsconn/wsconn.go | gorilla/websocket |
| `Pusher` | push/service.go | push/ExpoPushService |
| `TurnRecorder` | memory/recorder.go | memory/filebackend/ |
| `Redactor` | memory/redactor.go | memory/redact/ |
| `MemoryBridge` | server/memorybridge.go | memory/bridge/ |
| `Runner` | schedule/executor.go | server/schedule_runner.go |
| `SessionAttacher` | relayclient/client.go | server/WSServer |

**Provider 插件架构**：
```go
// 新增 Provider 只需：
// 1. 实现 AgentClient + AgentSession 接口
// 2. 在 NewDaemon() 中 registry.Register(NewXxxClient())
// Manager 层零修改 → 符合 OCP
```

**消息处理管线**（OCP 良好）：
```
WebSocket Read → inboundQueue (chan, 64) → processLoop → handlerRegistry.Handle()
```
handler_registry.go 采用注册模式，新增消息类型只需 `r.Register()` 一行调用。

#### TypeScript 前端：Pragmatic React 架构（非 Clean Architecture）

**违反依赖规则**：
```
components/ (presentation)
  → hooks/ (application logic mixed with React hooks)
    → stores/ (state management)
      → @server/client/daemon-client (infrastructure)
```

Presentation 层直接依赖基础设施层，无中间的 domain 或 use-case 层。

**God Object: session-context.tsx**（1,672 行）：
- 订阅 14+ daemon 事件类型
- 管理 OS 通知逻辑
- 编排 Agent 消息发送
- 调度 revalidation
- 处理权限请求/响应
- 连接 React hooks 和 Zustand store

**缺少抽象层**：
- 无 `interface AgentService` / `interface WorkspaceRepository`
- DaemonClient 直接被 30+ 文件导入
- 新增 daemon 事件需修改 context → store → hooks → components 四层

**正面模式**：
- Zustand store 有良好的 serverId scoping（支持多 daemon 连接）
- Panel Registry 是 OCP 的好例子
- session-stream-reducers.ts（807 行）将纯 reducer 逻辑从 context 中提取
- 类型安全：TS 侧 `any` 使用为零

### 5.4 可观测性

**Go daemon**：
- **日志**：slog 结构化 JSON，INFO/WARN/ERROR 级别
- **指标**：Prometheus 4 项指标（SessionsActive, ConnectionsTotal, MessagesSent, MessagesReceived）+ /metrics + /api/health
- **告警**：无主动告警机制（本地工具定位，可接受）
- **链路追踪**：无（本地工具，可接受）

**日志有效性评估**：
- ✅ Agent crash 有 panic recovery + error state 设置
- ✅ Push 通知失败 logged as warning（不阻塞主流程）
- ⚠️ Session Memory persistTurn() 静默吞错（P0 连续 2 次未修复）
- ⚠️ CORS 空列表不报错（P0 连续 2 次未修复）

### 5.6 AI-Context 友好度

| 指标 | 评估 | 详情 |
|------|------|------|
| **文件粒度** | ⚠️ | 最大文件：daemon-client.ts 4,386 行、messages.ts 3,753 行、message.tsx 2,916 行 |
| **类型安全（Go）** | ❌ 恶化 | interface{} 987 处（+28.7%）、map[string]interface{} 754 处（+25.2%） |
| **类型安全（TS）** | ✅ 优秀 | `any` 使用为零，Zod schema 强类型 |
| **修改半径** | ⚠️ | 新增 daemon 事件需 touch 4+ 文件（Go protocol → daemon handler → TS schema → context） |
| **跨语言同步** | ❌ | Go struct 与 TS Zod schema 手动维护双份定义，无自动生成 |
| **命名清晰度** | ✅ | 公开 API 零缩写，接口签名自描述 |
| **AI 迭代循环** | ✅ | `make test` 单一入口；Go `go test -short -race` < 30s；JS `npm test` < 60s |

---

## 8. 4+1 架构设计视图

### 8.1 逻辑视图 (Logical View)

```
┌─────────────────────────────────────────────────────────┐
│                    Domain Layer                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │  Agent    │  │ Workspace│  │ Schedule │              │
│  │  Manager  │  │ Registry │  │ Executor │              │
│  │          │  │          │  │          │              │
│  │ AgentClient│ │ Record   │  │ Runner   │              │
│  │ AgentSession│ │ GitService│ │ Store    │              │
│  └──────────┘  └──────────┘  └──────────┘              │
│  ┌──────────┐  ┌──────────┐                            │
│  │ Timeline │  │ Memory   │                            │
│  │ Store    │  │ TurnRec. │                            │
│  │ Coalescer│  │ Redactor │                            │
│  └──────────┘  └──────────┘                            │
└─────────────────────────────────────────────────────────┘
                           ↑ (interfaces)
┌─────────────────────────────────────────────────────────┐
│                    Application Layer                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │  Server  │  │  Session │  │ Handler  │              │
│  │  (WS)    │  │  Manager │  │ Registry │              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
                           ↑ (concrete types)
┌─────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Provider │  │ FileBack.│  │ Push     │              │
│  │ Claude   │  │ Registry │  │ ExpoAPI  │              │
│  │ OpenCode │  │ JSON     │  │ WebSocket│              │
│  │ Kimi     │  │          │  │          │              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
```

### 8.2 进程视图 (Process View)

```
┌──────────┐     WebSocket      ┌──────────┐
│  App     │ ←────────────────→ │  Daemon  │
│ (RN/Expo)│     JSON frames    │  (Go)    │
└──────────┘                    └────┬─────┘
                                     │ subprocess stdin/stdout
                              ┌──────▼──────┐
                              │  Provider   │
                              │  (Claude/   │
                              │   Kimi/     │
                              │   OpenCode) │
                              └─────────────┘

并发模型：
- Daemon: goroutine per WS connection + goroutine per message processLoop
- Agent: goroutine per AgentSession.Run() + goroutine per EventPump
- App: React async state updates via Zustand subscriptions
```

### 8.3 开发视图 (Development View)

```
solo/
├── protocol/          ← leaf package (zero imports)
├── daemon/
│   ├── main.go        ← composition root (65 lines)
│   ├── internal/
│   │   ├── agent/     ← domain layer (provider interfaces + manager)
│   │   ├── server/    ← application layer (WS handlers)
│   │   ├── workspace/ ← domain models + infrastructure
│   │   ├── memory/    ← domain (interfaces) + infra (file backend)
│   │   └── schedule/  ← domain (store/executor) + runner
├── relay-go/          ← standalone relay server
├── cli/               ← CLI tool
├── app-bridge/        ← TypeScript bridge (client/relay/server/shared)
└── app/               ← React Native frontend
    └── src/
        ├── app/       ← Expo Router routes
        ├── components/← UI components
        ├── stores/    ← Zustand state
        ├── hooks/     ← React hooks
        └── contexts/  ← React contexts
```

### 8.4 物理视图 (Physical View)

```
┌─────────────────────────────────────┐
│        Developer Machine             │
│  ┌─────────┐    ┌──────────────┐   │
│  │  Daemon  │    │  Expo Dev    │   │
│  │  :17612  │←──→│  Server      │   │
│  │  (Go)    │ WS │  :19000      │   │
│  └─────────┘    └──────────────┘   │
│       │                              │
│  ┌────▼─────┐                       │
│  │ Provider │                       │
│  │ CLI tools│                       │
│  └──────────┘                       │
└─────────────────────────────────────┘
         │ (optional, via internet)
┌────────▼────────────────────────────┐
│        Relay Server                  │
│  ┌─────────┐    ┌──────────────┐   │
│  │  Nginx   │    │  Relay       │   │
│  │  :443    │───→│  :8081       │   │
│  │  (SSL)   │    │  (Go)        │   │
│  └─────────┘    └──────────────┘   │
└─────────────────────────────────────┘
```

### 8.5 场景视图 (Scenarios, +1)

**场景 1：用户发送 Agent 消息**
1. 用户在 composer.tsx 输入消息
2. session-context.tsx 调用 DaemonClient.sendMessage()
3. DaemonClient 通过 WebSocket 发送 JSON frame
4. daemon session.go 的 handlerRegistry 分发到 handleSendAgentMessage
5. agent/ManagedAgent.SendTurn() 将消息写入 provider subprocess stdin
6. Provider 输出流经 EventPump → InMemoryTimelineStore → Session.broadcastEvent()
7. 消息流回 app，session-stream-reducers.ts 处理，Zustand store 更新，React 重渲染

**场景 2：E2EE Relay 连接**
1. App 扫描 QR code 获取 ConnectionOfferV2 (base64url)
2. DaemonClient 使用 RelayE2EETransport 连接 Relay WebSocket
3. EncryptedChannel 执行 Curve25519 ECDH 握手
4. 握手完成后，所有消息经 XSalsa20-Poly1305 加密传输
5. Daemon 端 E2EEConn 解密后转发到本地 WebSocket server

**场景 3：新增 AI Provider**
1. 创建 `agent/provider_new.go`，实现 `AgentClient` + `AgentSession` 接口
2. 嵌入 `base.BaseSession` 获取共享状态管理
3. 在 `NewDaemon()` 中 `registry.Register(agent.NewNewClient())`
4. Manager 层零修改，所有现有 Agent 管理功能自动可用
5. （理想情况）在 app-bridge 中添加对应的 Zod schema — 但当前无自动同步机制
