package base

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func newTestBaseSession(t *testing.T) *BaseSession {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := &protocol.AgentSessionConfig{
		Cwd: "/tmp/test",
	}
	return NewBaseSession("claude", config, logger)
}

func TestBaseSession_Provider(t *testing.T) {
	s := newTestBaseSession(t)
	if s.Provider() != "claude" {
		t.Errorf("Provider: got %q, want claude", s.Provider())
	}
}

func TestBaseSession_SessionID(t *testing.T) {
	s := newTestBaseSession(t)
	if s.SessionID() != "" {
		t.Error("expected empty session ID initially")
	}
	s.SetSessionID("sess-123")
	if s.SessionID() != "sess-123" {
		t.Errorf("SessionID: got %q", s.SessionID())
	}
}

func TestBaseSession_Config(t *testing.T) {
	s := newTestBaseSession(t)
	cfg := s.Config()
	if cfg == nil || cfg.Cwd != "/tmp/test" {
		t.Errorf("Config: got %+v", cfg)
	}
}

func TestBaseSession_CurrentMode(t *testing.T) {
	s := newTestBaseSession(t)
	if s.CurrentMode() != "" {
		t.Error("expected empty mode initially")
	}
	_ = s.SetMode("plan")
	if s.CurrentMode() != "plan" {
		t.Errorf("CurrentMode: got %q", s.CurrentMode())
	}
}

func TestBaseSession_CurrentModel(t *testing.T) {
	s := newTestBaseSession(t)
	if s.CurrentModel() != "" {
		t.Error("expected empty model initially")
	}
	_ = s.SetModel("claude-sonnet")
	if s.CurrentModel() != "claude-sonnet" {
		t.Errorf("CurrentModel: got %q", s.CurrentModel())
	}
}

func TestBaseSession_CurrentThinking(t *testing.T) {
	s := newTestBaseSession(t)
	if s.CurrentThinking() != "" {
		t.Error("expected empty thinking initially")
	}
	_ = s.SetThinkingOption("extended")
	if s.CurrentThinking() != "extended" {
		t.Errorf("CurrentThinking: got %q", s.CurrentThinking())
	}
}

func TestBaseSession_IsClosed(t *testing.T) {
	s := newTestBaseSession(t)
	if s.IsClosed() {
		t.Error("expected not closed initially")
	}
	_ = s.Close()
	if !s.IsClosed() {
		t.Error("expected closed after Close")
	}
}

func TestBaseSession_Close_Idempotent(t *testing.T) {
	s := newTestBaseSession(t)
	_ = s.Close()
	_ = s.Close() // should not panic
	if !s.IsClosed() {
		t.Error("expected closed")
	}
}

func TestBaseSession_Cancel(t *testing.T) {
	s := newTestBaseSession(t)
	ctx, cancel := context.WithCancel(context.Background())
	s.SetCancelFn(cancel)

	s.Cancel()
	select {
	case <-ctx.Done():
		// good
	default:
		t.Error("expected context to be cancelled")
	}
}

func TestBaseSession_GetRuntimeInfo(t *testing.T) {
	s := newTestBaseSession(t)
	s.SetSessionID("sess-1")
	_ = s.SetMode("plan")
	_ = s.SetModel("sonnet")
	_ = s.SetThinkingOption("extended")

	info := s.GetRuntimeInfo()
	if info.Provider != "claude" {
		t.Errorf("Provider: got %q", info.Provider)
	}
	if info.SessionID == nil || *info.SessionID != "sess-1" {
		t.Error("expected SessionID")
	}
	if info.ModeID == nil || *info.ModeID != "plan" {
		t.Error("expected ModeID")
	}
	if info.Model == nil || *info.Model != "sonnet" {
		t.Error("expected Model")
	}
	if info.ThinkingOptionID == nil || *info.ThinkingOptionID != "extended" {
		t.Error("expected ThinkingOptionID")
	}
}

func TestBaseSession_GetRuntimeInfo_Empty(t *testing.T) {
	s := newTestBaseSession(t)
	info := s.GetRuntimeInfo()
	if info.SessionID != nil {
		t.Error("expected nil SessionID when empty")
	}
	if info.ModeID != nil {
		t.Error("expected nil ModeID when empty")
	}
	if info.Model != nil {
		t.Error("expected nil Model when empty")
	}
	if info.ThinkingOptionID != nil {
		t.Error("expected nil ThinkingOptionID when empty")
	}
}

func TestBaseSession_DescribePersistence(t *testing.T) {
	s := newTestBaseSession(t)
	if s.DescribePersistence() != nil {
		t.Error("expected nil persistence before session ID is set")
	}

	s.SetSessionID("sess-1")
	handle := s.DescribePersistence()
	if handle == nil {
		t.Fatal("expected persistence handle")
	}
	if handle.Provider != "claude" {
		t.Errorf("Provider: got %q", handle.Provider)
	}
	if handle.SessionID != "sess-1" {
		t.Errorf("SessionID: got %q", handle.SessionID)
	}
}

func TestBaseSession_SetCurrentModel(t *testing.T) {
	s := newTestBaseSession(t)
	s.SetCurrentModel("sonnet")
	if s.CurrentModel() != "sonnet" {
		t.Errorf("CurrentModel: got %q", s.CurrentModel())
	}
	s.SetCurrentModel("") // empty should not overwrite
	if s.CurrentModel() != "sonnet" {
		t.Errorf("CurrentModel should not be overwritten by empty string")
	}
}

func TestBaseSession_SetCurrentMode(t *testing.T) {
	s := newTestBaseSession(t)
	s.SetCurrentMode("plan")
	if s.CurrentMode() != "plan" {
		t.Errorf("CurrentMode: got %q", s.CurrentMode())
	}
	s.SetCurrentMode("") // empty should not overwrite
	if s.CurrentMode() != "plan" {
		t.Errorf("CurrentMode should not be overwritten by empty string")
	}
}

func TestStrPtr(t *testing.T) {
	if strPtr("") != nil {
		t.Error("expected nil for empty string")
	}
	p := strPtr("test")
	if p == nil || *p != "test" {
		t.Error("expected pointer to 'test'")
	}
}
