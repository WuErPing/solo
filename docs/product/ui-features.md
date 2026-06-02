# Solo App UI 功能详细分析

> 分析日期：2026-05-25
> 代码库：/Users/wuerping/code/wuerping/solo/app
> 技术栈：React Native / Expo / TypeScript

---

## 一、App 导航结构

App 使用 **Expo Router** 作为导航框架，采用文件系统路由：

```
app/
├── _layout.tsx              # 根布局（全局 Provider、侧边栏、命令中心）
├── index.tsx                # 入口（启动引导、重定向）
├── welcome.tsx              # 欢迎页（首次使用）
├── dashboard.tsx            # 仪表盘
├── pair-scan.tsx            # 扫码配对
├── settings/
│   ├── index.tsx            # 设置首页
│   ├── [section].tsx        # 设置分类页
│   ├── hosts/
│   │   └── [serverId].tsx   # 主机详情
│   └── projects/
│       ├── index.tsx        # 项目列表
│       └── [projectKey].tsx # 项目详情
└── h/
    └── [serverId]/
        ├── index.tsx          # 主机首页（重定向到 open-project）
        ├── new.tsx            # 新建工作区
        ├── open-project.tsx   # 打开项目
        ├── sessions.tsx       # 会话列表
        ├── settings.tsx       # 主机设置
        ├── agent/
        │   └── [agentId].tsx  # Agent 详情（重定向到工作区）
        └── workspace/
            └── [workspaceId]/
                ├── _layout.tsx # 工作区布局
                └── index.tsx   # 工作区首页
```

---

## 二、屏幕（Screens）功能

### 2.1 启动与引导

| 屏幕 | 文件 | 功能描述 |
|------|------|----------|
| **启动画面** | `startup-splash-screen.tsx` | 应用启动加载画面，显示加载状态 |
| **欢迎页** | `welcome.tsx` | 首次使用引导，介绍产品功能 |

### 2.2 主界面

| 屏幕 | 文件 | 功能描述 |
|------|------|----------|
| **仪表盘** | `dashboard/dashboard-screen.tsx` | 主控制台，显示所有 Agent 状态概览 |
| **项目列表** | `projects-screen.tsx` | 查看和管理所有项目 |
| **打开项目** | `open-project-screen.tsx` | 选择并打开项目 |
| **会话列表** | `sessions-screen.tsx` | 查看历史会话记录 |

**仪表盘功能细节：**
- Agent 状态卡片（运行中、空闲、错误、需关注）
- 状态筛选（全部、运行中、空闲、错误、需权限）
- Agent 列表（名称、路径、状态、最后活动时间）
- 快捷操作（打开工作区、查看详情）

### 2.3 工作区（Workspace）

| 屏幕/组件 | 文件 | 功能描述 |
|-----------|------|----------|
| **工作区主屏** | `workspace/workspace-screen.tsx` | 多面板工作区布局 |
| **桌面标签行** | `workspace-desktop-tabs-row.tsx` | 顶部标签栏（桌面端） |
| **面板内容** | `workspace-pane-content.tsx` | 面板内容渲染 |
| **Git 操作** | `workspace-git-actions.tsx` | Git 分支切换、提交等 |
| **脚本按钮** | `workspace-scripts-button.tsx` | 运行工作区脚本 |
| **编辑器打开按钮** | `workspace-open-in-editor-button.tsx` | 在编辑器中打开 |

**工作区功能细节：**
- **多标签布局**：支持多个 Agent/终端/文件标签
- **面板系统**：可拖拽调整面板大小
- **Git 集成**：分支显示、切换、操作
- **文件浏览器**：侧边栏文件树（ExplorerSidebar）
- **脚本执行**：一键运行工作区脚本
- **Diff 统计**：显示代码变更统计
- **工作区设置**：配置工作区参数

**标签类型：**
- Agent 标签（运行中的 AI Agent）
- 终端标签（PTY 终端）
- 文件标签（代码文件编辑）
- Draft 标签（草稿 Agent）
- Setup 标签（工作区设置）

### 2.4 Agent 交互

| 屏幕/组件 | 文件 | 功能描述 |
|-----------|------|----------|
| **Agent 准备屏** | `agent-ready-screen-bottom-anchor.ts` | Agent 路由底部锚点计算 |

**Agent 界面功能（通过工作区 Agent 标签）：**
- 实时流式输出显示
- 多模态输入（文本 + 附件）
- 代码块渲染与高亮
- **Mermaid 图表预览**：Markdown 文件面板内实时渲染 Mermaid 流程图/时序图
- 工具调用显示
- 权限请求处理
- 模型选择器（Claude、Kimi、OpenCode）

### 2.5 设置（Settings）

| 屏幕/组件 | 文件 | 功能描述 |
|-----------|------|----------|
| **设置主屏** | `settings-screen.tsx` | 设置首页，分类导航 |
| **设置分类** | `settings-section.tsx` | 通用设置项组件 |
| **主机页面** | `settings/host-page.tsx` | 主机配置详情 |
| **Provider 配置** | `settings/providers-section.tsx` | AI Provider 配置 |
| **快捷键设置** | `settings/keyboard-shortcuts-section.tsx` | 键盘快捷键配置 |

**设置功能细节：**
- **主题切换**：亮色/暗色/系统主题
- **发送行为**：Enter 发送 / Cmd+Enter 发送
- **主机管理**：添加/编辑/删除主机
- **Provider 配置**：Claude、OpenCode 等 API 配置
- **快捷键**：自定义键盘快捷键
- **版本信息**：应用版本、更新检查
- **桌面权限**：桌面端特殊权限设置

### 2.6 新建工作区

| 屏幕/组件 | 文件 | 功能描述 |
|-----------|------|----------|
| **新建工作区** | `new-workspace-screen.tsx` | 创建新工作区向导 |
| **选择器项** | `new-workspace-picker-item.ts` | 工作区模板选择 |

**新建工作区功能：**
- 选择项目
- 选择工作区模板
- 配置工作区参数
- Git 工作树选项

---

## 三、核心组件（Components）

### 3.1 Agent 相关组件

| 组件 | 文件 | 功能 |
|------|------|------|
| **Agent 列表** | `agent-list.tsx` | 显示 Agent 列表 |
| **Agent 状态栏** | `agent-status-bar.tsx` | 底部状态栏，显示当前 Agent 状态 |
| **Agent 状态点** | `agent-status-dot.tsx` | 状态指示圆点 |
| **Agent 流视图** | `agent-stream-view.tsx` | 实时流式输出渲染 |
| **流渲染模型** | `agent-stream-render-model.ts` | 流事件渲染策略 |
| **流渲染策略** | `agent-stream-render-strategy.ts` | 不同事件类型的渲染逻辑 |
| **模型选择器** | `combined-model-selector.tsx` | 选择 AI 模型 |

### 3.2 输入组件

| 组件 | 文件 | 功能 |
|------|------|------|
| **Composer** | `composer.tsx` | 主输入框，支持多模态 |
| **输入提交** | `agent-input-submit.ts` | 输入提交逻辑 |
| **附件药丸** | `attachment-pill.tsx` | 附件标签显示 |
| **附件灯箱** | `attachment-lightbox.tsx` | 附件预览（图片、文件） |
| **SearchInput** | `search-input.tsx` | 搜索输入框，已禁用浏览器 autocomplete 防止重复 suggestion overlay |

### 3.3 导航与布局

| 组件 | 文件 | 功能 |
|------|------|------|
| **左侧边栏** | `left-sidebar.tsx` | 主导航侧边栏 |
| **侧边栏工作区列表** | `sidebar-workspace-list.tsx` | 工作区列表 |
| **命令中心** | `command-center.tsx` | 命令面板（Cmd+K） |
| **菜单头部** | `headers/menu-header.tsx` | 顶部菜单栏 |
| **屏幕头部** | `headers/screen-header.tsx` | 屏幕标题栏 |
| **返回头部** | `headers/back-header.tsx` | 带返回按钮的头部 |

### 3.4 模态框与对话框

| 组件 | 文件 | 功能 |
|------|------|------|
| **添加主机方式** | `add-host-method-modal.tsx` | 选择添加主机方式 |
| **添加主机** | `add-host-modal.tsx` | 输入主机信息 |
| **项目选择器** | `project-picker-modal.tsx` | 选择项目 |
| **配对链接** | `pair-link-modal.tsx` | 显示配对链接/二维码 |
| **工具调用面板** | `tool-call-sheet.tsx` | 显示工具调用详情 |
| **自适应底部弹窗** | `adaptive-modal-sheet.tsx` | 自适应底部弹窗 |
| **工作区设置对话框** | `workspace-setup-dialog.tsx` | 工作区初始化设置 |

### 3.5 Git 相关组件

| 组件 | 文件 | 功能 |
|------|------|------|
| **分支切换器** | `branch-switcher.tsx` | 切换 Git 分支 |
| **Diff 统计** | `diff-stat.tsx` | 显示代码变更统计 |
| **Git 操作面板** | `workspace-git-actions.tsx` | 提交、推送等操作 |

### 3.6 文件浏览器

| 组件 | 文件 | 功能 |
|------|------|------|
| **资源管理器侧边栏** | `explorer-sidebar.tsx` | 文件树浏览器 |
| **代码插图** | `code-insets.ts` | 代码片段插入 |

### 3.7 UI 基础组件

| 组件 | 文件 | 功能 |
|------|------|------|
| **按钮** | `ui/button.tsx` | 基础按钮 |
| **下拉菜单** | `ui/dropdown-menu.tsx` | 下拉菜单 |
| **组合框** | `ui/combobox.tsx` | 可搜索选择框 |
| **工具提示** | `ui/tooltip.tsx` | 悬停提示 |
| **快捷方式** | `ui/shortcut.tsx` | 快捷键显示 |
| **分段控制** | `ui/segmented-control.tsx` | 分段选择器 |
| **上下文菜单** | `ui/context-menu.tsx` | 右键菜单 |
| **隔离底部弹窗** | `ui/isolated-bottom-sheet-modal.tsx` | 独立底部弹窗 |

---

## 四、Context（全局状态）

| Context | 文件 | 功能 |
|---------|------|------|
| **SessionContext** | `session-context.tsx` | 会话状态管理 |
| **ToastContext** | `toast-context.tsx` |  toast 通知 |
| **VoiceContext** | `voice-context.tsx` | 语音输入状态 |
| **HorizontalScroll** | `horizontal-scroll-context.tsx` | 水平滚动控制 |
| **SidebarAnimation** | `sidebar-animation-context.tsx` | 侧边栏动画 |
| **SidebarCallout** | `sidebar-callout-context.tsx` | 侧边栏提示 |
| **ExplorerSidebarAnimation** | `explorer-sidebar-animation-context.tsx` | 文件浏览器动画 |

---

## 五、Hooks（自定义钩子）

| Hook | 文件 | 功能 |
|------|------|------|
| **useAggregatedAgents** | `use-aggregated-agents.ts` | 聚合所有 Agent 状态 |
| **useAgentInputDraft** | `use-agent-input-draft.ts` | Agent 输入草稿 |
| **useExplorerOpenGesture** | `use-explorer-open-gesture.ts` | 文件浏览器打开手势 |
| **useActiveWorktreeNewAction** | `use-active-worktree-new-action.ts` | 工作树新建操作 |
| **useSettings** | `use-settings.ts` | 应用设置 |

---

## 六、Stores（状态存储）

| Store | 文件 | 功能 |
|-------|------|------|
| **SessionStore** | `session-store.ts` | 会话状态（Zustand） |
| **PanelStore** | `panel-store.ts` | 面板状态 |
| **WorkspaceTabsStore** | `workspace-tabs-store.ts` | 工作区标签状态 |
| **WorkspaceLayoutStore** | `workspace-layout-store.ts` | 工作区布局状态 |
| **NavigationActiveWorkspaceStore** | `navigation-active-workspace-store.ts` | 导航工作区选择 |

---

## 七、App 支持的产品功能汇总

### ✅ 已实现功能

#### 7.1 连接管理
- [x] 多主机管理（添加/编辑/删除）
- [x] 直接连接（本地 daemon）
- [x] Relay 远程连接
- [x] QR 码配对
- [x] 配对链接生成
- [x] 连接状态实时显示

#### 7.2 Agent 管理
- [x] Agent 列表查看
- [x] Agent 状态监控（运行中、空闲、错误）
- [x] Agent 实时流式输出
- [x] 多模态输入（文本 + 附件）
- [x] 代码块渲染与高亮
- [x] 工具调用显示
- [x] 权限请求处理
- [x] 模型选择器（Claude、OpenCode）

#### 7.3 工作区
- [x] 多工作区管理
- [x] 多标签布局（Agent/终端/文件）
- [x] 可拖拽面板
- [x] 文件浏览器（树形结构）
- [x] Git 集成（分支显示、切换）
- [x] 脚本执行
- [x] Diff 统计
- [x] 在编辑器中打开

#### 7.4 项目管理
- [x] 项目列表
- [x] 项目配置
- [x] 打开项目
- [x] 新建工作区

#### 7.5 终端
- [x] PTY 终端（通过工作区标签）
- [x] 多终端支持

#### 7.6 设置
- [x] 主题切换（亮/暗/系统）
- [x] 发送行为配置
- [x] 主机配置
- [x] Provider API 配置
- [x] 键盘快捷键
- [x] 版本检查

#### 7.7 通知
- [x] Push 通知（iOS/Android）
- [x] Toast 提示
- [x] Agent 状态变更通知

#### 7.8 桌面端特有
- [x] 桌面 daemon 集成
- [x] 窗口控制
- [x] 更新检查
- [x] 桌面权限管理

### ⚠️ 部分实现/占位

- [ ] **语音输入**：VoiceContext 存在但功能可能不完整
- [ ] **Chat 系统**：无独立 Chat 界面（通过 Agent 交互实现）
- [x] **Schedule**：✅ 已实现定时任务界面（创建/编辑/详情/列表）
- [ ] **Loop**：无循环工作流界面

### ❌ 未实现功能（与 Paseo 对比）

- [ ] **GitHub 集成**：无 PR、Issue 界面
- [ ] **语音合成（TTS）**：无语音输出
- [ ] **语音识别（STT）**：无语音转文字
- [ ] **Dictation**：无听写功能
- [ ] **File Download**：无独立下载界面
- [ ] **Workspace 归档**：无归档管理

---

## 八、UI 技术亮点

1. **跨平台适配**：一套代码支持 iOS、Android、Web
2. **响应式设计**：适配手机和桌面端
3. **手势支持**：文件浏览器滑动手势
4. **动画流畅**：React Native Reanimated 动画
5. **主题系统**：Unistyles 动态主题
6. **虚拟化**：长列表虚拟化渲染
7. **键盘支持**：完整的键盘快捷键系统
8. **无障碍**：支持屏幕阅读器

---

*分析完成于 2026-05-20*
*基于 App 前端代码遍历*
