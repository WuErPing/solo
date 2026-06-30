// Package httpx provides shared HTTP clients with sane timeouts.
//
// A bare &http.Client{} has no timeout: if the remote target is unreachable or
// stops responding mid-request, the calling goroutine blocks forever, leaking
// goroutines and connections. This package centralises the timeout policy so
// callers cannot reintroduce that bug.
//
// Two shapes are offered:
//
//   - Standard() — short request/response calls (e.g. push notifications). The
//     whole request is bounded by an overall http.Client.Timeout.
//   - Streaming() — long-lived streams (e.g. SSE). It deliberately has NO
//     overall timeout, because that would kill a healthy long-running stream;
//     the caller bounds the stream with a context. Connect, TLS handshake,
//     response-header and idle-connection phases are still bounded.
package httpx

import (
	"net"
	"net/http"
	"time"
)

// Shared timeout defaults. These bound the phases where an unreachable or
// stalled target would otherwise hang a goroutine indefinitely.
const (
	// ConnectTimeout bounds the TCP connect and the TLS handshake.
	ConnectTimeout = 5 * time.Second
	// ResponseHeaderTimeout bounds the wait for response headers after the
	// request is written. It does NOT limit reading the response body, so it is
	// safe to apply to long-lived streaming connections.
	ResponseHeaderTimeout = 10 * time.Second
	// IdleConnTimeout bounds how long an idle keep-alive connection is pooled
	// before being closed.
	IdleConnTimeout = 90 * time.Second
	// RequestTimeout is the overall per-request cap for the standard client.
	RequestTimeout = 30 * time.Second
)

// Config controls the timeouts of a client built by NewClient.
type Config struct {
	// ConnectTimeout bounds TCP connect and TLS handshake.
	ConnectTimeout time.Duration
	// ResponseHeaderTimeout bounds waiting for response headers.
	ResponseHeaderTimeout time.Duration
	// IdleConnTimeout bounds idle keep-alive connections in the pool.
	IdleConnTimeout time.Duration
	// RequestTimeout sets http.Client.Timeout — the overall cap for a single
	// request. Leave it zero for streaming clients that keep the body open.
	RequestTimeout time.Duration
}

// NewClient builds an *http.Client with a dedicated transport from cfg.
func NewClient(cfg Config) *http.Client {
	return &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   cfg.ConnectTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			TLSHandshakeTimeout:   cfg.ConnectTimeout,
			ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
			IdleConnTimeout:       cfg.IdleConnTimeout,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

var (
	standardClient = NewClient(Config{
		ConnectTimeout:        ConnectTimeout,
		ResponseHeaderTimeout: ResponseHeaderTimeout,
		IdleConnTimeout:       IdleConnTimeout,
		RequestTimeout:        RequestTimeout,
	})

	streamingClient = NewClient(Config{
		ConnectTimeout:        ConnectTimeout,
		ResponseHeaderTimeout: ResponseHeaderTimeout,
		IdleConnTimeout:       IdleConnTimeout,
		// No overall RequestTimeout: a streaming response stays open and is
		// bounded by the caller's context instead.
		RequestTimeout: 0,
	})
)

// Standard returns a shared client for short request/response calls. The whole
// request is bounded by RequestTimeout.
func Standard() *http.Client { return standardClient }

// Streaming returns a shared client for long-lived streaming (e.g. SSE). It has
// no overall timeout — the caller must bound the stream with a context — but
// connect, TLS handshake, response headers and idle connections are bounded.
func Streaming() *http.Client { return streamingClient }
