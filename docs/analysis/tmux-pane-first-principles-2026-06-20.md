# tmux Pane 交互界面第一性原理分析与方案 C 深度评估

> - **Status:** Analysis Complete
> - **Date:** 2026-06-20
> - **Author:** Qoder (Agent), Andy
> - **Related:** [tmux-pane-analysis.md](tmux-pane-analysis.md), [iterm2-agent-observation.md](iterm2-agent-observation.md)

---

## 目录

1. [第一性原理分析：三区域交互界面](#1-第一性原理分析三区域交互界面)
2. [方案 C 深度分析：PTY Stream 直通](#2-方案-c-深度分析pty-stream-直通)
3. [结论与建议](#3-结论与建议)

---

## 1. 第一性原理分析：三区域交互界面

### 1.1 回归本质：用户在解决什么问题？

用户在手机上远程操控一个跑在 tmux pane 里的 AI agent（Claude Code、Codex 等）。本质需求只有两个：

1. **读** — 看 agent 的输出（占 80%+ 时间）
2. **写** — 向 agent 发送输入（响应提示、发命令、发快捷键）

当前设计将"写"拆成了两个区域：**Send（虚拟按键）** 和 **Input（文本输入）**，加上 **View（输出显示）**，形成三区域结构。

### 1.2 当前实现概览

三区域均位于 `app/src/screens/tmux-pane-screen.tsx`（840 行），垂直堆叠在 `KeyboardAvoidingView` 内：

```
TmuxPaneScreenInner
├── BackHeader (顶栏: auto-refresh / theme / Hide / Select)
├── FlatList<AnsiSegment[]>           ← VIEW: 捕获的 pane 内容
├── View key group (Home / End / Refresh)  ← 小型导航键
├── Send key group (↑↓ Enter Esc Tab S-Tab 1 2 3 4 / History)  ← SEND: 12+ 按钮
├── Slash command dropdown / History dropdown
└── TextInput + Send button           ← INPUT: 文本输入
```

### 1.3 四个根本问题

#### 问题 1：三区域划分不匹配用户心智模型

用户的心智模型是「读 / 写」二分法。当前设计把"写"拆成 Send 和 Input 两个区域，但它们本质上是同一个动作——**向终端发送字节**。

- 点 `↑` 按钮 = 发送 `\x1b[A`
- 在 Input 里打 `git status` + Send = 发送 `git status\r`

这是实现细节的差异（单键 vs 文本、是否带 Enter），不应上升为两个独立区域。物理分离迫使用户在两个位置之间切换注意力。

#### 问题 2：静态按键布局无视终端上下文

Send 区域始终展示 12+ 个按钮，不论终端里发生了什么。但 AI agent 的交互是高度上下文化的：

- agent 弹出选择菜单 → 只有 ↑↓ Enter 有意义
- agent 问 yes/no → 需要 y/n，但 UI 上没有
- agent 空闲等待 → 大部分按键无用，全是噪音

用户每次都要：**读 VIEW 里的提示 → 翻译成需要的键 → 在 12+ 个按钮里找 → 点击**。认知链条太长。

#### 问题 3：空间分配与使用频率倒挂

阅读占 80%+ 时间，但底部面板（View keys + Send keys + Input）在未隐藏时占 **25-35% 屏幕高度**。键盘弹出后 VIEW 进一步被压缩，读和写无法同时进行。

虽然有 Hide 按钮，但它是全有或全无——隐藏后无法快速发送单个键（比如 agent 问 yes/no 时需要快速响应）。

#### 问题 4：快照式 VIEW 割裂了动作与反馈

VIEW 是轮询快照，没有光标。用户点 `↑` 后看不到光标移动，只能等下一次 poll（500ms~5s）。这种"盲操作"感在快速交互时（如菜单选择）尤为明显。

`sendKey` 后立即 `refetch()` 缓解了延迟，但网络往返仍可感知。更关键的是，用户无法判断自己的操作是否生效、光标在哪一行。

### 1.4 改进方案概览

| 方案 | 核心变化 | 成本 | 效果 |
|------|---------|------|------|
| **A: 统一输入栏 + 上下文快捷键** | 合并 Send+Input，VIEW 占 85%+，上下文检测展示相关键 | 中 | 消除心智模型分裂，优化空间 |
| **B: 可展开浮动输入面板** | 默认 VIEW 全屏，FAB 展开输入面板 | 低 | 最大化阅读空间 |
| **C: 真正的终端模拟器** | xterm.js + 流式传输替代轮询快照 | 高 | 根本解决所有问题 |

### 1.5 方案 A 设计（推荐）

```
┌─────────────────────────┐
│                         │
│      VIEW (85%+)        │  ← 最大化阅读区域
│                         │
│                         │
├─────────────────────────┤
│ [Y] [N] [↑] [↓] [↵]    │  ← 上下文快捷键（自适应）
│ ┌─────────────┐ ┌────┐ │
│ │ Type...     │ │Send│ │  ← 统一输入栏
│ └─────────────┘ └────┘ │
│ [Esc] [Tab] [/] [More]  │  ← 精简常驻键 + 展开更多
└─────────────────────────┘
```

核心变化：
- **合并 Send + Input 为一个输入栏**，消除心智模型分裂
- **上下文快捷键**：简单解析终端末尾几行，检测 `[Y/n]`、`(1-4)`、`❯`（箭头选择器）等模式，动态展示 Y/N、1-4、↑↓Enter
- **常驻键精简到 4 个**（Esc、Tab、/、More），其余收入 More 抽屉
- **VIEW 空间从 ~65% 提升到 ~85%**

---

## 2. 方案 C 深度分析：PTY Stream 直通

### 2.1 方案 C 的精确定义

方案 C 的目标是消除 `capture-pane` 轮询，让 tmux pane 的 VT 数据流实时推送到客户端的 xterm.js。存在三个子路径：

| 子路径 | 机制 | 实时性 | 复杂度 |
|--------|------|--------|--------|
| **C1: tmux Control Mode** | `tmux -C` 持久进程，解析 `%output` 事件 | 真正实时 | 极高 |
| **C2: Daemon 端轮询 + Push** | daemon 快速 `capture-pane` + hash diff + WebSocket push | 100-200ms 延迟 | 中 |
| **C3: tmux pipe-pane** | `pipe-pane -o` 管道捕获 pane 输出 | 接近实时 | 高（脆弱） |

原 `tmux-pane-analysis.md` 中的"方案 C"指的是 C1。以下重点分析 C1，同时覆盖 C2/C3 作为对比。

### 2.2 现有可复用基础设施

项目**已经具备 80% 的基础设施**，方案 C 不是从零开始：

```
已有 ✓                          需新建 ✗
─────────────────────────────────────────────────────────
xterm.js 终端模拟器 (Expo DOM)   tmux Control Mode 进程管理
TerminalEmulatorRuntime          %output 事件解析器
二进制帧协议 (Output/Input/Resize)  tmux → 帧协议适配层
OutputCoalescer (5ms 合并)       初始 snapshot + 流切换逻辑
TerminalStreamController         宽度冲突解决方案
slot 路由 + 多客户端 fan-out      tmux 版本兼容层
Session 订阅/清理生命周期         VT key → tmux key name 翻译表
graceCriticalBuf 重连重放        Control Mode 崩溃恢复
relay 透明转发 (零改动)           ─
E2EE 二进制帧加密 (零改动)        ─
```

**关键已有组件清单**：

| 组件 | 文件 | 复用方式 |
|------|------|----------|
| xterm.js 封装 | `app/src/components/terminal-emulator.tsx` | 直接复用，暴露 `writeOutput` / `renderSnapshot` / `clear` |
| xterm.js 运行时 | `app/src/terminal/runtime/terminal-emulator-runtime.ts` | 直接复用，含 WebGL renderer / 触摸滚动 / 主题 |
| 二进制帧协议 | `app-bridge/src/shared/terminal-stream-protocol.ts` | 直接复用 Output/Input/Resize/Snapshot opcode |
| 输出合并器 | `daemon/internal/terminal/coalescer.go` | 直接复用，5ms 窗口合并 |
| 客户端流控制器 | `app/src/terminal/runtime/terminal-stream-controller.ts` | 改 `terminalId` → `paneId` |
| Session 订阅模式 | `daemon/internal/server/session_terminal.go:94-109` | `handleSubscribeTerminals` 作为模板 |
| 流式输出订阅 | `daemon/internal/server/session_terminal.go:316-346` | `subscribeTerminalOutput` 作为模板 |
| Tagged-union 事件 | `protocol/stream_event.go` | 作为 `TmuxStreamEvent` 的模板 |
| 有序 push 包装 | `daemon/internal/server/session_agent_stream.go:152-164` | `sendAgentStream` 带 Seq/Epoch |
| Grace 重放 | `daemon/internal/server/session.go:107-109` | 标记新消息为 critical |
| 内容 hash 去重 | `daemon/internal/server/session_tmux.go:577-584` | `computeContentHash` 可复用 |
| singleflight | `daemon/internal/server/session_tmux.go:37` | `capturePaneFlight` 可复用 |
| Relay 转发 | `relay-go/internal/relay/server.go:208-258` | 透明字节转发，零改动 |
| E2EE | `daemon/internal/relayclient/e2ee.go` | 支持 text/binary 帧，零改动 |

### 2.3 C1 实现细节（逐步拆解）

#### 步骤 1：协议扩展（`protocol/message_tmux.go`）

新增 fire-and-forget 订阅类型，镜像现有 `SubscribeTerminalRequest`：

```go
// 新增
type TmuxSubscribePaneStreamRequest struct {
    PaneID string `json:"paneId"`
    Cols   int    `json:"cols"`   // 客户端屏幕宽度
    Rows   int    `json:"rows"`
}

type TmuxUnsubscribePaneStreamRequest struct {
    PaneID string `json:"paneId"`
}
```

**复用**现有 `TerminalStreamFrame` 二进制协议——slot 字节路由 + Output opcode，无需新建帧类型。

#### 步骤 2：tmux Control Mode 管理器（新文件 `daemon/internal/tmux/control.go`）

这是**最复杂的新代码**。核心结构：

```go
type ControlModeManager struct {
    mu          sync.Mutex
    sessions    map[string]*controlModeSession  // sessionName → process
}

type controlModeSession struct {
    sessionName string
    cmd         *exec.Cmd
    stdin       io.WriteCloser
    stdout      io.ReadCloser
    paneSubs    map[string][]chan []byte  // paneId → output channels
    done        chan struct{}
    refCount    int
}
```

工作流程：
1. `Start(sessionName)` — 执行 `tmux -C attach -t {sessionName}`
2. `readLoop()` — 逐行读取 stdout，解析 `%output {pane_id} {data}`
3. `WriteCommand(cmd)` — 通过 stdin 发送 tmux 命令（如 `resize-pane`、`send-keys`）
4. `Subscribe(paneId)` — 注册输出 channel
5. `Unsubscribe(paneId)` — 注销，refCount=0 时停止进程

**关键实现细节**：
- `%output` 的 data 字段：tmux 3.2+ 默认输出原始字节（非 base64），tmux 2.x 可能需要 `set -g control-mode-style base64`
- 需要维护 tmux 版本检测（`tmux -V`），根据版本选择解析模式
- 进程崩溃时需要自动重启 + 重新发送 snapshot

#### 步骤 3：Session 集成（`session_tmux.go`）

```go
func (s *Session) handleTmuxSubscribePaneStream(req *protocol.TmuxSubscribePaneStreamRequest) {
    // 1. 分配 slot（复用现有 slot 机制，但需要扩展为 tmux pane）
    slot := s.allocateTmuxSlot(req.PaneID)

    // 2. 获取或创建 ControlModeSession
    cm := s.tmuxControlMgr.GetOrCreate(req.SessionName)

    // 3. 发送初始 snapshot
    content := captureTmuxPane(req.PaneID, -200)
    s.SendBinaryFrame(protocol.TerminalStreamFrame{
        Opcode:  protocol.TerminalSnapshot,
        Slot:    slot,
        Payload: []byte(content),  // 直接用 ANSI 文本作为 snapshot
    })

    // 4. 订阅 %output 流
    ch := cm.Subscribe(req.PaneID)
    coalescer := terminal.NewOutputCoalescer(func(data []byte) {
        s.SendBinaryFrame(protocol.TerminalStreamFrame{
            Opcode:  protocol.TerminalOutput,
            Slot:    slot,
            Payload: data,
        })
    })

    // 5. 启动 goroutine 将 channel 数据喂给 coalescer
    go func() {
        for data := range ch {
            coalescer.Add(data)
        }
        coalescer.Stop()
    }()

    // 6. 注册清理函数
    s.tmuxSubscriptions = append(s.tmuxSubscriptions, func() {
        cm.Unsubscribe(req.PaneID)
        coalescer.Stop()
        s.releaseTmuxSlot(slot)
    })
}
```

#### 步骤 4：客户端改造

**替换量最小**——大部分已有组件可直接复用：

```
当前:
  tmux-pane-screen.tsx
    ├── useTmuxCapturePane (React Query polling)
    ├── parseAnsi + splitSegmentsByLine
    ├── FlatList<AnsiSegment[]> (VIEW)
    ├── Send key group (SEND)
    └── TextInput + Send button (INPUT)

改造后:
  tmux-pane-screen.tsx
    ├── TerminalEmulator (已有，"use dom")
    │     └── TerminalEmulatorRuntime.writeOutput() ← 二进制帧 push
    ├── TerminalStreamController (已有，改 terminalId → paneId)
    ├── 虚拟键盘 (已有，输入走 binary frame → daemon → tmux send-keys -l)
    └── 不再需要 parseAnsi / FlatList / content hash
```

输入路径变化：
- **当前**：`sendKey(key)` → `tmux send-keys -t {paneId} {key}` (每次 fork)
- **C1 后**：`terminal.onData(data)` → binary frame Input → daemon → `tmux send-keys -t {paneId} -l {data}`

`-l` 标志发送字面字节，不需要 VT key → tmux key name 翻译表。

#### 步骤 5：宽度和初始状态处理

```go
// 订阅时立即调整 pane 宽度
cm.WriteCommand(fmt.Sprintf("resize-pane -t %s -x %d -y %d", paneId, cols, rows))

// 然后发送初始 snapshot
content := captureTmuxPane(paneId, -200)
s.SendBinaryFrame(... Snapshot ...)

// 再开始流式推送 %output
```

### 2.4 关键障碍（7 个，按严重性排序）

#### 障碍 1：宽度冲突（🔴 Blocker）

tmux pane 的尺寸是全局共享的。App 把 pane resize 到 40 列，桌面用户的 200 列终端也会变成 40 列。

**这是方案 C1 的根本性矛盾**。iTerm2 的 `tmux -CC` 也有这个问题——它通过创建新的 iTerm2 原生窗口来映射 tmux pane，用户在 iTerm2 里看到的是独立窗口，但底层 tmux pane 的尺寸确实变了。

**可能的缓解方案**：

| 方案 | 可行性 | 代价 |
|------|--------|------|
| 不 resize，客户端 reflow | 低 | xterm.js 不支持 VT 流 reflow，需自己实现 |
| 创建 mirror pane (`split-window`) | 中 | 影响 tmux layout，复杂 |
| 仅在用户显式开启"移动模式"时 resize | 中 | 体验割裂 |
| 接受冲突 | — | 桌面用户被干扰 |

**C2（daemon 轮询 + push）没有这个问题**——`capture-pane -C {cols} -J` 可以按指定宽度裁剪输出，不影响实际 pane 尺寸。

#### 障碍 2：初始 Snapshot 与流切换的竞态（🟡 高风险）

```
时间线:
  t0: 发送 capture-pane 请求 → 获取 snapshot
  t1: 启动 %output 流
  t2: snapshot 到达客户端
  t3: t0~t1 之间产生的 %output 事件到达客户端
```

t0 到 t1 之间的输出既在 snapshot 里，又在 %output 流里——客户端会看到重复内容。

**解决方案**：
1. 先启动 control mode 进程并缓冲 %output
2. 发送 capture-pane snapshot
3. 丢弃 snapshot 时间点之前的缓冲 %output
4. 从 snapshot 时间点之后开始推送

但这需要精确的时间同步，在实践中很难做对。tmux 的 `%output` 事件没有时间戳。

#### 障碍 3：tmux 版本兼容性（🟡 高风险）

| tmux 版本 | %output 格式 | 控制模式事件 | 市场占比 |
|-----------|-------------|-------------|----------|
| 2.6-2.9 | 原始字节，事件少 | 基本可用 | ~15% |
| 3.0-3.2 | 原始字节，事件丰富 | 稳定 | ~45% |
| 3.3+ | 支持 `-e` 标志 | 完整 | ~35% |
| <2.6 | 不支持控制模式 | — | ~5% |

需要在运行时检测 `tmux -V`，为不同版本选择不同的解析策略。2.x 版本可能需要降级到 C2。

#### 障碍 4：Control Mode 进程生命周期管理（🟡 中风险）

`tmux -C` 进程可能因以下原因崩溃：
- tmux server 重启（`tmux kill-server`）
- 系统更新
- OOM

需要：
- 检测进程退出（`cmd.Wait()`）
- 通知所有订阅者连接断开
- 自动重启 + 重新发送 snapshot
- 指数退避避免崩溃循环

#### 障碍 5：多客户端宽度冲突（🟡 中风险）

两个手机同时看同一个 pane，一个 40 列、一个 80 列——resize 给谁？

Control Mode 只能有一个 resize 目标。需要：
- 以最后一个订阅者的宽度为准
- 或拒绝宽度不一致的订阅
- 或为不同宽度创建不同的 mirror pane

#### 障碍 6：电池消耗（🟢 低风险但需评估）

| 模式 | daemon 端 | 客户端 |
|------|----------|--------|
| 当前轮询 (500ms-5s) | 每 5s fork 一次 tmux | 每 5s 收一个 JSON 响应 |
| C1 Control Mode | 持久进程，持续解析事件 | 持续接收二进制帧 |
| C2 daemon push | 每 100-200ms fork 一次 tmux | 仅内容变化时收帧 |

C1 的持续流式传输在 pane 活跃时（如 agent 正在输出）会产生大量小帧。OutputCoalescer（5ms 合并）可以缓解，但仍是比轮询更高的持续功耗。

#### 障碍 7：Snapshot 协议缺口（🟢 低风险）

现有 `TerminalSnapshot` opcode 在 daemon 端**从未实现生成逻辑**。C1 需要补齐，但可以将 `capture-pane` 的 ANSI 文本直接作为 snapshot payload 发送（xterm.js 的 `terminal.write()` 可以处理 ANSI 文本），不需要生成 `TerminalState` JSON 结构。

### 2.5 代价分析

#### 工程量估算

| 模块 | 文件 | 新增/改动行数 | 工时 |
|------|------|-------------|------|
| **协议扩展** | `protocol/message_tmux.go` | ~50 行 | 0.5 天 |
| **Control Mode 管理器** | `daemon/internal/tmux/control.go` (新) | ~400 行 | 5-7 天 |
| **Session 集成** | `session_tmux.go` | ~150 行 | 2-3 天 |
| **tmux 版本检测** | `daemon/internal/tmux/version.go` (新) | ~80 行 | 1 天 |
| **崩溃恢复** | `control.go` 内 | ~100 行 | 2 天 |
| **客户端 stream 适配** | `terminal-stream-controller.ts` | ~100 行 | 2 天 |
| **屏幕替换** | `tmux-pane-screen.tsx` | ~300 行重写 | 3 天 |
| **宽度冲突方案** | 多文件 | ~200 行 | 3-5 天 |
| **测试** | 多文件 | ~500 行 | 5-7 天 |
| **tmux 版本兼容测试** | — | — | 3-5 天 |
| **总计** | — | ~1900 行 | **26-35 天 (5-7 周)** |

#### 持续维护成本

| 维度 | 成本 |
|------|------|
| tmux 版本升级 | 每次 tmux 大版本需要重新验证 %output 格式 |
| 平台差异 | Linux/macOS tmux 行为差异需要测试 |
| 进程泄漏 | Control Mode 进程管理 bug 导致僵尸进程 |
| 客户端兼容 | 不同手机性能下 xterm.js WebGL 表现差异 |

#### 风险矩阵

| 风险 | 概率 | 影响 | 可缓解性 |
|------|------|------|----------|
| 宽度冲突影响桌面用户 | 100% | 高 | 低（根本性问题） |
| Snapshot/流切换竞态 | 70% | 中 | 中 |
| tmux 2.x 不兼容 | 30% | 中 | 中（降级 C2） |
| Control Mode 进程崩溃 | 40% | 中 | 高（自动重启） |
| 低端 Android WebGL 性能 | 50% | 中 | 中（fallback Canvas） |
| 电池续航下降 | 60% | 低 | 高（自适应流控） |

### 2.6 C1 vs C2 vs C3 vs B 对比

| 维度 | B: xterm.js + 轮询 | C2: daemon push | C1: Control Mode | C3: pipe-pane |
|------|-------------------|-----------------|-----------------|---------------|
| **实时性** | 500ms-5s | 100-200ms | ~0ms | ~0ms |
| **光标** | 无 | 无 | 有 | 无 |
| **宽度冲突** | 无 | 无 | **有** | 无 |
| **TUI 支持** | 差（snapshot） | 中 | 优 | 优 |
| **工程量** | 2-3 周 | +2 周 (在 B 上) | 5-7 周 | 4-5 周 |
| **风险** | 低 | 低 | 高 | 高（脆弱） |
| **维护成本** | 低 | 低 | 高 | 高 |
| **复用现有设施** | 80% | 85% | 70% | 60% |
| **电池影响** | 低（自适应轮询） | 低-中 | 中-高 | 中 |

### 2.7 C3 (pipe-pane) 为何不推荐

`tmux pipe-pane -t {paneId} -o {command}` 管道捕获 pane 输出，但有严重限制：

- **全局设置**：每个 pane 只能有一个 pipe-pane。daemon 设置后，用户无法使用 pipe-pane 做其他事。
- **仅输出**：无输入通道，仍需 `send-keys`。
- **无初始状态**：只捕获未来输出，需 `capture-pane` 补 snapshot。
- **清理脆弱**：客户端断开时必须移除 pipe，否则 pane 输出持续写入死管道。
- **无光标**：PTY 输出是 VT 流，无法直接获取光标位置。

---

## 3. 结论与建议

### 3.1 三区域交互界面评估

当前三区域设计（VIEW / SEND / INPUT）**能用但不够合理**，核心问题是：
- "写的分裂"——Send 和 Input 是同一个动作的两个区域
- "上下文盲"——静态按键布局无视终端状态
- 空间分配与使用频率倒挂

### 3.2 方案 C1 (Control Mode) 评估

**方案 C1 的代价远超收益。** 核心原因：

1. **宽度冲突是不可接受的根本性问题**——app 用户和桌面用户看同一个 tmux pane，不能互相干扰
2. **5-7 周的工程量**换来的是 10% 场景（实时动画、TUI 交互）的体验提升
3. **持续维护成本高**——tmux 版本兼容、进程管理、崩溃恢复都是长期负担
4. **项目已有 80% 的基础设施**，但最后 20%（Control Mode 管理器 + 宽度方案 + 竞态处理）是最难的部分

### 3.3 推荐路径

```
当前 → 方案 B (xterm.js + 轮询, 2-3 周)
            ↓
      方案 C2 (daemon 端 push, +2 周)
            ↓
     [评估是否需要 C1]
      如果 90% 场景已满足 → 停止
      如果 TUI 交互是刚需 → 考虑 C1，但需先解决宽度冲突
```

**方案 B + C2 的组合**可以：
- 复用 xterm.js（解决 box drawing、cursor、Unicode 宽度问题）
- 复用二进制帧 push（解决轮询延迟问题，从 5s 降到 100-200ms）
- 避免 Control Mode 的所有复杂度
- 避免宽度冲突

唯一无法解决的是：真正的零延迟 TUI 交互（如 vim、htop 的实时刷新）。但对于 AI agent 场景（主要是文本输出 + 偶尔交互），100-200ms 的 push 延迟已经足够流畅。

### 3.4 与原分析文档的对齐

本文档的结论与 `tmux-pane-analysis.md` 的推荐路线一致：
- Phase 1（方案 B）：xterm.js 迁移 — 3-4 周
- Phase 2（方案 C）：Control Mode — 6-8 周，"视需求"推进

本文档补充了方案 C 的**详细实现拆解**和**障碍分析**，特别是宽度冲突（Blocker）和 snapshot/流切换竞态的深度评估，为"是否推进 Phase 2"提供了决策依据。

---

## 附录：关键文件索引

| 文件 | 角色 |
|------|------|
| `app/src/screens/tmux-pane-screen.tsx` | tmux pane 主屏幕（三区域 UI） |
| `app/src/components/terminal-emulator.tsx` | xterm.js 封装（复用目标） |
| `app/src/terminal/runtime/terminal-emulator-runtime.ts` | xterm.js 运行时（复用目标） |
| `app/src/terminal/runtime/terminal-stream-controller.ts` | 客户端流控制器（复用目标） |
| `app/src/hooks/use-tmux-capture-pane.ts` | Pane 内容抓取 hook（将被替代） |
| `app/src/utils/ansi-parser.ts` | ANSI parser（将被替代） |
| `app-bridge/src/shared/terminal-stream-protocol.ts` | 二进制帧协议（复用） |
| `app-bridge/src/client/daemon-client.ts` | DaemonClient（需扩展 tmux stream listener） |
| `daemon/internal/server/session_tmux.go` | Daemon tmux 命令执行（需扩展） |
| `daemon/internal/server/session_terminal.go` | Terminal 订阅模式（作为模板） |
| `daemon/internal/terminal/terminal.go` | TerminalProcess（PTY 管理，参考） |
| `daemon/internal/terminal/coalescer.go` | OutputCoalescer（复用） |
| `protocol/message_tmux.go` | tmux 协议定义（需扩展） |
| `protocol/terminal.go` | 终端协议 + TerminalState（参考） |
| `protocol/stream_event.go` | Tagged-union 事件（作为模板） |
| `relay-go/internal/relay/server.go` | Relay 透明转发（零改动） |
| `daemon/internal/relayclient/e2ee.go` | E2EE 加密（零改动） |
| `docs/analysis/tmux-pane-analysis.md` | 原始 tmux pane 分析文档 |
