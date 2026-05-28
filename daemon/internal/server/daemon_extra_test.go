package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/config"
)

func TestCheckOrigin_NoOriginHeader(t *testing.T) {
	ws := &WSServer{cfg: &config.Config{}}
	req := httptest.NewRequest("GET", "/ws", nil)
	if !ws.checkOrigin(req) {
		t.Error("expected true when no Origin header")
	}
}

func TestCheckOrigin_EmptyCORSOrigins(t *testing.T) {
	ws := &WSServer{cfg: &config.Config{CORSOrigins: []string{}}}
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://evil.com")
	if !ws.checkOrigin(req) {
		t.Error("expected true when CORSOrigins is empty")
	}
}

func TestCheckOrigin_AllowedOrigin(t *testing.T) {
	ws := &WSServer{cfg: &config.Config{CORSOrigins: []string{"https://solo.up2ai.top", "http://localhost:19000"}}}

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://solo.up2ai.top")
	if !ws.checkOrigin(req) {
		t.Error("expected true for allowed origin")
	}

	req2 := httptest.NewRequest("GET", "/ws", nil)
	req2.Header.Set("Origin", "http://localhost:19000")
	if !ws.checkOrigin(req2) {
		t.Error("expected true for second allowed origin")
	}
}

func TestCheckOrigin_RejectedOrigin(t *testing.T) {
	ws := &WSServer{cfg: &config.Config{CORSOrigins: []string{"https://solo.up2ai.top"}}}
	ws.logger = newTestLogger()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://evil.com")
	if ws.checkOrigin(req) {
		t.Error("expected false for rejected origin")
	}
}

func TestHandleStatus(t *testing.T) {
	cfg := &config.Config{
		ServerID: "test-server-123",
		Version:  "1.2.3",
		Listen:   "127.0.0.1:17612",
	}

	handler := handleStatus(cfg)
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test-server-123") {
		t.Error("expected serverId in response")
	}
	if !strings.Contains(body, "1.2.3") {
		t.Error("expected version in response")
	}
	if !strings.Contains(body, "127.0.0.1:17612") {
		t.Error("expected listen address in response")
	}
}
