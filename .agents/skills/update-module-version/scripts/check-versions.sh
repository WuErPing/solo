#!/usr/bin/env bash
# check-versions.sh — Show current module versions and detect changed modules
# Usage: ./check-versions.sh [REF]
#   REF  Git ref to compare against (default: HEAD~1)

set -euo pipefail

REF="${1:-HEAD~1}"
REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
cd "$REPO_ROOT"

echo "=========================================="
echo "  Solo Module Version Report"
echo "  Comparing against: $REF"
echo "=========================================="
echo ""

# --- Current versions ---
echo "--- Current Versions ---"
echo ""

# npm
for pkg in app app-bridge packages/highlight; do
  name=$(jq -r '.name' "$pkg/package.json")
  ver=$(jq -r '.version' "$pkg/package.json")
  printf "  %-25s %-20s %s\n" "$pkg" "$name" "$ver"
done

# Go (portable sed, no grep -P)
echo ""
cli_ver=$(sed -n 's/.*Version:[[:space:]]*"\([^"]*\)".*/\1/p' cli/cmd/root.go | head -1 || echo "?")
daemon_ver=$(sed -n 's/.*Version:[[:space:]]*"\([^"]*\)".*/\1/p' daemon/internal/config/config.go | head -1 || echo "?")
relay_ver=$(sed -n 's/.*const version = "\([^"]*\)".*/\1/p' relay-go/internal/relay/server.go | head -1 || echo "?")
ws_proto=$(sed -n 's/.*WSProtocolVersion[[:space:]][[:space:]]*int[[:space:]]*=[[:space:]]*\([0-9][0-9]*\).*/\1/p' protocol/protocol.go | head -1 || echo "?")
relay_proto=$(sed -n 's/.*RelayProtocolVersion[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' protocol/protocol.go | head -1 || echo "?")

printf "  %-25s %-20s %s\n" "cli/" "solo-cli" "$cli_ver"
printf "  %-25s %-20s %s\n" "daemon/" "solo-daemon" "$daemon_ver"
printf "  %-25s %-20s %s\n" "relay-go/" "solo-relay" "$relay_ver"
printf "  %-25s %-20s %s\n" "protocol/" "WSProtocolVersion" "$ws_proto"
printf "  %-25s %-20s %s\n" "protocol/" "RelayProtocolVersion" "$relay_proto"

# --- Changed modules ---
echo ""
echo "--- Changes since $REF ---"
echo ""

modules=("app" "app-bridge" "packages/highlight" "cli" "daemon" "relay-go" "protocol")
changed=()

for mod in "${modules[@]}"; do
  files=$(git diff --name-only "$REF" -- "$mod" 2>/dev/null || true)
  if [[ -n "$files" ]]; then
    changed+=("$mod")
    count=$(echo "$files" | wc -l | tr -d ' ')
    echo "  [$mod] $count file(s) changed"
    echo "$files" | sed 's/^/    /' | head -5
    [[ "$count" -gt 5 ]] && echo "    ... and $((count - 5)) more"
  fi
done

if [[ ${#changed[@]} -eq 0 ]]; then
  echo "  No changes detected in any module."
fi

echo ""
echo "=========================================="
echo "  Done."
echo "=========================================="
