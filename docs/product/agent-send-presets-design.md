# Agent-Specific Send Button Presets

## Problem

The tmux pane "Send" view (`app/src/screens/tmux-pane-screen.tsx`, lines 480-511) shows a hardcoded set of buttons identical for every agent: `Up`, `Down`, `Enter`, `Esc`, `Tab`, `S-Tab`, `1-4`. Different agents (claude, opencode, codex, kimi) have different common workflows and key bindings. The view should adapt to the detected agent.

## Design Decision: Hybrid Approach (Option D)

Ship sensible built-in defaults for known agents. Users can override per-agent via `config.json`. If user defines keys for an agent, built-in preset for that agent is fully replaced (**replace semantics**, not extend).

Merging happens **on the frontend** (TypeScript). Button layout is a UI concern — the backend passes through raw user config, the frontend owns built-in presets and renders the final list.

## Config Format

```jsonc
// ~/.solo/config.json
{
  "daemon": {
    "tmuxAgentKeys": {
      "claude": [
        { "label": "Esc", "key": "Escape" },
        { "label": "^C", "key": "C-c" },
        { "label": "/help", "text": "/help" },
        { "label": "/clear", "text": "/clear" }
      ],
      "my-custom-agent": [
        { "label": "F1", "key": "F1" },
        { "label": "status", "text": "status" }
      ]
    }
  }
}
```

Each button has:
- `label` — display text
- `key` — raw tmux key to send (sendEnter=false), OR
- `text` — literal string to send + Enter (sendEnter=true)

## Data Flow

```
config.json (user overrides via tmuxAgentKeys)
    ↓
Go daemon: read config, include in TmuxListAgentsResponse
    ↓
Frontend: merge built-in TS presets with user overrides
    ↓
Render dynamic buttons based on agent.agentName
```

## Merge Logic

```
if userConfig has keys for this agent → use user config (full replace)
else if built-in preset exists for this agent → use built-in
else → use default generic buttons (current behavior)
```

## Files To Change

| Layer | File | Change |
|-------|------|--------|
| Go config | `daemon/internal/config/config.go` | Add `TmuxAgentKeys` to `DaemonConfig` |
| Go protocol | `protocol/message_tmux.go` | Add `AgentKeys` field to `TmuxListAgentsResponsePayload` |
| Go handler | `daemon/internal/server/session_tmux.go` | Include config keys in response |
| TS config | `app/src/config/agent-send-presets.ts` (new) | Types, built-in presets, `getAgentSendButtons()` |
| TS screen | `app/src/screens/tmux-pane-screen.tsx` | Dynamic buttons, `sendText` helper |

## Built-In Presets (Draft)

| Agent | Buttons |
|-------|---------|
| claude | `Esc`, `^C`, `Enter`, `Tab`, `↑`, `↓`, `/help`, `/clear` |
| opencode | `Esc`, `Enter`, `Tab`, `↑`, `↓`, `?` |
| codex | `Esc`, `Enter`, `Tab`, `↑`, `↓` |
| default | `↑`, `↓`, `Enter`, `Esc`, `Tab`, `S-Tab`, `1`, `2`, `3`, `4` |

## Open Questions

1. Visual distinction between raw-key buttons and quick-send (text+Enter) buttons
2. Whether to cap preset size at one row (~6-8 buttons) or add overflow
