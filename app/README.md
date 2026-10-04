# Solo App

Cross-platform client (iOS, Android, Web) for the Solo AI coding assistant platform.

## Tech Stack

- **Framework**: Expo 57 / React Native 0.86 / React 19.2
- **Routing**: Expo Router (file-based)
- **State**: Zustand + @tanstack/react-query
- **Styling**: Unistyles (dynamic theming)
- **Terminal**: @xterm/xterm v6
- **Testing**: Vitest (unit) + Playwright (E2E)

## Getting Started

```bash
# Install dependencies (from repo root)
npm install

# Start the web app (from repo root)
make dev-web

# Or start with npx
npx expo start --web
```

The app connects to the local daemon at `127.0.0.1:17612` by default.

## Project Structure

```
app/
├── src/
│   ├── app/              # Expo Router routes
│   │   ├── h/[serverId]/ # Per-host routes (agent, loops, schedules, sessions, settings, workspace, usage)
│   │   ├── settings/     # Global settings routes
│   │   ├── schedules.tsx
│   │   ├── tmux-dashboard.tsx
│   │   ├── tmux-pane.tsx
│   │   ├── tmux-pane-xterm.tsx
│   │   ├── usage.tsx
│   │   ├── pair-scan.tsx
│   │   └── welcome.tsx
│   ├── screens/          # Screen components
│   │   ├── agent/        # Agent detail and interaction
│   │   ├── dashboard/    # Main dashboard
│   │   ├── schedules/    # Schedule automation dashboard
│   │   ├── settings/     # Settings sections
│   │   ├── tmux-dashboard/ # Tmux agent discovery
│   │   ├── usage/        # Usage/quota dashboards
│   │   ├── workspace/    # Workspace management
│   │   └── *-screen.tsx  # Loop, session, project screens (top-level files)
│   ├── components/       # Reusable components
│   ├── hooks/            # Custom hooks
│   ├── stores/           # Zustand state stores
│   ├── contexts/         # React contexts
│   ├── styles/           # App themes + terminal theme presets
│   ├── terminal/         # Terminal emulation (xterm)
│   ├── desktop/          # Desktop-specific modules
│   ├── utils/            # Utility functions
│   └── constants/        # App constants
├── e2e/                  # Playwright E2E tests (44 specs)
├── maestro/              # Maestro mobile UI flows (Android)
└── assets/               # Images, fonts, icons
```

## Key Screens

| Screen | Description |
|--------|-------------|
| Dashboard | Host overview with agent status cards |
| Agent Detail | Agent interaction, timeline, streaming output |
| Schedules | Timezone-aware cron schedule management + AI assistant |
| Loops | Loop template CRUD, instance detail, execution tracking |
| Tmux Dashboard | AI agent discovery across tmux sessions |
| Tmux Pane | Live terminal view with ANSI rendering and key injection |
| Workspace | Project management, file explorer, git status |
| Usage | Per-provider quota, usage %, and reset countdown |
| Settings | Providers, tmux agents, terminal themes, LLM config, host version switching |

## Testing

```bash
# Unit tests (from repo root)
make test-app

# E2E tests (requires daemon + relay running)
cd app && npx playwright test
```

## Related Docs

- [Architecture Overview](../docs/architecture/README.md)
- [Component Specifications](../docs/architecture/components.md)
- [Product Features](../docs/product/features.md)
- [Maestro Flows](maestro/README.md)
