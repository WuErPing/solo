# Semantic Verify (PADD Semantic-Validation Layer)

LLM-based first-pass reviewers that complement the deterministic checks
(compiler, lint, tests, E2E). They answer questions static analysis
cannot: "does this change comply with our recorded architecture
decisions?"

## check-adr-consistency.mjs

Evaluates the current branch's diff against every **accepted** ADR in
`docs/decisions/` and produces a Markdown + JSON report with one verdict
per ADR: `compliant` / `violation` / `uncertain`.

Advisory by design (the LLM verdict is probabilistic):

- exits 0 even on violations unless `--fail-on-violation` is passed;
- evaluator errors (API down, bad response) degrade to `uncertain`,
  never to a hard failure;
- intended as a first-pass filter for human reviewers, not a gate.

### Local usage

```bash
# Inspect the prompts without an API key
node scripts/semantic-verify/check-adr-consistency.mjs --dry-run

# Real run against origin/main
export LLM_API_KEY=...
node scripts/semantic-verify/check-adr-consistency.mjs \
  --base origin/main --out adr-report.md --json adr-report.json
```

### Configuration

| Variable       | Required | Default                         | Notes                              |
|----------------|----------|---------------------------------|------------------------------------|
| `LLM_API_KEY`  | yes*     | —                               | *unless `--dry-run`                |
| `LLM_BASE_URL` | no       | `https://api.openai.com/v1`     | any OpenAI-compatible endpoint     |
| `LLM_MODEL`    | no       | `gpt-4o-mini`                   | cheap model is fine for first pass |

### CI

`.github/workflows/semantic-check.yml` runs the checker when a PR
carries the `semantic-check` label (and on manual dispatch), then posts
or updates a sticky advisory comment on the PR. Configure
`secrets.LLM_API_KEY` (and optionally `vars.LLM_BASE_URL`,
`vars.LLM_MODEL`) in the repository settings.

## Adding a new checker

One file per check, Node 22 standard library only (no npm dependencies),
`--dry-run` support, structured JSON output, and graceful degradation —
never let an evaluator outage break CI.
