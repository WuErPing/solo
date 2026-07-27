# Tmux-Pane 按键栏 UI 重设计方案

> Date: 2026-07-25
> Status: Proposal

## 现状问题

当前 native 屏 11 键平铺、xterm 屏 6 键平铺，存在：

- 缺乏意图分层：导航、动作、编辑、Agent 快捷键混在一行
- 静态不感知上下文：无论 pane 内容如何，按键集不变
- 数字键含义不透明：裸 "1234" 无选项语义
- 缺少关键键：native 版无 C-c
- 两屏不一致：native 11 键 vs xterm 6 键
- 垂直空间浪费：组标签占 14pt 但无信息增量

## 设计约束

- 屏幕宽度 ~390pt（iPhone），按键栏可用高度 <= 88pt（不含输入框）
- 主要场景：与 tmux 中运行的 AI agent 交互（选择、确认、中断）
- 需同时服务 native 渲染和 xterm 渲染两个页面（共享组件）

## 方案总览：三层按键栏

```
┌─────────────────────────────────────────────────────┐
│  Terminal content (FlatList / xterm)                │
│                                                     │
│                                          ⤓ ←悬浮   │
├─────────────────────────────────────────────────────┤
│ ┌─ Contextual Strip (动态，仅在有上下文时出现) ─────┐ │
│ │  ①Yes   ②No   ③Always allow   ④Skip             │ │
│ └──────────────────────────────────────────────────┘ │
│ ┌─ Primary Keys ───────────────────────────────────┐ │
│ │  ↑   ↓   │  Enter   Esc   ^C  │  ⋯ more          │ │
│ └──────────────────────────────────────────────────┘ │
│ ┌─ Input Row ──────────────────────────────────────┐ │
│ │  [ Type a command...              ] [➤]          │ │
│ └──────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

## Layer 1: Primary Keys（常驻行，高度 40pt）

```
  ↑    ↓    │   Enter    Esc    ^C   │   ⋯
 ────────   ─   ────────────────────  ─  ───
 导航组        │    动作组（拇指热区→右） │  展开
```

| 键 | 宽度 | 说明 |
|----|------|------|
| `↑` `↓` | 36pt | 历史/选项导航 |
| `Enter` | 56pt，**primary 色填充** | 最高频动作，视觉权重最大 |
| `Esc` | 40pt | 取消/退出模式 |
| `^C` | 40pt，**destructive 色文字** | 中断，与 Enter 形成对称的"确认 vs 中止" |
| `⋯` | 36pt | 展开/收起扩展层 |

设计决策：

- Enter 用实心 primary 背景（与 Send 按钮同色），是整行唯一的"实心强调"，拇指自然落点
- `^C` 用 destructive 色文字 + 普通背景，表达"危险操作"但不抢视觉
- 组间用 1pt divider（复用现有 `keyGroupDivider` 样式）
- 去掉组标签（"Send"/"View"）— 5 个键不需要标签解释，省 14pt 高度

## 悬浮"跳到最新"按钮（⤓，内容视图覆盖层）

"跳到最新输出"是视图操作而非 tmux 按键，且 agent 流式输出时高频使用，因此不占用按键栏席位，而是作为悬浮按钮覆盖在内容视图上：

- 图标 `ArrowDownToLine`（⤓），圆形 38pt，**半透明**（opacity 0.55），反色配色（bg=foreground / icon=background）保证在任意终端内容上可见
- 定位在内容区域右下角（`bottom: 12, right: 12`）。按钮挂在内容容器内——内容容器与按键栏/输入行是上下排列的 flex 兄弟——因此天然位于输入栏上方，不遮挡输入区
- 点击 = 视图滚动到底部（native: `FlatList.scrollToEnd`；xterm: viewport `scrollTop = scrollHeight`），**不发送任何 tmux 按键**
- native / xterm 两屏一致

## Layer 2: Expanded Keys（展开行，高度 40pt，默认收起）

点击 `⋯` 后在 Primary 上方滑出：

```
  Tab   S-Tab   ←   →   /   1   2   3   4   Home
```

| 键 | 分组逻辑 |
|----|----------|
| `Tab` `S-Tab` | 编辑/补全 |
| `←` `→` | 光标移动 |
| `/` | 搜索/slash 命令触发 |
| `1` `2` `3` `4` | Agent 选项（无上下文时的手动模式） |
| `Home` | 跳到顶部（低频；跳到最新由悬浮 ⤓ 按钮承担） |

设计决策：

- 展开状态时 `⋯` 旋转 180° 变为 `⌃`，再按收起
- 展开行用 `surface1` 底色与 primary 行区分层级
- 展开状态持久化到 AsyncStorage（用户习惯不被重置）

## Layer 3: Contextual Strip（动态选项条，条件出现）

### 触发条件

capture-pane 最后 N 行匹配到编号选项模式：

```regex
/^\s*[（(]?\s*([1-4])\s*[)）.：:]\s*(.+)$/   →  "1. Yes" / "(2) No"
/^\s*([1-4])\.\s+(.+)$/
```

### UI

```
┌────────────────────────────────────────────┐
│  1·Yes    2·No    3·Always    4·Skip      │  ← 横向可滚动
└────────────────────────────────────────────┘
```

- 每个 chip = 数字圆点（primary 色）+ 选项文本（截断 12 字符 + `…`）
- 点击 = 发送对应数字键（等价于按 `1`）
- 出现在 Primary Keys **上方**，出现/消失带 150ms slide + fade 动画
- 若选项 > 4 个，横向 ScrollView
- 出现 contextual strip 时，扩展层的 `1234` 键自动隐藏（避免重复）

无匹配时：strip 不占空间（height: 0），界面退化为纯双层结构。

## 统一两屏的组件结构

```tsx
<TmuxKeyBar
  onSendKey={(key: string) => void}
  onSendText={(text: string) => void}
  contextOptions={parsedOptions}   // 从 capture-pane 内容解析
  variant="native" | "xterm"       // 仅影响 xterm 额外键（如 C-c 已有）
/>
```

内部结构：

```
TmuxKeyBar
├── ContextualOptionStrip     (条件渲染)
├── PrimaryKeyRow             (↑ ↓ | Enter Esc ^C | ⋯)
├── ExpandedKeyRow            (展开态)
└── InputRow                  (TextInput + Send button，现有逻辑不变)
```

两屏共享此组件，xterm 屏不再单独维护 `VIRTUAL_KEYS` 数组。

## 交互细节

| 行为 | 规格 |
|------|------|
| 按键反馈 | pressed 态背景变 primary（现有行为保留），加 `android_ripple` |
| 长按 `^C` | 无特殊（避免误触设计，单击即发 C-c 已足够危险 — 用颜色警示） |
| 长按 `Enter` | 不做，保持 POLA |
| `⋯` 展开动画 | `LayoutAnimation.easeInEaseOut` 或 Reanimated height 0→40 |
| Contextual strip 消失 | 选项被消费后（下一次 capture-pane 不再匹配）自动淡出 |
| 键盘弹出时 | 按键栏保留（TextInput focus 不隐藏按键栏 — 用户可能需要 Tab 补全） |
| 横屏/宽屏 | Primary 行所有键居中，expanded 行不换行改为横向滚动 |

## 视觉规格（对齐现有 theme tokens）

```
Primary 行:
  背景: transparent, borderTop: 1px border
  按键: borderRadius 6, border 1px theme.border, bg surface1
  Enter: bg primary, text background色, 无边框
  ^C: text destructive, bg surface1
  高度: 32pt 按键 + 上下 4pt padding = 40pt

Expanded 行:
  bg: surface0 (略深一层)
  按键: 同 primary 行样式，label fontSize 11

Contextual strip:
  bg: surface1, borderRadius 8, marginHorizontal 12
  chip: 数字圆点 16pt primary色 + text 12pt foreground
  高度: 36pt
```

## 对比现状

| 维度 | 现在 | 方案 |
|------|------|------|
| 默认可见键数 | 11（native）/ 6+4（xterm） | 5 + 按需展开 |
| 信息层级 | 平铺 | 3 层（contextual → primary → expanded） |
| Enter 可发现性 | 与数字键同权重 | 唯一实心强调键 |
| 数字键语义 | 裸数字 | 带选项文本的 chip |
| 中断操作 | native 缺失 C-c | 两屏均有 ^C |
| 视图操作 | 独占 "View" 组 | 跳到最新=悬浮 ⤓ 按钮（内容视图上），Home 收入 expanded |
| 两屏一致性 | 不同按键集 | 共享 TmuxKeyBar 组件 |
| 垂直空间占用 | ~74pt（label+keys） | 40pt 默认 / 80pt 展开 |

## 设计原则

1. **以用户意图分组** — 按键栏映射"我要做什么"，而非罗列键名
2. **频率决定位置** — 高频键（Enter/数字）在拇指热区，低频键收入展开层
3. **上下文感知** — 按键集随 pane 内容动态适配
4. **渐进披露** — 默认 5 键，按需展开，不超 7±2 认知极限
5. **一致性** — native/xterm 共享同一按键组件
6. **可发现性** — 数字键绑定语义标签，消除记忆负担
