package plantuml

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/config"
)

const sampleSVG = `<?plantuml 1.2026.2?><svg xmlns="http://www.w3.org/2000/svg" width="10px" height="20px" viewBox="0 0 10 20"></svg>`

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-plantuml")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func TestRender_CommandBackend(t *testing.T) {
	script := writeScript(t, "cat >/dev/null\ncat <<'EOF'\n"+sampleSVG+"\nEOF\n")
	r := NewRenderer(config.PlantUMLConfig{Command: script}, testLogger())

	svg, err := r.Render(context.Background(), "@startuml\nBob -> Alice : hi\n@enduml", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Errorf("expected SVG, got %q", svg)
	}
}

// TestRender_RealPlantUMLBinary exercises the golden path against an actually
// installed PlantUML (including JVM start and dark skinparams). Skipped when the
// binary is absent or in -short mode (CI).
func TestRender_RealPlantUMLBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("real PlantUML render skipped in -short mode")
	}
	binary, err := exec.LookPath("plantuml")
	if err != nil {
		t.Skip("plantuml not installed")
	}

	r := NewRenderer(config.PlantUMLConfig{Command: binary, TimeoutSeconds: 30}, testLogger())
	svg, err := r.Render(context.Background(), "@startuml\nAlice -> Bob: hello\n@enduml", true)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Fatalf("expected SVG output, got %q", svg)
	}
	// Dark skinparams must reach the rendered figure.
	if !strings.Contains(svg, "#161B22") {
		t.Errorf("expected dark shape colour in SVG")
	}
	if strings.Contains(svg, "background:#FFFFFF") {
		t.Errorf("expected transparent background in dark mode")
	}
}

func TestRender_CommandFailureIncludesStderr(t *testing.T) {
	script := writeScript(t, "cat >/dev/null\necho 'syntax error near line 2' >&2\nexit 1\n")
	r := NewRenderer(config.PlantUMLConfig{Command: script}, testLogger())

	_, err := r.Render(context.Background(), "@startuml\nbogus\n@enduml", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "syntax error near line 2") {
		t.Errorf("error should surface stderr, got %v", err)
	}
}

func TestRender_CommandTimeout(t *testing.T) {
	// exec replaces the shell so CommandContext kills the sleeping process
	// itself (a plain child would keep the inherited pipes open).
	script := writeScript(t, "cat >/dev/null\nexec sleep 5\n")
	r := NewRenderer(config.PlantUMLConfig{Command: script, TimeoutSeconds: 1}, testLogger())

	start := time.Now()
	_, err := r.Render(context.Background(), "@startuml\nBob -> Alice : hi\n@enduml", false)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Errorf("render did not time out promptly: %v", elapsed)
	}
}

func TestRender_CachesBySourceAndTheme(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	script := writeScript(t, "cat >/dev/null\necho x >> "+counter+"\ncat <<'EOF'\n"+sampleSVG+"\nEOF\n")
	r := NewRenderer(config.PlantUMLConfig{Command: script}, testLogger())

	for i := 0; i < 3; i++ {
		if _, err := r.Render(context.Background(), "@startuml\nBob -> Alice : hi\n@enduml", false); err != nil {
			t.Fatalf("Render: %v", err)
		}
	}
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if got := strings.Count(string(data), "x"); got != 1 {
		t.Errorf("expected one execution for repeated source, got %d", got)
	}

	// Dark mode renders a different source (skinparams), so it must not reuse
	// the light cache entry.
	if _, err := r.Render(context.Background(), "@startuml\nBob -> Alice : hi\n@enduml", true); err != nil {
		t.Fatalf("Render dark: %v", err)
	}
	data, _ = os.ReadFile(counter)
	if got := strings.Count(string(data), "x"); got != 2 {
		t.Errorf("expected a second execution for dark mode, got %d", got)
	}
}

func TestRender_ServerBackend(t *testing.T) {
	var gotPath, gotContentType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotContentType = req.Header.Get("Content-Type")
		body, _ := io.ReadAll(req.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(sampleSVG))
	}))
	defer server.Close()

	r := NewRenderer(config.PlantUMLConfig{ServerURL: server.URL + "/plantuml/"}, testLogger())
	svg, err := r.Render(context.Background(), "@startuml\nBob -> Alice : hi\n@enduml", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Errorf("expected SVG, got %q", svg)
	}
	if gotPath != "/plantuml/svg" {
		t.Errorf("path: got %q", gotPath)
	}
	if !strings.HasPrefix(gotContentType, "text/plain") {
		t.Errorf("content type: got %q", gotContentType)
	}
	if !strings.Contains(gotBody, "Bob -> Alice") {
		t.Errorf("body: got %q", gotBody)
	}
}

func TestRender_ServerErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "syntax error", http.StatusBadRequest)
	}))
	defer server.Close()

	r := NewRenderer(config.PlantUMLConfig{ServerURL: server.URL}, testLogger())
	_, err := r.Render(context.Background(), "bad", false)
	if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "syntax error") {
		t.Errorf("expected status and body in error, got %v", err)
	}
}

func TestRender_InvalidServerURL(t *testing.T) {
	r := NewRenderer(config.PlantUMLConfig{ServerURL: "ftp://example.com"}, testLogger())
	_, err := r.Render(context.Background(), "@startuml\n@enduml", false)
	if err == nil || !strings.Contains(err.Error(), "http(s)") {
		t.Errorf("expected invalid-url error, got %v", err)
	}
}

func TestRender_Unavailable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	t.Setenv(commandEnvVar, missing)
	r := NewRenderer(config.PlantUMLConfig{}, testLogger())

	_, err := r.Render(context.Background(), "@startuml\n@enduml", false)
	if err == nil || !strings.Contains(err.Error(), "not accessible") {
		t.Errorf("expected unavailable error, got %v", err)
	}
}

func TestRender_SourceLimits(t *testing.T) {
	r := NewRenderer(config.PlantUMLConfig{Command: "/bin/true"}, testLogger())
	if _, err := r.Render(context.Background(), "   ", false); err == nil {
		t.Error("expected empty-source error")
	}
	tooLarge := strings.Repeat("x", maxSourceBytes+1)
	if _, err := r.Render(context.Background(), tooLarge, false); err == nil {
		t.Error("expected source-size error")
	}
}

func TestApplyDarkSkinparams(t *testing.T) {
	withStart := "@startuml\nBob -> Alice : hi\n@enduml"
	got := applyDarkSkinparams(withStart)
	if !strings.HasPrefix(got, "@startuml\nskinparam backgroundColor transparent") {
		t.Errorf("expected preamble after @startuml, got:\n%s", got[:min(len(got), 120)])
	}
	if !strings.HasSuffix(got, "Bob -> Alice : hi\n@enduml") {
		t.Errorf("source tail was modified:\n%s", got)
	}

	withoutStart := "Bob -> Alice : hi"
	got = applyDarkSkinparams(withoutStart)
	if !strings.HasPrefix(got, "skinparam backgroundColor transparent") {
		t.Errorf("expected preamble at the top, got:\n%s", got[:min(len(got), 120)])
	}
	if !strings.HasSuffix(got, withoutStart) {
		t.Errorf("source tail was modified:\n%s", got)
	}
}

func TestCappedBuffer(t *testing.T) {
	b := &cappedBuffer{limit: 4}
	n, err := b.Write([]byte("abcdef"))
	if err != nil || n != 6 {
		t.Fatalf("Write: n=%d err=%v", n, err)
	}
	if !b.overflow || b.String() != "abcd" {
		t.Errorf("overflow=%v content=%q", b.overflow, b.String())
	}
}

func TestCacheEvictsLeastRecentlyUsed(t *testing.T) {
	r := NewRenderer(config.PlantUMLConfig{Command: "/bin/true"}, testLogger())
	for i := 0; i < maxCacheEntries+5; i++ {
		r.cacheSet(fmt.Sprintf("k%d", i), "svg")
	}
	if r.lru.Len() != maxCacheEntries {
		t.Errorf("cache length: got %d, want %d", r.lru.Len(), maxCacheEntries)
	}
	if _, ok := r.cacheGet("k0"); ok {
		t.Error("oldest entry should have been evicted")
	}
	if _, ok := r.cacheGet(fmt.Sprintf("k%d", maxCacheEntries+4)); !ok {
		t.Error("newest entry should be cached")
	}
}
