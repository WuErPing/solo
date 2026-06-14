# Solo Provider Hub：cc-switch 功能迁移 + 多 Agent 配置转写方案

> **文档类型**：产品/架构设计方案
> **日期**：2026-06-13
> **基线版本**：Solo v0.6.0
> **目标读者**：产品、后端、前端、CLI 开发者
> **参考项目**：[farion1231/cc-switch](https://github.com/farion1231/cc-switch)

---

## 1. 背景

### 1.1 cc-switch 是什么

[farion1231/cc-switch](https://github.com/farion1231/cc-switch) 是一款跨平台桌面应用（Tauri + Rust），定位为 **Claude Code / Codex / Gemini CLI / OpenCode / OpenClaw 的 All-in-One 配置管理器**。核心功能包括：

| 功能 | 说明 |
|------|------|
| **Provider 管理** | 统一管理各 coding agent 的 API Provider，支持一键切换 |
| **50+ Provider 预设** | OpenRouter、DeepSeek、Kimi、智谱、AWS Bedrock、Azure 等 |
| **本地代理模式** | 内置本地代理，支持 failover、断路器、透明 header 转发 |
| **MCP 统一管理** | 一个面板管理所有工具的 MCP Server，新增/修改可同步到多个工具 |
| **Skills 管理** | Claude Skills 的发现、安装、批量更新、SHA-256 变更检测 |
| **Prompts 管理** | 多预设 System Prompt |
| **用量监控** | 各 Provider 的请求数、token、花费追踪 |
| **会话历史** | 浏览和恢复跨工具的会话 |
| **系统托盘** | 托盘菜单快速切换 Provider |
| **云同步** | Dropbox / OneDrive / iCloud / WebDAV |

### 1.2 为什么要迁移到 Solo

Solo 与 cc-switch 有天然互补性：

| cc-switch 强项 | Solo 现状 | 结合价值 |
|---|---|---|
| 多 Agent Provider 配置管理 | 已有多 Provider 抽象（Claude/Kimi/OpenCode/Pi） | 把 Provider 管理从 Agent 内部提升到平台层 |
| 本地代理 / 路由 | 已有 E2EE Relay | 可在本地 daemon 内建 Provider Hub，移动端也能用 |
| MCP / Skills / Prompts 统一管理 | MCP 已有 daemon 实现，App 有注入开关 | 做成跨 Agent 的通用工具和规则中枢 |
| 用量监控 | Prometheus 指标较基础 | 补充细粒度 token/成本追踪 |
| 跨平台桌面 GUI | 已有 React Native / Expo App | 用 App 替代 Tauri，覆盖 iOS/Android/Web |

> **迁移目标**：在 Solo 中构建 **Provider Hub**，让它既能服务 Solo 内部 Agent，也能为外部 coding agent（Claude Code、Codex、Cursor、OpenCode 等）提供统一配置、代理和规则转写。

---

## 2. 核心设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Solo App / CLI                               │
│  Provider Hub UI · MCP Manager · Skills Market · Usage Dashboard     │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ WebSocket / CLI
┌──────────────────────────────▼──────────────────────────────────────┐
│                         Solo Daemon                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │ Provider Hub │  │ Local API    │  │ Config       │  │ Usage    │ │
│  │ (registry +  │  │ Proxy /      │  │ Exporter     │  │ Tracker  │ │
│  │  router)     │  │ Router       │  │ (转写器)      │  │          │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └────┬─────┘ │
│         │                 │                  │               │       │
│         └─────────────────┴──────────────────┘               │       │
│                           │                                  │       │
│         ┌─────────────────┼──────────────────┐               │       │
│         ▼                 ▼                  ▼               ▼       │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐  ┌───────────┐ │
│  │ Solo Agents │   │ External    │   │ Project     │  │ Cost      │ │
│  │ (Claude/    │   │ Coding      │   │ Config      │  │ Reports   │ │
│  │  Kimi/etc)  │   │ Agents      │   │ Files       │  │           │ │
│  └─────────────┘   └─────────────┘   └─────────────┘  └───────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 设计原则

1. **配置即代码**：Provider、MCP、Skills、Prompts 都作为可版本化的配置项。
2. **一份配置，多处生效**：Solo 内部 Agent 和外部 coding agent 共享同一套 Provider/MCP/规则配置。
3. **可插拔 target**：新增一种外部 agent 只需新增一个 Exporter。
4. **本地优先**：所有敏感配置（API Key、MCP 命令）默认只存在本地 `~/.solo/`。
5. **移动端可用**：App 可远程管理 Provider Hub，甚至通过 Relay 让外部 agent 走 Solo 代理。

---

## 3. Provider Hub 数据模型

### 3.1 Provider 配置

存储位置：`~/.solo/provider-hub/providers/{provider-id}.json`

```json
{
  "id": "siliconflow-claude",
  "type": "third-party",
  "targetAgents": ["claude", "codex", "cursor", "opencode"],
  "api": {
    "baseURL": "https://api.siliconflow.cn/v1",
    "authType": "bearer",
    "apiKeyRef": "env:SILICONFLOW_API_KEY",
    "protocol": "anthropic-messages",
    "fullURLEndpoint": false
  },
  "models": [
    { "id": "claude-sonnet-4-6", "label": "Claude Sonnet 4.6", "isDefault": true },
    { "id": "claude-opus-4-7", "label": "Claude Opus 4.7" }
  ],
  "routing": {
    "priority": 1,
    "enabled": true,
    "failoverTo": ["official-claude"]
  },
  "cost": {
    "inputPer1M": 3.0,
    "outputPer1M": 15.0,
    "currency": "USD"
  },
  "metadata": {
    "label": "SiliconFlow Claude",
    "icon": "siliconflow",
    "region": "CN"
  }
}
```

### 3.2 Agent Target 配置

存储位置：`~/.solo/provider-hub/agents/{agent}.json`

```json
{
  "agent": "claude",
  "activeProvider": "siliconflow-claude",
  "providerPreference": ["siliconflow-claude", "official-claude"],
  "mcpServers": ["postgres", "github"],
  "skills": ["go-backend", "testing"],
  "prompts": ["senior-backend"],
  "commonConfig": {
    "allowedTools": ["Read", "Write", "Bash", "Test"],
    "approvalPolicy": "ask-for-dangerous"
  },
  "exportTargets": {
    "cursor": { "enabled": true, "outPath": ".cursorrules" },
    "codex": { "enabled": true, "outPath": ".codex/AGENTS.md" }
  }
}
```

### 3.3 MCP / Skills / Prompts 配置

```json
// ~/.solo/provider-hub/mcp/postgres.json
{
  "id": "postgres",
  "name": "PostgreSQL MCP",
  "runtime": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-postgres"],
  "env": { "DATABASE_URL": "${env:DATABASE_URL}" },
  "targetAgents": ["claude", "codex", "cursor"]
}

// ~/.solo/provider-hub/skills/go-backend.json
{
  "id": "go-backend",
  "name": "Go Backend Expert",
  "source": "github:org/go-backend-skill",
  "sha256": "abc123...",
  "targetAgents": ["claude", "codex"]
}

// ~/.solo/provider-hub/prompts/senior-backend.json
{
  "id": "senior-backend",
  "name": "Senior Backend Engineer",
  "text": "You are a senior backend engineer...",
  "targetAgents": ["claude", "codex", "cursor", "opencode"]
}
```

---

## 4. 关键模块设计

### 4.1 Provider Registry & Router

新增 `daemon/internal/providerhub/`：

```go
package providerhub

// Provider 是统一的上游模型供应商定义。
type Provider struct {
    ID          string
    Type        string // official, third-party, oauth
    TargetAgents []string
    API         APIConfig
    Models      []ModelConfig
    Routing     RoutingConfig
    Cost        CostConfig
    Metadata    ProviderMetadata
}

type APIConfig struct {
    BaseURL         string
    AuthType        string // bearer, api-key-header, oauth
    APIKeyRef       string // env:KEY_NAME or file:path
    Protocol        string // anthropic-messages, openai-chat, google-generative, opencode
    FullURLEndpoint bool
    CustomHeaders   map[string]string
}

type Router struct {
    registry *Registry
    fallback map[string][]string // agent -> ordered provider IDs
}

// Resolve 根据 agent 和偏好返回实际 provider + model。
func (r *Router) Resolve(agent string, intent string) (*ResolvedProvider, error)
```

**与现有架构集成**：
- `ProviderRegistry` 继续负责 AgentClient 注册（Claude/Kimi/OpenCode/Pi）。
- 新增 `providerhub.Registry` 管理上游 API Provider。
- Agent 启动时，先通过 `Router.Resolve()` 得到 provider，再交给对应 AgentClient 执行。

### 4.2 Local API Proxy / Router

这是 cc-switch 的核心能力之一：在本地暴露一个统一代理端点，所有外部 coding agent 指向它，Solo 负责协议转换和路由。

```
Claude Code ──▶ ANTHROPIC_BASE_URL=http://127.0.0.1:17613 ──▶ Solo Proxy ──▶ SiliconFlow / Official / OpenRouter
Codex ────────▶ OPENAI_BASE_URL=http://127.0.0.1:17613/v1 ──▶ Solo Proxy ──▶ ...
Cursor ───────▶ API Base URL=http://127.0.0.1:17613/v1 ────▶ Solo Proxy ──▶ ...
```

**实现要点**：
- 监听独立端口（如 `:17613`），避免与 daemon WS 端口 `:17612` 冲突。
- 根据请求 header / path / model name 识别目标 agent 和 protocol。
- 协议转换：`anthropic-messages` ↔ `openai-chat` 等。
- 透明 header 转发，支持自定义 header。
- failover：一个 provider 失败时自动切换到下一个。
- 请求/响应日志用于用量统计。

### 4.3 Config Exporter（配置转写）

一个配置（Provider / MCP / Skill / Prompt）转写到不同 coding agent 各自的配置文件。

**Exporter 接口**：

```go
type Exporter interface {
    Name() string
    Supports(agent string) bool
    ExportProvider(provider *Provider, target *AgentTarget) ([]ExportedFile, error)
    ExportMCP(mcp *MCPConfig, target *AgentTarget) ([]ExportedFile, error)
    ExportSkill(skill *SkillConfig, target *AgentTarget) ([]ExportedFile, error)
    ExportPrompt(prompt *PromptConfig, target *AgentTarget) ([]ExportedFile, error)
}

type ExportedFile struct {
    Path    string
    Content string
}
```

**支持的外部 agent 和输出格式**：

| Agent | Provider 配置 | MCP 配置 | Skill / Prompt |
|-------|--------------|----------|----------------|
| **Claude Code** | `~/.claude/settings.json` + env | `~/.claude/mcp.json` | `AGENTS.md` / `~/.claude/skills/` |
| **Codex** | `~/.codex/config.json` + env | `~/.codex/mcp.json` | `AGENTS.md` / `~/.codex/instructions.md` |
| **Cursor** | `.cursor/rules` / settings | `.cursor/mcp.json` | `.cursorrules` |
| **OpenCode** | `~/.opencodesrc` / config | `~/.opencode/mcp.json` | `AGENTS.md` |
| **Gemini CLI** | `~/.gemini/settings.json` | `~/.gemini/mcp.json` | `AGENTS.md` |
| **Aider** | `.aider.conf.yml` | — | `CONVENTIONS.md` |
| **Continue** | `.continue/config.json` | `.continue/config.json` | `.continue/config.json` |

**自动同步策略**：
- 手动：`solo providerhub sync --agent claude`
- 自动：当 Provider / MCP / Skill / Prompt 变更时，触发对应 agent 的配置重写。
- 项目级：项目根目录 `.solo/provider-hub.json` 声明该项目的 target agents，执行 `solo providerhub sync` 时一次性导出。

### 4.4 MCP 统一管理

- Solo daemon 已支持 MCP Server 注入 Agent。
- Provider Hub 层新增跨 agent 的 MCP 注册表。
- 用户添加一个 MCP 时，可选择同步到哪些 target agents。
- 对每个 agent 调用对应的 Exporter 生成其专有 MCP 配置文件。

### 4.5 Skills / Prompts 管理

**Skills**：
- 存储位置：`~/.solo/provider-hub/skills/{id}/`
- 支持来源：GitHub repo、本地目录、URL
- 变更检测：SHA-256 比对，支持批量更新
- 与 `.agents/skills/` 标准兼容（Solo 项目本身已在用 skill 系统）

**Prompts**：
- 存储位置：`~/.solo/provider-hub/prompts/{id}.json`
- 支持按 agent 导出为 system prompt / AGENTS.md / .cursorrules

### 4.6 Usage Tracker（用量监控）

```go
type UsageRecord struct {
    Timestamp   time.Time
    Agent       string   // claude / codex / solo-internal
    ProviderID  string
    Model       string
    RequestID   string
    InputTokens int64
    OutputTokens int64
    Cost        float64
    Currency    string
    Status      string // success / error / cached
}
```

- 通过 Proxy 拦截请求/响应，解析 token 用量。
- 对 Solo 内部 Agent，在 ProviderClient 层 hook。
- 存储：SQLite 或按日 JSONL（复用 memory 的 file backend）。
- App 展示：仪表盘、按 provider/agent 过滤、趋势图。

---

## 5. CLI / App 界面

### 5.1 CLI 命令

```bash
# Provider 管理
solo provider ls
solo provider add --preset siliconflow
solo provider edit <id>
solo provider enable <id> --agent claude
solo provider disable <id> --agent claude
solo provider rm <id>

# Agent target 管理
solo agent-config ls
solo agent-config set <agent> --provider <id>

# 切换（cc-switch 核心能力）
solo switch claude --provider siliconflow
solo switch codex --model gpt-5.4

# MCP / Skills / Prompts
solo mcp ls
solo mcp add <name> --command "npx -y ..."
solo skill install github:org/skill
solo skill update --all
solo prompt add <name> --file prompt.md

# 导出/同步
solo providerhub sync                # 同步所有已启用 target 的 agent
solo providerhub export claude       # 只导出 Claude Code 配置
solo providerhub check               # 检查各 agent 配置健康状态

# 用量
solo usage                           # 今日用量
solo usage --provider siliconflow    # 按 provider
solo usage --agent claude            # 按 agent
```

### 5.2 App 界面

- **Provider Hub 主页**：provider 卡片列表、余额、启用状态、快速切换。
- **Agent Config 页**：为每个外部 agent 选择 provider、model、MCP、skills、prompts。
- **MCP Manager**：新增/编辑/测试 MCP Server，选择同步目标。
- **Skills Market**：浏览、安装、更新 skills。
- **Prompts 库**：管理 system prompt 预设。
- **Usage Dashboard**：用量、花费、趋势。
- **Switch Sheet**：底部弹窗快速切换 provider（类似 cc-switch 托盘菜单）。

---

## 6. 与 Solo 现有模块的集成

| 现有模块 | 集成方式 |
|----------|----------|
| `daemon/internal/config` | `config.json` 新增 `providerHub` 字段；`CustomModels` 迁移到 Provider Hub |
| `daemon/internal/agent` | Agent 启动时从 Provider Hub 解析 provider/model；用量 hook |
| `daemon/internal/relayclient` | Relay 可转发 Proxy 流量，支持远程使用 Local API Proxy |
| `daemon/internal/memory` | Usage 记录可复用 file backend 或 SQLite |
| `app` | 新增 Provider Hub 屏幕、MCP Manager、Usage Dashboard |
| `cli` | 新增 `provider`、`switch`、`mcp`、`skill`、`prompt`、`usage` 子命令 |
| `.agents/skills/` | Skills 存储与 Solo 现有 skill 标准兼容 |

---

## 7. 安全与隐私

1. **API Key 不持久化明文**：使用 keychain / OS credential store，或 `env:`/`file:` 引用。
2. **本地代理默认只监听 127.0.0.1**：防止外部网络访问。
3. **OAuth 流程隔离**：与 cc-switch 类似，reverse proxy 模式需明确风险告知。
4. **危险操作审批**：即使外部 agent 通过 Solo 代理，Solo 不替代其审批流程。

---

## 8. 实施路线图

### Phase 1：Provider Hub 基础（3–4 周）

1. 新增 `daemon/internal/providerhub/`：
   - `registry.go`：Provider 增删改查
   - `router.go`：provider 选择逻辑
   - `preset.go`：内置 provider 预设
2. 扩展 `config.json` 支持 `providerHub`。
3. CLI：`solo provider`、`solo switch`、`solo agent-config`。
4. App：Provider Hub 主页、Agent Config 页。
5. 集成到 Solo 内部 Agent：启动时从 Hub 解析 provider/model。

### Phase 2：Local API Proxy（2–3 周）

1. 新增独立 HTTP 代理端口（`:17613`）。
2. 实现 anthropic-messages ↔ openai-chat 协议转换。
3. failover 和断路器。
4. 用量拦截和记录。
5. CLI/App：Proxy 开关、健康检查。

### Phase 3：Config Exporter（2 周）

1. 定义 `Exporter` 接口。
2. 实现 Claude Code、Codex、Cursor、OpenCode Exporter。
3. MCP / Skill / Prompt 转写。
4. `solo providerhub sync`。

### Phase 4：MCP / Skills / Prompts 管理（2–3 周）

1. MCP 统一管理面板。
2. Skills Market（GitHub 集成、SHA-256 更新检测）。
3. Prompts 库和导出。

### Phase 5：Usage Dashboard（1–2 周）

1. Usage 记录存储和查询。
2. App 仪表盘和报表。
3. Provider 余额查询（对支持 API 的 provider）。

---

## 9. 风险与取舍

| 风险 | 应对 |
|---|---|
| 外部 agent 配置路径变化快 | Exporter 接口隔离；单元测试覆盖；版本标注 |
| 协议转换复杂 | 先支持 anthropic/openai 两大协议；其他协议逐步添加 |
| API Key 安全 | keychain 存储；env/file 引用；不记录请求 body |
| 与现有 `CustomModels` 冲突 | 迁移期兼容：读取 `CustomModels` 并同步到 Provider Hub |
| OAuth reverse proxy 合规风险 | 明确风险提示；默认关闭；用户手动启用 |

---

## 10. 与 Solo 产品定位的关系

该方案直接强化 Solo 的差异化定位：

- **本地优先的 AI 开发中枢**：Provider Hub 让 Solo 成为本地 coding agent 的“总控台”。
- **移动端指挥中心**：App 可远程切换 provider、查看用量、管理 MCP。
- **多 Provider 中立入口**：不绑定单一模型，支持一键切换和 failover。
- **开放生态**：MCP、Skills、Prompts 都可跨 agent 复用。

---

## 参考文档

- [Product Feature Directions 2026](feature-directions-2026.md)
- [Product Features](features.md)
- [Session Memory Spec](session-memory-spec.md)
- [Kimi & Cursor-Agent Integration](../providers/kimi-cursor-integration.md)
- [Architecture Components](../architecture/components.md)
