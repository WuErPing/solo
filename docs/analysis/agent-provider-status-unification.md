# Agent Provider Status Unification Plan

## Current State Analysis

The current codebase contains **three redundant state definitions**:

### 1. Protocol Layer (`protocol/protocol.go`)
- `AgentLifecycleStatus`: initializing, idle, running, error, closed
- `ProviderStatus`: ready, loading, error, unavailable

### 2. Internal Implementation Layer (`daemon/internal/agent/agent.go`)
- `AgentLifecycle`: initializing, idle, running, error, closed (identical values to AgentLifecycleStatus)

### 3. Frontend Layer (`app-bridge/src/shared/agent-lifecycle.ts`)
- `AGENT_LIFECYCLE_STATUSES` array definition

### Core Problems

- **Redundant definitions**: `AgentLifecycle` and `AgentLifecycleStatus` have identical values but different type names
- **Leakage of differences**: `ProviderStatus` exposes underlying Provider differences (ready/loading/unavailable) to the protocol layer and upper-level code
- **Confused responsibilities**: The core scheduling layer must understand both `AgentStatus` and `ProviderAvailabilityStatus`, and handle their combination logic
- **Extension cost**: Adding a new Provider requires modifying the protocol layer definition, core scheduling logic, and frontend UI to adapt to new state semantics

---

## Design Principle: OCP First

**The core meaning of the Open/Closed Principle in this scenario**: Differences between Agent Providers should leak into upper layers as little as possible.

> When adding a new Provider (extension), the core scheduler, protocol layer, and frontend UI should not be modified (closed).

Specific principles:

1. **Difference cohesion**: Provider-specific state semantics are encapsulated within their respective implementations
2. **Unified abstraction**: Upper layers interact with only one stable state interface
3. **Open for extension**: Adding a new Provider only requires implementing the unified interface, without touching existing code
4. **Closed for modification**: The core layer does not perceive or handle state differences between Providers
5. **Backward compatibility**: Preserve old type aliases to avoid breaking changes

---

## 统一方案

### 核心洞察：删除 Provider 层独立状态类型

`ProviderStatus`（ready/loading/error/unavailable）的存在是差异外泄的根源。不同 Provider 的可用性模型完全不同：

- OpenAI 可能只需要"连接中 → 就绪"
- 本地模型可能需要"下载中 → 加载中 → 就绪"
- 某个 Provider 可能有"预热中"状态

这些差异不应该成为协议层的共享概念。它们应该被各自 Provider 映射为统一的 `AgentStatus`。

### 方案：单一状态抽象 + Provider 自治映射

```go
// protocol/protocol.go

// AgentStatus 是上层唯一的共享状态抽象
// 所有 Provider 的内部状态最终都映射为这个统一类型
type AgentStatus string

const (
    AgentInitializing AgentStatus = "initializing"
    AgentIdle         AgentStatus = "idle"
    AgentRunning      AgentStatus = "running"
    AgentError        AgentStatus = "error"
    AgentClosed       AgentStatus = "closed"
)

func (s AgentStatus) IsTerminal() bool {
    return s == AgentError || s == AgentClosed
}

func (s AgentStatus) IsActive() bool {
    return s == AgentRunning || s == AgentIdle
}

// ---- 向后兼容别名 ----
// 保留旧类型名，避免破坏性变更
type AgentLifecycleStatus = AgentStatus

// ProviderStatus 被删除。Provider 的可用性语义内聚在各自实现中。
```

### Provider 接口：差异封装点

```go
// protocol/provider.go

type Provider interface {
    Name() string
    // Status 返回统一的 AgentStatus
    // Provider 内部负责将自身特有状态映射为统一语义
    Status() AgentStatus
    Send(ctx context.Context, msg Message) error
    // ...
}
```

### Provider 实现示例：差异完全内聚

```go
// daemon/internal/provider/openai/provider.go

type OpenAIProvider struct {
    internalState string // "connecting", "ready", "generating", "fail"
}

func (p *OpenAIProvider) Status() AgentStatus {
    switch p.internalState {
    case "connecting", "authenticating":
        return AgentInitializing
    case "ready":
        return AgentIdle
    case "generating":
        return AgentRunning
    case "fail":
        return AgentError
    default:
        return AgentClosed
    }
}

// daemon/internal/provider/local/provider.go

type LocalProvider struct {
    internalState string // "downloading", "loading", "standby", "inferencing"
}

func (p *LocalProvider) Status() AgentStatus {
    switch p.internalState {
    case "downloading", "loading":
        return AgentInitializing
    case "standby":
        return AgentIdle
    case "inferencing":
        return AgentRunning
    // ...
    }
}
```

**关键：上层代码完全不感知 Provider 的内部状态差异。**

---

## 修改清单（TDD 实际执行）

### Protocol 层

| 文件 | 修改类型 | 内容 |
|------|---------|------|
| `protocol/protocol.go` | 修改 | 引入 `AgentStatus` + `IsTerminal()`/`IsActive()`；`AgentLifecycleStatus` 改为 `AgentStatus` 别名；`ProviderStatus` 类型删除，改为字符串常量 |
| `protocol/statemachine.go` | **新增** | 泛型 `StateMachine[S]` 框架：Allow / OnEnter / OnExit / CanTransition / Transition / Current |
| `protocol/message_agent_outbound.go` | 修改 | `ProviderSnapshotEntry.Status` 从 `ProviderStatus` → `string` |
| `protocol/status_test.go` | **新增** | AgentStatus 常量、IsTerminal/IsActive、向后兼容别名测试 |
| `protocol/statemachine_test.go` | **新增** | 合法/非法转移、terminal 状态、hooks、多步转移测试 |

### Daemon 层

| 文件 | 修改类型 | 内容 |
|------|---------|------|
| `daemon/internal/agent/agent.go` | 修改 | 删除 `AgentLifecycle` 类型；`Lifecycle` 字段改为 `protocol.AgentStatus`；常量引用替换 |
| `daemon/internal/agent/manager.go` | 修改 | 所有 `LifecycleXxx` → `protocol.AgentXxx` |
| `daemon/internal/agent/provider_registry.go` | 修改 | `protocol.ProviderReady` → `"ready"`；`protocol.ProviderUnavailable` → `"unavailable"` |
| `daemon/internal/agent/provider_registry_test.go` | 修改 | 字符串常量替换 |
| `daemon/internal/agent/*_test.go`（4 个） | 修改 | 常量引用替换 + `protocol` 包导入 |
| `daemon/internal/agent/agent_status_test.go` | **新增** | ManagedAgent 状态类型、SetLifecycle、IsBusy、ToSnapshot 测试 |

### 前端层

| 文件 | 修改类型 | 内容 |
|------|---------|------|
| `app-bridge/src/shared/agent-lifecycle.ts` | 修改 | `AGENT_STATUSES` 取代 `AGENT_LIFECYCLE_STATUSES`；`AgentStatus` 为主类型；保留 `AgentLifecycleStatus` 别名 |
| `app-bridge/src/shared/messages.ts` | 修改 | 导入名 `AGENT_LIFECYCLE_STATUSES` → `AGENT_STATUSES` |
| `app/src/components/agent-status-dot.tsx` | 修改 | 导入和函数名同步更新 |

---

## 状态映射关系

### 层级职责

| 层级 | 状态类型 | 状态机归属 | 对上层暴露 |
|------|---------|-----------|-----------|
| **协议层** (`protocol`) | `AgentStatus` | 无（纯类型定义） | `AgentStatus` + `StateMachine[S]` 框架 |
| **核心调度** (`daemon/agent/manager`) | `AgentStatus` | `AgentStateMachine`（统一规则） | `AgentStatus`（通过 `agent.stateMachine.Current()`） |
| **Provider 实现** (`daemon/provider/*`) | 私有状态（如 `localStatus`） | 各 Provider 私有状态机 | `AgentStatus`（通过 `Status()` 映射） |
| **前端** (`app` / `app-bridge`) | `AgentStatus` | 无（只读消费） | `AgentStatus` 字符串值 |

### 数据流

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Provider A    │     │   Provider B    │     │   Provider C    │
│  (localStatus)  │     │  (openaiStatus) │     │  (customStatus) │
│   private SM    │     │   private SM    │     │   private SM    │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │ Status()                │ Status()                │ Status()
         │ (映射为统一状态)         │ (映射为统一状态)         │ (映射为统一状态)
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                        protocol.AgentStatus                       │
│              （统一的 5 状态值：initializing/idle/running/error/closed）│
└─────────────────────────────────────────────────────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Agent Manager  │     │   前端 UI 组件   │     │   监控/日志系统  │
│ (状态机校验转移) │     │ (状态显示/动画)  │     │ (状态统计/告警)  │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

---

## 好处

| 维度 | 重构前 | 重构后 |
|------|--------|--------|
| **重复定义** | `AgentLifecycle` + `AgentLifecycleStatus` 值相同、类型不同 | 单一 `AgentStatus`，`AgentLifecycleStatus` 为别名 |
| **Provider 差异** | `ProviderStatus` 暴露到协议层，上层必须理解 ready/loading/unavailable 差异 | 差异完全内聚在 Provider 内部，上层只看 `AgentStatus` |
| **状态转移** | 隐式直接赋值：`agent.Lifecycle = LifecycleRunning` | 显式状态机：`sm.Transition(AgentRunning)`，非法转移被拒绝 |
| **副作用管理** | `SetError()`、`SetLifecycle()` 散落各处 | 集中在 `OnEnter`/`OnExit` 注册，转移矩阵一目了然 |
| **新增 Provider** | 改 protocol + 改 daemon + 改前端 | 只新增一个 Provider 实现文件 |
| **类型安全** | 字符串硬编码 | `protocol.AgentStatus` 编译期检查 |
| **向后兼容** | — | `AgentLifecycleStatus` 别名保留，JSON 序列化不变 |
| **前端** | 两套状态概念 | 只处理 `AgentStatus`，Provider 状态透明 |

---

## 前端同步

```typescript
// app-bridge/src/shared/agent-lifecycle.ts

export const AGENT_STATUSES = [
  "initializing",
  "idle",
  "running",
  "error",
  "closed",
] as const;

export type AgentStatus = (typeof AGENT_STATUSES)[number];

// ❌ 删除 Provider 状态相关定义
// Provider 特有状态（如"模型下载中"）映射为 "initializing"
// 或在 Provider 自身模块内处理，不污染共享层
```

---

## 实施建议（TDD 顺序）

```
Step 0: 基线 ──► 跑通所有测试，记录绿色基线
    │
Step 1: Protocol 测试（红）──► protocol/status_test.go + statemachine_test.go
    │
Step 1: Protocol 实现（绿）──► protocol/protocol.go + statemachine.go + message_agent_outbound.go
    │
Step 1.3: 向后兼容验证 ──► cd daemon && go build ./...（零修改编译通过）
    │
Step 2: Daemon 测试（红）──► agent_status_test.go
    │
Step 2: Daemon 实现（绿）──► agent.go + manager.go + 测试文件常量替换
    │
Step 2.3: 全量验证 ──► go test -race ./internal/agent/...（82s 通过）
    │
Step 3: ProviderStatus 删除 ──► provider_registry.go + provider_registry_test.go
    │
Step 3: 验证 ──► protocol + daemon 测试全绿
    │
Step 4: 前端同步 ──► agent-lifecycle.ts + messages.ts + agent-status-dot.tsx
    │
Step 4: 验证 ──► app-bridge test + app test (1307 passed) + expo lint
    │
Step 5: 清理 ──► 全局搜索残留引用 + JSON 序列化验证 + 全模块 go build
```

### 回滚策略

每完成一个 Step，如果全量测试失败且无法在 15 分钟内定位：

1. `git stash` 当前改动
2. 回到上一 Step 的通过状态
3. 分析问题，拆分更小的子步骤

---

## 进阶：引入状态机框架

当前方案中 `AgentStatus` 仅为字符串枚举，状态转换是**隐式的直接赋值**，存在几个问题：

- 没有编译期或运行期保护：代码可以轻易写出 `AgentInitializing → AgentClosed` 这样的非法跳跃
- 状态副作用散落各处：进入 `error` 时的清理、进入 `running` 时的资源分配，逻辑分散在多个文件
- 新增状态时需要人工保证所有判断逻辑一致更新

引入状态机框架可以解决这些问题，同时保持 OCP。

### 设计原则

1. **框架封闭**：状态机引擎是通用代码，不随新 Provider/Agent 改变
2. **配置开放**：每个实体（Agent、各 Provider）各自配置自己的状态转移规则
3. **差异内聚**：Provider 内部使用私有状态机，最终统一输出 `AgentStatus`

### 状态机框架

```go
// protocol/statemachine.go

// StateMachine 是通用的有限状态机实现
// S 为状态类型，由调用方定义
type StateMachine[S comparable] struct {
    current     S
    transitions map[S]map[S]struct{} // from -> set of valid to's
    onEnter     map[S]func(from S)
    onExit      map[S]func(to S)
}

func NewStateMachine[S comparable](initial S) *StateMachine[S] {
    return &StateMachine[S]{
        current:     initial,
        transitions: make(map[S]map[S]struct{}),
        onEnter:     make(map[S]func(from S)),
        onExit:      make(map[S]func(to S)),
    }
}

// Allow 注册一个合法的转移方向
func (sm *StateMachine[S]) Allow(from, to S) *StateMachine[S] {
    if sm.transitions[from] == nil {
        sm.transitions[from] = make(map[S]struct{})
    }
    sm.transitions[from][to] = struct{}{}
    return sm
}

// OnEnter 注册进入某状态时的回调
func (sm *StateMachine[S]) OnEnter(state S, fn func(from S)) *StateMachine[S] {
    sm.onEnter[state] = fn
    return sm
}

// OnExit 注册离开某状态时的回调
func (sm *StateMachine[S]) OnExit(state S, fn func(to S)) *StateMachine[S] {
    sm.onExit[state] = fn
    return sm
}

// CanTransition 检查是否可以转移到目标状态
func (sm *StateMachine[S]) CanTransition(to S) bool {
    _, ok := sm.transitions[sm.current][to]
    return ok
}

// Transition 执行状态转移，非法转移返回错误
func (sm *StateMachine[S]) Transition(to S) error {
    if !sm.CanTransition(to) {
        return fmt.Errorf("invalid transition: %v -> %v", sm.current, to)
    }
    from := sm.current
    if fn := sm.onExit[from]; fn != nil {
        fn(to)
    }
    sm.current = to
    if fn := sm.onEnter[to]; fn != nil {
        fn(from)
    }
    return nil
}

// Current 返回当前状态
func (sm *StateMachine[S]) Current() S {
    return sm.current
}
```

### Agent 状态机：显式定义转移规则

#### 状态定义（State-Transition-Action 表）

```
State initializing {
  // Trans          Next State      OnExit Actions
  initComplete     idle            { emitState(); persistSnapshot() }
  initFailed       error           { logError(); emitState() }
}

State idle {
  // Trans          Next State      OnEnter / OnExit Actions
  sendMessage      running         { OnExit: reserveSession() }
  fatalError       error           { OnEnter: logError(); emitState() }
  closeAgent       closed          { OnEnter: cleanup() }
}

State running {
  // Trans          Next State      OnEnter / OnExit Actions
  turnCompleted    idle            { OnExit: releaseSession(); flushBuffer() }
  turnFailed       error           { OnEnter: logError(); emitState() }
  closeAgent       closed          { OnEnter: cleanup() }
}

State error {
  // Trans          Next State      OnEnter Actions
  closeAgent       closed          { OnEnter: cleanup(); persistFinalSnapshot() }
}

State closed {
  // (absorbing state — no outgoing transitions)
}
```

#### 汇总表格

| State | Trans | Next State | Actions |
|-------|-------|------------|---------|
| **initializing** | `initComplete` | `idle` | `emitState()`; `persistSnapshot()` |
| **initializing** | `initFailed` | `error` | `logError()`; `emitState()` |
| **idle** | `sendMessage` | `running` | `OnExit`: `reserveSession()` |
| **idle** | `fatalError` | `error` | `OnEnter`: `logError()`; `emitState()` |
| **idle** | `closeAgent` | `closed` | `OnEnter`: `cleanup()` |
| **running** | `turnCompleted` | `idle` | `OnExit`: `releaseSession()`; `flushBuffer()` |
| **running** | `turnFailed` | `error` | `OnEnter`: `logError()`; `emitState()` |
| **running** | `closeAgent` | `closed` | `OnEnter`: `cleanup()` |
| **error** | `closeAgent` | `closed` | `OnEnter`: `cleanup()`; `persistFinalSnapshot()` |
| **closed** | — | — | *(absorbing state — no outgoing transitions)* |

#### 关键设计决策

- **initializing 不能直接到 closed**：Agent 必须先进入 idle 或 error，再走到 closed。这防止了初始化流程被中途强行中断而不留痕迹。
- **error 是单向的**：一旦进入 error，只能走向 closed，不能恢复。恢复逻辑应通过新建 Agent 实现。
- **closed 是吸收态（absorbing state）**：没有任何出口，确保资源释放后不会再被误用。

```go
// daemon/internal/agent/statemachine.go

var agentTransitions = map[AgentStatus][]AgentStatus{
    AgentInitializing: {AgentIdle, AgentError},
    AgentIdle:         {AgentRunning, AgentError, AgentClosed},
    AgentRunning:      {AgentIdle, AgentError, AgentClosed},
    AgentError:        {AgentClosed},
    AgentClosed:       {},
}

func NewAgentStateMachine() *StateMachine[AgentStatus] {
    sm := NewStateMachine(AgentInitializing)
    for from, tos := range agentTransitions {
        for _, to := range tos {
            sm.Allow(from, to)
        }
    }
    return sm.
        OnEnter(AgentRunning, func(from AgentStatus) {
            // 分配资源
        }).
        OnExit(AgentRunning, func(to AgentStatus) {
            // 释放资源
        }).
        OnEnter(AgentError, func(from AgentStatus) {
            // 记录日志、通知监控
        }).
        OnEnter(AgentClosed, func(from AgentStatus) {
            // 清理所有资源
        })
}
```

### Provider 内部状态机：差异内聚在各自实现中

Provider 使用**私有状态空间**，各自定义内部转移规则，最终统一映射为 `AgentStatus`。

#### LocalProvider 状态定义

```
State downloading {
  // Trans          Next State      Actions
  downloadDone     loading         {}
  downloadFailed   failed          { logError() }
}

State loading {
  // Trans          Next State      Actions
  loadComplete     standby         { notifyReady() }
  loadFailed       failed          { logError() }
}

State standby {
  // Trans          Next State      Actions
  startInference   inferencing     { acquireGPU() }
  dispose          disposed        { releaseAll() }
  runtimeError     failed          { logError() }
}

State inferencing {
  // Trans          Next State      Actions
  inferenceDone    standby         { releaseGPU() }
  inferenceFailed  failed          { logError() }
}

State failed {
  // Trans          Next State      Actions
  dispose          disposed        { releaseAll() }
}

State disposed {
  // (absorbing state — no outgoing transitions)
}
```

#### 汇总表格

| State | Trans | Next State | Actions |
|-------|-------|------------|---------|
| **downloading** | `downloadDone` | `loading` | — |
| **downloading** | `downloadFailed` | `failed` | `logError()` |
| **loading** | `loadComplete` | `standby` | `notifyReady()` |
| **loading** | `loadFailed` | `failed` | `logError()` |
| **standby** | `startInference` | `inferencing` | `acquireGPU()` |
| **standby** | `dispose` | `disposed` | `releaseAll()` |
| **standby** | `runtimeError` | `failed` | `logError()` |
| **inferencing** | `inferenceDone` | `standby` | `releaseGPU()` |
| **inferencing** | `inferenceFailed` | `failed` | `logError()` |
| **failed** | `dispose` | `disposed` | `releaseAll()` |
| **disposed** | — | — | *(absorbing state — no outgoing transitions)* |

#### 内部状态 → AgentStatus 映射表

| Provider 内部状态 | 映射为 AgentStatus | 语义说明 |
|------------------|-------------------|---------|
| `downloading` | `AgentInitializing` | 模型文件下载中 |
| `loading` | `AgentInitializing` | 模型加载到内存中 |
| `standby` | `AgentIdle` | 就绪，等待用户输入 |
| `inferencing` | `AgentRunning` | 正在生成回复 |
| `failed` | `AgentError` | 加载或推理失败 |
| `disposed` | `AgentClosed` | 资源已释放 |

> **关键**：上层只看到这个映射表的右侧（统一的 `AgentStatus`），左侧的私有状态对上层完全不可见。

#### OpenAI Provider 映射（对比）

| Provider 内部状态 | 映射为 AgentStatus | 语义说明 |
|------------------|-------------------|---------|
| `connecting` | `AgentInitializing` | 建立网络连接 |
| `authenticating` | `AgentInitializing` | API 密钥验证中 |
| `ready` | `AgentIdle` | 连接就绪 |
| `generating` | `AgentRunning` | 流式生成中 |
| `fail` | `AgentError` | 请求失败 |
| （默认） | `AgentClosed` | 连接断开 |

> 两个 Provider 的**内部状态完全不同**，但**输出给上层的 AgentStatus 完全一致**。

```go
// daemon/internal/provider/local/provider.go

type localStatus string

const (
    localDownloading localStatus = "downloading"
    localLoading     localStatus = "loading"
    localStandby     localStatus = "standby"
    localInferencing localStatus = "inferencing"
    localFailed      localStatus = "failed"
    localDisposed    localStatus = "disposed"
)

type LocalProvider struct {
    sm *StateMachine[localStatus]
}

func NewLocalProvider() *LocalProvider {
    p := &LocalProvider{}
    p.sm = NewStateMachine(localDownloading).
        Allow(localDownloading, localLoading, localFailed).
        Allow(localLoading, localStandby, localFailed).
        Allow(localStandby, localInferencing, localFailed, localDisposed).
        Allow(localInferencing, localStandby, localFailed).
        Allow(localFailed, localDisposed)
    return p
}

func (p *LocalProvider) Status() AgentStatus {
    switch p.sm.Current() {
    case localDownloading, localLoading:
        return AgentInitializing
    case localStandby:
        return AgentIdle
    case localInferencing:
        return AgentRunning
    case localFailed:
        return AgentError
    case localDisposed:
        return AgentClosed
    default:
        return AgentClosed
    }
}
```

### 核心调度层：只与统一抽象交互

```go
// daemon/internal/agent/manager.go

func (m *Manager) StartAgent(id string) error {
    agent := m.agents[id]
    // 只关心统一的 AgentStatus，不感知 Provider 内部差异
    if err := agent.stateMachine.Transition(AgentRunning); err != nil {
        return fmt.Errorf("cannot start agent %s: %w", id, err)
    }
    // ...
}
```

### 状态机带来的收益

| 维度 | 无状态机 | 有状态机 |
|------|---------|---------|
| **非法转移** | 运行时可能产生 bug | `Transition()` 返回错误，提前暴露 |
| **副作用管理** | 散落在各调用点 | 集中在 `OnEnter`/`OnExit` 注册 |
| **状态可视化** | 需要读代码推断 | 转移规则是显式声明的数据 |
| **新增 Provider** | 人工保证映射正确 | 私有状态机 + 统一输出，框架保障 |
| **测试验证** | 覆盖所有分支困难 | 可单独测试状态机转移矩阵 |

### OCP 验证

- **状态机框架**（`protocol/statemachine.go`）：通用代码，**封闭**
- **Agent 转移规则**（`daemon/internal/agent/statemachine.go`）：Agent 专属配置，其他实体不依赖
- **Provider 私有状态机**（`daemon/internal/provider/*/provider.go`）：各自独立配置，**开放扩展**
- **核心调度层**：只操作 `AgentStatus`，不感知任何 Provider 差异，**封闭**

新增一个 Provider：新增一个实现文件，定义自己的 `localStatus` 和 `StateMachine[localStatus]`，实现 `Status() AgentStatus`。零修改框架、零修改核心调度、零修改前端。

---

## 重构验证结果

| 检查项 | 结果 |
|-------|------|
| `protocol` 测试（race） | ✅ ok (1.610s) |
| `daemon/internal/agent` 测试（race） | ✅ ok (80.229s) |
| `daemon` 全包测试（race） | ✅ 16/16 包通过 |
| `cli` 全包测试（race） | ✅ 4/4 包通过 |
| `relay-go` 全包测试（race） | ✅ 4/4 包通过 |
| `app-bridge` 测试 | ✅ 32 passed |
| `app` 测试 | ✅ 207 files, 1307 passed |
| `expo lint` | ✅ 通过 |
| `tsc --noEmit` | 无新增类型错误 |
| JSON 序列化不变 | ✅ AgentSnapshotPayload.Status / ProviderSnapshotEntry.Status |
| 全模块 `go build` | ✅ protocol + daemon + cli |

---

*文档更新时间：2026-05-31*
*重构执行时间：TDD 5 步，全量测试通过*
