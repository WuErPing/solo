# Tmux Pane 子系统分析

> 本文档合并了 tmux pane 相关的 jitter 修复、性能瓶颈分析和渲染优化方案。
>
> - **Status:** Analysis Complete
> - **Date:** 2026-06-09 (consolidated from 2026-06-07 ~ 2026-06-09 analyses)
> - **Author:** Andy, Kimi (Agent)
> - **Related:** [tmux-pane-content-loading.md](../architecture/tmux-pane-content-loading.md), [tmux-transport-disposed-race.md](tmux-transport-disposed-race.md)

---

## 1. 结论

**tmux pane 的性能和稳定性问题是核心架构问题，不是近期变更引起的。**

近期提交（2026-06-04 ~ 06-09）实际上在**修复**性能和稳定性问题，但修复治标不治本。架构层面存在根本性效率瓶颈：snapshot polling + React Native Text tree 模型与主机 tmux 的 cell-based incremental VT stream 模型存在架构级差距。

v0.4.1 通过三层防抖消除了静态场景下的 jitter，但动态内容（spinner、TUI、vim）的体验仍与主机有明显落差。

**推荐路线：**
- **Phase 1（1 个月内）**：将 tmux pane 迁移至已有成熟的 `TerminalEmulator`（xterm.js + Expo DOM）
- **Phase 2（视需求）**：引入 tmux Control Mode (`tmux -C`) 实现 PTY stream 直通

---

## 2. 近期变更回顾

| 提交 | 日期 | 性质 | 影响 |
|------|------|------|------|
| `7e87a06` 修复白屏崩溃 | 06-04 | 修复 | keepPreviousData, ErrorBoundary, sanitizeForNative |
| `335315c` 修复 disposed transport | 06-05 | 修复 | withLiveTmuxClient 重试机制 |
| `9b57b5c` 懒加载历史 | 06-06 | 功能 | scrollback 从 200→5000 行，增加了数据量 |
| `68c66e5` 终端主题和窗口列表 | 06-07 | 功能 | 增加了渲染复杂度 |
| `a1cc7bd` 修复 Android 5s 抖动 | 06-08 | 修复 | 自适应轮询 200ms/1s/5s, React.memo, content dedup |
| `4750a5f` UI 按钮分组 | 06-09 | 样式 | 无性能影响 |

这些变更让 tmux pane 从"经常崩溃"变成了"能用但慢"。

---

## 3. Jitter 根因分析

### 3.1 核心架构差距

| | 主机 tmux | App tmux pane |
|---|---|---|
| 渲染模型 | Cell-based grid, incremental updates | Full string → full React tree re-render |
| 数据流 | Real-time streaming | 5s polling with full content replacement |
| 滚动管理 | Terminal-internal cell grid | React Native ScrollView + setTimeout |
| 背景颜色 | Fixed | Dynamically detected from content |
| Box drawing | Rendered as-is | Stripped during ANSI parsing |
| 光标 | 精确位置 + 样式 + blink | **无** |
| 字符宽度 | `wcwidth` | 无计算 |

### 3.2 Jitter 来源

| 因素 | 文件 | 问题 |
|------|------|------|
| 全量内容替换 | `use-tmux-capture-pane.ts` | 5s 轮询替换整个 content string |
| 每次内容变化触发滚动 | `tmux-pane-screen.tsx` | `scrollToEnd({ animated: true })` 在每次 content 变化时触发 |
| 背景颜色从内容派生 | `tmux-pane-screen.tsx` | ANSI 颜色变化导致 ScrollView 背景闪烁 |
| ANSI 每次重新解析 | `tmux-pane-screen.tsx` | `parseAnsi(content)` 每次产生新 segment 数组引用 |
| `isLoadingMore` 闪烁 | `use-tmux-capture-pane.ts` | `isFetching && !!data` 每 5s 切换 → "Loading more..." 行挂载/卸载 → 内容高度脉冲 |
| 缺少 `React.memo` | `ansi-text-renderer.tsx` | stable props 不能阻止 re-render → 整棵 `<Text>` 树重新 reconcile |
| Box drawing 被 strip | `ansi-parser.ts` | 所有 Box Drawing / Braille 字符被丢弃 |

### 3.3 为什么主机 tmux 不 jitter

主机 tmux 使用 **cell-based rendering** — 维护内部字符网格及其样式，只重绘实际变化的 cell。ASCII 动画（spinner、进度条）更新特定 cell，不影响布局或滚动位置。

- 主机 tmux: 增量 cell 更新 → 只有变化的像素重绘
- App: 全量 content string → 全量 parse → 全量 React tree re-render → ScrollView relayout → 滚动位置调整

---

## 4. 四层架构瓶颈

```
tmux capture-pane (shell fork, 全量捕获)
  → 大字符串通过 WebSocket 传输
    → string === 逐字符比较 (dedup)
      → parseAnsi() 全量重解析 → 新 segments[] 引用
        → React.memo 失效 → 数千 <Text> 全量 re-layout
          → ScrollView re-render → 卡顿、内存飙升
```

每一层都在放大上一层的问题。终端内容越多，每层的开销越大，整体呈乘法关系。

### 第 1 层：轮询式全量捕获（最核心问题）

`daemon/internal/server/session_tmux.go:262` — 每次轮询执行 `tmux capture-pane`，返回完整终端内容字符串。即使终端只变了 1 个字符，也要传输和处理整个字符串（可达 5000 行 × 每行含大量 ANSI 转义码）。**没有增量机制。**

### 第 2 层：客户端全量 ANSI 解析

`app/src/utils/ansi-parser.ts:147` — `parseAnsi(content)` 在 `useMemo` 中以 `[content]` 为依赖运行。每次解析产生新的 `AnsiSegment[]` 数组引用，直接击穿下游 `React.memo`。

### 第 3 层：React Native 无虚拟化渲染

`app/src/components/ansi-text-renderer.tsx:14` — `AnsiTextContent` 把每个 segment 渲染为一个 `<Text>` 子节点。5000 行终端内容可能产生数千个 `<Text>` 元素，**全部同时挂载**。没有 `FlatList` 或任何窗口化机制。

### 第 4 层：string 全等比较做 dedup

`app/src/hooks/use-tmux-capture-pane.ts:87` — 用 `===` 比较新旧 content 字符串。对于带 ANSI 转义码的 5000 行文本，每 200ms 做一次逐字符比较。

---

## 5. 已实施的修复 (v0.4.1)

三层防抖修复，每层阻断一个独立的 re-render 路径。任何单层修复都不够——移除其中一层就会在设备上重新出现可见 jitter。

### Layer 1 — Content 引用稳定化

**文件**: `app/src/hooks/use-tmux-capture-pane.ts`

`queryFn` 在 payload 字节不变时返回同一个对象引用。React Query 的 `structuralSharing` 保持 `data` 不变，下游 `useMemo` 短路 `parseAnsi` 和 `terminalColors`。

```typescript
const prevResultRef = useRef<{ content: string; error: string | null } | null>(null);
queryFn: async () => {
  const payload = await withLiveTmuxClient(serverId, (c) =>
    c.tmuxCapturePane(paneId, -scrollbackLines),
  );
  const newContent = payload.content ?? "";
  if (prevResultRef.current && prevResultRef.current.content === newContent) {
    return prevResultRef.current;
  }
  const result = { content: newContent, error: payload.error ?? null };
  prevResultRef.current = result;
  return result;
},
```

### Layer 2 — Pagination-Only `isLoadingMore`

**文件**: `app/src/hooks/use-tmux-capture-pane.ts`

`isLoadingMore` 从 `isFetching && !!data` 改写为 `isPaginating` 状态，仅在 `loadMoreHistory()` 改变 `scrollbackLines` 时设置。轮询 refetch 不改变 `scrollbackLines`，所以 "Loading more history..." 行不会在 5s 周期上挂载/卸载。

### Layer 3 — `React.memo` on `AnsiTextContent`

**文件**: `app/src/components/ansi-text-renderer.tsx`

`React.memo` 使 stable `segments` / `terminalColors` / `style` props 短路整个 `<Text>` 子树 reconcile。即使屏幕因无关状态转换重新渲染，`<Text>` 树也不会被触碰。

### Layer 4 — 其他优化

| 优化 | 文件 | 效果 |
|------|------|------|
| Content dedup | `use-tmux-capture-pane.ts` | 字节不变时跳过下游 memo |
| Adaptive polling | `use-tmux-capture-pane.ts` | 空闲时降至 5s，节省电量 |
| Pagination-only loading | `use-tmux-capture-pane.ts` | 消除 5s poll 内容高度脉冲 |
| `React.memo` on `AnsiTextContent` | `ansi-text-renderer.tsx` | stable props 时跳过 Text tree reconcile |

### TDD 回归测试

| 测试 | 文件 | 保护层 |
|------|------|--------|
| `preserves content STRING identity across many polls with identical payload` | `use-tmux-capture-pane.test.ts` | Layer 1 — content 引用稳定性 |
| `resets dedup cache when paneId changes so new pane gets fresh content` | `use-tmux-capture-pane.test.ts` | Layer 1 — per-pane scoping |
| `does NOT set isLoadingMore during a poll refetch with unchanged scrollback` | `use-tmux-capture-pane.test.ts` | Layer 2 — pagination-only loading |
| `does NOT re-parse ANSI content when content reference is stable across re-renders` | `tmux-pane-screen.test.tsx` | Layer 1+2 — screen-level memo 短路 |
| `is wrapped in React.memo so stable props do not cause re-renders` | `ansi-text-renderer.test.tsx` | Layer 3 — 组件 bail-out |

### 验证结果

- 设备: PLT140 (Android)
- APK: `app-release.apk` built via `APP_VARIANT=production ./gradlew assembleRelease`
- 场景: tmux pane 静态 shell prompt, 无 ASCII 动画, auto-refresh on, 5s poll interval
- 60 秒观察: 无可见脉冲, 无滚动偏移, 无背景闪烁
- Debug overlay 确认: `ansi=1` (stable), 所有其他计数器在初始加载后稳定

---

## 6. 渲染优化方案

### 方案 A：ANSI Text Renderer 局部增强（短期，1-2 周）

在现有架构上打补丁：恢复 Box Drawing、Daemon width crop、Cursor 渲染、`parseAnsi` LRU 缓存、逐行 diff。

**优点**: 改动集中，风险低，不引入新依赖。
**缺点**: 天花板明显——仍是 snapshot polling，无法支持实时动画、复杂 TUI 交互。

### 方案 B：Tmux Pane 复用 xterm.js（中期，3-4 周）⭐ 推荐

将现有 `TerminalEmulator` 组件（workspace terminal 已使用）应用到 tmux pane。

Solo 已在 workspace 场景使用 **xterm.js v6** + **WebGL renderer**，通过 Expo `"use dom"` 在移动端 WebView 中运行。该基础设施已完全具备，只是未用于 tmux pane 场景。

**实施**:
1. 新建 `TmuxXtermPane`，复用 `TerminalEmulator` + `TerminalEmulatorRuntime`
2. Adaptor 层：每次 `capture-pane` 返回后 `terminal.reset(); terminal.write(captureOutput)`
3. 宽度适配：测量 DOM 容器宽度，Daemon 侧 `capture-pane -C {cols}`
4. Theme 桥接：`TERMINAL_THEME_PRESETS` → `ITheme`

**优点**: 一次解决 box drawing、cursor、true color、scroll 稳定性、Unicode 宽度所有问题。与现有 polling 模型兼容。
**缺点**: 大 pane 的 `reset()` + `write()` 有 CPU 成本；Expo DOM WebView 在低端 Android 设备上的性能需验证。

### 方案 C：PTY Stream 直通（长期，6-8 周）

引入 tmux Control Mode (`tmux -C`)，让 tmux pane 通过 PTY 实时流式传输。

```
当前: tmux pane ──► tmux server ──► capture-pane ──► ANSI snapshot ──► App
目标: tmux pane ──► tmux server ──► tmux -C (Control Mode) ──► VT stream ──► xterm.js
```

**优点**: 与主机 tmux 完全一致：实时、无 jitter、支持所有 VT 序列。
**缺点**: Daemon 大量新代码、tmux 版本兼容性风险、电池消耗。业界参考：iTerm2 的 tmux integration mode。

### 方案 D：Daemon-side Cell Grid Diff（创新/折中，4-6 周）

Daemon 内存中维护 pane cell grid，只发送 diff 到客户端。与 xterm.js 能力重复造轮子，不推荐。

### 综合评估

| 方案 | 效果接近主机 tmux | 工作量 | 技术风险 | 维护成本 | 推荐度 |
|------|------|------|------|------|------|
| A. ANSI Text 增强 | ⭐⭐ | 低 | 低 | 低 | 短期过渡 |
| **B. xterm.js for tmux pane** | ⭐⭐⭐⭐ | **中** | **中** | **低** | **⭐ 主推** |
| C. PTY Stream / Control Mode | ⭐⭐⭐⭐⭐ | 高 | 高 | 高 | 长期 |
| D. Daemon Cell Grid Diff | ⭐⭐⭐⭐ | 中高 | 中高 | 高 | 不推荐 |

---

## 7. 推荐路线图

### Phase 1（1 个月内）：方案 B — xterm.js 迁移

Solo 已 100% 具备技术条件（`TerminalEmulator` + `TerminalEmulatorRuntime` + Expo DOM 已成熟运行）。工作量可控：主要是 adaptor 层，不是从零开发。

```
TmuxPaneScreen
  │
  ▼
<TmuxXtermPane> (新组件，复用 TerminalEmulator)
  ├── props: streamKey, xtermTheme, initialCapture
  ├── useEffect: 监听 useTmuxCapturePane 的 content
  │     └── if content changed: runtime.reset(); runtime.write(content)
  ├── 复用 TerminalEmulatorRuntime: WebGL renderer, Touch scroll, Theme, Focus/Resize
  └── Input: 保留现有 send-keys + virtual key bar
```

Daemon 侧小改动：`capture-pane -C {cols}` 按手机屏幕宽度裁剪。

### Phase 2（视需求）：方案 C — Control Mode

仅 10% 场景需要真正实时流（跑测试、看动画）。90% 场景（看 agent 输出、读日志）snapshot + xterm.js 已足够。

---

## 8. 业界方案对标

| 产品 | Tmux 支持方式 | 渲染技术 | 效果 |
|------|------|------|------|
| **Termius** | SSH + tmux attach | 自研 native terminal emulator | 接近原生 |
| **Blink Shell** | 原生 mosh/ssh + tmux | `libxterm.js` fork | 支持完整 tmux |
| **iTerm2** | `tmux -CC` Control Mode | Native cell grid | 将 tmux pane 映射为 native tab |
| **VS Code** | 内置终端 + tmux | `xterm.js` + PTY stream | 与 Solo workspace terminal 相同架构 |
| **WezTerm** | `tmux` 协议原生支持 | GPU rendering | 直接解析 tmux 协议 |

**共性结论**：所有效果好的方案都有一个**真正的 terminal emulator**，而不是 ANSI→Text 的转换器。

---

## 9. 稳定性问题

| 问题 | 根因 | 修复状态 |
|------|------|----------|
| 白屏/崩溃 | 大量 `<Text>` 节点快速更新触发 RN 渲染管线超时 | `7e87a06` 已修复 |
| Transport disposed | 轮询期间 WebSocket 连接状态变化导致竞态 | `335315c` 已修复 |
| Android 5s 抖动 | 固定轮询 + isLoadingMore 误触发导致 UI 脉冲 | `a1cc7bd` 已修复 |

稳定性修复是有效的，但它们是在一个本质上低效的架构上打补丁。

---

## 10. 数据流全景

```
┌─────────────────────────────────────────────────────────────┐
│ Daemon (Go)                                                  │
│  tmux capture-pane -t %N -p -e -S -200                       │
│       ↓ exec.Command                                         │
│  captureTmuxPane() → raw ANSI string                         │
│       ↓ WebSocket                                            │
│  TmuxCapturePaneResponse { content, error }                  │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│ App Bridge (TypeScript)                                      │
│  DaemonClient.tmuxCapturePane(paneId, startLine)             │
│       ↓ withLiveTmuxClient (retry × 3, 150ms delay)         │
│  useTmuxCapturePane (TanStack Query)                         │
│       ↓ adaptive refetch: 200ms → 1s → 5s                   │
│       ↓ dedup: content === prevContent                       │
│  { content: string, isLoading, error }                       │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│ Screen (React Native)                                        │
│  TmuxPaneScreen                                              │
│       ↓ useMemo(() => parseAnsi(content), [content])         │
│  AnsiSegment[] → new reference every poll                    │
│       ↓ <AnsiTextContent segments={...} />                   │
│  React.memo → shallow compare → ALWAYS re-render             │
│       ↓ segments.map → <Text style={...}>{seg.text}</Text>  │
│ 数千 <Text> nodes in <ScrollView> (no virtualization)        │
└─────────────────────────────────────────────────────────────┘
```

---

## 11. On-Device 调试方法论

单元测试不足以定位 jitter。测试使用同步 mock client，`isFetching` 在同一个 React tick 内从 `true` 变为 `false`，中间渲染不可观察。设备上的可见症状（"每 5 秒某物脉冲"）需要运行时探针。

**关键教训**：在 React 组件树中插桩运行时诊断时，始终：
1. 在**模块作用域**定义探针包装器，使 React 将其视为稳定的组件类型
2. 用 `React.memo` 包装以匹配被测组件的优化路径
3. 否则探针本身会成为被测量行为的主要来源

---

## 12. Open Questions

1. **CJK 字符宽度**: 如何计算混合 CJK/ASCII 内容的 `CHARACTER_WIDTH`？
2. **动态宽度变化**: 设备旋转或窗口调整大小时，是否应以新宽度重新捕获？
3. **Box drawing + reflow**: 使用 `-C` 裁剪时，box drawing 框在裁剪处是否会被切断？
4. **Expo DOM WebView 性能**: 低端 Android 设备上 xterm.js WebGL renderer 的帧率？
5. **大 pane 的 `reset()` 性能**: 5000 行 xterm.js `write()` 在移动端是否卡顿？
6. **tmux control mode 兼容性**: 目标用户 tmux 版本范围（2.x ~ 3.x）对 `%output` 协议的一致性？
7. **电量影响**: 从 5s polling 切换到持续 stream 后的续航下降？

---

## 13. 相关文件

| 文件 | 角色 |
|------|------|
| `app/src/screens/tmux-pane-screen.tsx` | Tmux pane 主屏幕 |
| `app/src/components/ansi-text-renderer.tsx` | ANSI Text 渲染器（将被替代） |
| `app/src/components/terminal-emulator.tsx` | xterm.js 封装（复用目标） |
| `app/src/terminal/runtime/terminal-emulator-runtime.ts` | xterm.js 运行时（复用目标） |
| `app/src/hooks/use-tmux-capture-pane.ts` | Pane 内容抓取 hook |
| `app/src/utils/ansi-parser.ts` | ANSI parser（Box Drawing strip 位置） |
| `daemon/internal/server/session_tmux.go` | Daemon tmux 命令执行 |
| `docs/architecture/tmux-pane-content-loading.md` | Tmux pane 内容加载架构文档 |
| `docs/analysis/tmux-transport-disposed-race.md` | Transport disposed 竞态分析 |
