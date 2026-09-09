package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/protocol"
)

// seedVersions creates executable fake builds with ascending mtimes (oldest
// first) and returns the versions dir.
func seedVersions(t *testing.T, soloHome string, names ...string) string {
	t.Helper()
	dir := filepath.Join(soloHome, "versions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	mtime := time.Now().Add(-time.Duration(len(names)) * time.Hour)
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("fake"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		mtime = mtime.Add(time.Hour)
	}
	// A non-executable and a wrongly-named file must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "solo-noexec"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other-file"), []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newVersionTestSession(t *testing.T, supervised bool) (*Session, *sendQueue, *exitRecorder) {
	t.Helper()
	s, q := newCaptureSession()
	s.cfg = &config.Config{
		ServerID:   "test-server",
		Supervised: supervised,
		SoloHome:   t.TempDir(),
		Version:    "v0.7.4",
	}
	s.clientID = "client-1"
	rec := newExitRecorder()
	s.SetExitRequester(rec.record)
	return s, q, rec
}

func TestSession_HandleListDaemonVersions(t *testing.T) {
	s, q, _ := newVersionTestSession(t, true)
	dir := seedVersions(t, s.cfg.SoloHome, "solo-v0.7.4", "solo-v0.8.0")
	if err := os.WriteFile(filepath.Join(dir, "current"), []byte("solo-v0.7.4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s.handleListDaemonVersions(&protocol.ListDaemonVersionsRequest{
		Type:      "list_daemon_versions_request",
		RequestID: "req-1",
	})

	msgs := drainMessages(q)
	resp := findSessionMessage(msgs, "list_daemon_versions_response")
	if resp == nil {
		t.Fatalf("expected list response, got %v", msgs)
	}
	payload := resp["payload"].(map[string]interface{})
	if payload["runningVersion"] != "v0.7.4" {
		t.Errorf("runningVersion = %v", payload["runningVersion"])
	}
	if payload["currentVersion"] != "solo-v0.7.4" {
		t.Errorf("currentVersion = %v", payload["currentVersion"])
	}
	versions := payload["versions"].([]interface{})
	if len(versions) != 2 {
		t.Fatalf("versions = %v (non-executable and wrongly-named files must be excluded)", versions)
	}
	// Newest mtime first.
	if versions[0].(map[string]interface{})["version"] != "solo-v0.8.0" {
		t.Errorf("versions[0] = %v, want newest solo-v0.8.0", versions[0])
	}
}

func TestSession_HandleListDaemonVersions_MissingDir(t *testing.T) {
	s, q, _ := newVersionTestSession(t, true)

	s.handleListDaemonVersions(&protocol.ListDaemonVersionsRequest{
		Type:      "list_daemon_versions_request",
		RequestID: "req-2",
	})

	msgs := drainMessages(q)
	resp := findSessionMessage(msgs, "list_daemon_versions_response")
	if resp == nil {
		t.Fatalf("expected list response, got %v", msgs)
	}
	payload := resp["payload"].(map[string]interface{})
	if versions := payload["versions"].([]interface{}); len(versions) != 0 {
		t.Errorf("versions = %v, want empty", versions)
	}
	if payload["error"] != nil {
		t.Errorf("error = %v, want nil for missing dir", payload["error"])
	}
	if _, present := payload["currentVersion"]; present {
		t.Errorf("currentVersion should be omitted, got %v", payload["currentVersion"])
	}
}

func TestSession_HandleListDaemonVersions_WithSupervisorState(t *testing.T) {
	s, q, _ := newVersionTestSession(t, true)
	stateJSON := `{"schemaVersion":1,"state":"backoff","pid":123,"spawnedBinary":"solo-bad",
		"pointerVersion":"solo-bad","consecutiveCrashes":2,"backoffMs":4000,
		"lastExitCode":1,"updatedAtMs":1700000000000,"lastEvent":"daemon crashed exit=1"}`
	if err := os.WriteFile(filepath.Join(s.cfg.SoloHome, supervisorStateFileName), []byte(stateJSON), 0644); err != nil {
		t.Fatal(err)
	}

	s.handleListDaemonVersions(&protocol.ListDaemonVersionsRequest{
		Type:      "list_daemon_versions_request",
		RequestID: "req-sup",
	})

	msgs := drainMessages(q)
	resp := findSessionMessage(msgs, "list_daemon_versions_response")
	if resp == nil {
		t.Fatalf("expected list response, got %v", msgs)
	}
	payload := resp["payload"].(map[string]interface{})
	sup, ok := payload["supervisor"].(map[string]interface{})
	if !ok {
		t.Fatalf("supervisor = %v, want object", payload["supervisor"])
	}
	if sup["state"] != "backoff" || sup["pid"] != float64(123) || sup["consecutiveCrashes"] != float64(2) {
		t.Errorf("supervisor = %v", sup)
	}
}

func TestSession_HandleListDaemonVersions_SupervisorStateDegrades(t *testing.T) {
	// Corrupt state file: response still succeeds without a supervisor entry.
	s, q, _ := newVersionTestSession(t, true)
	if err := os.WriteFile(filepath.Join(s.cfg.SoloHome, supervisorStateFileName), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	s.handleListDaemonVersions(&protocol.ListDaemonVersionsRequest{
		Type:      "list_daemon_versions_request",
		RequestID: "req-corrupt",
	})
	payload := findSessionMessage(drainMessages(q), "list_daemon_versions_response")["payload"].(map[string]interface{})
	if _, present := payload["supervisor"]; present {
		t.Errorf("supervisor = %v, want omitted for corrupt state file", payload["supervisor"])
	}

	// Unsupervised daemon: never relays a supervisor state, even if a stale
	// file is present.
	s2, q2, _ := newVersionTestSession(t, false)
	if err := os.WriteFile(filepath.Join(s2.cfg.SoloHome, supervisorStateFileName), []byte(`{"state":"running"}`), 0644); err != nil {
		t.Fatal(err)
	}
	s2.handleListDaemonVersions(&protocol.ListDaemonVersionsRequest{
		Type:      "list_daemon_versions_request",
		RequestID: "req-unsup",
	})
	payload2 := findSessionMessage(drainMessages(q2), "list_daemon_versions_response")["payload"].(map[string]interface{})
	if _, present := payload2["supervisor"]; present {
		t.Errorf("supervisor = %v, want omitted when unsupervised", payload2["supervisor"])
	}
}

func TestSession_HandleSwitchDaemonVersion_Latest(t *testing.T) {
	s, q, rec := newVersionTestSession(t, true)
	dir := seedVersions(t, s.cfg.SoloHome, "solo-v0.7.4", "solo-v0.8.0")

	s.handleSwitchDaemonVersion(&protocol.SwitchDaemonVersionRequest{
		Type:      "switch_daemon_version_request",
		RequestID: "req-3",
	})

	msgs := drainMessages(q)
	status := findSessionMessage(msgs, "status")
	if status == nil {
		t.Fatalf("expected status reply, got %v", msgs)
	}
	payload := status["payload"].(map[string]interface{})
	if payload["status"] != "daemon_version_switch_requested" {
		t.Errorf("payload.status = %v", payload["status"])
	}
	if payload["version"] != "solo-v0.8.0" {
		t.Errorf("payload.version = %v, want newest", payload["version"])
	}
	if payload["requestId"] != "req-3" {
		t.Errorf("payload.requestId = %v", payload["requestId"])
	}

	// Pointer file written atomically with the newest build.
	data, err := os.ReadFile(filepath.Join(dir, "current"))
	if err != nil {
		t.Fatalf("read pointer: %v", err)
	}
	if got := string(data); got != "solo-v0.8.0\n" {
		t.Errorf("pointer = %q", got)
	}

	select {
	case req := <-rec.ch:
		if req.Code != protocol.ExitCodeRestartRequested {
			t.Errorf("exit code = %d, want %d", req.Code, protocol.ExitCodeRestartRequested)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no exit requested after version switch")
	}
}

func TestSession_HandleSwitchDaemonVersion_Explicit(t *testing.T) {
	s, q, _ := newVersionTestSession(t, true)
	dir := seedVersions(t, s.cfg.SoloHome, "solo-v0.7.4", "solo-v0.8.0")

	version := "solo-v0.7.4"
	s.handleSwitchDaemonVersion(&protocol.SwitchDaemonVersionRequest{
		Type:      "switch_daemon_version_request",
		Version:   &version,
		RequestID: "req-4",
	})

	msgs := drainMessages(q)
	status := findSessionMessage(msgs, "status")
	if status == nil {
		t.Fatalf("expected status reply, got %v", msgs)
	}
	if v := status["payload"].(map[string]interface{})["version"]; v != "solo-v0.7.4" {
		t.Errorf("payload.version = %v", v)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "current"))
	if string(data) != "solo-v0.7.4\n" {
		t.Errorf("pointer = %q", data)
	}
}

func TestSession_HandleSwitchDaemonVersion_NoOpWhenAlreadyRunning(t *testing.T) {
	s, q, rec := newVersionTestSession(t, true)
	dir := seedVersions(t, s.cfg.SoloHome, "solo-v0.7.4", "solo-v0.8.0")
	// Pointer targets the build the daemon is running (Version "v0.7.4" ↔ solo-v0.7.4).
	if err := os.WriteFile(filepath.Join(dir, "current"), []byte("solo-v0.7.4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	version := "solo-v0.7.4"
	s.handleSwitchDaemonVersion(&protocol.SwitchDaemonVersionRequest{
		Type:      "switch_daemon_version_request",
		Version:   &version,
		RequestID: "req-5",
	})

	msgs := drainMessages(q)
	if findSessionMessage(msgs, "status") == nil {
		t.Fatalf("expected status reply, got %v", msgs)
	}
	select {
	case req := <-rec.ch:
		t.Fatalf("no exit expected for no-op switch, got %+v", req)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestSession_HandleSwitchDaemonVersion_Errors(t *testing.T) {
	t.Run("unsupervised refused", func(t *testing.T) {
		s, q, rec := newVersionTestSession(t, false)
		seedVersions(t, s.cfg.SoloHome, "solo-v0.8.0")

		s.handleSwitchDaemonVersion(&protocol.SwitchDaemonVersionRequest{
			Type:      "switch_daemon_version_request",
			RequestID: "req-6",
		})

		msgs := drainMessages(q)
		rpcErr := findSessionMessage(msgs, "rpc_error")
		if rpcErr == nil {
			t.Fatalf("expected rpc_error, got %v", msgs)
		}
		if code := rpcErr["payload"].(map[string]interface{})["code"]; code != "NOT_SUPERVISED" {
			t.Errorf("code = %v", code)
		}
		select {
		case req := <-rec.ch:
			t.Fatalf("no exit expected, got %+v", req)
		case <-time.After(300 * time.Millisecond):
		}
	})

	t.Run("no versions", func(t *testing.T) {
		s, q, _ := newVersionTestSession(t, true)

		s.handleSwitchDaemonVersion(&protocol.SwitchDaemonVersionRequest{
			Type:      "switch_daemon_version_request",
			RequestID: "req-7",
		})

		msgs := drainMessages(q)
		rpcErr := findSessionMessage(msgs, "rpc_error")
		if rpcErr == nil {
			t.Fatalf("expected rpc_error, got %v", msgs)
		}
		if code := rpcErr["payload"].(map[string]interface{})["code"]; code != "NO_VERSIONS" {
			t.Errorf("code = %v", code)
		}
	})

	t.Run("version not found", func(t *testing.T) {
		s, q, _ := newVersionTestSession(t, true)
		seedVersions(t, s.cfg.SoloHome, "solo-v0.8.0")

		version := "solo-v9.9.9"
		s.handleSwitchDaemonVersion(&protocol.SwitchDaemonVersionRequest{
			Type:      "switch_daemon_version_request",
			Version:   &version,
			RequestID: "req-8",
		})

		msgs := drainMessages(q)
		rpcErr := findSessionMessage(msgs, "rpc_error")
		if rpcErr == nil {
			t.Fatalf("expected rpc_error, got %v", msgs)
		}
		if code := rpcErr["payload"].(map[string]interface{})["code"]; code != "VERSION_NOT_FOUND" {
			t.Errorf("code = %v", code)
		}
	})
}
