# Solo

Solo 是一个 AI 编程助手平台，通过安全、端到端加密的架构将您的本地开发环境与 AI 服务提供商连接起来。它由本地守护进程、用于远程连接的转发服务器、跨平台移动/网页应用以及 CLI 工具组成。

---

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端层                              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   网页应用   │  │  移动应用   │  │    CLI      │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
└─────────┼────────────────┼────────────────┼────────────────┘
          └────────────────┴────────────────┘
                         │
                ┌────────▼────────┐
                │   App-Bridge    │
                └────────┬────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│                     网络层                                   │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Nginx (可选)                            │   │
│  └────────────────────────┬────────────────────────────┘   │
│                           │                                  │
│  ┌────────────────────────▼────────────────────────────┐   │
│  │            转发服务器 (信令转发)                      │   │
│  └────────────────────────┬────────────────────────────┘   │
└───────────────────────────┼──────────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────────┐
│                      服务层                                  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐   │
│  │              守护进程 (核心服务)                      │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 核心组件

| 组件 | 目录 | 语言 | 职责 |
|-----------|-----------|----------|----------------|
| **应用** | [`app/`](app/) | TypeScript / React Native | 用户界面 (iOS, Android, Web) |
| **应用桥接** | [`app-bridge/`](app-bridge/) | TypeScript | 客户端通信库 |
| **守护进程** | [`daemon/`](daemon/) | Go | 核心服务 — 管理会话、代理和提供商连接 |
| **转发** | [`relay-go/`](relay-go/) | Go | 用于远程/移动访问的连接转发 |
| **CLI** | [`cli/`](cli/) | Go | 用于会话和代理管理的命令行工具 |
| **协议** | [`protocol/`](protocol/) | Go | 共享协议定义 |

---

## 技术栈

| 层级 | 技术 |
|-------|------------|
| 后端 | Go 1.25 · gorilla/websocket · creack/pty · slog |
| 前端 | Expo 54 · React Native 0.81 · React 19 · TypeScript |
| 状态管理 | Zustand · @tanstack/react-query · React Context |
| 加密 | X25519 密钥交换 + XSalsa20-Poly1305 (E2EE) |
| 测试 | Vitest · Playwright (E2E) · Go test |
| 部署 | Systemd · Docker · Nginx + Let's Encrypt |
| CI/CD | GitHub Actions · golangci-lint v2 · ESLint |

---

## 快速开始

### 前置要求

- [Go](https://go.dev/) 1.25+
- [Node.js](https://nodejs.org/) 20+
- [Expo CLI](https://docs.expo.dev/get-started/installation/) (用于移动/网页开发)

### 构建

```bash
# 构建所有 Darwin 二进制文件 (守护进程, 转发, CLI)
make darwin

# 构建所有 Linux 二进制文件
make linux

# 构建所有内容
make all
```

输出二进制文件放置在 `output/darwin/` 和 `output/linux/` 中。

### 开发

```bash
# 同时启动守护进程 + 网页应用
make dev

# 仅启动网页应用
make dev-web

# 仅启动守护进程 (必须先构建)
make dev-daemon

# 重启守护进程
make restart

# 停止所有开发进程
make stop
```

- **守护进程** 监听 `127.0.0.1:17612`
- **网页应用** 运行在 `http://localhost:19000`

### 运行测试

```bash
# Go 测试 (所有模块)
cd protocol && go test -short -race ./...
cd cli && go test -short -race ./...
cd daemon && go test -short -race ./...
cd relay-go && go test -short -race ./...

# JavaScript / TypeScript 测试
npm run lint
npm run test --workspaces --if-present
```

---

## 项目结构

```
solo/
├── app/                 # React Native / Expo 应用
├── app-bridge/          # 客户端通信库
├── cli/                 # Go CLI 工具
├── daemon/              # Go 核心服务
├── docs/                # 架构与产品文档
├── packages/highlight/  # 共享语法高亮包
├── protocol/            # Go 协议定义
├── relay-go/            # Go 转发服务器
├── Makefile             # 构建与开发命令
├── go.work              # Go 工作区
└── package.json         # Node.js 工作区根目录
```

详细文档请参见 [`docs/README.md`](docs/README.md)。

---

## 支持的 AI 提供商

- **Claude** — print / stream-json 模式
- **Kimi** — Wire 模式 (JSON-RPC 2.0 over stdio)
- **OpenCode** — SSE 模式
- **Codex** — 仅定义
- **Mock** — 用于测试

有关提供商集成研究和计划添加的内容，请参见 [`docs/providers/`](docs/providers/)。

---

## 会话记忆

守护进程将每个会话的每次用户/助手对话持久化到磁盘，格式为带 YAML 前置信息的 Markdown，为您提供 Solo 所做一切的本地、可查询的转录记录。

- **存储**: `~/.solo/memory/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md`
- **索引**: `~/.solo/memory/sessions.jsonl` (每个会话一行 JSONL)
- **流式处理**: 助手流式响应被合并为每个逻辑回合的**单个** `assistant.md` — 您不会因为一个答案而得到几十个文件。
- **脱敏**: OpenAI / GitHub / Anthropic / AWS 令牌和常见的 env-file 密钥在写入前会被替换为 `[redacted:<reason>]`。
- **隔离**: 记录器在 `SafeBridge` 包装器后运行，该包装器可以从 panic 中恢复并在重复失败时触发断路器，因此存储问题永远不会导致守护进程的主会话循环崩溃。

### 配置

会话记忆**默认开启**。要选择退出，请添加到 `~/.solo/config.json`：

```json
{
  "memory": { "enabled": false }
}
```

其他配置项 (`backend`, `root`, `retention_days`, `queue_size`, `overflow`, `redact.*`, `safe.failure_threshold`, `safe.failure_cooldown`) 记录在 [`docs/product/session-memory-spec.md`](docs/product/session-memory-spec.md) 中。

---

## Tmux 仪表板

应用内置 Tmux 仪表板，可自动检测在 tmux 会话中运行的 AI 代理，并提供交互式控制。

### 代理检测

三层检测机制确保即使 `pane_current_command` 报告不同的进程名（例如 pi 报告为 `node`）也能识别代理：

1. **命令名** — 匹配 `claude`、`pi`、`kimi`、`kimi-cli`、`opencode`、`qoder`、`cursor`
2. **窗格标题** — Unicode 归一化（如 `π` → `pi`）配合词边界匹配
3. **子进程检查** — 通过 `pgrep`/`ps` 回退检测包装启动器

### 功能

- **代理卡片** — 按代理名称分组，点击可筛选
- **窗格内容捕获** — 实时终端视图（最近 500 行），每 5 秒自动刷新
- **交互式控制** — 发送文本命令（带 Enter），或使用快捷操作按钮：
  - 方向键（↑↓←→）用于 TUI 菜单导航
  - Enter、Esc、Tab、Ctrl+C 用于控制
  - 数字键（1–4）用于 TUI 菜单选择

### 支持的代理

| 代理 | 检测方式 |
|------|---------|
| claude | 命令 / 标题 |
| pi | 命令 / 标题（π unicode）/ 子进程 |
| kimi | 命令 / 标题 |
| kimi-cli | 命令 / 标题 |
| opencode | 命令 / 标题 |
| qoder | 命令 |
| cursor | 命令 |

---

## 安全

- **端到端加密**: 客户端和守护进程之间的所有通信都使用 X25519 密钥交换 + XSalsa20-Poly1305 进行加密。
- **配对链接**: 通过 `https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}` 进行安全配对。

---

## CI/CD

该项目使用 GitHub Actions (`.github/workflows/ci.yml`)，包含以下任务：

| 任务 | 步骤 |
|-----|-------|
| **Go** (矩阵: protocol, cli, daemon, relay-go) | `go mod verify` → `go build` → `go test -short -race` → `golangci-lint` |
| **JS** | `npm ci` → lint → typecheck → test |

---

## 文档

- [架构概览](docs/architecture/README.md)
- [组件规范](docs/architecture/components.md)
- [数据流与会话生命周期](docs/architecture/data-flow.md)
- [网络架构](docs/architecture/network-architecture.md)
- [部署指南](docs/architecture/deployment.md)
- [产品功能](docs/product/features.md)
- [会话记忆规范](docs/product/session-memory-spec.md)

---

## 许可证

[在此处添加您的许可证]

---

📄 [English Version](README.md)
