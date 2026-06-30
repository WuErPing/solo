package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestStandard_HasOverallTimeout guards the push-style client: a short
// request/response call must be bounded end-to-end so a stuck request cannot
// leak a goroutine forever.
func TestStandard_HasOverallTimeout(t *testing.T) {
	c := Standard()
	if c.Timeout <= 0 {
		t.Fatalf("standard client must set an overall timeout, got %v", c.Timeout)
	}
}

// TestStreaming_NoOverallTimeout guards the SSE-style client: a long-lived
// stream must NOT be killed by an overall http.Client.Timeout. The stream is
// bounded by the caller's context (the 120s idle watchdog) instead.
func TestStreaming_NoOverallTimeout(t *testing.T) {
	c := Streaming()
	if c.Timeout != 0 {
		t.Fatalf("streaming client must not set an overall timeout (it would kill SSE), got %v", c.Timeout)
	}
}

// TestSharedTransportTimeouts verifies both clients carry the transport-level
// bounds that actually stop a leak against an unreachable/stalled target.
func TestSharedTransportTimeouts(t *testing.T) {
	clients := map[string]*http.Client{"standard": Standard(), "streaming": Streaming()}
	for name, c := range clients {
		tr, ok := c.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("%s: expected *http.Transport, got %T", name, c.Transport)
		}
		if tr.DialContext == nil {
			t.Errorf("%s: DialContext must be set (connect timeout)", name)
		}
		if tr.TLSHandshakeTimeout != ConnectTimeout {
			t.Errorf("%s: TLSHandshakeTimeout = %v, want %v", name, tr.TLSHandshakeTimeout, ConnectTimeout)
		}
		if tr.ResponseHeaderTimeout != ResponseHeaderTimeout {
			t.Errorf("%s: ResponseHeaderTimeout = %v, want %v", name, tr.ResponseHeaderTimeout, ResponseHeaderTimeout)
		}
		if tr.IdleConnTimeout != IdleConnTimeout {
			t.Errorf("%s: IdleConnTimeout = %v, want %v", name, tr.IdleConnTimeout, IdleConnTimeout)
		}
	}
}

// TestOverallTimeoutFires proves the standard-style client aborts a hung
// request promptly instead of blocking forever — the core goroutine-leak guard.
func TestOverallTimeoutFires(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // hang until the client gives up
	}))
	defer srv.Close()

	c := NewClient(Config{
		ConnectTimeout: time.Second,
		RequestTimeout: 100 * time.Millisecond,
	})

	start := time.Now()
	resp, err := c.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("request did not time out promptly: %v", elapsed)
	}
}

// TestResponseHeaderTimeoutFires proves a streaming-style client (no overall
// timeout) still aborts when the server stalls before sending response headers.
func TestResponseHeaderTimeoutFires(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // never send headers
	}))
	defer srv.Close()

	c := NewClient(Config{
		ConnectTimeout:        time.Second,
		ResponseHeaderTimeout: 100 * time.Millisecond,
		// RequestTimeout left 0, mirroring the streaming client.
	})

	start := time.Now()
	resp, err := c.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected response-header timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("request did not time out promptly: %v", elapsed)
	}
}
