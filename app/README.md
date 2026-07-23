# Solo App

Cross-platform client (iOS, Android, Web) for the Solo AI coding assistant platform.

## Tech Stack

- **Framework**: Expo 57 / React Native 0.86 / React 19
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
│   │   ├── h/[serverId]/ # Per-host routes (agent, loops, schedules, sessions, settings, workspace)
│   │   ├── schedules.tsx
│   │   ├── tmux-dashboard.tsx
│   │   ├── tmux-pane.tsx
│   │   └── welcome.tsx
│   ├── screens/          # Screen components
│   │   ├── agent/        # Agent detail and interaction
│   │   ├── dashboard/    # Main dashboard
│   │   ├── loops/        # Loop automation screens
│   │   ├── schedules/    # Schedule automation dashboard
│   │   ├── settings/     # Settings sections
│   │   ├── tmux-dashboard/ # Tmux agent discovery
│   │   └── workspace/    # Workspace management
│   ├── components/       # Reusable components
│   ├── hooks/            # Custom hooks
│   ├── stores/           # Zustand state stores
│   ├── styles/           # Theme and style definitions
│   ├── utils/            # Utility functions
│   └── constants/        # App constants
├── e2e/                  # Playwright E2E tests
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
| Settings | Providers, tmux agents, keyboard shortcuts, operations |

## Testing

```bash
# Unit tests (from repo root)
make test-app

# E2E tests (requires daemon + relay running)
cd app && npx playwright test
```

## Dictation Debugging

Set `EXPO_PUBLIC_ENABLE_AUDIO_DEBUG=1` before running `npx expo start` to render the in-app audio debug card. Pair it with the server-side `STT_DEBUG_AUDIO_DIR` flag so every dictation includes a copyable path to the saved raw audio file.

## Related Docs

- [Architecture Overview](../docs/architecture/README.md)
- [Component Specifications](../docs/architecture/components.md)
- [Product Features](../docs/product/features.md)
- [Maestro Flows](maestro/README.md)
