package memory

import (
	"strings"
	"testing"
	"time"
)

// ---------- Construction & field access ----------

func TestNewTurn_RequiredFields(t *testing.T) {
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	turn := NewTurn("sess-1", RoleUser, SourceCLI, ts, "hello")

	if turn.ID == "" {
		t.Error("expected non-empty ID")
	}
	if !IsTurnID(turn.ID) {
		t.Errorf("ID %q does not match turn ID format", turn.ID)
	}
	if turn.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", turn.SessionID, "sess-1")
	}
	if turn.Seq != 0 {
		t.Errorf("Seq = %d, want 0", turn.Seq)
	}
	if turn.Role != RoleUser {
		t.Errorf("Role = %q, want %q", turn.Role, RoleUser)
	}
	if turn.Source != SourceCLI {
		t.Errorf("Source = %q, want %q", turn.Source, SourceCLI)
	}
	if !turn.Ts.Equal(ts) {
		t.Errorf("Ts = %v, want %v", turn.Ts, ts)
	}
	if turn.Content != "hello" {
		t.Errorf("Content = %q, want %q", turn.Content, "hello")
	}
	if turn.ParentID != "" {
		t.Errorf("ParentID = %q, want empty", turn.ParentID)
	}
}

// ---------- Role validation ----------

func TestTurn_ValidRoles(t *testing.T) {
	valid := []TurnRole{RoleUser, RoleAssistant, RoleSystem}
	for _, r := range valid {
		turn := Turn{ID: "x", SessionID: "s", Role: r}
		if err := turn.Validate(); err != nil {
			t.Errorf("role %q should be valid, got: %v", r, err)
		}
	}
}

func TestTurn_InvalidRole(t *testing.T) {
	turn := Turn{ID: "x", SessionID: "s", Role: "bot"}
	err := turn.Validate()
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "role") {
		t.Errorf("error %q should mention role", err)
	}
}

// ---------- Source validation ----------

func TestTurn_ValidSources(t *testing.T) {
	valid := []TurnSource{SourceCLI, SourceApp, SourceRelay}
	for _, s := range valid {
		turn := Turn{ID: "x", SessionID: "s", Role: RoleUser, Source: s}
		if err := turn.Validate(); err != nil {
			t.Errorf("source %q should be valid, got: %v", s, err)
		}
	}
}

func TestTurn_InvalidSource(t *testing.T) {
	turn := Turn{ID: "x", SessionID: "s", Role: RoleUser, Source: "web"}
	err := turn.Validate()
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Errorf("error %q should mention source", err)
	}
}

// ---------- Required-field validation ----------

func TestTurn_ValidateRequiresID(t *testing.T) {
	turn := Turn{SessionID: "s", Role: RoleUser, Source: SourceCLI}
	if err := turn.Validate(); err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestTurn_ValidateRequiresSessionID(t *testing.T) {
	turn := Turn{ID: "x", Role: RoleUser, Source: SourceCLI}
	if err := turn.Validate(); err == nil {
		t.Error("expected error for empty SessionID")
	}
}

func TestTurn_ValidateRequiresRole(t *testing.T) {
	turn := Turn{ID: "x", SessionID: "s", Source: SourceCLI}
	if err := turn.Validate(); err == nil {
		t.Error("expected error for empty Role")
	}
}

func TestTurn_ValidMinimal(t *testing.T) {
	turn := Turn{ID: "x", SessionID: "s", Role: RoleUser}
	if err := turn.Validate(); err != nil {
		t.Errorf("minimal valid turn should pass, got: %v", err)
	}
}

// ---------- ID generation ----------

func TestNewTurnID_Format(t *testing.T) {
	id := NewTurnID()
	if !IsTurnID(id) {
		t.Errorf("NewTurnID() = %q, not recognized by IsTurnID", id)
	}
}

func TestNewTurnID_Monotonic(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	prev := ""
	for i := 0; i < n; i++ {
		id := NewTurnID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID %q on iteration %d", id, i)
		}
		seen[id] = struct{}{}
		if id == "" {
			t.Fatalf("empty ID on iteration %d", i)
		}
		if prev != "" && id <= prev {
			t.Fatalf("IDs not monotonic: %q followed by %q", prev, id)
		}
		prev = id
	}
}

func TestIsTurnID_RejectsBadInput(t *testing.T) {
	bad := []string{
		"",
		"not-an-id",
		"0123456789abcdef-0123456",   // counter too short
		"0123456789abcdef-012345678", // counter too long
		"0123456789abcde-01234567",   // timestamp too short
		"0123456789abcdeff-01234567", // timestamp too long
		"0123456789ABCDEF-01234567",  // upper-case hex
		"0123456789abcdef_01234567",  // wrong separator
	}
	for _, s := range bad {
		if IsTurnID(s) {
			t.Errorf("IsTurnID(%q) = true, want false", s)
		}
	}
}

func TestIsTurnID_AcceptsValidFormat(t *testing.T) {
	if !IsTurnID("0123456789abcdef-01234567") {
		t.Error("IsTurnID should accept the documented 16hex-8hex format")
	}
}

// ---------- Frontmatter YAML serialization ----------

func TestTurn_FrontmatterYAML_Roundtrip(t *testing.T) {
	ts := time.Date(2026, 5, 28, 10, 23, 45, 0, time.UTC)
	meta := &TurnMetadata{
		Model:        "solo-v1",
		Tokens:       &TokenUsage{Prompt: 1234, Completion: 567},
		ToolCalls:    []string{"Read", "Bash"},
		FinishReason: "stop",
	}
	turn := Turn{
		ID:        NewTurnID(),
		SessionID: "sess-1",
		Seq:       3,
		Role:      RoleAssistant,
		Ts:        ts,
		Source:    SourceCLI,
		Content:   "the body — should NOT be in frontmatter",
		Metadata:  meta,
		ParentID:  "parent-2",
	}

	out, err := turn.FrontmatterYAML()
	if err != nil {
		t.Fatalf("FrontmatterYAML: %v", err)
	}
	s := string(out)

	// Required fields present
	for _, needle := range []string{
		"id: " + turn.ID,
		"sessionId: sess-1",
		"seq: 3",
		"role: assistant",
		"ts: 2026-05-28T10:23:45Z",
		"source: cli",
		"parent: parent-2",
		"model: solo-v1",
		"prompt: 1234",
		"completion: 567",
		"finishReason: stop",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("frontmatter missing %q\n---\n%s", needle, s)
		}
	}

	// Content must NOT leak into frontmatter
	if strings.Contains(s, "the body") {
		t.Errorf("frontmatter leaked Content:\n%s", s)
	}
}

func TestTurn_FrontmatterYAML_OmitsEmptyOptional(t *testing.T) {
	turn := Turn{
		ID:        "id-x",
		SessionID: "s",
		Role:      RoleUser,
		Ts:        time.Unix(0, 0).UTC(),
		// Source empty, Metadata nil, ParentID empty
	}

	out, err := turn.FrontmatterYAML()
	if err != nil {
		t.Fatalf("FrontmatterYAML: %v", err)
	}
	s := string(out)

	if strings.Contains(s, "source:") {
		t.Errorf("empty Source should be omitted:\n%s", s)
	}
	if strings.Contains(s, "metadata:") {
		t.Errorf("nil Metadata should be omitted:\n%s", s)
	}
	if strings.Contains(s, "parent:") {
		t.Errorf("empty ParentID should be omitted:\n%s", s)
	}
}

// ---------- Role classification helpers ----------

func TestTurnRole_IsUser(t *testing.T) {
	if !RoleUser.IsUser() {
		t.Error("RoleUser.IsUser should be true")
	}
	if RoleAssistant.IsUser() {
		t.Error("RoleAssistant.IsUser should be false")
	}
}

func TestTurnRole_IsAssistant(t *testing.T) {
	if !RoleAssistant.IsAssistant() {
		t.Error("RoleAssistant.IsAssistant should be true")
	}
	if RoleUser.IsAssistant() {
		t.Error("RoleUser.IsAssistant should be false")
	}
}

func TestTurnRole_IsSystem(t *testing.T) {
	if !RoleSystem.IsSystem() {
		t.Error("RoleSystem.IsSystem should be true")
	}
	if RoleUser.IsSystem() {
		t.Error("RoleUser.IsSystem should be false")
	}
}
