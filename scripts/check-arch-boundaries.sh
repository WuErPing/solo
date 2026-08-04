#!/bin/bash
# CI lint: enforce the Go module boundaries documented in .agents/rules/architecture.md.
#
#   protocol/   ← zero dependencies on other solo modules, imported by all
#   daemon/     ← may import protocol and usage
#   cli/        ← may import protocol only (talks to daemon via WebSocket)
#   relay-go/   ← may import protocol only (stateless relay, no business logic)
#   usage/      ← zero dependencies on other solo modules, imported by daemon only
#
# Uses `go list -deps` so it checks the real import graph, not text matches.
set -euo pipefail

MODULES=(protocol daemon cli relay-go usage)
ERRORS=0
VIOLATIONS=""

for module in "${MODULES[@]}"; do
  if [ ! -d "$module" ]; then
    echo "ERROR: Module directory not found: $module" >&2
    exit 1
  fi

  # Every solo package this module depends on (transitively), excluding itself.
  deps=$(cd "$module" && go list -deps ./... 2>/dev/null \
    | grep -E '^github\.com/WuErPing/solo/(protocol|daemon|cli|relay|usage)(/|$)' \
    | sort -u || true)

  # Determine each module's own import path prefix from its go.mod.
  # (`go list -m` lists every workspace module when go.work is present.)
  self=$(awk '/^module /{print $2; exit}' "$module/go.mod")

  while IFS= read -r dep; do
    [ -z "$dep" ] && continue
    # Skip the module's own packages.
    case "$dep" in
      "$self"|"$self"/*) continue ;;
    esac
    # protocol is the shared contract and may be imported by every module.
    case "$dep" in
      github.com/WuErPing/solo/protocol|github.com/WuErPing/solo/protocol/*)
        if [ "$module" != "protocol" ]; then
          continue
        fi
        ;;
    esac
    # usage is a leaf library imported by the daemon only.
    case "$dep" in
      github.com/WuErPing/solo/usage|github.com/WuErPing/solo/usage/*)
        if [ "$module" = "daemon" ]; then
          continue
        fi
        ;;
    esac
    ERRORS=$((ERRORS + 1))
    VIOLATIONS="${VIOLATIONS}  - ${module} imports forbidden package: ${dep}\n"
  done <<< "$deps"
done

if [ "$ERRORS" -gt 0 ]; then
  echo "ERROR: Found $ERRORS architecture boundary violation(s):" >&2
  echo -e "$VIOLATIONS" >&2
  echo "Module boundaries (see .agents/rules/architecture.md):" >&2
  echo "  protocol/ and usage/ must stay dependency-free; daemon/ may import protocol and usage;" >&2
  echo "  cli/ and relay-go/ may import protocol only." >&2
  echo "If two modules genuinely need to share code, extract it into protocol/." >&2
  exit 1
fi

# Handler-file size guard (moved out of Go tests): each domain handler file
# must stay under 500 lines so the Session struct does not accumulate
# unrelated concerns.
LINE_LIMIT=500
for f in session_terminal.go session_tmux.go session_schedule.go; do
  path="daemon/internal/server/$f"
  if [ ! -f "$path" ]; then
    echo "ERROR: expected handler file not found: $path" >&2
    exit 1
  fi
  lines=$(wc -l < "$path")
  if [ "$lines" -ge "$LINE_LIMIT" ]; then
    echo "ERROR: $path has $lines lines (limit $LINE_LIMIT); split domain logic into focused files" >&2
    exit 1
  fi
done

echo "OK: Module boundaries respected (protocol and usage are the only shared dependencies)."
exit 0
