package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"
)

const defaultHost = "127.0.0.1:17612"

// ResolveHost determines the WebSocket URL to connect to.
// Priority: explicit host > SOLO_LISTEN env > config.json > default.
func ResolveHost(explicitHost string) (wsURL string, err error) {
	host := explicitHost
	if host == "" {
		host = os.Getenv("SOLO_LISTEN")
	}
	if host == "" {
		host = configListen()
	}
	if host == "" {
		host = defaultHost
	}
	return toWSURL(host), nil
}

// IsDaemonRunning checks if the daemon process is alive by reading the PID file.
func IsDaemonRunning() (bool, int, error) {
	pid, err := readPIDFile()
	if err != nil {
		return false, 0, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid, nil
	}
	return true, pid, nil
}

// ReadServerID reads the daemon server ID from ~/.solo/server-id.
func ReadServerID() (string, error) {
	home := soloHome()
	data, err := os.ReadFile(filepath.Join(home, "server-id"))
	if err != nil {
		return "", fmt.Errorf("read server-id: %w", err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", fmt.Errorf("server-id is empty")
	}
	return id, nil
}

// DialerForHost returns a websocket.Dialer appropriate for the host type.
// For unix:// or pipe:// hosts, returns a dialer with a custom NetDial.
func DialerForHost(host string) *websocket.Dialer {
	path := extractIPCPath(host)
	if path == "" {
		return websocket.DefaultDialer
	}
	return &websocket.Dialer{
		NetDial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", path)
		},
	}
}

// toWSURL converts a host specification to a WebSocket URL.
func toWSURL(host string) string {
	h := strings.TrimSpace(host)

	// IPC targets
	if strings.HasPrefix(h, "unix://") || strings.HasPrefix(h, "pipe://") {
		path := extractIPCPath(h)
		return "ws+unix://" + path + "/ws"
	}

	// Bare port number
	if _, err := strconv.Atoi(h); err == nil {
		return "ws://127.0.0.1:" + h + "/ws"
	}

	// Already has colon (host:port)
	if strings.Contains(h, ":") {
		return "ws://" + h + "/ws"
	}

	// Fallback
	return "ws://" + h + "/ws"
}

func extractIPCPath(host string) string {
	h := strings.TrimSpace(host)
	if strings.HasPrefix(h, "unix://") {
		return strings.TrimPrefix(h, "unix://")
	}
	if strings.HasPrefix(h, "pipe://") {
		return strings.TrimPrefix(h, "pipe://")
	}
	return ""
}

// configListen reads the listen address from ~/.solo/config.json.
func configListen() string {
	home := soloHome()
	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		return ""
	}
	var cfg struct {
		Daemon *struct {
			Listen *string `json:"listen,omitempty"`
		} `json:"daemon,omitempty"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	if cfg.Daemon != nil && cfg.Daemon.Listen != nil && *cfg.Daemon.Listen != "" {
		return *cfg.Daemon.Listen
	}
	return ""
}

// readPIDFile reads the daemon PID from ~/.solo/solo.pid.
// Supports both plain integer PID and JSON {"pid": N} formats.
func readPIDFile() (int, error) {
	home := soloHome()
	data, err := os.ReadFile(filepath.Join(home, "solo.pid"))
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(data))

	// Try plain integer first
	if pid, err := strconv.Atoi(s); err == nil {
		return pid, nil
	}

	// Try JSON format
	var pidFile struct {
		PID    int    `json:"pid"`
		Listen string `json:"listen,omitempty"`
	}
	if err := json.Unmarshal(data, &pidFile); err == nil && pidFile.PID > 0 {
		return pidFile.PID, nil
	}

	return 0, fmt.Errorf("invalid PID file content: %s", s)
}
