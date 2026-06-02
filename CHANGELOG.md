# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-06-03

### Added

- **Timezone-aware cron scheduling**: Users input cron time in their local timezone; frontend converts to UTC for storage; backend evaluates UTC expressions directly; display converts back to local time in 24-hour format
- **Timezone field to ScheduleCadence protocol** (Go + TypeScript)
- **Cron-timezone utilities**: `detectTimezone`, `cronToUTC`, `cronFromUTC`, `describeCron`
- **fixupNextRunAt**: Self-heal stale stored values on daemon load
- **Redesigned create/edit modals**: Frequency presets, time input, timezone display
- **Friendly cadence text**: Display "每天 00:25" and raw UTC expression in detail screen
- **Local timezone display**: Timestamps in local timezone with 24-hour format (zh-CN locale)

### Fixed

- **NextRunAt double-conversion bug**: Evaluate cron in UTC since expression is already UTC

### Changed

- **Schedule Management UI**: Fully implemented with timezone-aware scheduling
- **Documentation**: Updated to reflect schedule feature completion and timezone support

## [0.1.0] - 2026-06-01

### Added

- Initial release of Solo AI coding assistant platform
- **AI Agent system**: Multi-provider support (Claude, Kimi, OpenCode, Pi, Mock)
- **Session management**: WebSocket multi-socket architecture with graceful reconnection
- **Workspace integration**: Git workflow, terminal, file operations
- **Cross-platform client**: iOS/Android/Web with React Native/Expo
- **Relay server**: E2EE encrypted remote connectivity
- **CLI tool**: Daemon, agent, and provider management
- **Push notifications**: Expo Push API integration
- **Testing**: 207 app unit tests, 129 daemon tests, 30 E2E tests
