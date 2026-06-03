# iTerm2 Agent Observation Analysis

**Date:** 2026-06-03
**Status:** Analysis Complete
**Priority:** High

---

## 1. Agent Observation Analysis

### User Requirement

**Scenario:**
- AI Coding Agent runs on **iTerm2 terminal**
- User wants to observe these agents from **Solo APP and Web pages**
- User wants to **interactively control** these agents from Solo

**Architecture:**
```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│  Solo App/Web   │ ◄─────► │     Daemon      │ ◄─────► │    iTerm2       │
│  (Observe/Interact)│  WS     │   (:17612)      │  API    │  (Agent运行)    │
└─────────────────┘         └─────────────────┘         └─────────────────┘
```

---

## 2. Proposed Solutions

### Solution A: iTerm2 Python API + Daemon Bridge

**Architecture:**
```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│  Solo App/Web   │ ◄─WS──► │     Daemon      │ ◄─WS──► │ iTerm2 Python   │
│                 │         │                 │         │ Script          │
└─────────────────┘         └─────────────────┘         └────────┬────────┘
                                                                 │
                                                        ┌────────▼────────┐
                                                        │    iTerm2       │
                                                        │  (Agent运行)    │
                                                        └─────────────────┘
```

**Implementation Steps:**
1. Daemon starts iTerm2 Python script on startup
2. Python script monitors iTerm2 Agent sessions
3. Forwards terminal content to Daemon
4. Daemon serves it to Solo App/Web via WebSocket
5. User interactions: Solo App/Web → Daemon → Python script → iTerm2

**Pros:**
- Full iTerm2 feature support
- Real-time observation
- Can control iTerm2 sessions

**Cons:**
- Requires iTerm2 running Python
- More complex implementation

### Solution B: iTerm2 WebSocket API

**Architecture:**
```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│  Solo App/Web   │ ◄─WS──► │     Daemon      │ ◄─WS──► │ iTerm2 WS       │
│                 │         │                 │         │ Server          │
└─────────────────┘         └─────────────────┘         └─────────────────┘
```

**Implementation Steps:**
1. Enable iTerm2 WebSocket server
2. Daemon connects to iTerm2 WebSocket
3. Read/write terminal content
4. Serve via Solo App/Web

**Pros:**
- Real-time
- Direct connection

**Cons:**
- Requires iTerm2 configuration
- Limited to iTerm2

### Solution C: tmux Control Mode

**Architecture:**
```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│  Solo App/Web   │ ◄─WS──► │     Daemon      │ ◄─WS──► │     tmux        │
│                 │         │                 │         │  control mode   │
└─────────────────┘         └─────────────────┘         └────────┬────────┘
                                                                 │
                                                        ┌────────▼────────┐
                                                        │    iTerm2       │
                                                        │  (Agent运行)    │
                                                        └─────────────────┘
```

**Implementation Steps:**
1. Agent runs in tmux session
2. Daemon uses tmux control mode to observe/control
3. Serve via Solo App/Web

**Pros:**
- Cross-platform
- Simple implementation

**Cons:**
- Requires tmux
- Less integrated with iTerm2

---

## 3. Implementation Plan

### Phase 3: iTerm2 Integration (1-2 weeks)

1. Implement iTerm2 Python API bridge
2. Add terminal content forwarding
3. Add interactive control (write to terminal)
4. Handle iTerm2-specific features (marks, triggers)

**Key Files:**
- `daemon/internal/iterm2/bridge.go`
- `daemon/internal/iterm2/python_script.py`
- `protocol/message_iterm2.go`

---

## 4. Technical Details

### iTerm2 Python API

iTerm2 has a comprehensive Python API that can:
- Read terminal content
- Write to terminal
- Get/set selection
- Monitor changes
- Control sessions
- Access metadata

**Example Python Script:**
```python
#!/usr/bin/env python3
import iterm2
import asyncio
import json
import websockets

async def main():
    connection = await iterm2.Connection.async_create()
    
    @connection.async_add_tab_change_monitor
    async def on_tab_change(tab):
        # Forward terminal content to daemon
        for session in tab.sessions:
            content = await session.async_get_contents()
            # Send to daemon via WebSocket
    
    @connection.async_add_keystroke_monitor
    async def on_keystroke(keystroke):
        # Handle user input from Solo App/Web
        pass

asyncio.run(main())
```

### Interactive Control Details

**Scenario:** Agent presents a choice prompt (e.g., `Continue? 1. Yes  2. No  3. Retry`) and user selects an option from Solo App/Web.

| Capability | Solution A | Solution B | Solution C |
|------------|-----------|-----------|-----------|
| **Read prompt/options** | `session.async_get_screen_contents()` returns full screen buffer including TUI state | Receive terminal output stream via WS | `capture-pane` snapshots pane content (may miss transient TUI frames) |
| **Send text choice** | `session.async_send_text("1\n")` — plain text with newline | Send data payload over WS | `send-keys "1" "Enter"` |
| **Send special keys** | Full support: arrows, Tab, Esc, Ctrl+C, etc. via `async_send_text()` with control sequences | Limited — depends on WS API surface | Partial — `send-keys` supports `Left`, `Right`, `Enter`, `Escape`, `C-c`, etc. |
| **Detect prompt state** | ✅ Marks / triggers / cursor position tracking | ❌ Not available | ❌ Not available |
| **Real-time feedback loop** | < 100ms (Python API is local) | < 100ms (direct WS) | 100-300ms (tmux command round-trip) |

### AI Agent CLI Discovery

**Goal:** Automatically detect which terminal sessions are running AI coding agents (e.g., `claude`, `opencode`, `qoder`, `pi`, `cursor`, `kimi`, `kimi-cli`, etc.).

**Detection Methods by Solution:**

| Method | Solution A | Solution B | Solution C |
|--------|-----------|-----------|-----------|
| **Process name matching** | `session.async_get_variable("jobName")` returns foreground command (e.g., `"claude"`, `"opencode"`) | Session metadata if exposed by WS API | `#{pane_current_command}` (e.g., `"claude"`, `"node"`) |
| **Process tree inspection** | ✅ Use `ps` on session PID to find child processes | ❌ Not available | ✅ Use `ps` on `#{pane_pid}` |
| **Window / session title** | `session.async_get_variable("terminalWindowName")` | WS session name | `#{window_name}` / `#{pane_title}` |
| **Auto-mark / trigger** | ✅ iTerm2 triggers can tag session when agent starts | ❌ | ❌ |
| **Badge / annotation** | ✅ `session.async_set_badge("🤖")` on detection | ❌ | ❌ |

**Known AI Agent CLI Signatures:**

| Tool | Process Name | Typical Arguments | Distinguishing Features |
|------|-------------|-------------------|------------------------|
| **Claude Code** | `claude` | `claude`, `claude "do something"` | Sets `TERM=screen` or `TERM=xterm-256color`; spawns node child processes |
| **OpenCode** | `opencode` | `opencode`, `opencode -p "prompt"` | Go binary; may spawn `git` subprocesses frequently |
| **Qoder** | `qoder` | `qoder`, `qoder --task "..."` | Often runs inside Docker or npx |
| **Pi** | `pi` | `pi`, `pi "prompt"` | AI assistant CLI |
| **Cursor** | `cursor` | Terminal within Cursor IDE | Runs inside Electron; `jobName` may show `node` or `cursor` |
| **Kimi** | `kimi` | `kimi`, `kimi "prompt"` | Kimi AI assistant CLI |
| **Kimi CLI** | `kimi-cli` | `kimi-cli`, `kimi-cli "prompt"` | Kimi CLI variant |

**Recommended Detection Logic (Solution A):**

```python
import iterm2
import asyncio
import subprocess

AI_AGENT_NAMES = {"claude", "opencode", "qoder", "pi", "cursor", "kimi", "kimi-cli"}

async def discover_ai_agents(connection):
    agents = []
    app = await iterm2.async_get_app(connection)
    for window in app.terminal_windows:
        for tab in window.tabs:
            for session in tab.sessions:
                job_name = await session.async_get_variable("jobName")
                if job_name in AI_AGENT_NAMES:
                    # Verify by checking full process tree
                    pid = await session.async_get_variable("sessionPID")
                    procs = subprocess.run(
                        ["ps", "-o", "comm=", "-g", str(pid)],
                        capture_output=True, text=True
                    )
                    detected = any(
                        name in procs.stdout
                        for name in AI_AGENT_NAMES
                    )
                    if detected:
                        await session.async_set_badge("🤖")
                        agents.append({
                            "session_id": session.session_id,
                            "agent": job_name,
                            "tab": tab.tab_id
                        })
    return agents
```

**Recommended Detection Logic (Solution C — tmux with Go library):**

Go 库 `github.com/GianlucaP106/gotmux` 提供了类型安全的 tmux 封装，支持 session/window/pane 管理和按键发送：

```go
package main

import (
    "fmt"
    "strings"
    "github.com/GianlucaP106/gotmux/gotmux"
)

var agentNames = []string{"claude", "opencode", "qoder", "pi", "cursor", "kimi", "kimi-cli"}

func main() {
    tmux, err := gotmux.DefaultTmux()
    if err != nil {
        panic(err)
    }

    sessions, _ := tmux.ListSessions()
    for _, session := range sessions {
        for _, window := range session.Windows {
            for _, pane := range window.Panes {
                cmd := pane.CurrentCommand // e.g., "claude", "node"
                for _, agent := range agentNames {
                    if strings.Contains(cmd, agent) {
                        fmt.Printf("Detected %s in pane %s (PID: %d)\n",
                            agent, pane.Id, pane.Pid)
                        // Send interactive selection
                        pane.SendKeys("2")   // select option 2
                        pane.SendKeys("Enter")
                    }
                }
            }
        }
    }
}
```

**`gotmux` 的能力与局限：**

| 能力 | 支持 | 说明 |
|------|------|------|
| Pane 枚举 + 当前命令 | ✅ | `pane.CurrentCommand` / `pane.Pid` |
| 发送文本/按键 | ✅ | `pane.SendKeys("...")` |
| 获取 pane 内容 | ⚠️ 间接 | 库本身未封装 `capture-pane`，需配合 `tmux` 命令调用 |
| Control Mode 持续连接 | ❌ | 基于命令调用（`os/exec`），非 `-C` 协议 |
| 实时流式输出 | ❌ | 需自行轮询 `capture-pane` 或实现 control mode |

**纯 Shell 检测（不依赖 Go 库）：**

```bash
# List all panes and filter by current command
tmux list-panes -a -F "#{pane_id} #{pane_current_command} #{pane_pid} #{window_name}" | \
  awk '$2 ~ /^(claude|opencode|qoder|pi|cursor|kimi|kimi-cli)$/ {print $0}'

# Recursively inspect process tree for each detected pane
inspect_tree() {
  local pid=$1 indent=${2:-0}
  local prefix="$(printf '%*s' "$indent" '')"
  ps -o pid=,comm= -p "$pid" 2>/dev/null | while IFS= read -r line; do
    echo "${prefix}${line}"
  done
  pgrep -P "$pid" 2>/dev/null | while IFS= read -r child; do
    inspect_tree "$child" $((indent + 2))
  done
}

tmux list-panes -a -F "#{pane_pid} #{pane_current_command}" | \
  awk '$2 ~ /^(claude|opencode|qoder|pi|cursor|kimi|kimi-cli)$/ {print $1}' | \
  while IFS= read -r pid; do
    echo "=== Agent process tree (PID: $pid) ==="
    inspect_tree "$pid"
  done
```

**Example — Selecting option "2" via each solution:**

```python
# Solution A: iTerm2 Python API
async def select_option(session, option_text: str):
    # Option 1: send the number directly
    await session.async_send_text("2\n")
    # Option 2: navigate with arrows and confirm
    # await session.async_send_text("\x1b[B\n")  # Down arrow + Enter
```

```javascript
// Solution B: iTerm2 WebSocket API (conceptual)
ws.send(JSON.stringify({
  type: "input",
  data: "2\r"   // or raw control sequences for arrow keys
}));
```

```bash
# Solution C: tmux Control Mode
tmux send-keys -t agent-session "2" "Enter"
# or for arrow navigation:
# tmux send-keys -t agent-session "Down" "Enter"
```

### Daemon WebSocket Protocol

**New Messages:**

```go
// ObserveAgentRequest - Subscribe to agent events
type ObserveAgentRequest struct {
    Type      string  `json:"type"`
    AgentID   string  `json:"agentId"`
    DeviceID  *string `json:"deviceId,omitempty"`
}

// AgentTerminalContent - Terminal content from iTerm2
type AgentTerminalContent struct {
    Type      string `json:"type"`
    AgentID   string `json:"agentId"`
    Content   string `json:"content"`
    CursorX   int    `json:"cursorX"`
    CursorY   int    `json:"cursorY"`
    Timestamp string `json:"timestamp"`
}

// AgentTerminalInput - User input to iTerm2
type AgentTerminalInput struct {
    Type    string `json:"type"`
    AgentID string `json:"agentId"`
    Input   string `json:"input"`
}
```

---

## 5. Approach Comparison

| Dimension | Solution A: iTerm2 Python API | Solution B: iTerm2 WebSocket API | Solution C: tmux Control Mode |
|-----------|-------------------------------|----------------------------------|-------------------------------|
| **Real-time** | ✅ Yes | ✅ Yes | ✅ Yes |
| **iTerm2-specific features** | ✅ Full (marks, triggers, metadata) | ⚠️ Limited | ❌ None |
| **Cross-platform** | ❌ iTerm2 only | ❌ iTerm2 only | ✅ Any terminal |
| **Implementation complexity** | High | Medium | Low |
| **Setup requirement** | iTerm2 + Python runtime | iTerm2 WebSocket server config | tmux installed |
| **Terminal control** | ✅ Read + Write + Control sessions | ✅ Read + Write | ✅ Read + Write |
| **Agent session detection** | ✅ Automatic (via iTerm2 API) | ⚠️ Manual configuration | ✅ Automatic (via tmux session) |
| **AI agent CLI discovery** | ✅ Process tree + job name + triggers | ⚠️ Limited (session-level only) | ⚠️ Process name only (`pane_current_command`) |
| **Interactive selection support** | ✅ Full (send text / special keys / detect prompts) | ⚠️ Basic (text input, limited special keys) | ⚠️ Basic (`send-keys` text, some special keys) |
| **Best for** | Deep iTerm2 integration, production use | Quick setup on iTerm2 | Multi-terminal / cross-platform environments |

---

## 6. Recommended Approach

**Recommended:** Solution A (iTerm2 Python API + Daemon Bridge)

**Reasons:**
1. Full iTerm2 feature support
2. Real-time observation
3. Can control iTerm2 sessions
4. Most flexible for future features

---

## 7. References

- [iTerm2 Python API Documentation](https://iterm2.com/python-api/)
- [iTerm2 WebSocket API](https://iterm2.com/documentation-websocket-api.html)
- [tmux Control Mode](https://man7.org/linux/man-pages/man1/tmux.1.html)
