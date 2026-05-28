package filebackend

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/memory"
)

// ---------- File layout ----------

func TestFileLayout_PathComponents(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	turn := memory.NewTurn("sess-A", memory.RoleAssistant, memory.SourceApp, ts, "body")
	turn.Seq = 7

	ctx := context.Background()
	if err := r.RecordTurn(ctx, "sess-A", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}
	if err := r.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Expected: <projectRoot>/.solo/memory/sessions/2026-05/sess-A/turns/0007-assistant.md
	want := filepath.Join(proj, "memory", "sessions", "2026-05", "sess-A", "turns", "0007-assistant.md")
	if _, err := os.Stat(want); err != nil {
		// Walk to show what we actually produced.
		var got []string
		_ = filepath.Walk(proj, func(p string, _ os.FileInfo, _ error) error {
			got = append(got, p)
			return nil
		})
		t.Fatalf("expected file at %s (err=%v); produced:\n  %s", want, err, strings.Join(got, "\n  "))
	}
}

func TestFileLayout_DirectoryPermissions(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Now().UTC()
	turn := memory.NewTurn("sess-perm", memory.RoleUser, memory.SourceCLI, ts, "x")
	turn.Seq = 1
	if err := r.RecordTurn(context.Background(), "sess-perm", turn); err != nil {
		t.Fatalf("RecordTurn: %v", err)
	}
	if err := r.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	turnsDir := filepath.Join(proj, "memory", "sessions",
		ts.Format("2006-01"), "sess-perm", "turns")
	sessionDir := filepath.Dir(turnsDir)

	for _, dir := range []string{turnsDir, sessionDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if got := info.Mode().Perm(); got != 0o755 {
			t.Errorf("%s mode = %o, want 755", dir, got)
		}
	}
}

func TestFileLayout_FilePermissions(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Now().UTC()
	turn := memory.NewTurn("sess-fmode", memory.RoleUser, memory.SourceCLI, ts, "x")
	turn.Seq = 1
	ctx := context.Background()
	_ = r.RecordTurn(ctx, "sess-fmode", turn)
	_ = r.Flush(ctx)

	ym := ts.Format("2006-01")
	path := filepath.Join(proj, "memory", "sessions", ym, "sess-fmode", "turns", "0001-user.md")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("file mode = %o, want 644", got)
	}
}

func TestFileLayout_DuplicateTurnID_DoesNotOverwrite(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	first := memory.NewTurn("sess-dup", memory.RoleUser, memory.SourceCLI, ts, "FIRST")
	first.Seq = 1
	first.ID = "dup-id"

	ctx := context.Background()
	_ = r.RecordTurn(ctx, "sess-dup", first)
	_ = r.Flush(ctx)

	second := first
	second.Content = "SECOND"
	_ = r.RecordTurn(ctx, "sess-dup", second)
	_ = r.Flush(ctx)

	ym := ts.Format("2006-01")
	path := filepath.Join(proj, "memory", "sessions", ym, "sess-dup", "turns", "0001-user.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "FIRST") {
		t.Errorf("first content missing:\n%s", data)
	}
	if strings.Contains(string(data), "SECOND") {
		t.Errorf("file was overwritten (should be write-once):\n%s", data)
	}
}

// ---------- Frontmatter ----------

func TestTurnFile_ContainsFrontmatterFences(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Date(2026, 5, 28, 10, 23, 45, 0, time.UTC)
	turn := memory.NewTurn("sess-fm", memory.RoleAssistant, memory.SourceCLI, ts, "hello")
	turn.Seq = 1

	ctx := context.Background()
	_ = r.RecordTurn(ctx, "sess-fm", turn)
	_ = r.Flush(ctx)

	path := filepath.Join(proj, "memory", "sessions",
		"2026-05", "sess-fm", "turns", "0001-assistant.md")
	data, _ := os.ReadFile(path)
	s := string(data)

	if !strings.HasPrefix(s, "---\n") {
		t.Errorf("file must start with '---\\n', got:\n%s", s)
	}
	// Find the closing fence: the second '---\n' after offset 4.
	rest := strings.Index(s[4:], "\n---\n")
	if rest < 0 {
		t.Fatalf("missing closing fence:\n%s", s)
	}
	frontmatter := s[4 : 4+rest]
	body := s[4+rest+5:] // skip "\n---\n"

	for _, needle := range []string{
		"id: " + turn.ID,
		"sessionId: sess-fm",
		"seq: 1",
		"role: assistant",
		"ts: 2026-05-28T10:23:45Z",
		"source: cli",
	} {
		if !strings.Contains(frontmatter, needle) {
			t.Errorf("frontmatter missing %q\n---\n%s", needle, frontmatter)
		}
	}
	if !strings.Contains(body, "hello") {
		t.Errorf("body missing 'hello':\n%s", body)
	}
	if strings.Contains(frontmatter, "hello") {
		t.Errorf("body leaked into frontmatter:\n%s", frontmatter)
	}
}

// ---------- sessions.jsonl ----------

func TestSessionsJSONL_FirstTurnCreatesEntry(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Now().UTC()
	turn := memory.NewTurn("sess-idx", memory.RoleUser, memory.SourceCLI, ts, "x")
	turn.Seq = 1

	ctx := context.Background()
	_ = r.RecordTurn(ctx, "sess-idx", turn)
	_ = r.Flush(ctx)

	indexPath := filepath.Join(proj, "memory", "sessions.jsonl")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d:\n%s", len(lines), data)
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["id"] != "sess-idx" {
		t.Errorf("id = %v, want sess-idx", entry["id"])
	}
}

func TestSessionsJSONL_MultipleSessionsProduceMultipleEntries(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Now().UTC()
	ctx := context.Background()
	for _, sess := range []string{"sess-A", "sess-B", "sess-C"} {
		turn := memory.NewTurn(sess, memory.RoleUser, memory.SourceCLI, ts, "x")
		turn.Seq = 1
		_ = r.RecordTurn(ctx, sess, turn)
	}
	_ = r.Flush(ctx)

	data, _ := os.ReadFile(filepath.Join(proj, "memory", "sessions.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), data)
	}
}

func TestSessionsJSONL_SameSessionDoesNotDuplicate(t *testing.T) {
	r, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()

	proj := r.cfg.BaseDir
	ts := time.Now().UTC()
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		turn := memory.NewTurn("sess-repeat", memory.RoleUser, memory.SourceCLI, ts, "x")
		turn.Seq = uint64(i)
		_ = r.RecordTurn(ctx, "sess-repeat", turn)
	}
	_ = r.Flush(ctx)

	data, _ := os.ReadFile(filepath.Join(proj, "memory", "sessions.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (one session), got %d:\n%s", len(lines), data)
	}
}
