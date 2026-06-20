# Makefile Benchmark: Sequential vs Parallel CI Targets

> **Date**: 2026-06-21
> **Environment**: macOS (Darwin 25.5.0), Apple Silicon
> **Makefile**: project root `Makefile`

---

## test-go (4 Go modules)

Sequential (`for mod in ...`) vs parallel (`xargs -P0`).

| Mode | Time | Speedup |
|------|------|---------|
| Sequential | 75.8s | — |
| Parallel (`-P0`) | 59.5s | **1.27x** |

Individual module times (test execution only, excluding compilation):

| Module | Time |
|--------|------|
| protocol | 1.4s |
| cli | 2.6s |
| relay-go | 3.6s |
| daemon | 4.8s |

**Why the speedup is modest**: Go compilation dominates the wall time, not test
execution. Multiple `go test` processes compile simultaneously and contend on
CPU and disk I/O. Parallel execution saves ~16s but the bottleneck remains the
daemon module's compile+test cycle.

## lint (3 packages)

Sequential run: ~13.4s. Each package is independent (ESLint / expo lint),
so `xargs -P0` would reduce this to ~5-8s (bounded by `app` which is the
heaviest).

## typecheck (3 packages)

Sequential run: ~10.5s (excluding `build-workspace-deps`). Same parallelism
opportunity as lint — bounded by `app` typecheck.

## ci target

`ci: lint test typecheck` — running `make -j3 ci` executes all three
prerequisites concurrently. Since they use entirely separate toolchains
(Go compiler, ESLint, TypeScript), there is zero resource contention and
the speedup is near-optimal.

Estimated sequential `ci` time: ~100s. With `-j3`: ~75s (bounded by `test`).

## Recommendation

| Change | Effort | Gain |
|--------|--------|------|
| `xargs -P0` in `test-go` | Low | ~16s |
| `xargs -P0` in `lint` | Low | ~5-8s |
| `xargs -P0` in `typecheck` | Low | ~3-5s |
| `make -j3 ci` invocation | Zero (no Makefile change) | ~25s |
| **Combined** | | **~30-35s** (100s → ~65s) |

`make -j3 ci` is the easiest win since it requires no Makefile changes —
only a different invocation. The `xargs` changes are low-effort and improve
individual target runtimes for local development.
