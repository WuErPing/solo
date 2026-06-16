package agent

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// --- OpenCode Server Manager (Singleton) ---

type OpenCodeServerGeneration struct {
	cmd      *exec.Cmd
	port     int
	url      string
	refCount int
	retired  bool
}

type OpenCodeServerManager struct {
	mu                   sync.Mutex
	logger               *slog.Logger
	binaryPath           string
	env                  []string
	currentServer        *OpenCodeServerGeneration
	retiredServers       map[*OpenCodeServerGeneration]struct{}
	startPromise         chan struct{}
	startErr             error
	exitHandlerOnce      sync.Once
	forcedRefreshPromise chan struct{}
	forcedRefreshErr     error
}

var (
	serverManagerInstance *OpenCodeServerManager
	serverManagerOnce     sync.Once
)

func ShutdownOpenCodeServerManager() {
	if serverManagerInstance != nil {
		serverManagerInstance.Shutdown()
	}
}

func GetOpenCodeServerManager(binaryPath string, logger *slog.Logger, env []string) *OpenCodeServerManager {
	serverManagerOnce.Do(func() {
		serverManagerInstance = &OpenCodeServerManager{
			logger:         logger.With("component", "opencode-server-manager"),
			binaryPath:     binaryPath,
			env:            env,
			retiredServers: make(map[*OpenCodeServerGeneration]struct{}),
		}
		serverManagerInstance.registerExitHandler()
	})
	return serverManagerInstance
}

func (m *OpenCodeServerManager) registerExitHandler() {
	m.exitHandlerOnce.Do(func() {
		cleanup := func() {
			m.Shutdown()
		}
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			<-sigCh
			cleanup()
		}()
		// Also register via atexit-style: no direct Go equivalent, but
		// the daemon's Stop() calls ShutdownOpenCodeServerManager() which covers normal exit.
	})
}

func (m *OpenCodeServerManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownLocked()
}

func (m *OpenCodeServerManager) shutdownLocked() {
	servers := make([]*OpenCodeServerGeneration, 0, 1+len(m.retiredServers))
	if m.currentServer != nil {
		servers = append(servers, m.currentServer)
	}
	for s := range m.retiredServers {
		servers = append(servers, s)
	}
	for _, s := range servers {
		m.killServer(s)
	}
	m.currentServer = nil
	m.retiredServers = make(map[*OpenCodeServerGeneration]struct{})
}

func (m *OpenCodeServerManager) killServer(s *OpenCodeServerGeneration) {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(opencodeServerShutdownTimeout):
		_ = s.cmd.Process.Kill()
	}
}

func (m *OpenCodeServerManager) Acquire(ctx context.Context, force ...bool) (*OpenCodeServerGeneration, func(), error) {
	shouldForce := len(force) > 0 && force[0]
	if shouldForce {
		return m.acquireForced(ctx)
	}
	return m.acquireNormal(ctx)
}

func (m *OpenCodeServerManager) acquireForced(ctx context.Context) (*OpenCodeServerGeneration, func(), error) {
	m.mu.Lock()
	if m.forcedRefreshPromise != nil {
		ch := m.forcedRefreshPromise
		m.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
		m.mu.Lock()
		if m.forcedRefreshErr != nil {
			return nil, nil, m.forcedRefreshErr
		}
		server := m.currentServer
		if server != nil {
			server.refCount++
			released := false
			m.mu.Unlock()
			return server, func() {
				m.mu.Lock()
				defer m.mu.Unlock()
				if released {
					return
				}
				released = true
				server.refCount--
				m.cleanupRetiredServers()
			}, nil
		}
		m.mu.Unlock()
		return nil, nil, fmt.Errorf("forced refresh produced no server")
	}

	ch := make(chan struct{})
	m.forcedRefreshPromise = ch
	m.mu.Unlock()

	err := m.RotateCurrentServer(ctx)

	m.mu.Lock()
	m.forcedRefreshErr = err
	close(ch)
	m.forcedRefreshPromise = nil

	if err != nil {
		m.mu.Unlock()
		return nil, nil, err
	}
	server := m.currentServer
	if server != nil {
		server.refCount++
		released := false
		m.mu.Unlock()
		return server, func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if released {
				return
			}
			released = true
			server.refCount--
			m.cleanupRetiredServers()
		}, nil
	}
	m.mu.Unlock()
	return nil, nil, fmt.Errorf("forced refresh produced no server")
}

func (m *OpenCodeServerManager) acquireNormal(ctx context.Context) (*OpenCodeServerGeneration, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentServer != nil && !m.currentServer.retired {
		m.currentServer.refCount++
		srv := m.currentServer
		released := false
		return srv, func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if released {
				return
			}
			released = true
			srv.refCount--
			m.cleanupRetiredServers()
		}, nil
	}

	if m.startPromise != nil {
		m.mu.Unlock()
		select {
		case <-m.startPromise:
		case <-ctx.Done():
			m.mu.Lock()
			return nil, nil, ctx.Err()
		}
		m.mu.Lock()
		if m.startErr != nil {
			return nil, nil, m.startErr
		}
		if m.currentServer != nil {
			m.currentServer.refCount++
			srv := m.currentServer
			released := false
			return srv, func() {
				m.mu.Lock()
				defer m.mu.Unlock()
				if released {
					return
				}
				released = true
				srv.refCount--
				m.cleanupRetiredServers()
			}, nil
		}
	}

	m.startPromise = make(chan struct{})
	m.mu.Unlock()

	server, err := m.startServer(ctx)

	m.mu.Lock()
	m.startErr = err
	if err != nil {
		close(m.startPromise)
		m.startPromise = nil
		return nil, nil, fmt.Errorf("start opencode server: %w", err)
	}
	m.currentServer = server
	close(m.startPromise)
	m.startPromise = nil

	server.refCount++
	released := false
	return server, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if released {
			return
		}
		released = true
		server.refCount--
		m.cleanupRetiredServers()
	}, nil
}

func (m *OpenCodeServerManager) EnsureRunning(ctx context.Context) (string, error) {
	server, release, err := m.Acquire(ctx, false)
	if err != nil {
		return "", err
	}
	release()
	return server.url, nil
}

func (m *OpenCodeServerManager) RotateCurrentServer(ctx context.Context) error {
	m.mu.Lock()
	if m.currentServer != nil {
		m.currentServer.retired = true
		m.retiredServers[m.currentServer] = struct{}{}
		m.currentServer = nil
	}
	m.mu.Unlock()

	server, err := m.startServer(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.currentServer = server
	m.cleanupRetiredServers()
	m.mu.Unlock()
	return nil
}

func (m *OpenCodeServerManager) startServer(ctx context.Context) (*OpenCodeServerGeneration, error) {
	port, err := findAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("find available port: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	args := []string{"serve", "--port", fmt.Sprintf("%d", port)}

	// Use exec.Command (not CommandContext) so the process is not killed
	// when the caller's context is cancelled after successful startup.
	// The caller's context is only honoured during the startup wait phase.
	cmd := exec.Command(m.binaryPath, args...)
	cmd.Env = m.buildEnv()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start opencode server: %w", err)
	}

	started := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "listening on") {
				select {
				case started <- struct{}{}:
				default:
				}
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			m.logger.Warn("opencode server stderr", "line", scanner.Text())
		}
	}()

	select {
	case <-started:
	case <-time.After(opencodeServerStartTimeout):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("opencode server startup timeout")
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("opencode server startup cancelled: %w", ctx.Err())
	}

	m.logger.Info("opencode server started", "port", port, "url", url)

	return &OpenCodeServerGeneration{
		cmd:  cmd,
		port: port,
		url:  url,
	}, nil
}

// parentSessionEnvVars are env vars that indicate a running Claude Code session.
// If the daemon itself is launched from inside Claude Code, these leak into
// child processes and cause "cannot be launched inside another session" errors.
var parentSessionEnvVars = []string{
	"CLAUDECODE",
	"CLAUDE_CODE_ENTRYPOINT",
	"CLAUDE_CODE_SSE_PORT",
	"CLAUDE_AGENT_SDK_VERSION",
	"CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING",
}

func (m *OpenCodeServerManager) buildEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !isParentSessionEnvVar(e) {
			env = append(env, e)
		}
	}
	if len(m.env) > 0 {
		env = append(env, m.env...)
	}
	return env
}

func isParentSessionEnvVar(e string) bool {
	for _, key := range parentSessionEnvVars {
		if strings.HasPrefix(e, key+"=") {
			return true
		}
	}
	return false
}

func (m *OpenCodeServerManager) cleanupRetiredServers() {
	for s := range m.retiredServers {
		if s.refCount <= 0 {
			m.killServer(s)
			delete(m.retiredServers, s)
		}
	}
}

// --- OpenCode Runtime Settings ---

type OpenCodeRuntimeSettings struct {
	BinaryPath string
	ExtraEnv   []string
}
