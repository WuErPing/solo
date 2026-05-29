---
name: update-module-version
description: Update module and package versions after code changes. Use when preparing a release, bumping versions after feature work or bug fixes, or coordinating protocol version changes across Go and TypeScript modules.
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

## Step 5: Verify Changes

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
