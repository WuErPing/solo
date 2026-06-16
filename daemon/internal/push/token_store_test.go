package push

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInMemoryTokenStore_RegisterAndGetAll(t *testing.T) {
	store := NewInMemoryTokenStore()

	store.Register("token1")
	store.Register("token2")

	tokens := store.GetAll()
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	// Check both tokens are present
	hasToken1, hasToken2 := false, false
	for _, tok := range tokens {
		if tok == "token1" {
			hasToken1 = true
		}
		if tok == "token2" {
			hasToken2 = true
		}
	}
	if !hasToken1 || !hasToken2 {
		t.Errorf("expected both tokens, got %v", tokens)
	}
}

func TestInMemoryTokenStore_DuplicateRegistration(t *testing.T) {
	store := NewInMemoryTokenStore()

	store.Register("token1")
	store.Register("token1") // duplicate

	tokens := store.GetAll()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after duplicate registration, got %d", len(tokens))
	}
}

func TestInMemoryTokenStore_Remove(t *testing.T) {
	store := NewInMemoryTokenStore()

	store.Register("token1")
	store.Register("token2")
	store.Remove("token1")

	tokens := store.GetAll()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after removal, got %d", len(tokens))
	}
	if tokens[0] != "token2" {
		t.Errorf("expected token2, got %s", tokens[0])
	}
}

func TestInMemoryTokenStore_RemoveNonExistent(t *testing.T) {
	store := NewInMemoryTokenStore()

	store.Register("token1")
	store.Remove("nonexistent")

	tokens := store.GetAll()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
}

func TestInMemoryTokenStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryTokenStore()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			store.Register("token")
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.GetAll()
		}()
	}

	wg.Wait()

	tokens := store.GetAll()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after concurrent registrations, got %d", len(tokens))
	}
}

func TestInMemoryTokenStore_EmptyStore(t *testing.T) {
	store := NewInMemoryTokenStore()

	tokens := store.GetAll()
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}

// --- PersistedTokenStore tests ---

func TestPersistedTokenStore_LoadFromDisk(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")
	os.WriteFile(filePath, []byte(`{"tokens":["tok-a","tok-b"]}`), 0644)

	store := NewPersistedTokenStore(filePath, slog.Default())
	tokens := store.GetAll()
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	hasA, hasB := false, false
	for _, tok := range tokens {
		if tok == "tok-a" {
			hasA = true
		}
		if tok == "tok-b" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Errorf("expected tok-a and tok-b, got %v", tokens)
	}
}

func TestPersistedTokenStore_PersistsOnRegister(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")
	store := NewPersistedTokenStore(filePath, slog.Default())

	store.Register("tok-new")

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	var raw struct {
		Tokens []string `json:"tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse file: %v", err)
	}
	found := false
	for _, tok := range raw.Tokens {
		if tok == "tok-new" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tok-new in persisted file, got %v", raw.Tokens)
	}
}

func TestPersistedTokenStore_PersistsOnRemove(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")
	os.WriteFile(filePath, []byte(`{"tokens":["tok-a","tok-b"]}`), 0644)

	store := NewPersistedTokenStore(filePath, slog.Default())
	store.Remove("tok-a")

	// Reload from disk
	store2 := NewPersistedTokenStore(filePath, slog.Default())
	tokens := store2.GetAll()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after reload, got %d", len(tokens))
	}
	if tokens[0] != "tok-b" {
		t.Errorf("expected tok-b, got %s", tokens[0])
	}
}

func TestPersistedTokenStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")

	store := NewPersistedTokenStore(filePath, slog.Default())
	store.Register("tok-1")

	// Verify final file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	// Verify no .tmp file remains
	if _, err := os.Stat(filePath + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected no .tmp file, but it exists")
	}
}

func TestPersistedTokenStore_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")
	os.WriteFile(filePath, []byte(`{invalid json`), 0644)

	store := NewPersistedTokenStore(filePath, slog.Default())
	tokens := store.GetAll()
	if len(tokens) != 0 {
		t.Errorf("expected empty tokens on corrupt file, got %d", len(tokens))
	}
}

func TestPersistedTokenStore_MissingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")

	store := NewPersistedTokenStore(filePath, slog.Default())
	tokens := store.GetAll()
	if len(tokens) != 0 {
		t.Errorf("expected empty tokens on missing file, got %d", len(tokens))
	}
}

func TestPersistedTokenStore_DedupAndTrimOnLoad(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")
	os.WriteFile(filePath, []byte(`{"tokens":["tok-a"," tok-a ","","tok-b"]}`), 0644)

	store := NewPersistedTokenStore(filePath, slog.Default())
	tokens := store.GetAll()
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens after dedup/trim, got %d: %v", len(tokens), tokens)
	}
	hasA, hasB := false, false
	for _, tok := range tokens {
		if tok == "tok-a" {
			hasA = true
		}
		if tok == "tok-b" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Errorf("expected tok-a and tok-b after dedup/trim, got %v", tokens)
	}
}

func TestPersistedTokenStore_EmptyTokenIgnored(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tokens.json")
	store := NewPersistedTokenStore(filePath, slog.Default())

	store.Register("")
	store.Register("   ")

	tokens := store.GetAll()
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens when registering empty/whitespace, got %d", len(tokens))
	}
}
