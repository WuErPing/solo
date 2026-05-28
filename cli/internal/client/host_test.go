package client

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestResolveHost_Explicit(t *testing.T) {
	wsURL, err := ResolveHost("localhost:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wsURL != "ws://localhost:8080/ws" {
		t.Errorf("expected ws://localhost:8080/ws, got %s", wsURL)
	}
}

func TestResolveHost_Default(t *testing.T) {
	os.Unsetenv("SOLO_LISTEN")
	wsURL, err := ResolveHost("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wsURL != "ws://127.0.0.1:17612/ws" {
		t.Errorf("expected default wsURL, got %s", wsURL)
	}
}

func TestResolveHost_EnvVar(t *testing.T) {
	os.Setenv("SOLO_LISTEN", "192.168.1.1:9000")
	defer os.Unsetenv("SOLO_LISTEN")

	wsURL, err := ResolveHost("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wsURL != "ws://192.168.1.1:9000/ws" {
		t.Errorf("expected ws://192.168.1.1:9000/ws, got %s", wsURL)
	}
}

func TestResolveHost_ConfigFile(t *testing.T) {
	// Override SOLO_HOME to a temp directory
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")
	os.Unsetenv("SOLO_LISTEN")

	listen := "10.0.0.1:7777"
	cfg := map[string]interface{}{
		"daemon": map[string]interface{}{
			"listen": listen,
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.MkdirAll(home, 0755)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	wsURL, err := ResolveHost("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wsURL != "ws://10.0.0.1:7777/ws" {
		t.Errorf("expected ws://10.0.0.1:7777/ws, got %s", wsURL)
	}
}

func TestToWSURL_BarePort(t *testing.T) {
	if got := toWSURL("17612"); got != "ws://127.0.0.1:17612/ws" {
		t.Errorf("expected ws://127.0.0.1:17612/ws, got %s", got)
	}
}

func TestToWSURL_HostPort(t *testing.T) {
	if got := toWSURL("myhost:9090"); got != "ws://myhost:9090/ws" {
		t.Errorf("expected ws://myhost:9090/ws, got %s", got)
	}
}

func TestToWSURL_HostOnly(t *testing.T) {
	if got := toWSURL("myhost"); got != "ws://myhost/ws" {
		t.Errorf("expected ws://myhost/ws, got %s", got)
	}
}

func TestToWSURL_Unix(t *testing.T) {
	if got := toWSURL("unix:///tmp/solo.sock"); got != "ws+unix:///tmp/solo.sock/ws" {
		t.Errorf("expected ws+unix:///tmp/solo.sock/ws, got %s", got)
	}
}

func TestToWSURL_Pipe(t *testing.T) {
	if got := toWSURL("pipe:///tmp/solo.pipe"); got != "ws+unix:///tmp/solo.pipe/ws" {
		t.Errorf("expected ws+unix:///tmp/solo.pipe/ws, got %s", got)
	}
}

func TestExtractIPCPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"unix:///tmp/solo.sock", "/tmp/solo.sock"},
		{"pipe:///tmp/solo.pipe", "/tmp/solo.pipe"},
		{"localhost:8080", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := extractIPCPath(tc.input)
		if got != tc.expected {
			t.Errorf("extractIPCPath(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestDialerForHost_IPC(t *testing.T) {
	dialer := DialerForHost("unix:///tmp/solo.sock")
	if dialer.NetDial == nil {
		t.Error("expected custom NetDial for IPC host")
	}
}

func TestDialerForHost_TCP(t *testing.T) {
	dialer := DialerForHost("localhost:8080")
	if dialer.NetDial != nil {
		t.Error("expected no custom NetDial for TCP host")
	}
}

func TestConfigListen_MissingFile(t *testing.T) {
	// Ensure no config file exists in temp home
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	got := configListen()
	if got != "" {
		t.Errorf("expected empty string for missing config, got %q", got)
	}
}

func TestConfigListen_InvalidJSON(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "config.json"), []byte("not json"), 0644)
	got := configListen()
	if got != "" {
		t.Errorf("expected empty string for invalid JSON, got %q", got)
	}
}

func TestConfigListen_Valid(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	listen := "0.0.0.0:8888"
	cfg := map[string]interface{}{
		"daemon": map[string]interface{}{
			"listen": listen,
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	got := configListen()
	if got != listen {
		t.Errorf("expected %q, got %q", listen, got)
	}
}

func TestConfigListen_NilDaemon(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	cfg := map[string]interface{}{}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	got := configListen()
	if got != "" {
		t.Errorf("expected empty string when daemon is missing, got %q", got)
	}
}

func TestReadPIDFile_PlainInt(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "solo.pid"), []byte("12345\n"), 0644)
	pid, err := readPIDFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 12345 {
		t.Errorf("expected pid 12345, got %d", pid)
	}
}

func TestReadPIDFile_JSON(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	data := []byte(`{"pid":9999,"listen":"127.0.0.1:17612"}`)
	_ = os.WriteFile(filepath.Join(home, "solo.pid"), data, 0644)
	pid, err := readPIDFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 9999 {
		t.Errorf("expected pid 9999, got %d", pid)
	}
}

func TestReadPIDFile_Invalid(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "solo.pid"), []byte("invalid"), 0644)
	_, err := readPIDFile()
	if err == nil {
		t.Error("expected error for invalid PID file")
	}
}

func TestReadPIDFile_Missing(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_, err := readPIDFile()
	if err == nil {
		t.Error("expected error for missing PID file")
	}
}

func TestIsDaemonRunning_NotRunning(t *testing.T) {
	// Use a PID that is extremely unlikely to exist
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "solo.pid"), []byte(strconv.Itoa(999999)), 0644)

	running, pid, err := IsDaemonRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Error("expected daemon not running")
	}
	if pid != 999999 {
		t.Errorf("expected pid 999999, got %d", pid)
	}
}

func TestReadServerID_Success(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "server-id"), []byte("my-server-id\n"), 0644)
	id, err := ReadServerID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-server-id" {
		t.Errorf("expected my-server-id, got %q", id)
	}
}

func TestReadServerID_Missing(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_, err := ReadServerID()
	if err == nil {
		t.Error("expected error for missing server-id")
	}
}

func TestReadServerID_Empty(t *testing.T) {
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")

	_ = os.WriteFile(filepath.Join(home, "server-id"), []byte("\n"), 0644)
	_, err := ReadServerID()
	if err == nil {
		t.Error("expected error for empty server-id")
	}
}

func TestResolveHost_ConfigURLFormat(t *testing.T) {
	// Ensure that ResolveHost returns a valid ws URL even when config has host:port
	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	defer os.Unsetenv("SOLO_HOME")
	os.Unsetenv("SOLO_LISTEN")

	cfg := map[string]interface{}{
		"daemon": map[string]interface{}{
			"listen": "127.0.0.1:17612",
		},
	}
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(home, "config.json"), data, 0644)

	wsURL, err := ResolveHost("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, err := url.Parse(wsURL)
	if err != nil {
		t.Fatalf("failed to parse wsURL: %v", err)
	}
	if u.Scheme != "ws" {
		t.Errorf("expected ws scheme, got %s", u.Scheme)
	}
	if u.Path != "/ws" {
		t.Errorf("expected /ws path, got %s", u.Path)
	}
}
