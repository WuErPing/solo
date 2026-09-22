# Test Strategy

## TDD Rules (from AGENTS.md)

- Core business logic and critical execution paths: write tests **before** implementation.
- Every ADR includes TDD-first acceptance criteria (see ADR-001 §5 as exemplar).

## Test Pyramid

```
        ╱ E2E (Playwright) ╲           — 44 specs, nightly
       ╱─────────────────────╲
      ╱ Integration (Go, Vitest) ╲        — store/runner/schema round-trips
     ╱─────────────────────────────╲
    ╱   Unit (Go -short, Vitest)   ╲    — per-module, every PR
   ╱───────────────────────────────────╲
```

## Commands

| Scope | Command |
|-------|---------|
| All CI checks | `make ci` |
| Go unit (all modules) | `make test-go` (`-short -race -count=1 -tags external_api`) |
| Go single module | `cd daemon && go test -short -race -count=1 -tags external_api ./...` |
| App + app-bridge JS | `make test-app` (or per-workspace: `cd app && npm run test -- --project=unit`, `cd app-bridge && npm test`) |
| E2E | `cd app && npx playwright test` (requires running daemon+relay+Metro) |
| Lint (JS + schema/arch checks) | `make lint` (Go lint runs via golangci-lint in CI only) |
| Typecheck | `cd app && npx tsc --noEmit` |

## Coverage Thresholds

Enforced by `codecov.yml` (repo root):

- **Patch coverage**: newly added/changed lines in a PR must be **≥70%** (threshold 5%) — hard gate, blocks merge.
- **Project coverage**: informational only (`target: auto`, threshold 5%) — app is ~36%, ramped up gradually.

Aspirational per-module targets (not enforced in CI):

- **protocol/**: ≥90% (serialization correctness is critical)
- **daemon/ core** (agent, loop, schedule): ≥80%
- **app-bridge/**: ≥80% (schema + transform correctness)
- **app/ UI**: no hard threshold; focus on stores and hooks
- **cli/, supervisor/, usage/**: no fixed threshold yet

## Flaky Test Policy

- Flaky tests must be tracked in [Test Quality Audit](reports/test-quality-audit-2026-07.md).
- A test that fails >2 times in CI without code change → quarantine + fix within 1 sprint.
