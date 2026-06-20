# Tmux Pane 客户端终端模拟器路径分析（第一性原理）

**Date:** 2026-06-20
**Status:** Analysis Complete
**Priority:** High
**Related:** [tmux-pane-analysis.md](tmux-pane-analysis.md), [tmux-pane-content-loading.md](../architecture/tmux-pane-content-loading.md), [architecture-first-principles-review-2026-06-18.md](architecture-first-principles-review-2026-06-18.md)

---

## Executive Summary

当前 Solo App/Web 端的 tmux pane 体验受限于一个根本的架构错配：tmux server 输出的是**基于 cell grid 的增量 VT 流**，而 App 端却用 **React Native `<Text>` 树去渲染完整的 ANSI 字符串快照**。v0.4.1 的三层防抖只是在这一低效架构上打补丁，无法解决动态 TUI、spinner、vim 等场景的抖动和延迟。

“在 app/web 端用 tmux 模拟器完成 tmux pane 工作”的正确理解，**不是在客户端运行 tmux server，而是在客户端放置一个真正的 terminal emulator（xterm.js）来渲染 tmux pane**。Solo 的 workspace terminal 已经验证了这条路径：`

**最佳实施路径：**

1. **Phase 1（立即执行）**：复用现有 `TerminalEmulator` / `TerminalEmulatorRuntime`，把 `TmuxPaneScreen` 的 ANSI Text 渲染替换为 xterm.js；daemon 侧给 `capture-pane` 增加 `-J` 合并 tmux 自动换行，并把客户端 `cols` 透传预留，用于未来服务端按目标宽度重排。
2. **Phase 2（按需）**：引入 tmux Control Mode (`tmux -C`)，把 VT 增量流直通到客户端 xterm.js，实现真正的实时无抖动体验。

---

## 1. 当前状态

### 1.1 数据流

```
host tmux server
  └──► daemon: tmux capture-pane -t %N -p -e -J -S -200
         └──► WebSocket: TmuxCapturePaneResponse { content }
                └──► App: useTmuxCapturePane (5s poll)
                       └──► parseAnsi(content)
                              └──► AnsiTextContent → 数千 <Text> nodes
```

### 1.2 核心组件

| 层级 | 文件 | 职责 |
|---|---|---|
| App | `app/src/screens/tmux-pane-screen.tsx` | 主屏幕、输入、滚动 |
| App | `app/src/hooks/use-tmux-capture-pane.ts` | 轮询、去重、懒加载历史 |
| App | `app/src/utils/ansi-parser.ts` | ANSI 字符串解析 |
| App | `app/src/components/ansi-text-renderer.tsx` | RN `<Text>` 树渲染 |
| Daemon | `daemon/internal/server/session_tmux.go` | `captureTmuxPane`、`sendKeysToTmuxPane` |

### 1.3 已做优化

- content 引用稳定化（`prevResultRef` 字节去重）。
- pagination-only `isLoadingMore`。
- `React.memo` 包裹 `AnsiTextContent`。
- 自适应轮询 200ms → 1s → 5s。

这些修复把 tmux pane 从“经常崩溃”变成了“静态可用但动态卡顿”。

---

## 2. 第一性原理分析

### 2.1 tmux pane 的本质

- 一个由 tmux server 维护的**字符单元格网格（cell grid）**。
- 更新是**增量 VT 转义序列流**：光标移动、字符写入、属性切换、清屏。
- 稳定、无抖动的关键是：只重绘变化的 cell，滚动位置由 grid 内部维护。

### 2.2 React Native `<Text>` 树的本质

- 声明式 UI，任何内容变化都要经历 **reconcile → layout → paint**。
- 把 5000 行终端内容拆成数千个 `<Text>` 节点，每次 poll 全量重建，天然与 cell grid 的增量模型冲突。
- 缺少：精确光标、box drawing、CJK 宽度计算、true color 稳定渲染。

### 2.3 网络传输的本质

| 模型 | 特征 | 适用场景 |
|---|---|---|
| **Snapshot** | 周期性拉取完整状态，简单省电，延迟高 | 看日志、agent 输出、读历史 |
| **Stream** | 持续传输增量 VT 事件，实时无抖动 | TUI、vim、spinner、实时测试 |

### 2.4 输入的本质

- 用户按键 → 编码成 VT 输入序列 → 注入 pane PTY。
- 通过 daemon 的 `tmux send-keys` 即可完成，**不一定需要 stream**。

---

## 3. 为什么客户端不能替代 tmux server

| 能力 | 客户端能否做 | 原因 |
|---|---|---|
| session/window/pane 状态管理 | ❌ | 需要主机进程树和 PTY 生命周期 |
| 子进程树维护 | ❌ | 依赖内核 PID/进程关系 |
| PTY 分配 | ❌ | 需要 `/dev/ptmx` 等系统资源 |
| **cell grid 渲染** | ✅ | 纯 UI 问题，可在 WebView/DOM 完成 |
| **键盘输入编码** | ✅ | 可映射为 `tmux send-keys` |

所以“tmux 模拟器”在 app/web 端只能是**终端模拟器（terminal emulator）**，而不是**tmux server 模拟器**。

---

## 4. 可选路径对比

| 路径 | 描述 | 接近原生 tmux | 工作量 | 风险 | 推荐度 |
|---|---|---|---|---|---|
| A. 继续优化 ANSI Text Renderer | 修补 parseAnsi、box drawing、光标、LRU cache | ⭐⭐ | 低 | 低 | 短期补丁 |
| **B. 客户端 xterm.js + capture-pane 快照** | 复用 `TerminalEmulator`，把 capture-pane 输出喂给 xterm.js | ⭐⭐⭐⭐ | 中 | 中 | **⭐ 最推荐** |
| C. 客户端 xterm.js + tmux Control Mode 流 | `tmux -C` 输出 `%output` 事件，直通 xterm.js | ⭐⭐⭐⭐⭐ | 高 | 高 | 长期目标 |
| D. Daemon 端 cell grid diff | 在 daemon 里维护 grid，只发变化 cell | ⭐⭐⭐⭐ | 中高 | 中高 | 重复造轮子，不推荐 |
| E. WebAssembly tmux client/server | 把 tmux 源码编译到 WASM | ⭐⭐⭐⭐⭐ | 极高 | 极高 | 不现实 |

---

## 5. 推荐方案：两阶段实施

Solo 已经具备方案 B 所需的全部基础设施：

- `app/src/components/terminal-emulator.tsx` + `app/src/terminal/runtime/terminal-emulator-runtime.ts`：xterm.js + WebGL + Unicode11 + touch scroll。
- `app/src/components/terminal-pane.tsx`：workspace terminal 已在生产环境验证 Expo DOM WebView 路径。
- `app-bridge/src/client/daemon-client.ts` 已提供 `tmuxCapturePane` / `tmuxSendKeys`。

### 5.1 Phase 1：xterm.js 替代 ANSI Text 渲染

#### 5.1.1 App 侧改动

新建 `TmuxXtermPane` 组件，复用 `TerminalEmulator`：

```tsx
<TerminalEmulator
  ref={emulatorRef}
  streamKey={`tmux:${serverId}:${paneId}`}
  xtermTheme={toXtermTheme(theme.colors.terminal)}
  onInput={handleTmuxInput}
  onResize={handleTmuxResize}
  // 可复用 TerminalPane 的虚拟按键条
/>
```

在 `TmuxPaneScreen` 中：

```ts
const { content } = useTmuxCapturePane(paneId);

useEffect(() => {
  emulatorRef.current?.clear();
  emulatorRef.current?.writeOutput(content);
}, [content]);
```

- xterm.js 自行维护 cell grid、光标、scrollback。
- 可移除 `parseAnsi`、`AnsiTextContent`、`detectColorsFromAnsi` 在 tmux pane 场景的使用。
- 虚拟按键条可复用 `TerminalPane` 的 modifier + 功能键实现。

#### 5.1.2 Daemon 侧改动

当前命令：

```go
tmux capture-pane -t {paneID} -p -e -S {startLine}
```

改为合并自动换行，让客户端 xterm.js 按自身列宽决定如何折行：

```go
tmux capture-pane -t {paneID} -p -e -J -S {startLine}
```

说明：

- `-J` 会把 tmux 因 pane 宽度不足而自动折行的逻辑行重新合并，避免主机 pane 宽度与客户端 xterm.js 宽度不一致时产生“双重换行”。
- tmux 的 `capture-pane` 不接受目标宽度参数，`-C` 是无参开关，因此不能直接用 `-C {cols}` 裁剪。
- `cols` 仍由前端 `onResize` 上报并在 `TmuxCapturePaneRequest` 中透传，预留未来做 ANSI-aware 服务端重排或分页裁剪。

#### 5.1.3 历史懒加载

当前 `scrollbackLines += 200`，最多 5000。xterm.js 下推荐：

- 每次加载更多历史时，用新的 `-S -{scrollbackLines}` 内容 `reset()` + `write()`。
- 先验证 5000 行在低端 Android 的耗时，必要时拆成 chunk write。

#### 5.1.4 主题桥接

`app/src/styles/terminal-themes.ts` 的 preset → `toXtermTheme()` → `ITheme`，workspace terminal 已经跑通。

---

### 5.2 Phase 2（按需）：tmux Control Mode 实时流

当 snapshot + xterm.js 仍无法满足实时 TUI/动画时，再引入：

```
tmux pane ──► tmux server ──► tmux -C (Control Mode)
                              │
                              ▼
                          Daemon
                              │
                              ▼
                          WebSocket stream
                              │
                              ▼
                          xterm.js (App/Web)
```

关键点：

- Daemon 为每个 pane/session 维护一个 `tmux -C` 连接。
- 订阅 `%output` 等事件，把 VT 数据直接透传给客户端 `TerminalEmulator.writeOutput()`。
- 输入可通过 control mode 的 `send-keys` 或保持现有 `tmuxSendKeys` RPC。
- 需要处理 tmux 2.x ~ 3.x 协议差异、连接重连、多 pane multiplexing。

---

## 6. 为什么不选其他路径

- **路径 A（继续优化 ANSI Text）**：治标不治本。RN Text 树与 cell grid 增量模型存在结构性冲突。
- **路径 D（Daemon cell grid diff）**：等于在 daemon 里重造半个 xterm.js，维护成本高，且客户端仍需要一个真正的 emulator。
- **路径 E（WASM tmux）**：tmux 源码依赖 POSIX PTY、procfs、大量系统调用，移植到浏览器/移动端得不偿失。

---

## 7. 风险与验证清单

| 风险点 | 缓解 / 验证 |
|---|---|
| Expo DOM WebView 低端 Android 性能 | 在 PLT140 等低端设备测试 5000 行 `reset+write` 帧率 |
| WebGL 不支持 | xterm.js 默认 DOM renderer 内建 fallback |
| 大 pane `reset()` 阻塞 | 测量 200/1000/5000 行耗时，必要时 chunk write |
| CJK / box drawing / true color | xterm.js + Unicode11Addon 已覆盖，需回归 |
| 输入延迟 | 保持 `tmuxSendKeys` RPC，键盘手感不会变差 |
| 电量 | snapshot 5s poll 仍保留，比 stream 更省电 |
| tmux Control Mode 兼容性 | Phase 2 再评估，先锁定最小支持版本 |

---

## 8. 结论

**最好的实施路径是：先“客户端 terminal emulator + 快照”，再“按需升级到 Control Mode stream”。**

- 不要在 app/web 端试图替代 tmux server。
- 把“终端模拟”这一层搬到客户端，复用已有的 `TerminalEmulator` + xterm.js。
- 保持 daemon 的 `tmux capture-pane` 和 `tmux send-keys` 作为数据和输入通道；`capture-pane` 增加 `-J` 合并自动换行，并透传客户端 `cols` 预留未来服务端按需重排。
- 这是业界验证过的通用架构：VS Code 终端、iTerm2 tmux integration、Termius 都遵循“真正的 terminal emulator 渲染 + 后端 tmux/PTY 驱动”。

该路径与 Solo 现有基础设施复用度最高、风险最可控，是当前阶段的最优解。
