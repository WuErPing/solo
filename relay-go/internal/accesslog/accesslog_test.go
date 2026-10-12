package accesslog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func readEvents(t *testing.T, data string) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Fatalf("line is not valid JSON: %q: %v", line, err)
		}
		events = append(events, v)
	}
	return events
}

func assertTimestamp(t *testing.T, v map[string]any) {
	t.Helper()
	s, ok := v["ts"].(string)
	if !ok {
		t.Fatalf("ts missing or not a string: %v", v)
	}
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		t.Fatalf("ts %q is not RFC3339: %v", s, err)
	}
}

func TestWriteEvents(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.HTTPRequest("GET", "/health", 200, 3*time.Millisecond, "1.2.3.4", "curl/8.0")
	l.WSOpen("srv1", "client", "conn_abc", "2", "1.2.3.4", "agent/1.0", "https://solo.up2ai.top")
	l.WSClose("srv1", "client", "conn_abc", 1500*time.Millisecond, 42)

	events := readEvents(t, buf.String())
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	e := events[0]
	if e["event"] != "http_req" || e["method"] != "GET" || e["path"] != "/health" ||
		e["status"] != float64(200) || e["cost_ms"] != float64(3) ||
		e["ip"] != "1.2.3.4" || e["ua"] != "curl/8.0" {
		t.Fatalf("unexpected http_req event: %v", e)
	}
	assertTimestamp(t, e)

	e = events[1]
	if e["event"] != "ws_open" || e["server_id"] != "srv1" || e["role"] != "client" ||
		e["conn_id"] != "conn_abc" || e["version"] != "2" ||
		e["ip"] != "1.2.3.4" || e["ua"] != "agent/1.0" || e["origin"] != "https://solo.up2ai.top" {
		t.Fatalf("unexpected ws_open event: %v", e)
	}
	assertTimestamp(t, e)

	e = events[2]
	if e["event"] != "ws_close" || e["server_id"] != "srv1" || e["role"] != "client" ||
		e["conn_id"] != "conn_abc" || e["dur_ms"] != float64(1500) || e["msgs"] != float64(42) {
		t.Fatalf("unexpected ws_close event: %v", e)
	}
	assertTimestamp(t, e)
}

func TestOpenAppendsAndCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.log")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	l.HTTPRequest("GET", "/a", 200, time.Millisecond, "10.0.0.1", "ua1")
	if err := l.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Re-opening must append, not truncate.
	l, err = Open(path)
	if err != nil {
		t.Fatalf("second Open failed: %v", err)
	}
	l.WSOpen("s", "server", "", "2", "10.0.0.2", "ua2", "")
	if err := l.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("expected mode 0644, got %v", info.Mode().Perm())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	events := readEvents(t, string(data))
	if len(events) != 2 {
		t.Fatalf("expected 2 events after append, got %d", len(events))
	}
	if events[0]["event"] != "http_req" || events[1]["event"] != "ws_open" {
		t.Fatalf("unexpected events: %v", events)
	}
}

func TestNewCloseIsNoop(t *testing.T) {
	var buf bytes.Buffer
	if err := New(&buf).Close(); err != nil {
		t.Fatalf("Close on writer-based logger must be a no-op, got %v", err)
	}
}

func TestConcurrentWrites(t *testing.T) {
	// The logger's own mutex must make concurrent writes race-free and keep
	// every line a complete JSON document (run with -race).
	var buf bytes.Buffer
	l := New(&buf)

	const goroutines, perG = 20, 25
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				l.HTTPRequest("GET", "/x", 200, time.Millisecond, "127.0.0.1", "ua")
			}
		}()
	}
	wg.Wait()

	events := readEvents(t, buf.String())
	if len(events) != goroutines*perG {
		t.Fatalf("expected %d events, got %d", goroutines*perG, len(events))
	}
}
