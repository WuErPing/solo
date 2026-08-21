package server

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/protocol"
)

// exitRecorder captures requestExit calls for assertions.
type exitRecorder struct {
	ch chan ExitRequest
}

func newExitRecorder() *exitRecorder {
	return &exitRecorder{ch: make(chan ExitRequest, 1)}
}

func (r *exitRecorder) record(code int, reason string) {
	r.ch <- ExitRequest{Code: code, Reason: reason}
}

// findSessionMessage returns the first session-envelope message with the given
// inner type, or nil.
func findSessionMessage(msgs []map[string]interface{}, innerType string) map[string]interface{} {
	for _, m := range msgs {
		if m["type"] != "session" {
			continue
		}
		if msg, ok := m["message"].(map[string]interface{}); ok && msg["type"] == innerType {
			return msg
		}
	}
	return nil
}

func TestSession_HandleRestartServer_Supervised(t *testing.T) {
	s, q := newCaptureSession()
	s.cfg = &config.Config{ServerID: "test-server", Supervised: true}
	s.clientID = "client-1"
	rec := newExitRecorder()
	s.SetExitRequester(rec.record)

	reason := "apply config"
	s.handleRestartServer(&protocol.RestartServerRequest{
		Type:      "restart_server_request",
		Reason:    &reason,
		RequestID: "req-1",
	})

	msgs := drainMessages(q)
	status := findSessionMessage(msgs, "status")
	if status == nil {
		t.Fatalf("expected status reply, got %v", msgs)
	}
	payload, ok := status["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("status payload missing: %v", status)
	}
	if payload["status"] != "restart_requested" {
		t.Errorf("payload.status = %v", payload["status"])
	}
	if payload["clientId"] != "client-1" {
		t.Errorf("payload.clientId = %v", payload["clientId"])
	}
	if payload["requestId"] != "req-1" {
		t.Errorf("payload.requestId = %v", payload["requestId"])
	}
	if payload["reason"] != "apply config" {
		t.Errorf("payload.reason = %v", payload["reason"])
	}

	select {
	case req := <-rec.ch:
		if req.Code != protocol.ExitCodeRestartRequested {
			t.Errorf("exit code = %d, want %d", req.Code, protocol.ExitCodeRestartRequested)
		}
		if req.Reason != "apply config" {
			t.Errorf("exit reason = %q", req.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no exit requested after restart reply")
	}
}

func TestSession_HandleRestartServer_Unsupervised_Refused(t *testing.T) {
	s, q := newCaptureSession()
	s.cfg = &config.Config{ServerID: "test-server", Supervised: false}
	s.clientID = "client-1"
	rec := newExitRecorder()
	s.SetExitRequester(rec.record)

	s.handleRestartServer(&protocol.RestartServerRequest{
		Type:      "restart_server_request",
		RequestID: "req-2",
	})

	msgs := drainMessages(q)
	rpcErr := findSessionMessage(msgs, "rpc_error")
	if rpcErr == nil {
		t.Fatalf("expected rpc_error reply, got %v", msgs)
	}
	payload, ok := rpcErr["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("rpc_error payload missing: %v", rpcErr)
	}
	if payload["requestId"] != "req-2" {
		t.Errorf("payload.requestId = %v", payload["requestId"])
	}
	if payload["code"] != "NOT_SUPERVISED" {
		t.Errorf("payload.code = %v", payload["code"])
	}

	select {
	case req := <-rec.ch:
		t.Fatalf("exit must not be requested for unsupervised daemon, got %+v", req)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestSession_HandleShutdownServer(t *testing.T) {
	s, q := newCaptureSession()
	s.cfg = &config.Config{ServerID: "test-server", Supervised: true}
	s.clientID = "client-1"
	rec := newExitRecorder()
	s.SetExitRequester(rec.record)

	s.handleShutdownServer(&protocol.ShutdownServerRequest{
		Type:      "shutdown_server_request",
		RequestID: "req-3",
	})

	msgs := drainMessages(q)
	status := findSessionMessage(msgs, "status")
	if status == nil {
		t.Fatalf("expected status reply, got %v", msgs)
	}
	payload, ok := status["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("status payload missing: %v", status)
	}
	if payload["status"] != "shutdown_requested" {
		t.Errorf("payload.status = %v", payload["status"])
	}
	if payload["requestId"] != "req-3" {
		t.Errorf("payload.requestId = %v", payload["requestId"])
	}

	select {
	case req := <-rec.ch:
		if req.Code != protocol.ExitCodeClean {
			t.Errorf("exit code = %d, want %d", req.Code, protocol.ExitCodeClean)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no exit requested after shutdown reply")
	}
}

func TestDaemon_RequestExit_KeepsFirstRequest(t *testing.T) {
	d := &Daemon{
		exitCh: make(chan ExitRequest, 1),
		logger: newTestLogger(),
	}
	d.RequestExit(protocol.ExitCodeRestartRequested, "first")
	d.RequestExit(protocol.ExitCodeClean, "second")

	select {
	case req := <-d.ExitRequested():
		if req.Code != protocol.ExitCodeRestartRequested || req.Reason != "first" {
			t.Errorf("got %+v, want first request", req)
		}
	default:
		t.Fatal("no exit request recorded")
	}
	select {
	case req := <-d.ExitRequested():
		t.Fatalf("second request should have been dropped, got %+v", req)
	default:
	}
}
