package supervisor

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// TestHelperProcess is re-executed as the fake daemon child when
// SOLO_FAKE_CHILD=1. Behavior is scripted via env:
//
//	SOLO_FAKE_CHILD_EXIT    exit code (default 0)
//	SOLO_FAKE_CHILD_SEQ     comma-separated exit codes by spawn index (last repeats)
//	SOLO_FAKE_CHILD_COUNTER file used to track the spawn index for SEQ
//	SOLO_FAKE_CHILD_BLOCK   "1" = block until SIGTERM, then exit 0
//	SOLO_FAKE_CHILD_RECORD  append os.Args[0] to this file on each spawn
//	SOLO_FAKE_CHILD_WRITE_POINTER  write a versions pointer file in this dir
//	SOLO_FAKE_CHILD_POINTER_NAME   pointer content (binary basename)
func TestHelperProcess(t *testing.T) {
	if os.Getenv("SOLO_FAKE_CHILD") != "1" {
		return
	}
	t.Log("helper process started")
	if record := os.Getenv("SOLO_FAKE_CHILD_RECORD"); record != "" {
		f, err := os.OpenFile(record, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString(os.Args[0] + "\n")
			_ = f.Close()
		}
	}
	if dir := os.Getenv("SOLO_FAKE_CHILD_WRITE_POINTER"); dir != "" {
		name := os.Getenv("SOLO_FAKE_CHILD_POINTER_NAME")
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "current"), []byte(name), 0644)
	}
	if os.Getenv("SOLO_FAKE_CHILD_BLOCK") == "1" {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM)
		<-sigCh
		os.Exit(0)
	}
	if seq := os.Getenv("SOLO_FAKE_CHILD_SEQ"); seq != "" {
		counterFile := os.Getenv("SOLO_FAKE_CHILD_COUNTER")
		n := readCounter(counterFile)
		writeCounter(counterFile, n+1)
		codes := strings.Split(seq, ",")
		idx := min(n, len(codes)-1)
		code, err := strconv.Atoi(codes[idx])
		if err != nil {
			os.Exit(99)
		}
		os.Exit(code)
	}
	code, _ := strconv.Atoi(os.Getenv("SOLO_FAKE_CHILD_EXIT"))
	os.Exit(code)
}

func readCounter(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return n
}

func writeCounter(path string, n int) {
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0644)
}

func newTestSupervisor(t *testing.T, extraEnv ...string) *Supervisor {
	t.Helper()
	sup, err := New(Config{
		DaemonBinary: os.Args[0],
		SoloHome:     t.TempDir(),
		ExtraEnv:     append([]string{"SOLO_FAKE_CHILD=1"}, extraEnv...),
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sup.childArgs = []string{"-test.run=TestHelperProcess"}
	sup.initialBackoff = time.Millisecond
	sup.maxBackoff = 2 * time.Millisecond
	sup.shutdownGrace = 2 * time.Second
	return sup
}

func TestRun_CleanExitStopsSupervisor(t *testing.T) {
	sup := newTestSupervisor(t, "SOLO_FAKE_CHILD_EXIT=0")
	if code := sup.Run(context.Background()); code != protocol.ExitCodeClean {
		t.Errorf("Run = %d, want %d", code, protocol.ExitCodeClean)
	}
	if sup.spawnCount.Load() != 1 {
		t.Errorf("spawnCount = %d, want 1", sup.spawnCount.Load())
	}
	if _, err := os.Stat(filepath.Join(sup.cfg.SoloHome, "solo.pid")); !os.IsNotExist(err) {
		t.Errorf("PID file should be removed after exit, stat err = %v", err)
	}
}

func TestRun_RestartRequestedRespawnsImmediately(t *testing.T) {
	counterFile := filepath.Join(t.TempDir(), "counter")
	sup := newTestSupervisor(t,
		"SOLO_FAKE_CHILD_SEQ=42,0",
		"SOLO_FAKE_CHILD_COUNTER="+counterFile,
	)
	if code := sup.Run(context.Background()); code != protocol.ExitCodeClean {
		t.Errorf("Run = %d, want %d", code, protocol.ExitCodeClean)
	}
	if sup.spawnCount.Load() != 2 {
		t.Errorf("spawnCount = %d, want 2 (exit 42 respawn, then clean exit)", sup.spawnCount.Load())
	}
}

func TestRun_CrashCircuitBreaker(t *testing.T) {
	sup := newTestSupervisor(t, "SOLO_FAKE_CHILD_EXIT=1")
	sup.maxFastCrashes = 3
	if code := sup.Run(context.Background()); code != 1 {
		t.Errorf("Run = %d, want 1 (gave up after crash loop)", code)
	}
	// Initial spawn + maxFastCrashes respawns, then the crash that trips the breaker.
	if sup.spawnCount.Load() != int32(sup.maxFastCrashes+1) {
		t.Errorf("spawnCount = %d, want %d", sup.spawnCount.Load(), sup.maxFastCrashes+1)
	}
}

func TestRun_ContextCancelStopsChild(t *testing.T) {
	sup := newTestSupervisor(t, "SOLO_FAKE_CHILD_BLOCK=1")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan int, 1)
	go func() { done <- sup.Run(ctx) }()

	// Let the child start, then cancel (simulates SIGTERM to the supervisor).
	deadline := time.Now().Add(2 * time.Second)
	for sup.spawnCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if sup.spawnCount.Load() == 0 {
		t.Fatal("child did not start")
	}
	// PID file must exist while the child runs.
	if _, err := os.Stat(filepath.Join(sup.cfg.SoloHome, "solo.pid")); err != nil {
		t.Errorf("PID file missing while child runs: %v", err)
	}
	cancel()

	select {
	case code := <-done:
		if code != protocol.ExitCodeClean {
			t.Errorf("Run = %d, want %d", code, protocol.ExitCodeClean)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
	if sup.spawnCount.Load() != 1 {
		t.Errorf("spawnCount = %d, want 1 (no respawn after shutdown)", sup.spawnCount.Load())
	}
}

func TestResolveSoloHome(t *testing.T) {
	if got, _ := resolveSoloHome("/custom/home"); got != "/custom/home" {
		t.Errorf("explicit home = %q", got)
	}
	t.Setenv("SOLO_HOME", "/env/home")
	if got, _ := resolveSoloHome(""); got != "/env/home" {
		t.Errorf("env home = %q", got)
	}
}

func TestResolveDaemonBinary(t *testing.T) {
	if got, _ := resolveDaemonBinary("/bin/fake-daemon"); got != "/bin/fake-daemon" {
		t.Errorf("explicit binary = %q", got)
	}
	t.Setenv("SOLO_DAEMON_BINARY", "/env/daemon")
	if got, _ := resolveDaemonBinary(""); got != "/env/daemon" {
		t.Errorf("env binary = %q", got)
	}
}

func TestExitCodeOf(t *testing.T) {
	if got := exitCodeOf(nil); got != 0 {
		t.Errorf("nil error = %d, want 0", got)
	}
}

// --- Version pointer resolution ---

func newPointerTestSupervisor(t *testing.T) *Supervisor {
	t.Helper()
	sup, err := New(Config{
		DaemonBinary: "/default/solo",
		SoloHome:     t.TempDir(),
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return sup
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveChildBinary_PointerWins(t *testing.T) {
	sup := newPointerTestSupervisor(t)
	target := filepath.Join(sup.cfg.SoloHome, "versions", "solo-v9")
	writeExecutable(t, target)
	if err := os.WriteFile(filepath.Join(sup.cfg.SoloHome, "versions", "current"), []byte("solo-v9\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := sup.resolveChildBinary(); got != target {
		t.Errorf("resolveChildBinary = %q, want %q", got, target)
	}
}

func TestResolveChildBinary_Fallbacks(t *testing.T) {
	cases := map[string]string{
		"missing target":         "solo-missing",
		"traversal":              "../evil",
		"absolute path":          "/tmp/evil",
		"pointer self-reference": "current",
	}
	for name, pointer := range cases {
		t.Run(name, func(t *testing.T) {
			sup := newPointerTestSupervisor(t)
			dir := filepath.Join(sup.cfg.SoloHome, "versions")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "current"), []byte(pointer), 0644); err != nil {
				t.Fatal(err)
			}
			if got := sup.resolveChildBinary(); got != sup.cfg.DaemonBinary {
				t.Errorf("resolveChildBinary = %q, want fallback %q", got, sup.cfg.DaemonBinary)
			}
		})
	}
}

func TestResolveChildBinary_NoPointer(t *testing.T) {
	sup := newPointerTestSupervisor(t)
	if got := sup.resolveChildBinary(); got != sup.cfg.DaemonBinary {
		t.Errorf("resolveChildBinary = %q, want %q", got, sup.cfg.DaemonBinary)
	}
}

// TestRun_SwitchTakesEffectOnRespawn: first spawn runs the default binary,
// which writes the pointer before exiting 42; the respawn must use the
// pointed-to copy.
func TestRun_SwitchTakesEffectOnRespawn(t *testing.T) {
	sup := newTestSupervisor(t) // SoloHome = t.TempDir(), child = os.Args[0]
	versionsDir := filepath.Join(sup.cfg.SoloHome, "versions")
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Install a "new version": a copy of the test binary under a version name.
	copied := filepath.Join(versionsDir, "solo-fake-v2")
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}
	if err := os.WriteFile(copied, data, 0755); err != nil {
		t.Fatalf("install fake version: %v", err)
	}

	recordFile := filepath.Join(t.TempDir(), "record")
	counterFile := filepath.Join(t.TempDir(), "counter")
	sup.cfg.ExtraEnv = append(sup.cfg.ExtraEnv,
		"SOLO_FAKE_CHILD_SEQ=42,0",
		"SOLO_FAKE_CHILD_COUNTER="+counterFile,
		"SOLO_FAKE_CHILD_RECORD="+recordFile,
		"SOLO_FAKE_CHILD_WRITE_POINTER="+versionsDir,
		"SOLO_FAKE_CHILD_POINTER_NAME=solo-fake-v2",
	)

	if code := sup.Run(context.Background()); code != protocol.ExitCodeClean {
		t.Errorf("Run = %d, want %d", code, protocol.ExitCodeClean)
	}
	if sup.spawnCount.Load() != 2 {
		t.Fatalf("spawnCount = %d, want 2", sup.spawnCount.Load())
	}
	record, err := os.ReadFile(recordFile)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(record)), "\n")
	if len(lines) != 2 {
		t.Fatalf("record lines = %v", lines)
	}
	if lines[0] != os.Args[0] {
		t.Errorf("first spawn = %q, want default %q", lines[0], os.Args[0])
	}
	if lines[1] != copied {
		t.Errorf("second spawn = %q, want pointed-to %q", lines[1], copied)
	}
}

// --- Crash fallback ---

// installFakeVersion copies the test binary into the versions dir under name
// with the given mtime, returning its full path.
func installFakeVersion(t *testing.T, versionsDir, name string, mtime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}
	path := filepath.Join(versionsDir, name)
	if err := os.WriteFile(path, data, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRun_CrashFallbackToOlderVersion: the pointed-to build crash-loops and
// trips the breaker; the supervisor must fall back to the next versioned
// build instead of exiting, and rewrite the pointer to it.
func TestRun_CrashFallbackToOlderVersion(t *testing.T) {
	sup := newTestSupervisor(t)
	sup.maxFastCrashes = 2
	versionsDir := filepath.Join(sup.cfg.SoloHome, "versions")
	now := time.Now()
	bad := installFakeVersion(t, versionsDir, "solo-bad", now)                 // newest
	good := installFakeVersion(t, versionsDir, "solo-good", now.Add(-time.Hour)) // older
	if err := os.WriteFile(filepath.Join(versionsDir, "current"), []byte("solo-bad\n"), 0644); err != nil {
		t.Fatal(err)
	}

	recordFile := filepath.Join(t.TempDir(), "record")
	counterFile := filepath.Join(t.TempDir(), "counter")
	sup.cfg.ExtraEnv = append(sup.cfg.ExtraEnv,
		// bad: exits 1 three times (trips breaker), good: exits 0.
		"SOLO_FAKE_CHILD_SEQ=1,1,1,0",
		"SOLO_FAKE_CHILD_COUNTER="+counterFile,
		"SOLO_FAKE_CHILD_RECORD="+recordFile,
	)

	if code := sup.Run(context.Background()); code != protocol.ExitCodeClean {
		t.Errorf("Run = %d, want %d (fallback build exits cleanly)", code, protocol.ExitCodeClean)
	}
	if sup.spawnCount.Load() != 4 {
		t.Fatalf("spawnCount = %d, want 4 (3 bad crashes + 1 good)", sup.spawnCount.Load())
	}
	record, err := os.ReadFile(recordFile)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(record)), "\n")
	for i := 0; i < 3; i++ {
		if lines[i] != bad {
			t.Errorf("spawn %d = %q, want bad build %q", i+1, lines[i], bad)
		}
	}
	if lines[3] != good {
		t.Errorf("fallback spawn = %q, want good build %q", lines[3], good)
	}
	// The pointer must be rewritten to the fallback build.
	pointer, err := os.ReadFile(filepath.Join(versionsDir, "current"))
	if err != nil {
		t.Fatalf("read pointer: %v", err)
	}
	if got := strings.TrimSpace(string(pointer)); got != "solo-good" {
		t.Errorf("pointer = %q, want solo-good", got)
	}
}

// TestRun_CrashFallbackExhausted: every candidate (versioned build, then the
// default binary) crash-loops; the supervisor must give up with exit 1.
func TestRun_CrashFallbackExhausted(t *testing.T) {
	sup := newTestSupervisor(t, "SOLO_FAKE_CHILD_EXIT=1")
	sup.maxFastCrashes = 1
	versionsDir := filepath.Join(sup.cfg.SoloHome, "versions")
	installFakeVersion(t, versionsDir, "solo-bad", time.Now())
	if err := os.WriteFile(filepath.Join(versionsDir, "current"), []byte("solo-bad\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if code := sup.Run(context.Background()); code != 1 {
		t.Errorf("Run = %d, want 1 (all candidates exhausted)", code)
	}
	// 2 spawns on solo-bad (breaker at 2 > 1), then 2 spawns on the default
	// binary after the pointer is removed.
	if sup.spawnCount.Load() != 4 {
		t.Errorf("spawnCount = %d, want 4", sup.spawnCount.Load())
	}
	if _, err := os.Stat(filepath.Join(versionsDir, "current")); !os.IsNotExist(err) {
		t.Errorf("pointer should be removed for default-binary fallback, stat err = %v", err)
	}
}
