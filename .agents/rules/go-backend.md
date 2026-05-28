# Go Backend Rules

Rules for all Go modules: `daemon`, `cli`, `relay-go`, `protocol`.

## Context Propagation

- Always pass `context.Context` as the first parameter of every exported function.
- Derive child contexts with `context.WithTimeout` or `context.WithCancel`; never store a context in a struct field.
- On shutdown paths, propagate the shutdown context so goroutines exit cleanly.
- Use `context.WithoutCancel` (Go 1.21+) only for fire-and-forget work that must survive parent cancellation (e.g., telemetry flush).

## Error Handling

- Wrap errors with `fmt.Errorf("operation: %w", err)` at each boundary; never use `%v` for error wrapping.
- Define sentinel errors with `var ErrXxx = errors.New("xxx")` at package level. Use `errors.Is` / `errors.As` for checks.
- Never call `panic` in production code paths. Panics are reserved for programmer-error invariants (e.g., unreachable switch cases in test helpers).
- Always check returned errors. If intentionally ignoring, assign to `_` with a comment explaining why.
- `errors.Join` for aggregating multiple errors in cleanup paths (e.g., defers).

## Concurrency

- Every goroutine must have a documented owner responsible for its lifecycle (start + shutdown).
- Prefer `errgroup.Group` over raw `go` + `sync.WaitGroup` when multiple goroutines contribute to a single result.
- Always pair goroutine launches with a shutdown mechanism: context cancellation, `done` channel, or `errgroup`.
- Never leak goroutines. If a function starts a goroutine, it must either wait for it or return a handle to stop it.
- Use `sync.Once` for one-time initialization; never use `init()` for logic that can fail.
- Channel sends must be guarded by `select` with a `ctx.Done()` or `default` case unless the channel is unbuffered and the receiver is guaranteed to be ready.

## Structured Logging

- Use `log/slog` exclusively. Never use `fmt.Println`, `log.Printf`, or `log.Fatal` in production code.
- Use `slog.With("key", value)` to create contextual loggers; pass loggers through function parameters, not globals.
- Log levels: `Debug` for development diagnostics, `Info` for significant lifecycle events, `Warn` for recoverable anomalies, `Error` for actionable failures.
- Never log secrets, tokens, passwords, or private keys. Use the redactor stack (`memory/redact`) when logging user-provided content.

## Naming

- Packages: short, lowercase, single-word when possible (`agent`, `workspace`, `relayclient`). No `util`, `common`, `helpers`, or `misc` packages.
- Interfaces: name by behavior (`Recorder`, `Transport`, `Provider`). Use `-er` suffix. One-method interfaces are fine.
- Receivers: 1-2 lowercase letters derived from the type name (`s *Server`, `p *Provider`).
- Acronyms: all caps (`HTTPClient`, `ID`, `URL`, `WSProtocolVersion`).
- Booleans: prefix with `is`, `has`, `can`, `should` for readability at call sites.

## Module Structure

- Each module (`daemon`, `cli`, `relay-go`, `protocol`) has its own `go.mod`. Cross-module deps go through `go.work` locally.
- `protocol` is dependency-free and importable by all other modules. Never add external dependencies to `protocol`.
- Use `replace` directives in `go.mod` only for local development; CI resolves via `go.work`.
- Internal packages (`internal/`) enforce Go's compiler-level import boundary. Use this deliberately.

## API Design

- Exported functions should have clear, narrow signatures. Prefer returning specific types over `interface{}`/`any`.
- Use functional options (`WithXxx`) for configuring structs with many optional fields.
- Prefer returning `(T, error)` over pointer-to-error or sentinel nil checks.
- Close resources with `defer` immediately after successful acquisition. Implement `io.Closer` where applicable.

## Configuration

- Config structs use `json` tags for `~/.solo/config.json`. Use pointer types for optional fields to distinguish zero-value from absent.
- Validate config at load time, not at use time. Fail fast with descriptive errors.
- Use `*bool` for tri-state flags (nil = default, true = explicit on, false = explicit off).

## Build Verification

Before committing Go changes, run:
```bash
go build ./...
go test -short -race ./...
```
For the affected module specifically (e.g., `cd daemon && go test -short -race ./...`).
