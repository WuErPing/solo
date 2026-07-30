# Test Strategy

## TDD Rules (from AGENTS.md)

- Core business logic and critical execution paths: write tests **before** implementation.
- Every ADR includes TDD-first acceptance criteria (see ADR-001 §5 as exemplar).

## Test Pyramid

```
        ╱ E2E (Playwright) ╲           — 43 specs, nightly
       ╱─────────────────────╲
      ╱ Integration (Go, Jest) ╲        — store/runner/schema round-trips
     ╱───────────────────────────╲
    ╱    Unit (Go -short, Jest)    ╲    — per-module, every PR
   ╱─────────────────────────────────╲
```

## Commands

| Scope | Command |
|-------|---------|
| All CI checks | `make ci` |
| Go unit (all modules) | `go test -short -race ./...` |
| Go single module | `cd daemon && go test -short -race ./...` |
| App + app-bridge JS | `npm test` (root) |
| E2E | `npx playwright test` (requires running daemon+relay+Metro) |
| Lint Go | `golangci-lint run` |
| Lint JS | `npx eslint app/ app-bridge/` |
| Typecheck | `cd app && npx tsc --noEmit` |

## Coverage Thresholds

- **protocol/**: ≥90% (serialization correctness is critical)
- **daemon/ core** (agent, loop, schedule): ≥80%
- **app-bridge/**: ≥80% (schema + transform correctness)
- **app/ UI**: no hard threshold; focus on stores and hooks

## Flaky Test Policy

- Flaky tests must be tracked in [Test Quality Audit](reports/test-quality-audit-2026-07.md).
- A test that fails >2 times in CI without code change → quarantine + fix within 1 sprint.
