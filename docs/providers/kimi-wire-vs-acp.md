# Kimi Wire vs ACP Comparative Analysis

> Analysis Date: 2026-05-21
> Scope: Kimi CLI v1.43.0 (Wire Protocol 1.10) vs Agent Client Protocol (ACP)

---

## 1. Executive Summary

| Dimension | **Kimi Wire** | **Kimi ACP** |
|-----------|--------------|-------------|
| **Protocol Positioning** | Kimi-specific internal protocol | Cross-agent universal standard (similar to LSP) |
| **Design Goal** | Custom UI / app embedding | IDE / editor integration |
| **Session Architecture** | Single session process | Multi-session server |
| **Event Granularity** | Very fine (Step/ContentPart/ToolCallPart) | Coarser (sessionUpdate chunk) |
| **Transport** | Local stdio only | Local stdio + remote HTTP/WebSocket |
| **Feature Coverage** | Full Kimi features (Plan/Steer/Hook/Subagent) | Common subset |
| **Solo Compatibility** | Very high | Medium |

**Conclusion: For the Solo platform, Wire mode is the superior choice.**

---

## 2. Protocol Positioning and Standardization

### 2.1 Kimi Wire — Specialized Protocol

- **Defined by**: Moonshot AI (Kimi team)
- **Documentation**: [Kimi Wire Mode docs](https://moonshotai.github.io/kimi-cli/en/customization/wire-mode.md)
- **Version**: 1.10 (consistent with `wire protocol: 1.10` shown by `kimi info`)
- **Scope**: Only supported by Kimi CLI
- **Evolution**: Controlled by the Kimi team, can iterate quickly

**Design Goal**: Wire is the internal messaging layer of Kimi Code CLI. When you interact with Kimi through the terminal, the Shell UI receives AI output via Wire; when you integrate with an IDE through ACP, the ACP Server also communicates with the Agent Core via Wire. `kimi --wire` exposes this protocol, allowing external programs to interact directly with Kimi.

**Use Cases**:
- Building custom UIs (Web/Desktop/Mobile)
- Embedding Kimi into other applications
- Automated testing of Agent behavior

### 2.2 ACP — Universal Standard

- **Defined by**: [agentclientprotocol.com](https://agentclientprotocol.com/) (cross-organization standard)
- **Analogy**: Similar to LSP (Language Server Protocol)
- **Version**: Integer major version (currently v1)
- **Scope**: Any Agent and Client implementing ACP
- **Evolution**: Must consider multi-implementer compatibility, changes are slower

**Design Goal**: Standardize communication between code editors/IDEs and coding Agents, solving the problem of "every editor must build custom integration for every Agent."

**Use Cases**:
- IDE plugin integration (Zed, JetBrains, etc.)
- Universal Clients that need to support multiple Agents simultaneously
- Remote Agent scenarios (HTTP/WebSocket)

---

## 三、Session 架构对比

### 3.1 Wire — 单 Session 进程模型

```
Solo 创建 Kimi Agent:

  Solo Agent (1:1 映射)
       |
       | fork/exec
       | kimi --wire --work-dir /project --session <id>
       v
  +---------------------+
  | kimi --wire 进程    |  <- 一个进程 = 一个 Session
  | - 独立的上下文      |
  | - 独立的历史记录    |
  | - 独立的工具状态    |
  +---------------------+

恢复 Session:
  kimi --wire --session <id>     (指定 session ID)
  kimi --wire --continue         (继续上一个 session)
```

**特点**：
- 每个 Agent 对应一个独立的 OS 进程
- 进程即 Session 边界，天然隔离
- Session 管理由 CLI 自身负责 (`~/.kimi/sessions/`)
- Solo 只需记住 session ID，通过 `--session` 或 `--continue` 恢复

### 3.2 ACP — 多 Session Server 模型

```
Solo 连接 Kimi ACP:

  Solo
    |
    | fork/exec (一次)
    | kimi acp
    v
  +---------------------+
  | kimi acp Server     |  <- 一个进程 = 多个 Session
  |                     |
    |
    | JSON-RPC
    | session/new {cwd, mcpServers}
    v
  Session A (id: sess_001)
  Session B (id: sess_002)
  Session C (id: sess_003)
```

**特点**：
- 一个 ACP Server 进程管理多个 Session
- Client 通过 `session/new` 创建，`session/load` 加载，`session/resume` 恢复
- Session 间共享同一进程资源
- 适合 IDE 同时打开多个项目的场景

### 3.3 架构差异对 Solo 的影响

| 场景 | Wire | ACP |
|------|------|-----|
| **创建 Agent** | 直接 fork `kimi --wire` | 先 fork `kimi acp`，再发 `session/new` |
| **关闭 Agent** | Kill 进程 | 发 `session/close` 或 Kill Server |
| **资源隔离** | 进程级隔离 | 逻辑隔离 |
| **并发 Agent** | 多个独立进程 | 共享一个 Server |
| **故障影响** | 单个 Agent 崩溃不影响其他 | Server 崩溃影响所有 Session |
| **Solo 适配** | 与现有 Claude provider 架构一致 | 需要额外的 Server 生命周期管理 |

**Solo 的 Agent 模型是"每个 Agent 一个独立 Provider Session"，与 Wire 的单进程模型天然契合。**

---

## 四、通信协议对比

### 4.1 基础协议

| 特性 | Wire | ACP |
|------|------|-----|
| **传输格式** | JSON-RPC 2.0 | JSON-RPC 2.0 |
| **传输层** | stdin/stdout | stdin/stdout (本地) / HTTP / WebSocket (远程) |
| **消息方向** | 双向 | 双向 |
| **编码** | 逐行 NDJSON | 逐行 NDJSON |

### 4.2 方法/事件对比

#### Agent 端提供的方法

| Wire 方法 | ACP 方法 | 说明 |
|-----------|---------|------|
| `initialize` | `initialize` | 握手协商版本和能力 |
| `prompt` | `session/prompt` | 发送用户输入启动 turn |
| `cancel` | `session/cancel` | 取消当前 turn |
| `set_plan_mode` | `session/set_mode` | 设置模式 |
| `steer` | (无直接对应) | 向运行中 turn 注入追加输入 |
| `replay` | (无直接对应) | 触发历史回放 |
| (无) | `session/new` | 创建新 Session |
| (无) | `session/load` | 加载并回放已有 Session |
| (无) | `session/resume` | 恢复 Session (不回放历史) |
| (无) | `session/close` | 关闭 Session |
| (无) | `authenticate` | 认证 |

#### Client 端提供的方法 (ACP 特有)

ACP 要求 Client (IDE) 实现一组方法供 Agent 调用，这是 Wire 没有的：

| ACP Client 方法 | 说明 | Solo 支持难度 |
|----------------|------|-------------|
| `fs/read_text_file` | 读取文件 | 低 (已有文件系统能力) |
| `fs/write_text_file` | 写入文件 | 低 |
| `terminal/create` | 创建终端 | 中 (需接入 TerminalManager) |
| `terminal/output` | 获取终端输出 | 中 |
| `terminal/release` | 释放终端 | 中 |
| `terminal/wait_for_exit` | 等待终端退出 | 中 |
| `terminal/kill` | 终止终端 | 中 |
| `session/request_permission` | 请求权限 | 低 (已有权限系统) |

**Wire 没有 Client 方法概念** —— Wire 的 `ApprovalRequest`/`ToolCallRequest` 通过 JSON-RPC `request` 方法发送，Client 直接回复 JSON-RPC response 即可，不需要实现额外的 RPC 服务。

### 4.3 通知/事件对比

#### Wire 事件 (Notification)

| Wire Event | 粒度 | Solo 映射 |
|-----------|------|----------|
| `TurnBegin` | Turn 级 | `thread_started` |
| `StepBegin` | Step 级 | — |
| `StepInterrupted` | Step 级 | — |
| `StepRetry` | Step 级 | `timeline(error)` |
| `ContentPart(text)` | Token/块级 | `timeline(assistant_message)` |
| `ContentPart(think)` | Token/块级 | `timeline(reasoning)` |
| `ToolCall` | 调用级 | `timeline(tool_call)` |
| `ToolCallPart` | 参数片段级 | — |
| `ToolResult` | 结果级 | `timeline(tool_call completed)` |
| `ToolProgress` | 进度级 | `timeline(tool_call running)` |
| `ApprovalResponse` | 审批级 | — |
| `TurnEnd` | Turn 级 | `turn_completed` |
| `CompactionBegin/End` | 压缩级 | `timeline(compaction)` |
| `StatusUpdate` | 状态级 | — |
| `SubagentEvent` | 子 Agent 级 | — |
| `BtwBegin/End` | 旁问级 | — |
| `SteerInput` | 输入级 | — |
| `PlanDisplay` | 计划级 | — |
| `HookTriggered/Resolved` | Hook 级 | — |

#### ACP 通知 (Notification)

| ACP Notification | 粒度 | Solo 映射 |
|-----------------|------|----------|
| `session/update` | Chunk 级 | 多种子类型 |
| `session/update` (plan) | Plan 级 | — |
| `session/update` (user_message_chunk) | Message 级 | `timeline(user_message)` |
| `session/update` (agent_message_chunk) | Message 级 | `timeline(assistant_message)` |
| `session/update` (tool_call) | 调用级 | `timeline(tool_call)` |
| `session/update` (tool_call_update) | 状态级 | `timeline(tool_call status)` |

### 4.4 事件粒度对比

**场景：Agent 执行一个包含 Thinking + Tool Call + 文本回复的 Turn**

```
Wire 事件流 (约 15-20 个事件):
  TurnBegin
  StepBegin(n=1)
  ContentPart(think="Analyzing...")
  ContentPart(think=" the code")
  ContentPart(text="I'll help")
  ToolCall(id=tc1, name=read_file)
  ToolCallPart(arguments_part="{\"path\":")
  ToolCallPart(arguments_part="\"main.py\"}")
  ToolResult(tool_call_id=tc1, ...)
  ContentPart(text="Based on")
  ContentPart(text=" the file")
  StepBegin(n=2)
  ContentPart(text="Here's the")
  ContentPart(text=" analysis:")
  StatusUpdate(token_usage={...})
  TurnEnd

ACP 事件流 (约 5-8 个事件):
  session/update: agent_message_chunk("I'll help")
  session/update: tool_call(id=tc1, title="Reading file", status=pending)
  session/update: tool_call_update(id=tc1, status=in_progress)
  session/update: tool_call_update(id=tc1, status=completed, content=[...])
  session/update: agent_message_chunk("Based on the file")
  session/update: agent_message_chunk("Here's the analysis:")
  session/prompt response: {stopReason: "end_turn"}
```

**关键差异**：
- **Wire** 暴露 Step 边界、Thinking 内容、Tool Call 参数片段、Token 使用统计等内部细节
- **ACP** 将多个内部步骤聚合为较粗的 `agent_message_chunk` 和 `tool_call_update`
- **Wire 更适合 Solo** 的 Timeline 模型，因为 Solo 需要展示 reasoning、tool_call running、compaction 等细粒度事件

---

## 五、功能覆盖对比

### 5.1 Wire 特有功能 (ACP 不支持)

| 功能 | Wire 支持 | 说明 |
|------|----------|------|
| **Steer** | `steer` 方法 | 向运行中的 turn 注入追加输入，无需等待 turn 结束 |
| **Plan Mode 细粒度控制** | `set_plan_mode` + `PlanDisplay` | 完整的 Plan 创建、显示、提交流程 |
| **Hooks** | `HookTriggered/Resolved` + `HookRequest` | 在工具调用前后执行自定义逻辑 |
| **Subagent** | `SubagentEvent` | 嵌套子 Agent 的事件透传 |
| **Btw (旁问)** | `BtwBegin/End` | 侧边栏快速问答 |
| **历史 Replay** | `replay` 方法 | 完整重放历史事件流 |
| **Skills/Plugins** | 通过 `initialize` 注册 | 自定义技能目录 |
| **Step Retry** | `StepRetry` 事件 | 显示重试状态和等待时间 |
| **Context Compaction** | `CompactionBegin/End` | 上下文压缩的完整生命周期 |

### 5.2 ACP 特有功能 (Wire 不支持)

| 功能 | ACP 支持 | 说明 |
|------|---------|------|
| **多 Session** | `session/new/load/resume/close` | 一个 Server 管理多个 Session |
| **远程传输** | HTTP / WebSocket | 支持云端 Agent |
| **MCP Server 配置** | `session/new` 参数 | 标准化的 MCP 连接配置 |
| **资源嵌入** | `ContentBlock::Resource` | 在 prompt 中嵌入文件/资源 |
| **Agent Plan** | `session/update: plan` | 结构化计划条目 (priority, status) |
| **通用 StopReason** | `end_turn/max_tokens/...` | 标准化的 turn 结束原因 |

### 5.3 两者共有功能

| 功能 | Wire | ACP |
|------|------|-----|
| 文本生成 | ContentPart(text) | agent_message_chunk |
| Thinking/Reasoning | ContentPart(think) | (通常嵌入在 chunk 中) |
| 工具调用 | ToolCall + ToolResult | tool_call + tool_call_update |
| 权限请求 | ApprovalRequest | session/request_permission |
| Session 恢复 | --session / --continue | session/load / session/resume |
| 取消 | cancel | session/cancel |
| 模型切换 | --model | (通过 session/set_mode 或配置) |

---

## 六、客户端职责对比

### 6.1 Wire — 轻量 Client

Client (Solo) 的职责：
1. 启动 `kimi --wire` 进程
2. 发送 `initialize` 握手
3. 发送 `prompt` 启动 turn
4. **读取 stdout 的 event/request**
5. 对于 `ApprovalRequest`，回复 JSON-RPC response
6. 对于 `ToolCallRequest` (外部工具)，执行并回复
7. 发送 `cancel` 取消 turn

**Client 不需要实现的 RPC 方法** —— 所有通信都是"Client 发请求 / Agent 推事件"或"Agent 发 request / Client 回复 response"的简单模式。

### 6.2 ACP — 重量 Client

Client (IDE) 的职责：
1. 启动 `kimi acp` Server (一次)
2. 发送 `initialize` 握手
3. 发送 `session/new` 创建 Session
4. 发送 `session/prompt` 启动 turn
5. **读取 stdout 的 session/update**
6. **实现 `fs/read_text_file` 等方法供 Agent 调用**
7. **实现 `terminal/create` 等方法供 Agent 调用**
8. 处理 `session/request_permission`
9. 发送 `session/cancel` 取消

**ACP 要求 Client 成为一个 RPC 服务端**，Agent 会主动调用 Client 的方法（如读取文件、创建终端）。这对 Solo 意味着：
- 需要在 stdin/stdout 上同时处理"Client 作为请求方"和"Client 作为响应方"两种角色
- 需要实现文件系统和终端的 ACP 适配层
- 虽然 Solo 已有文件系统和终端能力，但增加了一层协议转换复杂度

---

## 七、Solo 平台适配分析

### 7.1 为什么 Wire 更适合 Solo

| 因素 | Wire 适配 | ACP 适配 |
|------|----------|---------|
| **架构一致性** | 与 Claude provider (stdio + 进程) 完全一致 | 需要引入 Server + Session 双层管理 |
| **事件粒度** | 细粒度事件直接映射到 Solo Timeline | 粗粒度 chunk 需要额外拆分/解析 |
| **Client 复杂度** | 轻量，无需实现 RPC 服务端 | 重，需实现 fs/terminal 等 Client 方法 |
| **权限系统** | ApprovalRequest 直接映射 permission_requested | session/request_permission 语义相似但需适配 |
| **进程管理** | 直接复用 base.ProcessManager | 需管理 ACP Server 生命周期 |
| **故障隔离** | 进程级隔离，与现有模型一致 | Server 单点故障影响多个 Agent |
| **功能完整** | 支持 Steer/Plan/Hook/Subagent 等全部功能 | 部分功能缺失或需额外实现 |

### 7.2 ACP 的潜在优势场景

尽管 Wire 更适合 Solo，ACP 在以下场景有优势：

1. **未来支持远程 Kimi Agent**：如果 Moonshot 推出云端 Kimi Agent，ACP 的 HTTP/WebSocket 传输可以直接使用
2. **统一多 Agent 协议**：如果 Solo 未来需要支持大量 ACP Agent（如 Copilot 等），统一的 ACP Client 实现可减少重复代码
3. **IDE 协同**：如果 Solo 需要与本地 IDE 共享同一个 Kimi ACP Server，ACP 的多 Session 模型更合适

### 7.3 推荐策略

```
当前阶段 (Solo v0.1.x):
  +----------------------------------+
  |  Kimi Provider: Wire 模式        |
  |  - 实现简单，与 Claude 架构一致   |
  |  - 功能完整，用户体验好           |
  +----------------------------------+

未来阶段 (如需扩展):
  +----------------------------------+
  |  引入 ACP Client 层 (可选)        |
  |  - 统一支持 Kimi/Copilot/其他    |
  |  - 作为 Provider 的另一种实现     |
  +----------------------------------+
```

---

## 八、决策矩阵

| 评估维度 | 权重 | Wire 评分 | ACP 评分 | 说明 |
|---------|------|----------|---------|------|
| 实现复杂度 | 高 | 9/10 | 5/10 | Wire 与现有架构一致 |
| 功能完整性 | 高 | 10/10 | 6/10 | Wire 支持所有 Kimi 功能 |
| 事件粒度 | 高 | 10/10 | 6/10 | Wire 细粒度适合 Timeline |
| 未来扩展性 | 中 | 6/10 | 8/10 | ACP 标准化，支持远程 |
| 维护成本 | 中 | 8/10 | 6/10 | Wire 协议由 Kimi 控制，文档完整 |
| 多 Agent 统一 | 低 | 4/10 | 9/10 | ACP 是跨 Agent 标准 |
| **加权总分** | | **8.7/10** | **6.3/10** | |

---

## 九、结论

**对于 Solo 平台的 Kimi Provider 集成，强烈推荐使用 Wire 模式。**

理由：
1. **架构一致**：与现有的 Claude stdio provider 架构完全一致，可复用 `base.ProcessManager`、`base.EventPump`、translator/detector 模式
2. **功能完整**：支持 Steer、Plan Mode、Hooks、Subagent 等 Kimi 特有功能，ACP 无法提供
3. **事件匹配**：Wire 的细粒度事件（Step、ContentPart、ToolCallPart、Compaction）与 Solo 的 Timeline 模型高度匹配
4. **实现简单**：Client 无需实现 RPC 服务端（fs/terminal 等方法），只需读取 stdout 并回复 response
5. **进程隔离**：单进程单 Session 模型与 Solo 的 Agent 生命周期管理天然契合

ACP 模式建议作为未来扩展选项保留，当需要：
- 支持远程 Kimi Agent 时
- 与 IDE 共享同一个 Kimi Server 时
- 统一接入大量 ACP 兼容 Agent 时

再考虑引入 ACP Client 实现。
