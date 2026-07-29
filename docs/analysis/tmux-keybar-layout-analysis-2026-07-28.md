# Tmux 多窗口按键栏布局与合理性分析

**Date:** 2026-07-28
**Status:** Analysis Complete
**Priority:** High (UX)

---

## Executive Summary

对 app 内两个 tmux pane 页面（native 渲染与 xterm 渲染）的共享按键栏 `TmuxKeyBar` 进行布局与合理性评估。三层结构（Contextual Strip / Primary Row / Expanded Row）的意图分层和两屏一致性目标已达成，设计方向合理。主要问题集中在**空间预算**：Primary Row 无 wrap、无滚动，在 xterm 屏常驻 3 个文字 extraButton 时按尺寸估算必然溢出（约 510pt > 390pt），排最后的 `⋯` 展开按钮会被裁剪；其次是 "History" 两屏同名异义、Refresh 条件显隐导致布局抖动等可预期性问题。

## Current State

两个页面共享 `app/src/components/tmux-key-bar.tsx`（设计方案见 `docs/design/tmux-keybar-ui-redesign.md`）：

- **Contextual Strip**（36pt，条件出现）：`app/src/utils/tmux-option-parser.ts` 从 capture-pane 尾部 10 行解析编号选项，渲染为 `1·Yes 2·No` 语义 chip
- **Primary Row**（常驻 40pt）：`↑ ↓ │ Enter Esc ^C │ ⋯`，中间 flex spacer，右侧挂各屏 `extraButtons`
- **Expanded Row**（40pt，默认收起，zustand + AsyncStorage 持久化）：`Tab S-Tab ← → / 1 2 3 4 Home`；strip 出现时自动隐藏 `1234`（`tmux-key-bar.tsx:102`）

两屏核心按键完全一致，差异仅在 `extraButtons`：

| extraButton | native 屏（`tmux-pane-screen.tsx:386`） | xterm 屏（`tmux-pane-xterm-screen.tsx:239`） |
|---|---|---|
| Refresh | 仅 `!autoRefresh` 时出现 | 常驻 |
| History | Clock 图标，切换命令历史下拉 | 文字按钮，分页加载 scrollback |
| Fit/1:1 | 无 | 常驻，active 态高亮 |

## Analysis

### 合理的部分

- **意图分层成立**：导航（↑↓）、动作（Enter/Esc/^C）、编辑（展开层）三组分离；Enter 是整行唯一实心 primary 色键，`^C` 用 destructive 文字色，视觉权重与使用频率/危险度匹配。
- **两屏一致性达成**：共享组件消除了旧版 native 11 键 vs xterm 6 键的不一致；native 屏补上了 `^C`。
- **上下文感知有真实价值**：选项 chip 把裸数字键升级为带语义按钮，出现时自动从展开层摘掉 `1234` 避免重复。
- **视图操作不进按键栏**：跳到最新为内容区悬浮 ⤓ 按钮，不占按键席位。

### 问题与风险（按严重度排序）

**1. Primary Row 在 xterm 屏几乎必然溢出（High）**

`primaryRow` 是普通 `flexDirection: "row"` 的 View，无 wrap、无 ScrollView；RN 的 `flexShrink` 默认为 0，子项超宽不压缩、直接向右裁剪。xterm 屏常驻 3 个文字 extraButton，按 `tmux-key-bar.tsx` 样式估算：

```
↑36 ↓36 divider Enter56 Esc40 ^C40 ≈ 213 + 3 个文字键 ≈ 180 + ⋯36 + gaps/padding ≈ 90
合计 ≈ 510pt  >>  390pt（iPhone 宽）
```

排最后的 `⋯` 会被推出屏幕，展开层可能无法打开。native 屏在 `autoRefresh` 关闭时（Refresh 文字键 + History 图标并存）同样临界溢出。**需真机验证。**

**2. "History" 两屏同名不同义（Medium）**

native 的 History 是"我曾发过的命令"（本地历史下拉），xterm 的 History 是"加载更早的终端输出"（scrollback 分页）。同一位置、相近命名、行为完全不同，违反 POLA。

**3. Refresh 条件性出现导致布局抖动（Medium）**

native 屏 `autoRefresh` 开关会增删一个键，按键位置随之移动，破坏肌肉记忆。

**4. Esc 与 ^C 相邻的误触代价不对称（Medium）**

Esc 高频"取消"，^C 单击即发、可能直接中断 agent 进程；两个 32pt 键间隔仅 6pt。设计文档明确拒绝长按确认（用颜色警示），但颜色不防误触。

**5. 选项解析静默失败（Low）**

`tmux-option-parser.ts` 只认数字 1-4、要求首选项必须为 1、至少 2 个选项才显示；字母选项、超过 4 项、选项滚出尾部 10 行时 strip 静默消失，用户无从感知规则。

**6. 多 window/pane 场景缺少切换入口（Low，待产品确认）**

按键栏只能向当前 pane 发键，无 tmux 窗口/面板切换入口（`C-b n/p/w` 类），多 window 场景只能退回 dashboard 重选。可能是有意的边界划分（导航归 app，按键归 pane）。

## Recommendations

1. **修复溢出（优先）**：真机验证 xterm 屏；修复方向为 extraButton 图标化，或 primary 行允许横向滚动。
2. **统一 History 语义**：xterm 侧改名为 "More"/"Earlier" 或换图标。
3. **Refresh 常驻化**：以 disabled 态替代条件显隐，或固定在最外侧位置。
4. **降低 ^C 误触风险**：考虑 ^C 与 ⋯ 换位，或加极短确认态。
5. **窗口切换入口**：利用现有 extraButton 机制低成本补一个入口，待产品层面确认边界后再做。

## Implementation Plan

| 阶段 | 内容 | 涉及文件 |
|---|---|---|
| P0 | 真机验证溢出；extraButton 图标化或 primary 行横向滚动 | `app/src/components/tmux-key-bar.tsx`, `app/src/screens/tmux-pane-xterm-screen.tsx` |
| P1 | xterm History 改名/换图标；Refresh 常驻 disabled 态 | 两屏 screen 文件 |
| P2 | ^C 位置/确认态调整；窗口切换入口（待产品确认） | `tmux-key-bar.tsx` |

## Related Files

- `app/src/components/tmux-key-bar.tsx` — 共享按键栏组件
- `app/src/stores/tmux-keybar-store.ts` — 展开状态持久化
- `app/src/utils/tmux-option-parser.ts` — 上下文选项解析
- `app/src/screens/tmux-pane-screen.tsx` — native 屏（extraButtons: Refresh/History）
- `app/src/screens/tmux-pane-xterm-screen.tsx` — xterm 屏（extraButtons: Fit/Refresh/History）
- `docs/design/tmux-keybar-ui-redesign.md` — 原始设计方案
