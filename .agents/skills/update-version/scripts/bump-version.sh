#!/usr/bin/env bash
# bump-version.sh — Bump version for a specific module
# Usage: ./bump-version.sh <module> <new-version>
#   module       One of: app, app-bridge, highlight, cli, daemon, relay-go
#   new-version  The new version string (e.g., 0.2.0, 0.1.1, relay-go-v2)

set -euo pipefail

MODULE="${1:-}"
NEW_VER="${2:-}"
REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
cd "$REPO_ROOT"

if [[ -z "$MODULE" || -z "$NEW_VER" ]]; then
  echo "Usage: $0 <module> <new-version>"
  echo "  Modules: app, app-bridge, highlight, cli, daemon, relay-go"
  exit 1
fi

case "$MODULE" in
  app)
    FILE="app/package.json"
    jq --arg v "$NEW_VER" '.version = $v' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
    echo "Updated $FILE → $NEW_VER"
    ;;
  app-bridge)
    FILE="app-bridge/package.json"
    jq --arg v "$NEW_VER" '.version = $v' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
    echo "Updated $FILE → $NEW_VER"
    ;;
  highlight)
    FILE="packages/highlight/package.json"
    jq --arg v "$NEW_VER" '.version = $v' "$FILE" > "$FILE.tmp" && mv "$FILE.tmp" "$FILE"
    echo "Updated $FILE → $NEW_VER"
    ;;
  cli)
    FILE="cli/cmd/root.go"
    sed -i '' "s/Version:[[:space:]]*\"[^\"]*\"/Version:       \"$NEW_VER\"/" "$FILE"
    echo "Updated $FILE → $NEW_VER"
    ;;
  daemon)
    FILE="daemon/internal/config/config.go"
    sed -i '' "s/Version:[[:space:]]*\"[^\"]*\"/Version:             \"$NEW_VER\"/" "$FILE"
    echo "Updated $FILE → $NEW_VER"
    ;;
  relay-go)
    FILE="relay-go/internal/relay/server.go"
    sed -i '' "s/const version = \"[^\"]*\"/const version = \"$NEW_VER\"/" "$FILE"
    echo "Updated $FILE → $NEW_VER"
    ;;
  *)
    echo "Unknown module: $MODULE"
    echo "Valid modules: app, app-bridge, highlight, cli, daemon, relay-go"
    exit 1
    ;;
esac
