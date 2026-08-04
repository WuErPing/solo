# Codex CLI real-output fixtures

Captured on this machine from the real `codex` binary. NDJSON: one JSON
event per line, exactly as printed on stdout (comments live here, not in the
data files, so the files can be fed line-by-line to the translator).

| File | Source command | codex-cli version | Captured |
| --- | --- | --- | --- |
| `real_stream_v0.146.ndjson` | `codex exec --experimental-json --ephemeral --skip-git-repo-check "say hi"` | 0.146.0 | 2026-08-04 |
| `real_stream_tool_call_v0.146.ndjson` | `codex exec --json --ephemeral --skip-git-repo-check --sandbox read-only "Run the command: echo solo-fixture-test ..."` | 0.146.0 | 2026-08-04 |
| `real_stream_error_v0.146.ndjson` | `codex exec --json --ephemeral --skip-git-repo-check -m gpt-nonexistent-model "say hi"` (exit code 1) | 0.146.0 | 2026-08-04 |

Notes:

- `--json` and the (hidden, legacy) `--experimental-json` flag produce the
  identical event schema in 0.146.0; `--json` is the documented flag.
- Event types seen: `thread.started`, `turn.started`, `item.started`,
  `item.completed` (item types `agent_message`, `command_execution`, `error`),
  `turn.completed`, `error`, `turn.failed`.
- The assistant text arrives whole inside `item.completed` /
  `item.type == "agent_message"` — there are no streaming delta events.
- Token usage is attached to `turn.completed.usage`, not a standalone event.
