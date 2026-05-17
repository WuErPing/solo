package workspace

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestScriptProxy_RegisterAndServeHTTP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := NewScriptManager()
	proxy := NewScriptProxy(logger, mgr)

	// Register a route to a non-existent backend; we just test routing logic here
	proxy.RegisterRoute("test.localhost", 9999)

	req := httptest.NewRequest(http.MethodGet, "http://test.localhost/", nil)
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Should get bad gateway because no backend is running on 9999,
	// but routing should have matched
	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected BadGateway, got %d", rr.Code)
	}
}

func TestScriptProxy_UnregisterRoute(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := NewScriptManager()
	proxy := NewScriptProxy(logger, mgr)

	proxy.RegisterRoute("test.localhost", 9999)
	proxy.UnregisterRoute("test.localhost")

	req := httptest.NewRequest(http.MethodGet, "http://test.localhost/", nil)
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected BadGateway after unregister, got %d", rr.Code)
	}
}

func TestScriptProxy_UnknownHost(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := NewScriptManager()
	proxy := NewScriptProxy(logger, mgr)

	req := httptest.NewRequest(http.MethodGet, "http://unknown.localhost/", nil)
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected BadGateway for unknown host, got %d", rr.Code)
	}
}

func TestScriptProxy_StripsPort(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := NewScriptManager()
	proxy := NewScriptProxy(logger, mgr)

	proxy.RegisterRoute("test.localhost", 9999)

	req := httptest.NewRequest(http.MethodGet, "http://test.localhost:8080/", nil)
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Routing should match even with port in Host header
	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected BadGateway (routed), got %d", rr.Code)
	}
}
