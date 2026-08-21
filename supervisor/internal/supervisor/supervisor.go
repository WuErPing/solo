// Package supervisor implements the solo-supervisor process: it spawns the
// Solo daemon as a child and decides, based on the child's exit code, whether
// to respawn it. See protocol/process_contract.go for the exit-code contract.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/WuErPing/solo/protocol"
)

// Default respawn policy values.
const (
	defaultInitialBackoff = 1 * time.Second
	defaultMaxBackoff     = 30 * time.Second
	defaultStableUptime   = 60 * time.Second
	defaultMaxFastCrashes = 5
	defaultShutdownGrace  = 10 * time.Second
)

// Config configures a Supervisor.
type Config struct {
	// DaemonBinary is the path to the daemon binary. Empty means resolve from
	// SOLO_DAEMON_BINARY, then next to the supervisor executable, then PATH.
	// A valid versions pointer file ($SoloHome/versions/current) overrides
	// this on each spawn — that is how version switching takes effect.
	DaemonBinary string
	// SoloHome is the daemon home directory (for the PID file and logs).
	// Empty means SOLO_HOME or ~/.solo.
	SoloHome string
	// ExtraEnv holds extra KEY=VALUE entries appended to the child's
	// environment (e.g. flag-to-env translations from the CLI).
	ExtraEnv []string
	// Logger receives supervisor logs. Nil means slog.Default().
	Logger *slog.Logger
}

// Supervisor spawns and watches the daemon child process.
type Supervisor struct {
	cfg Config

	// Respawn policy knobs (defaults in New; shrunk by tests).
	initialBackoff time.Duration
	maxBackoff     time.Duration
	stableUptime   time.Duration
	maxFastCrashes int
	shutdownGrace  time.Duration

	// failed marks versioned builds (by basename) that tripped the crash
	// breaker during this supervisor's lifetime; failedDefault does the same
	// for the default binary. The breaker consults them to pick a fallback
	// instead of giving up while an untried build remains.
	failed        map[string]bool
	failedDefault bool

	// childArgs are extra arguments passed to the daemon binary (test hook;
	// the production daemon takes no arguments).
	childArgs []string
	// spawnCount records how many children were started (test hook).
	spawnCount atomic.Int32
}

// New resolves the daemon binary and solo home and returns a Supervisor.
func New(cfg Config) (*Supervisor, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	home, err := resolveSoloHome(cfg.SoloHome)
	if err != nil {
		return nil, err
	}
	cfg.SoloHome = home
	bin, err := resolveDaemonBinary(cfg.DaemonBinary)
	if err != nil {
		return nil, err
	}
	cfg.DaemonBinary = bin
	return &Supervisor{
		cfg:            cfg,
		initialBackoff: defaultInitialBackoff,
		maxBackoff:     defaultMaxBackoff,
		stableUptime:   defaultStableUptime,
		maxFastCrashes: defaultMaxFastCrashes,
		shutdownGrace:  defaultShutdownGrace,
		failed:         make(map[string]bool),
	}, nil
}

// Run spawns the daemon and supervises it until the daemon exits cleanly or
// the context is cancelled (e.g. SIGTERM to the supervisor). The returned
// code is the supervisor's own exit code.
func (s *Supervisor) Run(ctx context.Context) int {
	log := s.cfg.Logger
	backoff := s.initialBackoff
	fastCrashes := 0

	for {
		started := time.Now()
		res := s.runChild(ctx)
		s.removePIDFile()

		if ctx.Err() != nil {
			log.Info("supervisor shutting down")
			return protocol.ExitCodeClean
		}

		switch {
		case res.code == protocol.ExitCodeClean && res.err == nil:
			log.Info("daemon exited cleanly, supervisor exiting")
			return protocol.ExitCodeClean

		case res.code == protocol.ExitCodeRestartRequested:
			log.Info("daemon requested restart, respawning")
			backoff = s.initialBackoff
			fastCrashes = 0

		default:
			uptime := time.Since(started)
			if uptime >= s.stableUptime {
				fastCrashes = 0
				backoff = s.initialBackoff
			}
			fastCrashes++
			log.Warn("daemon crashed", "exitCode", res.code, "error", res.err,
				"uptime", uptime.Round(time.Millisecond), "consecutiveCrashes", fastCrashes)
			if fastCrashes > s.maxFastCrashes {
				// A freshly-switched-to build that won't start must not take
				// the host down for good: fall back to another build so the
				// app can reconnect and the user keeps working.
				if s.tryCrashFallback(res.binary) {
					backoff = s.initialBackoff
					fastCrashes = 0
					continue
				}
				log.Error("too many consecutive daemon crashes, giving up",
					"maxFastCrashes", s.maxFastCrashes)
				return 1
			}
			log.Info("respawning daemon after backoff", "backoff", backoff)
			if !sleepContext(ctx, backoff) {
				return protocol.ExitCodeClean
			}
			backoff = min(backoff*2, s.maxBackoff)
		}
	}
}

// childResult describes how one daemon child terminated.
type childResult struct {
	code   int    // process exit code; -1 when killed by a signal
	err    error  // cmd.Wait() error, nil on clean exit
	binary string // the binary that was spawned (for crash fallback)
}

// runChild starts the daemon once, forwards a context cancellation to it
// (SIGTERM, escalating to SIGKILL after shutdownGrace), and waits for it.
func (s *Supervisor) runChild(ctx context.Context) childResult {
	log := s.cfg.Logger

	var logFile *os.File
	if f, err := s.openLogFile(); err == nil {
		logFile = f
	} else {
		log.Warn("cannot open daemon log file, using supervisor stderr", "error", err)
	}

	binary := s.resolveChildBinary()

	cmd := exec.Command(binary, s.childArgs...)
	cmd.Env = append(os.Environ(), "SOLO_SUPERVISED=1")
	cmd.Env = append(cmd.Env, s.cfg.ExtraEnv...)
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return childResult{code: -1, err: fmt.Errorf("start daemon: %w", err), binary: binary}
	}
	s.spawnCount.Add(1)
	s.writePIDFile(cmd.Process.Pid)
	log.Info("daemon started", "pid", cmd.Process.Pid, "binary", binary)

	waitCh := make(chan childResult, 1)
	go func() {
		err := cmd.Wait()
		waitCh <- childResult{code: exitCodeOf(err), err: err, binary: binary}
	}()

	// Forward supervisor shutdown to the child: SIGTERM first, SIGKILL after
	// the grace period if the child is still running.
	stopDone := make(chan struct{})
	defer close(stopDone)
	go func() {
		select {
		case <-ctx.Done():
			log.Info("forwarding shutdown to daemon", "pid", cmd.Process.Pid)
			_ = cmd.Process.Signal(syscall.SIGTERM)
			timer := time.NewTimer(s.shutdownGrace)
			defer timer.Stop()
			select {
			case <-stopDone:
			case <-timer.C:
				log.Warn("daemon did not stop in time, killing", "pid", cmd.Process.Pid)
				_ = cmd.Process.Kill()
			}
		case <-stopDone:
		}
	}()

	res := <-waitCh
	if logFile != nil {
		_ = logFile.Close()
	}
	log.Info("daemon exited", "pid", cmd.Process.Pid, "exitCode", res.code, "error", res.err)
	return res
}

// exitCodeOf extracts the exit code from a cmd.Wait() error. It returns 0 for
// a clean exit and -1 when the process was killed by a signal.
func exitCodeOf(waitErr error) int {
	if waitErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// --- Path resolution ---

func resolveSoloHome(home string) (string, error) {
	if home != "" {
		return home, nil
	}
	if v := os.Getenv("SOLO_HOME"); v != "" {
		return v, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	return filepath.Join(userHome, ".solo"), nil
}

func resolveDaemonBinary(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if v := os.Getenv("SOLO_DAEMON_BINARY"); v != "" {
		return v, nil
	}
	// Next to the supervisor executable (release layout: solo + solo-supervisor).
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "solo")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	// On PATH.
	for _, name := range []string{"solo-daemon", "solo"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("cannot find daemon binary; ensure 'solo' is next to solo-supervisor or on PATH")
}

// versionPointerFile is the name of the pointer file inside the versions
// directory. Its content is the basename of the daemon binary to run.
const versionPointerFile = "current"

// versionBinaryPrefix is the required filename prefix for runnable builds in
// the versions directory (same convention as the daemon's version scanner).
const versionBinaryPrefix = "solo-"

// resolveChildBinary picks the daemon binary for one spawn. A valid pointer
// file ($SoloHome/versions/current) wins — this is how daemon version
// switching takes effect on respawn. Anything invalid (missing pointer,
// missing/non-executable target, non-basename content) falls back to the
// default binary resolved at startup.
func (s *Supervisor) resolveChildBinary() string {
	log := s.cfg.Logger
	name := s.readVersionPointer()
	if name == "" {
		return s.cfg.DaemonBinary
	}
	if name != filepath.Base(name) || name == "." || name == versionPointerFile {
		log.Warn("invalid version pointer, using default binary", "pointer", name)
		return s.cfg.DaemonBinary
	}
	candidate := filepath.Join(s.cfg.SoloHome, "versions", name)
	if !isExecutableFile(candidate) {
		log.Warn("pointed daemon binary not found or not executable, using default binary",
			"pointer", name, "path", candidate)
		return s.cfg.DaemonBinary
	}
	return candidate
}

// readVersionPointer returns the trimmed content of the versions pointer
// file, or "" when absent/unreadable.
func (s *Supervisor) readVersionPointer() string {
	data, err := os.ReadFile(filepath.Join(s.cfg.SoloHome, "versions", versionPointerFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// tryCrashFallback reacts to a tripped crash breaker: it blacklists the
// crashed build and points the supervisor at the newest versioned build that
// hasn't failed yet (rewriting the versions pointer, the same convention the
// daemon's switch handler uses), or at the default binary when no versioned
// build is left. Returns false when every candidate has already failed — the
// caller then gives up.
func (s *Supervisor) tryCrashFallback(crashed string) bool {
	log := s.cfg.Logger
	versionsDir := filepath.Join(s.cfg.SoloHome, "versions")
	if filepath.Dir(crashed) == versionsDir {
		s.failed[filepath.Base(crashed)] = true
	} else {
		s.failedDefault = true
	}
	for _, name := range s.scanVersionNames(versionsDir) {
		if s.failed[name] {
			continue
		}
		log.Error("daemon build keeps crashing, falling back to another version",
			"crashed", crashed, "fallback", name)
		if err := writeVersionPointer(versionsDir, name); err != nil {
			log.Warn("cannot rewrite version pointer for fallback", "error", err)
			return false
		}
		return true
	}
	if !s.failedDefault {
		log.Error("daemon build keeps crashing, falling back to default binary",
			"crashed", crashed, "fallback", s.cfg.DaemonBinary)
		// Removing the pointer makes resolveChildBinary use the default binary.
		if err := os.Remove(filepath.Join(versionsDir, versionPointerFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn("cannot remove version pointer for fallback", "error", err)
		}
		return true
	}
	return false
}

// scanVersionNames returns the basenames of runnable builds in the versions
// dir (executable regular files named solo-*), newest mtime first.
func (s *Supervisor) scanVersionNames(versionsDir string) []string {
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil
	}
	type candidate struct {
		name  string
		mtime time.Time
	}
	var candidates []candidate
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, versionBinaryPrefix) {
			continue
		}
		if !isExecutableFile(filepath.Join(versionsDir, name)) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{name, info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].mtime.After(candidates[j].mtime) })
	names := make([]string, 0, len(candidates))
	for _, c := range candidates {
		names = append(names, c.name)
	}
	return names
}

// writeVersionPointer atomically records the build to run (tmp + rename in
// the same directory), the same convention the daemon's switch handler uses.
func writeVersionPointer(versionsDir, name string) error {
	tmp, err := os.CreateTemp(versionsDir, ".current-*")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(name + "\n"); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(versionsDir, versionPointerFile))
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return info.Mode().Perm()&0111 != 0
}

// --- PID file and log file ---

func (s *Supervisor) pidPath() string {
	return filepath.Join(s.cfg.SoloHome, "solo.pid")
}

// writePIDFile records the daemon child PID in the same format pidlock uses
// when the daemon runs unsupervised, so `solo-cli daemon status` keeps working.
func (s *Supervisor) writePIDFile(pid int) {
	if err := os.MkdirAll(s.cfg.SoloHome, 0755); err != nil {
		s.cfg.Logger.Warn("cannot create solo home for PID file", "error", err)
		return
	}
	if err := os.WriteFile(s.pidPath(), []byte(strconv.Itoa(pid)), 0644); err != nil {
		s.cfg.Logger.Warn("cannot write PID file", "error", err)
	}
}

func (s *Supervisor) removePIDFile() {
	_ = os.Remove(s.pidPath())
}

func (s *Supervisor) openLogFile() (*os.File, error) {
	dir := filepath.Join(s.cfg.SoloHome, "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "daemon.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}
