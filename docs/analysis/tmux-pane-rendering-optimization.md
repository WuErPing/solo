# Android Tmux Pane 渲染优化深度分析

> 对比主机 tmux 效果，评估当前 App 端 tmux pane 渲染的优化空间，提出分阶段改进方案。

- **Status:** Analysis Complete
- **Priority:** High (UX)
- **Date:** 2026-06-09
- **Author:** Kimi (Agent)
- **Related:** [tmux-pane-jitter-analysis.md](tmux-pane-jitter-analysis.md), [tmux-pane-content-loading.md](../architecture/tmux-pane-content-loading.md)

---

## 1. Executive Summary

当前 Solo App 的 tmux pane 渲染采用 **snapshot polling + React Native Text tree** 模型，与主机 tmux 的 **cell-based incremental VT stream** 模型存在架构级差距。虽然 v0.4.1 通过三层防抖（content dedup、`React.memo`、pagination-only loading）消除了静态场景下的 jitter，但动态内容（spinner、TUI、vim）的体验仍与主机有明显落差。

**核心差距：**
1. Box Drawing / Braille 字符被 strip，TUI 边框缺失
2. 无 cursor 渲染、无精确 Unicode width 计算
3. 主机终端宽度 ≠ 手机宽度，内容折行错位
4. Snapshot polling 无法支持真正的实时动画

**推荐路线：**
- **Phase 1（1 个月内）**：将 tmux pane 迁移至已有成熟的 `TerminalEmulator`（xterm.js + Expo DOM），复用 workspace terminal 的基础设施
- **Phase 2（视需求）**：引入 tmux Control Mode (`tmux -C`) 实现 PTY stream 直通，彻底对齐主机体验

---

## 2. 当前状态

### 2.1 数据流

```
主机 tmux:  PTY ──► tmux server ──► cell grid ──► incremental redraw ──► 终端
                         ↑
Solo App:   PTY ──► tmux server ──► capture-pane -p -e ──► ANSI string ──► parseAnsi ──► React Text
```

- **抓取命令**：`tmux capture-pane -t {paneId} -p -e -S -{scrollbackLines}`
- **轮询策略**：adaptive polling（200ms → 1s → 5s）
- **渲染路径**：`parseAnsi()` → `AnsiSegment[]` → `AnsiTextContent`（React.memo `<Text>` 树）→ `ScrollView`

### 2.2 已实施的优化（v0.4.1）

| 优化 | 文件 | 效果 |
|---|---|---|
| Content dedup | `use-tmux-capture-pane.ts` | 字节不变时跳过下游 memo |
| `React.memo` on `AnsiTextContent` | `ansi-text-renderer.tsx` | stable props 时跳过 Text tree reconcile |
| Pagination-only `isLoadingMore` | `use-tmux-capture-pane.ts` | 消除 5s poll 内容高度脉冲 |
| Adaptive polling | `use-tmux-capture-pane.ts` | 空闲时降至 5s，节省电量 |

### 2.3 现有 Workspace Terminal（参照物）

Solo 已在 workspace 场景使用 **xterm.js v6**（`@xterm/xterm`）+ **WebGL renderer**，通过 Expo `"use dom"` 在移动端 WebView 中运行：

- 实时 VT stream（WebSocket binary）
- Cell-based rendering、box drawing、Braille、cursor、true color
- Unicode 11、ligatures、自定义 touch scroll
- 成熟的生命周期管理（`TerminalEmulatorRuntime`）

该基础设施已完全具备，只是未用于 tmux pane 场景。

---

## 3. 根因分析

### 3.1 渲染模型差距

| 维度 | 主机 tmux | 当前 App |
|---|---|---|
| 渲染单位 | Cell grid | React `<Text>` 组件 |
| 更新方式 | 增量（只重绘变化 cell） | Full reconcile（整棵树） |
| 滚动管理 | Terminal-internal buffer | `ScrollView` + `setTimeout` |
| 光标 | 精确位置 + 样式 + blink | **无** |
| 字符宽度 | `wcwidth` | 无计算 |

### 3.2 Box Drawing 被 Strip

`ansi-parser.ts` 219-223 行：

```typescript
const cp = input.codePointAt(i)!;
if ((cp >= 0x2500 && cp <= 0x259f) || (cp >= 0x2800 && cp <= 0x28ff)) {
  i += cp > 0xffff ? 2 : 1;
  continue;  // ← 直接丢弃
}
```

**原因**：`capture-pane` 按主机终端宽度输出，与手机显示宽度不匹配，box drawing 直接渲染会错位。
**影响**：`htop`、`lazygit`、`claude-code` 的边框和 Braille 进度条全部消失。

### 3.3 宽度不匹配

- `tmux capture-pane` 按主机终端宽度（如 120 cols）输出
- 手机屏幕有效宽度约 40-80 cols
- 长行硬折行位置与手机显示不一致，导致视觉错位

### 3.4 Snapshot vs Stream

`capture-pane` 是**快照 API**，而主机 tmux 是**流式 emulator**。

- 每次 poll 传输整个 pane（可能 50KB+）
- 即使只有 1 个字符变化，也需完整 parse + re-render
- 对 ASCII 动画（spinner、进度条）完全无力

---

## 4. 业界方案对标

| 产品 | Tmux 支持方式 | 渲染技术 | 效果 |
|---|---|---|---|
| **Termius** | SSH + tmux attach | 自研 native terminal emulator | 接近原生，支持所有 VT 序列 |
| **Blink Shell** | 原生 mosh/ssh + tmux | `libxterm.js` fork / 自研 | 支持完整 tmux，含 pane 分割 |
| **iTerm2** | `tmux -CC` Control Mode | Native cell grid | **将 tmux pane 映射为 native tab** |
| **VS Code** | 内置终端 + tmux | `xterm.js` + PTY stream | 与 Solo workspace terminal 相同架构 |
| **WezTerm** | `tmux` 协议原生支持 | GPU (WebGPU) rendering | 直接解析 tmux 协议 |

**共性结论**：所有效果好的方案都有一个**真正的 terminal emulator**，而不是 ANSI→Text 的转换器。

---

## 5. 优化方案比较

### 方案 A：ANSI Text Renderer 局部增强（短期，1-2 周）

**思路**：在现有架构上打补丁。

| 改动 | 做法 | 效果 |
|---|---|---|
| 恢复 Box Drawing | 去掉 `ansi-parser.ts` strip 逻辑 | TUI 边框可见 |
| Daemon width crop | `tmux capture-pane -C {cols}` | 按手机屏幕宽度裁剪 |
| Cursor 渲染 | 解析 ANSI cursor 序列，渲染 `▌` | 视觉上有 cursor |
| `parseAnsi` LRU 缓存 | `Map<string, AnsiSegment[]>` | 减少重复 parse |
| 逐行 diff | 对比前后行数组，只更新变化行 | 减少 reconcile 范围 |

**优点**：改动集中，风险低，不引入新依赖。
**缺点**：天花板明显——仍是 snapshot polling，无法支持实时动画、复杂 TUI 交互。

---

### 方案 B：Tmux Pane 复用 xterm.js（中期，3-4 周）⭐ 推荐

**思路**：将现有 `TerminalEmulator` 组件（workspace terminal 已使用）应用到 tmux pane。

**实施**：

1. **新建 `TmuxXtermPane`**，复用 `TerminalEmulator` + `TerminalEmulatorRuntime`
2. **Adaptor 层**：每次 `capture-pane` 返回后：
   ```typescript
   terminal.reset();
   terminal.write(captureOutput);
   ```
   xterm.js 内部有 cell grid，只重绘实际变化的 cell。
3. **宽度适配**：测量 DOM 容器宽度，Daemon 侧 `capture-pane -C {cols}`
4. **Theme 桥接**：`TERMINAL_THEME_PRESETS` → `ITheme`

**优点**：
- 复用现有成熟代码（touch scroll、WebGL、resize、theme 已完备）
- 一次解决 box drawing、cursor、true color、scroll 稳定性、Unicode 宽度
- 滚动由 xterm.js viewport 内部管理，比 ScrollView 稳定
- 与现有 polling 模型兼容

**缺点**：
- 大 pane（5000 行）的 `reset()` + `write()` 有 CPU 成本
- 内容未变时仍需避免 `reset()` —— 可加 content hash 判断
- Expo DOM WebView 在低端 Android 设备上的性能需验证

**关键优化**：不做 `reset()` 除非内容结构变化大。可做行级 diff，只 `write()` 变化的行。

---

### 方案 C：PTY Stream 直通（长期，6-8 周）

**思路**：像 workspace terminal 一样，让 tmux pane 通过 PTY 实时流式传输。

```
当前: tmux pane ──► tmux server ──► capture-pane ──► ANSI snapshot ──► App
目标: tmux pane ──► tmux server ──► tmux -C (Control Mode) ──► VT stream ──► xterm.js
```

**实施**：

1. **Daemon 侧启动 `tmux -C`**
   - 输出 `%output %pane-id data` 格式
   - Daemon 订阅特定 pane，通过 WebSocket binary stream 发送

2. **App 侧复用 `TerminalStreamController`**
   - `terminal.write(streamData)` 增量更新
   - 完全消除 polling

3. **Input 反向传输**
   - App 键盘输入 → Daemon → `tmux send-keys` / control mode

**优点**：
- 与主机 tmux **完全一致**：实时、无 jitter、支持所有 VT 序列
- 网络效率最高（只传变化数据）
- 可支持交互式 TUI（vim、htop 在手机上操作）

**缺点**：
- Daemon 大量新代码：tmux control mode parser、pane 生命周期管理
- tmux 版本兼容性风险（control mode 协议各版本有差异）
- 电池消耗：持续 stream 比 5s polling 耗电
- 复杂度高，与 workspace terminal 的 PTY stream 相当，但 tmux 多一层 indirection

**业界参考**：iTerm2 的 tmux integration mode 就是此方案。

---

### 方案 D：Daemon-side Cell Grid Diff（创新/折中，4-6 周）

**思路**：Daemon 内存中维护 pane cell grid，只发送 diff 到客户端。

```
tmux capture-pane ──► Daemon cell grid ──► diff engine ──► cell patches ──► Lite Renderer
```

**Solo Lite Renderer** 可选：
- 自定义 Canvas 2D renderer（`st` / `alacritty` 简化版）
- 或轻量级 xterm.js（DOM renderer only）

**优点**：传输最小化、不依赖 tmux control mode。
**缺点**：与 xterm.js 能力重复造轮子，维护成本高。

---

## 6. 综合评估

| 方案 | 效果接近主机 tmux | 工作量 | 技术风险 | 维护成本 | 推荐度 |
|---|---|---|---|---|---|
| A. ANSI Text 增强 | ⭐⭐ | 低 | 低 | 低 | 短期过渡 |
| **B. xterm.js for tmux pane** | ⭐⭐⭐⭐ | **中** | **中** | **低** | **⭐ 主推** |
| C. PTY Stream / Control Mode | ⭐⭐⭐⭐⭐ | 高 | 高 | 高 | 长期 |
| D. Daemon Cell Grid Diff | ⭐⭐⭐⭐ | 中高 | 中高 | 高 | 不推荐 |

---

## 7. 推荐路线图

### Phase 1（1 个月内）：方案 B — xterm.js 迁移

**理由**：
1. Solo 已 100% 具备技术条件（`TerminalEmulator` + `TerminalEmulatorRuntime` + Expo DOM 已成熟运行）
2. 工作量可控：主要是 adaptor 层，不是从零开发
3. 一次改动解决渲染质量、box drawing、cursor、scroll 稳定性、Unicode 宽度所有问题
4. 与现有架构兼容：保留 `tmux capture-pane` polling，只换渲染器

**粗略设计**：

```
TmuxPaneScreen
  │
  ▼
<TmuxXtermPane> (新组件，复用 TerminalEmulator)
  │
  ├── props: streamKey, xtermTheme, initialCapture
  │
  ├── useEffect: 监听 useTmuxCapturePane 的 content
  │     │
  │     └── if content changed:
  │           runtime.reset()
  │           runtime.write(content)
  │
  ├── 复用 TerminalEmulatorRuntime:
  │     ├── WebGL renderer
  │     ├── Touch scroll
  │     ├── Custom scrollbar
  │     ├── Theme (toXtermTheme)
  │     └── Focus / Resize
  │
  └── Input: 保留现有 send-keys + virtual key bar
```

**Daemon 侧小改动**：

```go
func captureTmuxPane(paneID string, startLine, width int) (string, error) {
    args := []string{"capture-pane", "-t", paneID, "-p", "-e", "-S", strconv.Itoa(startLine)}
    if width > 0 {
        args = append(args, "-C", strconv.Itoa(width))
    }
    out, err := exec.Command("tmux", args...).Output()
    // ...
}
```

> **注意**：`tmux capture-pane -C` 是**截断**到指定宽度，不是 reflow。box drawing 可能在截断处被切断。对于 mobile 场景，建议额外考虑水平滚动，或接受截断（日志类内容影响不大）。

### Phase 2（视需求）：方案 C — Control Mode

如果 Phase 1 后用户反馈「spinner/动画不够实时」，再投入 PTY stream。

- 仅 10% 场景需要真正实时流（跑测试、看动画）
- 90% 场景（看 agent 输出、读日志）snapshot + xterm.js 已足够

---

## 8. Open Questions

1. **Expo DOM WebView 性能**：低端 Android 设备上 xterm.js WebGL renderer 的帧率表现如何？是否需要 fallback 到 DOM renderer？
2. **`-C` width 的精确计算**：如何从 React Native 的 `useWindowDimensions` 精确映射到 xterm.js 的 cols？（需考虑字体、DPR、padding）
3. **大 pane 的 `reset()` 性能**：5000 行 xterm.js `write()` 在移动端是否卡顿？是否需要行级 diff 优化？
4. **tmux control mode 兼容性**：目标用户使用的 tmux 版本范围（2.x ~ 3.x）对 `%output` 协议的支持是否一致？
5. **电量影响**：从 5s polling 切换到持续 stream 后，手机续航下降多少？是否需要动态降级策略？

---

## 9. 相关文件

| 文件 | 角色 |
|---|---|
| `app/src/screens/tmux-pane-screen.tsx` | Tmux pane 主屏幕 |
| `app/src/components/ansi-text-renderer.tsx` | ANSI Text 渲染器（将被替代） |
| `app/src/components/terminal-emulator.tsx` | xterm.js 封装（复用目标） |
| `app/src/terminal/runtime/terminal-emulator-runtime.ts` | xterm.js 运行时（复用目标） |
| `app/src/hooks/use-tmux-capture-pane.ts` | Pane 内容抓取 hook |
| `app/src/utils/ansi-parser.ts` | ANSI parser（Box Drawing strip 位置） |
| `daemon/internal/server/session_tmux.go` | Daemon tmux 命令执行 |
| `docs/analysis/tmux-pane-jitter-analysis.md` | 已实施的 jitter 修复分析 |
| `docs/architecture/tmux-pane-content-loading.md` | Tmux pane 内容加载架构文档 |
