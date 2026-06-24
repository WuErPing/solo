#!/bin/bash
# CI lint: detect Go structs with // genzod that still have hand-written
# z.object() schemas in messages.ts. After migration, the hand-written schema
# should be replaced with an import from the generated protocol-schemas.ts.
set -euo pipefail

PROTOCOL_DIR="${1:-protocol}"
MESSAGES_FILE="${2:-app-bridge/src/shared/messages.ts}"

if [ ! -d "$PROTOCOL_DIR" ]; then
  echo "ERROR: Protocol directory not found: $PROTOCOL_DIR" >&2
  exit 1
fi

if [ ! -f "$MESSAGES_FILE" ]; then
  echo "ERROR: Messages file not found: $MESSAGES_FILE" >&2
  exit 1
fi

ERRORS=0
DUPLICATES=""

# Find Go structs with // genzod annotation. Scan forward past doc comments
# to find the actual 'type Foo struct' declaration.
while IFS=: read -r file linenum _rest; do
  # Scan forward from the annotation to find the struct declaration
  struct_name=""
  for offset in 1 2 3 4 5; do
    check_line=$((linenum + offset))
    decl=$(sed -n "${check_line}p" "$file" 2>/dev/null || true)
    name=$(echo "$decl" | sed -n 's/^type \([A-Z][a-zA-Z0-9]*\) struct.*/\1/p')
    if [ -n "$name" ]; then
      struct_name="$name"
      break
    fi
    # Stop scanning if we hit a non-comment, non-blank line that isn't a struct
    if [ -n "$decl" ] && ! echo "$decl" | grep -qE '^\s*(//|$)'; then
      break
    fi
  done

  if [ -z "$struct_name" ]; then
    continue
  fi

  schema_name="${struct_name}Schema"

  # Check if a hand-written z.object() schema definition exists in messages.ts.
  # Import-only references (from generated protocol-schemas.ts) don't match this pattern.
  if grep -qE "^(export )?(const|let) ${schema_name}\s*=\s*z\.object" "$MESSAGES_FILE"; then
    ERRORS=$((ERRORS + 1))
    DUPLICATES="${DUPLICATES}  - ${struct_name} (${file})\n"
  fi
done < <(grep -rn '// genzod' "$PROTOCOL_DIR"/*.go)

if [ "$ERRORS" -gt 0 ]; then
  echo "ERROR: Found $ERRORS genzod-annotated struct(s) that still have hand-written z.object() schemas in messages.ts:" >&2
  echo -e "$DUPLICATES" >&2
  echo "Replace the hand-written schema with an import from generated protocol-schemas.ts." >&2
  exit 1
fi

echo "OK: No schema duplication found."
exit 0
