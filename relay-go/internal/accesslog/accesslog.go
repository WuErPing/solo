// Package accesslog writes relay access events as JSON lines (one event per
// line) for offline analysis with DuckDB. All writes are serialized through a
// mutex, so a single Logger is safe to share across request goroutines.
package accesslog

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Logger appends JSON-lines events to an io.Writer. Write errors are dropped
// on purpose: access logging must never break request handling.
type Logger struct {
	mu  sync.Mutex
	enc *json.Encoder
	c   io.Closer // non-nil only when the Logger owns the underlying file
}

// New returns a Logger writing to w. Close is a no-op for such loggers.
func New(w io.Writer) *Logger {
	return &Logger{enc: json.NewEncoder(w)}
}

// Open creates (or appends to) the file at path and returns a Logger that owns
// it; Close then flushes and closes the file handle.
func Open(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	l := New(f)
	l.c = f
	return l, nil
}

// Close closes the underlying file when the Logger was created via Open.
func (l *Logger) Close() error {
	if l.c == nil {
		return nil
	}
	return l.c.Close()
}

func (l *Logger) write(v any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.enc.Encode(v)
}

func ts() string {
	return time.Now().Format(time.RFC3339)
}

type httpReqEvent struct {
	TS     string `json:"ts"`
	Event  string `json:"event"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	CostMS int64  `json:"cost_ms"`
	IP     string `json:"ip"`
	UA     string `json:"ua"`
}

// HTTPRequest logs one handled HTTP request (WebSocket upgrades included; a
// successful upgrade shows status 101).
func (l *Logger) HTTPRequest(method, path string, status int, cost time.Duration, ip, ua string) {
	l.write(httpReqEvent{
		TS: ts(), Event: "http_req",
		Method: method, Path: path, Status: status,
		CostMS: cost.Milliseconds(), IP: ip, UA: ua,
	})
}

type wsOpenEvent struct {
	TS       string `json:"ts"`
	Event    string `json:"event"`
	ServerID string `json:"server_id"`
	Role     string `json:"role"`
	ConnID   string `json:"conn_id"`
	Version  string `json:"version"`
	IP       string `json:"ip"`
	UA       string `json:"ua"`
	Origin   string `json:"origin"`
}

// WSOpen logs a WebSocket connection right after upgrade + registration.
func (l *Logger) WSOpen(serverID, role, connID, version, ip, ua, origin string) {
	l.write(wsOpenEvent{
		TS: ts(), Event: "ws_open",
		ServerID: serverID, Role: role, ConnID: connID, Version: version,
		IP: ip, UA: ua, Origin: origin,
	})
}

type wsCloseEvent struct {
	TS       string `json:"ts"`
	Event    string `json:"event"`
	ServerID string `json:"server_id"`
	Role     string `json:"role"`
	ConnID   string `json:"conn_id"`
	DurMS    int64  `json:"dur_ms"`
	Msgs     int64  `json:"msgs"`
}

// WSClose logs a closed WebSocket connection with its lifetime and the number
// of messages read from it while it lived.
func (l *Logger) WSClose(serverID, role, connID string, dur time.Duration, msgs int64) {
	l.write(wsCloseEvent{
		TS: ts(), Event: "ws_close",
		ServerID: serverID, Role: role, ConnID: connID,
		DurMS: dur.Milliseconds(), Msgs: msgs,
	})
}
