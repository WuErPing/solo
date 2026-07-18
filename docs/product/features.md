# Solo - Product Feature Detailed Analysis

> Analysis Date: 2026-06-15
> Repository: /Users/wuerping/code/wuerping/solo
> Version: v0.6.3

## Product Overview

**Solo** is an AI-driven development assistant platform with a full-stack architecture:

- **App** (React Native/Expo): Cross-platform client (iOS/Android/Web)
- **Daemon** (Go): Core server, manages AI Agents, sessions, workspaces
- **CLI** (Go): Command-line tool for managing daemon and agents
- **Relay** (Go): WebSocket relay service with end-to-end encryption (E2EE)
- **Protocol** (Go): Communication protocol definitions

## Core Feature Modules

### 1. AI Agent 系统

#### 1.1 Agent Lifecycle Management
- **Create/Delete/List Agents**: Manage agents via CLI or App
- **Start/Stop/Attach**: Support background execution and interactive sessions
- **State Management**: initializing → idle ↔ running → error/closed
- **Persistence**: `~/.solo/agents/` JSON storage
- **Session Recovery**: Restore agent state from persistence handle

#### 1.2 Multi-Provider Support
Currently built-in 4 providers:
- **Claude**: Integrated via CLI `--print --output-format stream-json`
- **Kimi**: Wire mode (`kimi --wire`), JSON-RPC 2.0 stdio communication, EventPump, dynamically reads `~/.kimi/config.toml` model list (~737 LOC, 31 executed test cases)
- **OpenCode**: SSE `/global/event` event stream, full reasoning/thinking support
- **Pi**: Minimal terminal coding harness

**Development-only**: **Mock** provider (opt-in via `SOLO_ENABLE_MOCK_PROVIDER=1`)

Defined but no backend implementation: **Codex**

Removed providers: Copilot
Planned providers: Cursor-Agent (Print mode), Generic ACP, ACP Agent

#### 1.3 Streaming Event Processing
- **Stream Coalescer**: 200-500ms dynamic window, reduces WS message volume
- **Coalescer Flush**: Critical event forced flush mechanism
- **Duplicate Detection**: Character length tracking + content_block_delta index marking
- **Dispatcher Priority**: Critical / SemiCritical / Normal three-level queue
- **MessageID Propagation**: All providers (Claude, Kimi, OpenCode, Pi, Mock) carry unique `MessageID` when generating `user_message`, used for backend timeline idempotent deduplication
- **Timeline Deduplication**: `InMemoryTimelineStore.Append()` precisely compares the last record by type (`MessageID` → `Text` → `CallID+Status`), preventing duplicate writes during multi-device sync

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
- **Multi-client Sync**：多个 Session 共用全局 timeline store，后端幂等写入 + 客户端 seq gate 确保一致性

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

### 5. 调度系统 (Schedule)

#### 5.1 核心功能
- **Cron 调度**：标准 cron 表达式支持
- **固定间隔调度**：毫秒级间隔配置
- **时区感知**：用户本地时区输入，UTC 存储，本地时间显示
- **Agent 目标**：支持现有 Agent 或新建 Agent 执行
- **执行历史**：完整运行记录追踪

#### 5.2 前端实现
- **创建模态框**：频率预设、时间输入、时区显示
- **编辑模态框**：完整配置编辑能力
- **详情屏幕**：友好调度文本（如"每天 00:25"）和 UTC 表达式显示
- **列表屏幕**：调度任务管理和状态监控
- **时间显示**：本地时区 24 小时格式（zh-CN locale）

#### 5.3 后端实现
- **协议层**：ScheduleCadence 包含 timezone 字段
- **Cron 解析**：UTC 表达式直接评估，避免双重转换
- **自愈机制**：fixupNextRunAt 修复存储的过期值
- **持久化存储**：JSON 文件存储调度配置

#### 5.4 工具函数
- **detectTimezone**：检测用户 IANA 时区
- **cronToUTC**：本地时区 cron 转 UTC
- **cronFromUTC**：UTC cron 转本地时区
- **describeCron**：生成友好调度描述文本

#### 5.5 日程助手 (Schedule Assistant)
- **自然语言创建/编辑**：对话面板中用一句话创建、编辑、暂停、恢复、删除日程（"Ask AI" 入口位于日程列表/仪表盘，"Edit with AI" 位于日程详情页）
- **确认卡片 (Confirm Cards)**：LLM 只产出提案（proposal），所有变更经结构化卡片确认后才走现有 `schedule/create|update|pause|resume|delete` RPC；update 卡片显示字段级 diff
- **主机默认 LLM 提供商**：解析使用主机 Settings → General → LLM Providers 中的默认提供商（第一个启用的提供商 + 其 `isDefault` 模型）；设置表单现可编辑 models 列表（逗号分隔 ID）
- **澄清循环 (Clarify Loop)**：名称歧义或信息不足时返回澄清问题与候选列表，绝不猜测
- **提供商指示器**：面板头部显示当前解析的 provider + model（来自 `schedule/assist` 响应回显）
- **无提供商引导**：未配置 LLM 提供商时显示设置卡片，深度链接至 `/settings/general`
- **安全不变式**：daemon 解析路径不触碰日程存储；执行路径（Target Agent）完全不变
- **E2E 覆盖**：Playwright + stub LLM endpoint 4 个用例（无提供商引导卡、创建+确认含 UTC 转换、更新 diff、歧义澄清）

### 6. Relay 中继系统

#### 5.1 Go Relay (relay-go)
- **WebSocket 中继**：控制通道 + 数据通道
- **E2EE 加密**：端到端加密通道实现
- **会话管理**：Session Manager 管理连接
- **Nudge 机制**：Cloudflare 半开连接应对
- **CORS 支持**：可配置跨域来源
- **Prometheus 指标**：sessions、connections、messages 监控

#### 6.2 连接适配
- **Relay 连接识别**：通过 E2EEConn 类型识别
- **Layer 1 Ping 禁用**：避免与 E2EE 状态机冲突
- **dataSocketOpenTimeout**：60s（防止长 thinking 阶段过早断开）

### 7. App 客户端功能

#### 7.1 核心界面
- **Dashboard**：Agent 列表和状态概览
- **Tmux Dashboard**：自动检测 tmux 会话中的 AI 代理，提供交互式控制
  - 三层代理检测（命令名 / 窗格标题 Unicode 归一化 / 子进程检查）
  - 代理卡片按名称分组，显示会话徽章（会话名、窗口、窗格），支持筛选
  - 新建会话 — 直接从仪表板创建新的 tmux 会话，支持可选的工作目录和命令
  - 非代理窗格显示 — 浏览和交互非代理 tmux 窗格（shell、编辑器等），按命令分组
  - 命令历史 — 跟踪和显示发送给编程代理的最近命令，支持删除过期条目
  - 会话管理 — 关闭（kill）tmux 会话，代理/窗格卡片带确认对话框
  - 窗格内容捕获（默认 200 行，5 秒自动刷新，可关闭自动刷新）
  - 懒加载历史（滚动驱动，每次 200 行，最大 5000 行）
  - 自定义终端主题（`system` / `dark` / `light` / `bash` / `auto`）
  - ANSI 文本渲染（状态栏支持 ANSI 颜色）
  - 窗口列表显示（状态栏显示 tmux 窗口信息，如 `0:claude*`）
  - 256 色调色板检测（`detect-ansi-colors.ts`）
  - 全屏窗格页面（替代底部弹窗）
  - 斜杠命令过滤（`agent-commands.ts`）：输入 `/` 时显示代理专属快捷命令
  - ErrorBoundary 崩溃保护
  - 快捷操作按钮：方向键（↑↓←→）、Enter、Esc、Tab、Ctrl+C、数字键（1–4）
  - 支持代理：claude、pi、kimi、kimi-cli、opencode、qoder、cursor
- **Schedule Dashboard**：Cron 调度任务管理（创建、编辑、列表、详情）
  - 频率预设、时区感知输入、UTC 存储 / 本地显示
  - 支持选择现有 Agent 或新建 Agent 执行
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
  - 终端主题选择器（`system` / `dark` / `light` / `tmux`）
  - Operations 设置区（`operations-section.tsx`）
  - Tmux Agents 设置区（`tmux-agents-section.tsx`）
  - Providers / Keyboard Shortcuts 设置区
- **Mermaid Preview**：Markdown 文件面板内嵌 Mermaid 图表实时渲染
- **SVG Preview**：SVG 文件预览，支持 Web 和移动端（WebView 渲染）
- **ANSI Text Renderer**：ANSI 转义序列渲染组件，用于终端内容和状态栏
- **Error Boundary**：React 错误边界，包裹 TmuxDashboard 和 TmuxPaneScreen

#### 7.2 连接管理
- **Host 管理**：添加/编辑/删除主机
- **连接方式**：直接连接、Relay 连接、QR 码配对
- **连接状态**：实时状态指示器

#### 7.3 输入系统
- **Composer**：多模态输入（文本、附件）
- **Command Center**：命令中心
- **附件管理**：附件预览、轻量查看

### 8. CLI 工具

#### 8.1 Daemon 管理
- `solo daemon start`：启动 daemon
- `solo daemon stop`：停止 daemon
- `solo daemon restart`：重启 daemon
- `solo daemon pair`：配对（生成链接和 QR 码）

#### 8.2 Agent 管理
- `solo agent run`：运行 Agent
- `solo agent ls`：列出 Agent
- `solo agent attach`：附加到 Agent
- `solo agent stop`：停止 Agent
- `solo agent delete`：删除 Agent
- `solo agent logs`：查看日志
- `solo agent wait`：等待 Agent 完成
- `solo agent send`：发送消息
- `solo agent mode`：切换模式

#### 8.3 Provider 管理
- `solo provider ls`：列出 Provider
- `solo provider models`：查看模型列表

### 9. 测试覆盖

> 详细覆盖率数据、模块级分析、根因和路线图见: [`docs/analysis/test-coverage.md`](../analysis/test-coverage.md)

#### 9.1 测试规模
- **App 单元测试**：**235** 个测试文件，**1,663** 个测试用例（Vitest），已接入 CI（含 tmux dashboard、pane screen、status line、ANSI renderer、SVG preview、loop CRUD 等新增测试）
- **App browser 测试**：1 个文件（Chromium via Playwright），未接入 CI
- **App-bridge 测试**：3 个文件，**32 个测试用例**（Vitest），已接入 CI
- **Daemon 测试文件**：**129** 个（Go），已接入 CI
- **Relay-go 测试文件**：**8** 个（Go），已接入 CI
- **Protocol 测试文件**：**4** 个（Go），已接入 CI
- **CLI 测试文件**：**13** 个（Go），已接入 CI
- **E2E 测试**：**35** 个 `.spec.ts`（Playwright），**nightly 运行**（含 loop-crud、tmux-close-session、SVG preview 等 E2E）
- **Maestro 移动端**：~20 个 YAML flow，ad-hoc / 未接入 CI

#### 9.2 关键测试域
- Agent：dispatcher、coalescer、reasoning/window、duplicate 检测
- Server：grace integration、reconnect、race 条件
- Relay：client、E2EE、control channel
- Terminal：output race、coalescer
- Workspace：registry、config、project

### 10. 基础设施

#### 10.1 构建系统
- **Makefile**：多目标构建（daemon、relay、app）
- **Go Workspace**：cli、daemon、protocol、relay-go
- **npm Workspace**：app、app-bridge、packages/highlight

#### 10.2 CI/CD
- **GitHub Actions `ci.yml`**：
  - `go` job（matrix: protocol/cli/daemon/relay-go）：build + `go test -short -race -coverprofile` + golangci-lint v2.10 + Codecov upload
  - `js` job：lint（app/app-bridge/highlight）+ typecheck（强制，0 errors）+ test（highlight/app/app-bridge）+ Codecov upload
- **GitHub Actions `e2e-nightly.yml`**：每天 02:00 UTC 自动运行 Playwright E2E，失败保留 trace/screenshot/video 7 天
- **Codecov**：`codecov.yml` 配置 flags（js / go-*）+ informational mode，需 `CODECOV_TOKEN` Secret
- **golangci-lint**：v2.10，`.golangci.yml` 配置 formatters 和 revive 规则
- **ESLint**：app（expo lint）、app-bridge、highlight 分别配置

#### 10.3 监控
- **Prometheus 指标**：sessions、connections、messages
- **日志系统**：slog（Go）、结构化日志

### 11. App 导航结构

App 使用 **Expo Router** 文件系统路由：

```
app/
├── _layout.tsx              # Root layout (Provider, sidebar, command center)
├── index.tsx                # Entry point (startup bootstrapping)
├── dashboard.tsx            # Dashboard
├── tmux-dashboard.tsx       # Tmux Dashboard
├── tmux-pane.tsx            # Tmux Pane (全屏)
├── schedules.tsx            # Schedule Dashboard 入口
├── pair-scan.tsx            # QR 码配对
├── settings/
│   ├── index.tsx            # 设置首页
│   ├── [section].tsx        # 设置分类页
│   ├── hosts/[serverId].tsx # Host 详情
│   └── projects/            # 项目管理
└── h/[serverId]/
    ├── workspace/[workspaceId]/  # Workspace 主页
    ├── agent/[agentId].tsx       # Agent 详情
    ├── sessions.tsx              # 会话列表
    └── schedules.tsx             # 主机调度任务列表
```

### 12. App 核心组件目录

#### Agent 组件
| 组件 | 文件 | 功能 |
|------|------|------|
| Agent List | `agent-list.tsx` | Agent 列表 |
| Agent Status Bar | `agent-status-bar.tsx` | 底部状态栏 |
| Agent Stream View | `agent-stream-view.tsx` | 实时流式输出 |
| Model Selector | `combined-model-selector.tsx` | AI 模型选择 |

#### 输入组件
| 组件 | 文件 | 功能 |
|------|------|------|
| Composer | `composer.tsx` | 多模态输入框 |
| Attachment Pill | `attachment-pill.tsx` | 附件标签 |
| Attachment Lightbox | `attachment-lightbox.tsx` | 附件预览 |

#### 导航与布局
| 组件 | 文件 | 功能 |
|------|------|------|
| Left Sidebar | `left-sidebar.tsx` | 主导航侧边栏 |
| Command Center | `command-center.tsx` | 命令面板 (Cmd+K) |

#### 预览与终端
| 组件 | 文件 | 功能 |
|------|------|------|
| Mermaid Preview | `mermaid-preview.tsx` | Mermaid 图表渲染 |
| SVG Preview | `svg-preview.tsx` | SVG 文件预览 |
| ANSI Text Renderer | `ansi-text-renderer.tsx` / `ansi-text-line.tsx` | ANSI 转义序列渲染 |
| Terminal Themes | `styles/terminal-themes.ts` | 5 个终端主题预设（`system` / `dark` / `light` / `bash` / `auto`） |
| Schedule Create/Edit Modal | `schedule-create-modal.tsx` / `schedule-edit-modal.tsx` | 调度任务创建/编辑 |
| Error Boundary | `error-boundary.tsx` | React 错误边界 |

#### UI 基础组件
Button, Dropdown Menu, Combobox, Tooltip, Shortcut, Segmented Control, Context Menu, Isolated Bottom Sheet

### 13. App Hooks / Context / Stores

#### Contexts
| Context | 功能 |
|---------|------|
| SessionContext | 会话状态管理 |
| ToastContext | Toast 通知 |
| VoiceContext | 语音输入状态 |

#### Hooks
| Hook | 功能 |
|------|------|
| useAggregatedAgents | 聚合所有 Agent 状态 |
| useAggregatedTmuxAgents | 聚合 tmux agent 发现 |
| useTmuxCapturePane | tmux pane 内容轮询 |
| useTmuxNewSession | 创建新 tmux 会话 |
| useTmuxTheme | 终端主题颜色 |
| useTmuxStatusLine | tmux 状态栏解析 |
| useTmuxStatusLines | 聚合多主机状态栏 |
| useSchedule* | 调度任务查询/创建/编辑 hooks |
| useSettings | App 设置 |

#### Stores (Zustand)
| Store | 功能 |
|-------|------|
| SessionStore | 会话状态 |
| PanelStore | 面板状态 |
| WorkspaceTabsStore | Workspace 标签状态 |
| TmuxAgentStore | 选中的 tmux agent |
| ScheduleStore | 调度任务状态 |

## 缺失功能（与 Paseo 对比）

### 已实现（此前标记为缺失）
1. **GitHub 集成**：PR 状态查看、Git diff、分支切换、Workspace git actions
2. **MCP 服务器**：Daemon 端完整实现，App 端设置页面有 "Automatically inject Solo MCP tools" 开关
3. **Tmux Dashboard**：完整的 tmux 代理检测、窗格内容捕获、终端主题、ANSI 渲染、新建会话、命令历史（支持删除）、非代理窗格显示、会话关闭（kill-session）
4. **SVG Preview**：Web 和移动端 SVG 文件预览
5. **Loop**：迭代工作流 CRUD（创建、查看、更新、运行、停止、删除），App UI + CLI 全覆盖

### 高优先级缺失
1. **Chat 系统**：多 Agent 协作场景
2. **Voice/Speech**：TTS/STT、Dictation、Voice Runtime（**已显式移除**）
3. **更多 Provider**：Cursor-Agent、Generic ACP、ACP Agent

### 中优先级缺失
4. **Tasks 系统**：执行顺序管理
5. **Workspace 归档**：归档管理

## 技术栈

### 后端
- **语言**：Go 1.25
- **WebSocket**：gorilla/websocket
- **PTY**：creack/pty
- **加密**：E2EE（X25519 + XSalsa20-Poly1305）
- **配置**：环境变量 + JSON 配置文件 + TOML (Kimi 模型读取)
- **日志**：slog

### 前端
- **框架**：React Native / Expo
- **语言**：TypeScript
- **状态管理**：React Context + Hooks
- **测试**：Vitest
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

当前完成度约 **85-90%**，主要差距在 Chat 系统（多 Agent 协作）、Cursor-Agent / ACP Provider 支持。
