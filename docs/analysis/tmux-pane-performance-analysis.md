# Tmux Pane 性能与稳定性根因分析

> 分析 tmux pane 性能差、不稳定的原因：是近期变更引起，还是有更核心的架构问题。

- **Status:** Analysis Complete
- **Date:** 2026-06-09
- **Author:** Andy
- **Related:** [tmux-pane-jitter-analysis.md](tmux-pane-jitter-analysis.md), [tmux-pane-rendering-optimization.md](tmux-pane-rendering-optimization.md), [tmux-transport-disposed-race.md](tmux-transport-disposed-race.md)

---

## 1. 结论

**核心架构问题，不是近期变更引起的。**

近期提交（2026-06-04 ~ 06-09）实际上是在**修复**性能和稳定性问题，而非引入问题。但修复治标不治本——架构层面有根本性的效率瓶颈。

---

## 2. 近期变更回顾

| 提交 | 日期 | 性质 | 影响 |
|------|------|------|------|
| `a1cc7bd` 修复 Android 5s 抖动 | 06-08 | 修复 | 自适应轮询 200ms/1s/5s, React.memo, content dedup |
| `335315c` 修复 disposed transport | 06-05 | 修复 | withLiveTmuxClient 重试机制 |
| `7e87a06` 修复白屏崩溃 | 06-04 | 修复 | keepPreviousData, ErrorBoundary, sanitizeForNative |
| `9b57b5c` 懒加载历史 | 06-06 | 功能 | scrollback 从 200→5000 行，增加了数据量 |
| `68c66e5` 终端主题和窗口列表 | 06-07 | 功能 | 增加了渲染复杂度 |
| `4750a5f` UI 按钮分组 | 06-09 | 样式 | 无性能影响 |

这些变更让 tmux pane 从"经常崩溃"变成了"能用但慢"。

---

## 3. 根本原因：4 层架构瓶颈

### 3.1 第 1 层：轮询式全量捕获（最核心问题）

**文件:** `daemon/internal/server/session_tmux.go:262`

```go
func captureTmuxPane(paneID string, startLine int) (string, error) {
    out, err := exec.Command("tmux", "capture-pane", "-t", paneID, "-p", "-e", "-S", strconv.Itoa(startLine)).Output()
    ...
}
```

每次轮询都执行 `tmux capture-pane`，返回**完整的终端内容字符串**。即使终端只变了 1 个字符，也要传输和处理整个字符串（可达 5000 行 × 每行含大量 ANSI 转义码）。

**没有增量机制。** 这是与主机 tmux（cell-based incremental VT stream）的根本差距。

### 3.2 第 2 层：客户端全量 ANSI 解析

**文件:** `app/src/utils/ansi-parser.ts:147`

`parseAnsi(content)` 在 `useMemo` 中以 `[content]` 为依赖运行。content 字符串每 200ms 变化一次，解析器就跑一次。5000 行 ANSI 文本的单遍解析虽然 O(n)，但每秒跑 5 次仍然昂贵。

更关键的是，**每次解析产生新的 `AnsiSegment[]` 数组引用**，直接击穿下游 `React.memo`——`React.memo` 对 `segments` 做浅比较，新数组引用 ≠ 旧数组引用，所以每次都触发 re-render。

### 3.3 第 3 层：React Native 无虚拟化渲染

**文件:** `app/src/components/ansi-text-renderer.tsx:14`

```tsx
<ScrollView>
  <AnsiTextContent segments={segments} ... />  // → 数千个嵌套 <Text>
</ScrollView>
```

`AnsiTextContent` 把每个 segment 渲染为一个 `<Text>` 子节点。5000 行终端内容可能产生数千个 `<Text>` 元素，**全部同时挂载**在 React Native 视图树中。没有 `FlatList` 或任何窗口化机制。

在移动端造成：
- 巨大的内存压力（所有节点同时存在）
- 高 layout 计算成本（每次 content 变化都全量 re-layout）
- 滚动卡顿（ScrollView 需要计算所有子节点的总高度）

### 3.4 第 4 层：string 全等比较做 dedup

**文件:** `app/src/hooks/use-tmux-capture-pane.ts:87`

```ts
if (prevResultRef.current && prevResultRef.current.content === newContent) {
    return prevResultRef.current;
}
```

用 `===` 比较新旧 content 字符串。对于带 ANSI 转义码的 5000 行文本，这是一个逐字符比较，每 200ms 做一次。虽然比"不做 dedup"好，但比较本身也有成本。

---

## 4. 性能影响链

```
tmux capture-pane (shell fork, 全量捕获)
  → 大字符串通过 WebSocket 传输
    → string === 逐字符比较 (dedup)
      → parseAnsi() 全量重解析 → 新 segments[] 引用
        → React.memo 失效 → 数千 <Text> 全量 re-layout
          → ScrollView re-render → 卡顿、内存飙升
```

每一层都在放大上一层的问题。终端内容越多，每层的开销越大，整体呈乘法关系。

---

## 5. 稳定性问题来源

| 问题 | 根因 | 修复状态 |
|------|------|----------|
| 白屏/崩溃 | 大量 `<Text>` 节点快速更新触发 RN 渲染管线超时 | `7e87a06` 已部分修复（keepPreviousData, ErrorBoundary） |
| Transport disposed | 轮询期间 WebSocket 连接状态变化导致竞态 | `335315c` 已修复（withLiveTmuxClient 重试） |
| Android 5s 抖动 | 固定轮询 + isLoadingMore 误触发导致 UI 脉冲 | `a1cc7bd` 已修复（自适应轮询, pagination-only loading） |

稳定性修复是有效的，但它们是在一个本质上低效的架构上打补丁。

---

## 6. 优化方向

| 优先级 | 方向 | 预期收益 | 复杂度 |
|--------|------|----------|--------|
| **P0** | **虚拟化渲染**：用 `FlatList` 替代 `ScrollView`，只渲染可见行 | 内存降 90%+，滚动流畅 | 中 |
| **P0** | **增量传输**：daemon 端维护 pane 快照，只发送 diff（changed lines） | 网络传输降 95%+，解析/渲染开销同步降低 | 高 |
| P1 | **解析缓存**：基于 content hash 判断是否需要重新 parseAnsi | 避免无变化时的解析和 re-render | 低 |
| P1 | **降低活跃轮询频率**：200ms → 500ms（大多数 agent 场景不需要 5fps） | CPU 降 60% | 低 |
| P2 | **ANSI 分段解析 + 差量更新**：只解析变化的行 | 解析成本降到 O(变化行数) | 高 |
| P2 | **迁移到 TerminalEmulator**：复用 xterm.js + Expo DOM 基础设施 | 彻底对齐主机体验 | 很高 |

P0 的两项（虚拟化 + 增量传输）组合起来可以从根本上解决问题。单独做任何一项都有明显改善，但两者结合才能让 tmux pane 在大数据量下也保持流畅。

详细渲染优化方案见 [tmux-pane-rendering-optimization.md](tmux-pane-rendering-optimization.md)。

---

## 7. 数据流全景

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
