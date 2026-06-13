# Testing Rules

Rules for writing and maintaining tests across Go and TypeScript.

## General Principles

- Write tests alongside production code, not after. A feature without tests is incomplete.
- Tests should be deterministic. No `time.Sleep` for synchronization, no flaky timing-dependent assertions.
- Tests should be fast. Unit tests must complete in milliseconds. Use `-short` flag to skip integration-heavy paths.
- Every test must have a clear arrange-act-assert structure. Separate setup, execution, and verification with blank lines.
- Name tests by behavior: `TestServerRejectsDuplicateSession` not `TestServer1`.

## Go Testing

### Structure

- Use table-driven tests with `t.Run` for subtests:
  ```go
  tests := []struct {
      name    string
      input   Input
      want    Output
      wantErr bool
  }{...}
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
          got, err := Do(tt.input)
          if (err != nil) != tt.wantErr {
              t.Fatalf("Do() error = %v, wantErr %v", err, tt.wantErr)
          }
          if !reflect.DeepEqual(got, tt.want) {
              t.Errorf("Do() = %v, want %v", got, tt.want)
          }
      })
  }
  ```
- Use `t.Helper()` in test helper functions to get correct line numbers in failure output.
- Use `t.Parallel()` where subtests are independent to speed up execution.
- Prefer `t.TempDir()` for test file system isolation; never write to `/tmp` directly.

### Race Detection

- Always run with `-race` flag. This is enforced in CI.
- Tests that spawn goroutines must wait for completion before returning. Use `sync.WaitGroup`, channels, or `errgroup`.
- Never access shared state without synchronization in tests.

### Mocking

- Define interfaces at the consumer, not the provider. Mock interfaces, not concrete types.
- Use hand-written mocks for simple interfaces (1-3 methods). Avoid code-generation for trivial cases.
- The `Mock` provider (`agent/mock_provider.go`) exists for testing agent flows. Extend it, don't replace it.

### Coverage

- Target meaningful coverage, not line coverage. Cover error paths, edge cases, and concurrency interactions.
- Use `-coverprofile` and inspect uncovered branches periodically. Focus on `daemon/internal/` packages.

## TypeScript Testing (Vitest)

### Structure

- Test files: `*.test.ts` or `*.test.tsx` colocated with source files.
- Use `describe`/`it` blocks. Group by module or feature, not by file.
- Use `expect(...).toBe(...)` for primitives, `toEqual(...)` for objects, `toContain(...)` for arrays/strings.
- Avoid snapshot tests for component logic. Use them only for stable rendered output that's hard to assert structurally.

### React Component Testing

- Render with `@testing-library/react-native` (or `react` for web).
- Test user behavior, not implementation details. Query by role, label, or testID — not by CSS class or internal state.
- Mock external services (daemon client, stores) at the boundary. Use `vi.mock()` for module mocks.
- Always clean up after component tests (Testing Library handles this via `afterEach`).

### Async Testing

- Use `waitFor` or `findBy*` queries for async assertions. Never use fixed timeouts (`setTimeout`).
- For promise-based tests, return the promise or use `async`/`await`. Never use `done` callbacks.
- Mock timers with `vi.useFakeTimers()` when testing time-dependent logic.

### App-Bridge Testing

- Tests run in Node environment. No DOM or React dependencies.
- Test crypto operations with known test vectors, not random data.
- Test serialization/deserialization roundtrips for protocol messages.

## Test Fixtures

- Keep test fixtures minimal and inline when possible. Extract to files only when reused across 3+ tests.
- For Go: use `testdata/` directories (ignored by the Go toolchain for builds).
- For TypeScript: colocate fixture data in the test file or a `__fixtures__/` directory.

## CI Requirements

- Go: `go test -short -race -count=1 -timeout=10m` must pass with zero failures.
- TypeScript: `npm run test` in `app/` (1617+ tests) and `app-bridge/` (32+ tests) must pass.
- Coverage reports are uploaded to Codecov. Coverage drops on changed lines should be justified.
- E2E tests (Playwright) run nightly. New user-facing features should add E2E specs in `app/e2e/`.

## What Not to Test

- Don't test third-party library internals.
- Don't test TypeScript type system behavior (that's what `tsc --noEmit` is for).
- Don't write tests that only assert on log output. Test behavior, not logging.
