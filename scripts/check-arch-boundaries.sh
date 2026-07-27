#!/bin/bash
# CI lint: enforce the Go module boundaries documented in .agents/rules/architecture.md.
#
#   protocol/   ← zero dependencies on other solo modules, imported by all
#   daemon/     ← may import protocol only
#   cli/        ← may import protocol only (talks to daemon via WebSocket)
#   relay-go/   ← may import protocol only (stateless relay, no business logic)
#
# Uses `go list -deps` so it checks the real import graph, not text matches.
set -euo pipefail

MODULES=(protocol daemon cli relay-go)
ERRORS=0
VIOLATIONS=""

for module in "${MODULES[@]}"; do
  if [ ! -d "$module" ]; then
    echo "ERROR: Module directory not found: $module" >&2
    exit 1
  fi

  # Every solo package this module depends on (transitively), excluding itself.
  deps=$(cd "$module" && go list -deps ./... 2>/dev/null \
    | grep -E '^github\.com/WuErPing/solo/(protocol|daemon|cli|relay)(/|$)' \
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
    ERRORS=$((ERRORS + 1))
    VIOLATIONS="${VIOLATIONS}  - ${module} imports forbidden package: ${dep}\n"
  done <<< "$deps"
done

if [ "$ERRORS" -gt 0 ]; then
  echo "ERROR: Found $ERRORS architecture boundary violation(s):" >&2
  echo -e "$VIOLATIONS" >&2
  echo "Module boundaries (see .agents/rules/architecture.md):" >&2
  echo "  protocol/ must stay dependency-free; daemon/, cli/, relay-go/ may import protocol only." >&2
  echo "If two modules genuinely need to share code, extract it into protocol/." >&2
  exit 1
fi

echo "OK: Module boundaries respected (protocol is the only shared dependency)."
exit 0
