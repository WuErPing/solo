# Solo App UI Feature Detailed Analysis

> Analysis Date: 2026-05-25
> Codebase: /Users/wuerping/code/wuerping/solo/app
> Tech Stack: React Native / Expo / TypeScript

---

## 1. App Navigation Structure

App uses **Expo Router** as the navigation framework with file-system routing:

```
app/
├── _layout.tsx              # Root layout (global Provider, sidebar, command center)
├── index.tsx                # Entry point (startup bootstrapping, redirect)
├── welcome.tsx              # Welcome page (first-time use)
├── dashboard.tsx            # Dashboard
├── tmux-dashboard.tsx       # Tmux Dashboard (AI agent detection & control)
├── pair-scan.tsx            # QR code scanning / pairing
├── settings/
│   ├── index.tsx            # Settings home
│   ├── [section].tsx        # Settings category page
│   ├── hosts/
│   │   └── [serverId].tsx   # Host details
│   └── projects/
│       ├── index.tsx        # Project list
│       └── [projectKey].tsx # Project details
└── h/
    └── [serverId]/
        ├── index.tsx          # Host home (redirect to open-project)
        ├── new.tsx            # New workspace
        ├── open-project.tsx   # Open project
        ├── sessions.tsx       # Session list
        ├── settings.tsx       # Host settings
        ├── agent/
        │   └── [agentId].tsx  # Agent details (redirect to workspace)
        └── workspace/
            └── [workspaceId]/
                ├── _layout.tsx # Workspace layout
                └── index.tsx   # Workspace home
```

---

## 2. Screens

### 2.1 Startup & Onboarding

| Screen | File | Description |
|--------|------|-------------|
| **Startup Splash** | `startup-splash-screen.tsx` | App startup loading screen, shows loading state |
| **Welcome** | `welcome.tsx` | First-time use onboarding, introduces product features |

### 2.2 Main Interface

| Screen | File | Description |
|--------|------|-------------|
| **Dashboard** | `dashboard/dashboard-screen.tsx` | Main console, shows overview of all Agent statuses |
| **Tmux Dashboard** | `tmux-dashboard/tmux-dashboard-screen.tsx` | Detect and interact with AI agents in tmux sessions |
| **Project List** | `projects-screen.tsx` | View and manage all projects |
| **Open Project** | `open-project-screen.tsx` | Select and open a project |
| **Session List** | `sessions-screen.tsx` | View historical session records |

**Dashboard Details:**
- Agent status cards (running, idle, error, needs attention)
- Status filters (all, running, idle, error, needs permission)
- Agent list (name, path, status, last activity time)
- Quick actions (open workspace, view details)

**Tmux Dashboard Details:**
- Auto-detects AI agents in tmux sessions (claude, pi, kimi, kimi-cli, opencode, qoder, cursor)
- Three-layer detection: command name, pane title (unicode normalization), child process inspection
- Agent cards grouped by name with count badges; tap to filter by agent name
- Tap agent card to open pane content modal (live terminal view, last 500 lines, auto-refreshes every 5s)
- Quick-action key buttons: ↑↓←→, Enter, Esc, Tab, Ctrl+C, 1-4 (for TUI menu navigation)
- Text input for sending commands (appends Enter automatically)

### 2.3 Workspace

| Screen/Component | File | Description |
|------------------|------|-------------|
| **Workspace Main Screen** | `workspace/workspace-screen.tsx` | Multi-pane workspace layout |
| **Desktop Tabs Row** | `workspace-desktop-tabs-row.tsx` | Top tab bar (desktop) |
| **Pane Content** | `workspace-pane-content.tsx` | Pane content rendering |
| **Git Actions** | `workspace-git-actions.tsx` | Git branch switching, commit, etc. |
| **Scripts Button** | `workspace-scripts-button.tsx` | Run workspace scripts |
| **Open in Editor Button** | `workspace-open-in-editor-button.tsx` | Open in editor |

**Workspace Details:**
- **Multi-tab layout**: Supports multiple Agent/terminal/file tabs
- **Pane system**: Draggable, resizable panes
- **Git integration**: Branch display, switching, operations
- **File browser**: Sidebar file tree (ExplorerSidebar)
- **Script execution**: One-click workspace script running
- **Diff stats**: Code change statistics display
- **Workspace settings**: Configure workspace parameters

**Tab Types:**
- Agent tab (running AI Agent)
- Terminal tab (PTY terminal)
- File tab (code file editing)
- Draft tab (draft Agent)
- Setup tab (workspace settings)

### 2.4 Agent Interaction

| Screen/Component | File | Description |
|------------------|------|-------------|
| **Agent Ready Screen** | `agent-ready-screen-bottom-anchor.ts` | Agent route bottom anchor calculation |

**Agent Interface Features (via workspace Agent tab):**
- Real-time streaming output display
- Multi-modal input (text + attachments)
- Code block rendering and highlighting
- **Mermaid diagram preview**: Real-time Mermaid flowchart/sequence diagram rendering in Markdown file panels
- Tool call display
- Permission request handling
- Model selector (Claude, Kimi, OpenCode)

### 2.5 Settings

| Screen/Component | File | Description |
|------------------|------|-------------|
| **Settings Main Screen** | `settings-screen.tsx` | Settings home, category navigation |
| **Settings Section** | `settings-section.tsx` | Generic settings item component |
| **Host Page** | `settings/host-page.tsx` | Host configuration details |
| **Provider Config** | `settings/providers-section.tsx` | AI Provider configuration |
| **Keyboard Shortcuts** | `settings/keyboard-shortcuts-section.tsx` | Keyboard shortcut configuration |

**Settings Details:**
- **Theme switching**: Light / Dark / System theme
- **Send behavior**: Enter to send / Cmd+Enter to send
- **Host management**: Add / Edit / Delete hosts
- **Provider configuration**: Claude, OpenCode API config
- **Keyboard shortcuts**: Custom keyboard shortcuts
- **Version info**: App version, update check
- **Desktop permissions**: Desktop-specific permission settings

### 2.6 New Workspace

| Screen/Component | File | Description |
|------------------|------|-------------|
| **New Workspace** | `new-workspace-screen.tsx` | Create new workspace wizard |
| **Picker Item** | `new-workspace-picker-item.ts` | Workspace template selection |

**New Workspace Features:**
- Select project
- Select workspace template
- Configure workspace parameters
- Git worktree options

---

## 3. Core Components

### 3.1 Agent Components

| Component | File | Function |
|-----------|------|----------|
| **Agent List** | `agent-list.tsx` | Display Agent list |
| **Agent Status Bar** | `agent-status-bar.tsx` | Bottom status bar, shows current Agent status |
| **Agent Status Dot** | `agent-status-dot.tsx` | Status indicator dot |
| **Agent Stream View** | `agent-stream-view.tsx` | Real-time streaming output rendering |
| **Stream Render Model** | `agent-stream-render-model.ts` | Stream event rendering strategy |
| **Stream Render Strategy** | `agent-stream-render-strategy.ts` | Rendering logic for different event types |
| **Model Selector** | `combined-model-selector.tsx` | Select AI model |

### 3.2 Input Components

| Component | File | Function |
|-----------|------|----------|
| **Composer** | `composer.tsx` | Main input box, supports multi-modal |
| **Input Submit** | `agent-input-submit.ts` | Input submission logic |
| **Attachment Pill** | `attachment-pill.tsx` | Attachment tag display |
| **Attachment Lightbox** | `attachment-lightbox.tsx` | Attachment preview (images, files) |
| **SearchInput** | `search-input.tsx` | Search input box, browser autocomplete disabled to prevent duplicate suggestion overlay |

### 3.3 Navigation & Layout

| Component | File | Function |
|-----------|------|----------|
| **Left Sidebar** | `left-sidebar.tsx` | Main navigation sidebar |
| **Sidebar Workspace List** | `sidebar-workspace-list.tsx` | Workspace list |
| **Command Center** | `command-center.tsx` | Command palette (Cmd+K) |
| **Menu Header** | `headers/menu-header.tsx` | Top menu bar |
| **Screen Header** | `headers/screen-header.tsx` | Screen title bar |
| **Back Header** | `headers/back-header.tsx` | Header with back button |

### 3.4 Modals & Dialogs

| Component | File | Function |
|-----------|------|----------|
| **Add Host Method** | `add-host-method-modal.tsx` | Select host addition method |
| **Add Host** | `add-host-modal.tsx` | Enter host information |
| **Project Picker** | `project-picker-modal.tsx` | Select project |
| **Pair Link** | `pair-link-modal.tsx` | Display pairing link / QR code |
| **Tool Call Sheet** | `tool-call-sheet.tsx` | Display tool call details |
| **Adaptive Bottom Sheet** | `adaptive-modal-sheet.tsx` | Adaptive bottom sheet modal |
| **Workspace Setup Dialog** | `workspace-setup-dialog.tsx` | Workspace initialization settings |

### 3.5 Git Components

| Component | File | Function |
|-----------|------|----------|
| **Branch Switcher** | `branch-switcher.tsx` | Switch Git branch |
| **Diff Stat** | `diff-stat.tsx` | Display code change statistics |
| **Git Actions Panel** | `workspace-git-actions.tsx` | Commit, push, etc. |

### 3.6 File Browser

| Component | File | Function |
|-----------|------|----------|
| **Explorer Sidebar** | `explorer-sidebar.tsx` | File tree browser |
| **Code Insets** | `code-insets.ts` | Code snippet insertion |

### 3.7 UI Base Components

| Component | File | Function |
|-----------|------|----------|
| **Button** | `ui/button.tsx` | Base button |
| **Dropdown Menu** | `ui/dropdown-menu.tsx` | Dropdown menu |
| **Combobox** | `ui/combobox.tsx` | Searchable select box |
| **Tooltip** | `ui/tooltip.tsx` | Hover tooltip |
| **Shortcut** | `ui/shortcut.tsx` | Keyboard shortcut display |
| **Segmented Control** | `ui/segmented-control.tsx` | Segmented selector |
| **Context Menu** | `ui/context-menu.tsx` | Right-click menu |
| **Isolated Bottom Sheet** | `ui/isolated-bottom-sheet-modal.tsx` | Independent bottom sheet modal |

---

## 4. Context (Global State)

| Context | File | Function |
|---------|------|----------|
| **SessionContext** | `session-context.tsx` | Session state management |
| **ToastContext** | `toast-context.tsx` | Toast notifications |
| **VoiceContext** | `voice-context.tsx` | Voice input state |
| **HorizontalScroll** | `horizontal-scroll-context.tsx` | Horizontal scroll control |
| **SidebarAnimation** | `sidebar-animation-context.tsx` | Sidebar animation |
| **SidebarCallout** | `sidebar-callout-context.tsx` | Sidebar callout |
| **ExplorerSidebarAnimation** | `explorer-sidebar-animation-context.tsx` | File browser animation |

---

## 5. Hooks (Custom Hooks)

| Hook | File | Function |
|------|------|----------|
| **useAggregatedAgents** | `use-aggregated-agents.ts` | Aggregate all Agent states |
| **useAgentInputDraft** | `use-agent-input-draft.ts` | Agent input draft |
| **useExplorerOpenGesture** | `use-explorer-open-gesture.ts` | File browser open gesture |
| **useActiveWorktreeNewAction** | `use-active-worktree-new-action.ts` | Worktree new action |
| **useSettings** | `use-settings.ts` | App settings |

---

## 6. Stores (State Stores)

| Store | File | Function |
|-------|------|----------|
| **SessionStore** | `session-store.ts` | Session state (Zustand) |
| **PanelStore** | `panel-store.ts` | Panel state |
| **WorkspaceTabsStore** | `workspace-tabs-store.ts` | Workspace tab state |
| **WorkspaceLayoutStore** | `workspace-layout-store.ts` | Workspace layout state |
| **NavigationActiveWorkspaceStore** | `navigation-active-workspace-store.ts` | Navigation workspace selection |

---

## 7. App Product Feature Summary

### ✅ Implemented Features

#### 7.1 Connection Management
- [x] Multi-host management (add/edit/delete)
- [x] Direct connection (local daemon)
- [x] Relay remote connection
- [x] QR code pairing
- [x] Pairing link generation
- [x] Real-time connection status display

#### 7.2 Agent Management
- [x] Agent list view
- [x] Agent status monitoring (running, idle, error)
- [x] Agent real-time streaming output
- [x] Multi-modal input (text + attachments)
- [x] Code block rendering and highlighting
- [x] Tool call display
- [x] Permission request handling
- [x] Model selector (Claude, OpenCode)

#### 7.3 Workspace
- [x] Multi-workspace management
- [x] Multi-tab layout (Agent/terminal/file)
- [x] Draggable panes
- [x] File browser (tree structure)
- [x] Git integration (branch display, switching)
- [x] Script execution
- [x] Diff statistics
- [x] Open in editor

#### 7.4 Project Management
- [x] Project list
- [x] Project configuration
- [x] Open project
- [x] New workspace

#### 7.5 Terminal
- [x] PTY terminal (via workspace tab)
- [x] Multi-terminal support

#### 7.6 Settings
- [x] Theme switching (light/dark/system)
- [x] Send behavior configuration
- [x] Host configuration
- [x] Provider API configuration
- [x] Keyboard shortcuts
- [x] Version check

#### 7.7 Notifications
- [x] Push notifications (iOS/Android)
- [x] Toast notifications
- [x] Agent status change notifications

#### 7.8 Desktop-Specific
- [x] Desktop daemon integration
- [x] Window controls
- [x] Update check
- [x] Desktop permission management

### ⚠️ Partially Implemented / Placeholder

- [ ] **Voice input**: VoiceContext exists but functionality may be incomplete
- [ ] **Chat system**: No independent Chat UI (implemented via Agent interaction)
- [x] **Schedule**: ✅ Scheduled task UI implemented (create/edit/detail/list)
- [ ] **Loop**: No loop workflow UI

### ❌ Not Implemented (Compared to Paseo)

- [ ] **GitHub integration**: No PR, Issue UI
- [ ] **Text-to-Speech (TTS)**: No voice output
- [ ] **Speech-to-Text (STT)**: No speech recognition
- [ ] **Dictation**: No dictation feature
- [ ] **File Download**: No independent download UI
- [ ] **Workspace archiving**: No archive management

---

## 8. UI Technical Highlights

1. **Cross-platform adaptation**: Single codebase supports iOS, Android, Web
2. **Responsive design**: Adapts to mobile and desktop
3. **Gesture support**: File browser swipe gestures
4. **Smooth animations**: React Native Reanimated animations
5. **Theme system**: Unistyles dynamic theming
6. **Virtualization**: Long list virtualized rendering
7. **Keyboard support**: Complete keyboard shortcut system
8. **Accessibility**: Screen reader support

---

*Analysis completed on 2026-05-20*
*Based on App frontend code traversal*
