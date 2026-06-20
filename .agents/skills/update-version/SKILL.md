---
name: update-version
description: Update module and package versions after code changes, and update CHANGELOG.md for the target version by aggregating commit messages since the previous release. Use when preparing a release, bumping versions after feature work or bug fixes, or coordinating protocol version changes across Go and TypeScript modules.
---

# Update Module Version

## Overview

Solo has multiple independently versioned modules. This skill ensures version bumps are applied consistently and correctly across:

- **npm workspaces**: `app`, `app-bridge`, `packages/highlight`
- **Go modules**: `cli`, `daemon`, `relay-go`, `protocol`
- **Protocol constants**: `WSProtocolVersion`, `RelayProtocolVersion`

## When to Use

- After making changes that warrant a version bump
- Before cutting a release or building binaries
- When protocol changes require coordinated version updates
- When fixing a bug that affects published artifacts

**When NOT to use:**

- Pure refactoring with no behavioral change (skip the bump)
- Changes only to tests, docs, or CI configuration
- Changes that haven't been committed yet

## Helper Scripts

This skill includes two helper scripts in `.agents/skills/update-module-version/scripts/`:

- `check-versions.sh [REF]` — Print current versions and show which modules changed since `REF` (default `HEAD~1`).
- `bump-version.sh <module> <new-version>` — Bump a single module's version string in its source file.

## Step 1: Identify Changed Modules

Run from the repo root to see which modules have changes since the last version bump:

```bash
# Show changed files per module
echo "=== app ==="       && git diff --name-only HEAD~1 -- app/       | head -5
echo "=== app-bridge ===" && git diff --name-only HEAD~1 -- app-bridge/ | head -5
echo "=== packages/highlight ===" && git diff --name-only HEAD~1 -- packages/highlight/ | head -5
echo "=== cli ==="      && git diff --name-only HEAD~1 -- cli/      | head -5
echo "=== daemon ==="   && git diff --name-only HEAD~1 -- daemon/   | head -5
echo "=== relay-go ===" && git diff --name-only HEAD~1 -- relay-go/ | head -5
echo "=== protocol ===" && git diff --name-only HEAD~1 -- protocol/ | head -5
```

If no files appear for a module, skip it.

## Step 2: Determine Bump Type

For each changed module, classify the most significant change:

| Change Type | Semver Bump | Examples |
|-------------|-------------|----------|
| Breaking API / protocol change | **MAJOR** | Modified message structs, changed CLI flags, removed exported functions, protocol wire format changes |
| New feature / additive change | **MINOR** | New provider support, new CLI command, new protocol message type |
| Bug fix / internal improvement | **PATCH** | Fixed race condition, corrected error handling, performance tweak |
| Docs / tests / CI only | **NONE** | Skip bump |

> **Protocol changes are special**: Any change to `protocol/` message structs or constants is **breaking** for cross-version compatibility and requires a `MAJOR` bump (or at minimum a `MINOR` with coordinated protocol version increment).

## Step 3: Apply Version Bumps

### npm Workspaces

Edit `version` in the corresponding `package.json`:

| Module | File |
|--------|------|
| app | `app/package.json` |
| app-bridge | `app-bridge/package.json` |
| highlight | `packages/highlight/package.json` |

```bash
# Using the helper script
.agents/skills/update-module-version/scripts/bump-version.sh app 0.2.0

# Or manually
node -e "
  const fs = require('fs');
  const p = JSON.parse(fs.readFileSync('app/package.json', 'utf8'));
  p.version = '0.2.0';
  fs.writeFileSync('app/package.json', JSON.stringify(p, null, 2) + '\\n');
"
```

### Go Modules

Go modules in this repo use **hardcoded version strings** (not git tags). Use the helper script or edit the source file directly:

| Module | File | Variable / Field |
|--------|------|------------------|
| cli | `cli/cmd/root.go` | `Version:` field in root command |
| daemon | `daemon/internal/config/config.go` | `Version:` field in default config |
| relay-go | `relay-go/internal/relay/server.go` | `version` constant |
| protocol | `protocol/protocol.go` | `WSProtocolVersion`, `RelayProtocolVersion` (only when wire format changes) |

> **Important**: `protocol/` has no semantic module version string. It only has protocol version constants. Only bump those when the wire format changes.
>
> Use `bump-version.sh` for automated edits:
> ```bash
> .agents/skills/update-module-version/scripts/bump-version.sh cli 0.2.0
> .agents/skills/update-module-version/scripts/bump-version.sh daemon 0.2.0
> .agents/skills/update-module-version/scripts/bump-version.sh relay-go relay-go-v2
> ```

## Step 4: Coordinate Cross-Module Versions

When `protocol/` constants change, **all consumers must be updated**:

1. Bump `WSProtocolVersion` or `RelayProtocolVersion` in `protocol/protocol.go`
2. Bump version in `daemon/internal/config/config.go` (daemon is the protocol server)
3. Bump version in `cli/cmd/root.go` (CLI pairs with daemon)
4. Bump `app/package.json` version (app is the protocol client)
5. Run tests to verify compatibility:
   ```bash
   make darwin
   go test -short -race ./...
   cd app && npm test
   ```

## Step 5: Update CHANGELOG.md

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). For every release, add a new section for the target version that aggregates all commits since the previous release. The CHANGELOG edit is part of the version-bump commit (do not commit it separately).

### 5.1 Determine the target version and previous release tag

The target version is the primary bumped version — the `app` version, since release tags track it as `v<app-version>` (e.g. `v0.7.0`). The previous release tag is the most recent tag reachable from `HEAD`:

```bash
PREV_TAG=$(git describe --tags --abbrev=0)        # e.g. v0.6.5
TARGET_VERSION=$(grep '"version"' app/package.json | head -1 | sed 's/.*"\([0-9.]*\)".*/\1/')  # e.g. 0.7.0
echo "Releasing $TARGET_VERSION (previous $PREV_TAG)"
```

### 5.2 Aggregate commits since the previous release

List every commit subject since the previous tag:

```bash
git log --pretty=format:"- %s" "$PREV_TAG"..HEAD
```

### 5.3 Group into Keep a Changelog sections

Map each commit's conventional-commit type to a section, and drop non-user-facing types:

| Commit type | CHANGELOG section |
|-------------|-------------------|
| `feat` | **Added** |
| `fix` | **Fixed** |
| `refactor`, `perf` | **Changed** |
| `revert`, `remove` | **Removed** |
| `docs`, `test`, `chore`, `build`, `ci`, `style` | omit (not user-facing) |

For each kept commit, format as `- **<scope>**: <subject>`, where `<scope>` is the conventional-commit scope (the part in parentheses) and `<subject>` is the message with the `type(scope): ` prefix and any leading `- ` stripped. Merge multiple bullets from one commit into a single line where sensible. Combine closely related commits into one bullet (e.g. several `test(tmux)` commits become one "Updated tmux screen and browser tests" bullet under the relevant section).

### 5.4 Prepend the new section to CHANGELOG.md

Insert a new section immediately after the `## [Unreleased]` line (or, if no Unreleased section exists, immediately after the header preamble), above the previous release section. Use today's date (`YYYY-MM-DD`):

```markdown
## [0.7.0] - 2026-06-20

### Added

- **Tmux pane**: render full native pane content with scale-to-fit and in-DOM 1:1 horizontal scroller
- **Terminal**: `fitToWidth` runtime support for scaling the native grid to fit the screen

### Fixed

- **Terminal**: make the root div the horizontal scroller so panning works inside the DOM iframe/WebView
- **Terminal**: eliminate snapshot flicker with in-place repaint (no full reset on every poll)

### Changed

- **Tmux pane**: stop requesting rewrapped cols from the daemon; render the native grid directly
```

Only include sections that have at least one entry. Omit empty sections.

### 5.5 Stage the CHANGELOG with the version bump

```bash
git add CHANGELOG.md app/package.json app-bridge/package.json   # plus any other bumped modules
```

Then commit everything together as the release commit (see Step 6 verification, then commit/tag/push per the user's release flow).

## Step 6: Verify Changes

After editing, confirm the new versions:

```bash
# npm
grep -h '"version"' app/package.json app-bridge/package.json packages/highlight/package.json

# Go
grep -n 'Version.*=' cli/cmd/root.go daemon/internal/config/config.go
grep -n 'const version' relay-go/internal/relay/server.go
grep -n 'ProtocolVersion.*=' protocol/protocol.go
```

## Quick Reference: Version Locations

```
solo/
├── app/package.json                          → @getsolo/app version
├── app-bridge/package.json                   → @solo/app-bridge version
├── packages/highlight/package.json           → @getsolo/highlight version
├── cli/cmd/root.go                           → CLI version (cobra root cmd)
├── daemon/internal/config/config.go          → Daemon version (default config)
├── relay-go/internal/relay/server.go         → Relay version (const string)
└── protocol/protocol.go                      → WSProtocolVersion, RelayProtocolVersion
```

## Example: Patch Release After Bug Fix

A race condition was fixed in `daemon/internal/server/`:

1. Only `daemon/` files changed → bump daemon only.
2. Bug fix → **PATCH** bump: `0.1.0` → `0.1.1`.
3. Edit `daemon/internal/config/config.go`: change `"0.1.0"` → `"0.1.1"`.
4. Verify: `grep 'Version' daemon/internal/config/config.go`.

## Example: Protocol Change Release

A new field was added to a protocol message struct:

1. `protocol/` changed → bump protocol constant (if wire format changes) and all consumers.
2. Breaking/additive protocol change → **MINOR** or **MAJOR** for all modules.
3. Update `protocol/protocol.go` → increment `WSProtocolVersion` or `RelayProtocolVersion`.
4. Update all Go module version strings and npm package versions.
5. Run full test suite.

## Changelog

### v2 — 2026-06-18 (current)

Updated from initial version (v1, commit `05b33fa`). No changes to skill logic or scripts — this changelog documents version state changes in the repo since the skill was created.

**Version changes since v1:**

| Module | v1 (05b33fa) | Current | Bump commits |
|--------|-------------|---------|--------------|
| app | 0.1.0 | 0.6.4 | `99e9078` → 0.2.0, `9204ee7` → 0.5.0, `9e1c11c` → 0.6.0, `daeffbd` → 0.6.3, `c7924a8` → 0.6.4 |
| app-bridge | 0.1.0 | 0.2.1 | `99e9078` → 0.2.0, `66b1d67` → 0.2.1 |
| highlight | 0.1.0 | 0.2.0 | bumped alongside app-bridge |
| daemon | 0.1.0 | 0.2.0 | `66b1d67` → 0.2.0 |
| cli | 0.1.0 | 0.1.0 | — |
| relay-go | relay-go-v1 | relay-go-v1 | — |
| protocol WSProtocolVersion | 1 | 1 | — |
| protocol RelayProtocolVersion | "2" | "2" | — |

**Notable changes affecting versioning workflow:**
- `99e9078` — first coordinated multi-module bump (app + app-bridge + highlight to 0.2.0)
- `9204ee7` — app jumped to 0.5.0 (tmux agent name customization, MIT LICENSE added)
- `9e1c11c` — app to 0.6.0 (loop CRUD end-to-end, CLI refactor)
- `66b1d67` — daemon bumped from 0.1.0 to 0.2.0 (workspace branch fixes, tmux agent detection, Solo-managed agent counts); app-bridge to 0.2.1 (removeProject RPC)
- `c7924a8` — app to 0.6.4 (sidebar agent count fix, stale git branch name fix, tmux pane deduplication)

### v1 — 2026-06-01 (initial)

Initial skill creation at commit `05b33fa`. Included:
- `SKILL.md` with 5-step workflow (identify → classify → apply → coordinate → verify)
- `scripts/check-versions.sh` — print current versions and changed modules
- `scripts/bump-version.sh` — bump a single module's version string
- Version locations reference for all 7 modules (app, app-bridge, highlight, cli, daemon, relay-go, protocol)
- Examples for patch release and protocol change release
