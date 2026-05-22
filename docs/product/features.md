# Solo - 产品功能详细分析

> 分析日期：2026-05-20
> 仓库：/Users/wuerping/code/wuerping/solo
> 版本：v0.1.0

## 产品概述

**Solo** 是一个 AI 驱动的开发助手平台，采用全栈架构设计：

- **App** (React Native/Expo)：跨平台客户端（iOS/Android/Web）
- **Daemon** (Go)：核心服务端，管理 AI Agent、会话、工作区
- **CLI** (Go)：命令行工具，用于管理 daemon 和 agent
- **Relay** (Go)：WebSocket 中继服务，支持端到端加密 (E2EE)
- **Protocol** (Go)：通信协议定义

## 核心功能模块

### 1. AI Agent 系统

#### 1.1 Agent 生命周期管理
- **创建/删除/列出 Agent**：通过 CLI 或 App 管理 Agent
- **启动/停止/附加**：支持后台运行和交互式会话
- **状态管理**：initializing → idle ↔ running → error/closed
- **持久化**：`~/.solo/agents/` JSON 存储
- **会话恢复**：从 persistence handle 恢复 Agent 状态

#### 1.2 多 Provider 支持
当前内置 4 个 Provider（+ Mock 测试用）：
- **Claude**：通过 CLI `--print --output-format stream-json` 集成
- **Kimi**：Wire 模式 (`kimi --wire`)，JSON-RPC 2.0 stdio 通信，EventPump 事件泵，动态读取 `~/.kimi/config.toml` 模型列表（758 LOC，23 个单元测试）
- **OpenCode**：SSE `/global/event` 事件流，完整支持 reasoning/thinking
- **Codex**：仅注册表定义，无后端实现
- **Mock**：测试用 Provider

已移除 Provider：Copilot、PI Direct
缺失 Provider（Paseo 有 9 个）：Generic ACP、ACP Agent、Cursor-Agent（计划中）

#### 1.3 流式事件处理
- **Stream Coalescer**：200-500ms 动态窗口，减少 WS 消息量
- **Coalescer Flush**：关键事件强制刷新机制
- **Duplicate 检测**：字符长度跟踪 + content_block_delta 索引标记
- **Dispatcher 优先级**：Critical / SemiCritical / Normal 三级队列

#### 1.4 Agent 监控
- **Agent Watchdog**：35min 超时中断卡住任务
- **Startup Crash 检测**：100ms 启动崩溃检测
- **Zombie 进程预防**：WaitForExit + SIGTERM-first 关闭
- **Process Manager**：管理 Agent 进程生命周期

### 2. 会话系统 (Session)

#### 2.1 WebSocket 连接管理
- **Multi-socket 架构**：Session 与单一连接解耦，支持并发多端
- **Grace Period 重连**：断线续传 + grace buffer 关键消息重放
- **WebSocket Ping/Pong**：
  - 直接连接：5s 间隔 + 60s mobile timeout
  - Relay 连接：禁用 Layer 1（避免 E2EE 冲突），保留 Layer 2 JSON keepalive
- **消息队列**：无界 sendQueue 替代固定 channel，解决背压问题

#### 2.2 会话状态管理
- **Timeline Store**：内存存储 + 分页查询
- **Timeline Hydration**：从 provider 懒加载历史
- **Activity Tracker**：客户端活动追踪
- **Attention Policy**：Agent 状态变更时向所有 Session 广播

#### 2.3 消息处理
- **user_message 处理**：非 coalescable 事件直接存储和转发
- **关键事件保护**：sendQueue 满时保护关键生命周期事件不被丢弃
- **消息优先级**：Critical / SemiCritical / Normal

### 3. 工作区系统 (Workspace)

#### 3.1 工作区管理
- **Workspace 注册表**：文件持久化
- **Project 注册表**：项目根目录追踪
- **Git 工作流**：分支检测、dirty 状态
- **Workspace 设置**：setup 命令执行
- **Project Config 读写**：paseo.json 原子读写 + revision 冲突检测

#### 3.2 终端系统
- **PTY 终端**：creack/pty 实现
- **二进制帧协议**：opcode-based
- **多终端支持**：slot 路由
- **终端调整大小**：SIGWINCH
- **Terminal Output Coalescer**：减少 WS 消息量

#### 3.3 文件操作
- **File Explorer**：list/read/write/delete 完整实现
- **Script Proxy**：HTTP 反向代理
- **Script 运行时存储**：运行时信息持久化

### 4. Push 通知系统

- **Push Token 存储**：PersistedTokenStore JSON 原子文件存储
- **Expo Push API**：完整实现
- **通知策略**：180s threshold
- **客户端活动追踪**：活跃窗口检测、时间窗口
- **通知 Payload 构建**：Markdown stripping + 截断
- **Token 清理**：自动清理无效 token

### 5. Relay 中继系统

#### 5.1 Go Relay (relay-go)
- **WebSocket 中继**：控制通道 + 数据通道
- **E2EE 加密**：端到端加密通道实现
- **会话管理**：Session Manager 管理连接
- **Nudge 机制**：Cloudflare 半开连接应对
- **CORS 支持**：可配置跨域来源
- **Prometheus 指标**：sessions、connections、messages 监控

#### 5.2 连接适配
- **Relay 连接识别**：通过 E2EEConn 类型识别
- **Layer 1 Ping 禁用**：避免与 E2EE 状态机冲突
- **dataSocketOpenTimeout**：60s（防止长 thinking 阶段过早断开）

### 6. App 客户端功能

#### 6.1 核心界面
- **Dashboard**：Agent 列表和状态概览
- **Workspace Screen**：多标签工作区管理
  - 桌面标签行
  - Agent 可见性控制
  - 批量关闭
  - Draft Agent 配置
  - Git 操作集成
- **Agent Screen**：Agent 交互界面
  - 流式消息渲染
  - 附件管理
  - 代码插入
- **Projects Screen**：项目管理
- **Sessions Screen**：会话历史
- **Settings Screen**：设置管理

#### 6.2 连接管理
- **Host 管理**：添加/编辑/删除主机
- **连接方式**：直接连接、Relay 连接、QR 码配对
- **连接状态**：实时状态指示器

#### 6.3 输入系统
- **Composer**：多模态输入（文本、附件）
- **Command Center**：命令中心
- **附件管理**：附件预览、轻量查看

### 7. CLI 工具

#### 7.1 Daemon 管理
- `solo daemon start`：启动 daemon
- `solo daemon stop`：停止 daemon
- `solo daemon restart`：重启 daemon
- `solo daemon pair`：配对（生成链接和 QR 码）

#### 7.2 Agent 管理
- `solo agent run`：运行 Agent
- `solo agent ls`：列出 Agent
- `solo agent attach`：附加到 Agent
- `solo agent stop`：停止 Agent
- `solo agent delete`：删除 Agent
- `solo agent logs`：查看日志
- `solo agent wait`：等待 Agent 完成
- `solo agent send`：发送消息
- `solo agent mode`：切换模式

#### 7.3 Provider 管理
- `solo provider ls`：列出 Provider
- `solo provider models`：查看模型列表

### 8. 测试覆盖

#### 8.1 测试规模
- **Daemon 测试文件**：76 个（Go）
- **App 测试文件**：大量（TypeScript）
- **E2E 测试**：relay timeout、direct-tcp reconnect、user_message 回归

#### 8.2 关键测试域
- Agent：dispatcher、coalescer、reasoning/window、duplicate 检测
- Server：grace integration、reconnect、race 条件
- Relay：client、E2EE、control channel
- Terminal：output race、coalescer
- Workspace：registry、config、project

### 9. 基础设施

#### 9.1 构建系统
- **Makefile**：多目标构建（daemon、relay、app）
- **Go Workspace**：cli、daemon、protocol、relay-go
- **npm Workspace**：app、app-bridge、packages/highlight

#### 9.2 CI/CD
- **GitHub Actions**：go test、js lint
- **golangci-lint**：v2 配置，formatters 和 revive 规则
- **ESLint**：app-bridge 和 highlight 配置

#### 9.3 监控
- **Prometheus 指标**：sessions、connections、messages
- **日志系统**：slog（Go）、结构化日志

## 缺失功能（与 Paseo 对比）

### 高优先级缺失
1. **GitHub 集成**：开发者工作流核心（Paseo 有 1,911 行实现）
2. **Chat 系统**：多 Agent 协作场景
3. **Voice/Speech**：TTS/STT、Dictation、Voice Runtime
4. **更多 Provider**：Cursor-Agent、Generic ACP、ACP Agent

### 中优先级缺失
5. **Schedule/Cron**：定时任务调度
6. **Loop**：迭代工作流
7. **Tasks 系统**：执行顺序管理
8. **Workspace 归档**：归档管理

## 技术栈

### 后端
- **语言**：Go 1.25.6
- **WebSocket**：gorilla/websocket
- **PTY**：creack/pty
- **加密**：E2EE（X25519 + XSalsa20-Poly1305）
- **配置**：环境变量 + JSON 配置文件 + TOML (Kimi 模型读取)
- **日志**：slog

### 前端
- **框架**：React Native / Expo
- **语言**：TypeScript
- **状态管理**：React Context + Hooks
- **测试**：Jest / React Testing Library
- **样式**：Unistyles

### 部署
- **Relay**：Go 二进制 / Docker
- **App**：iOS / Android / Web
- **配置**：systemd / Cloudflare Workers

## 总结

Solo 是一个功能完整的 AI 开发助手平台，核心功能包括：

1. **AI Agent 管理**：多 Provider、流式事件、生命周期管理
2. **实时会话**：WebSocket、多 socket、断线重连
3. **工作区集成**：Git、终端、文件操作
4. **跨平台客户端**：iOS/Android/Web 统一体验
5. **安全中继**：E2EE 加密、CORS 保护
6. **生产就绪**：测试覆盖、监控、CI/CD

当前完成度约 **80-87%**，主要差距在 GitHub 集成、语音能力和更多 AI Provider 支持。
