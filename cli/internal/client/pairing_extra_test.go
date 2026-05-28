package client

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateDaemonKeyPair_GeneratesNew(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SOLO_HOME", tmpDir)

	kp, err := LoadOrCreateDaemonKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp.PublicKeyB64 == "" {
		t.Error("expected non-empty PublicKeyB64")
	}
	if kp.SecretKeyB64 == "" {
		t.Error("expected non-empty SecretKeyB64")
	}

	// Verify file was written
	filePath := filepath.Join(tmpDir, keypairFilename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("keypair file not written: %v", err)
	}

	var stored struct {
		V            int    `json:"v"`
		PublicKeyB64 string `json:"publicKeyB64"`
		SecretKeyB64 string `json:"secretKeyB64"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("invalid JSON in keypair file: %v", err)
	}
	if stored.V != 2 {
		t.Errorf("expected v=2, got %d", stored.V)
	}
	if stored.PublicKeyB64 != kp.PublicKeyB64 {
		t.Error("stored PublicKeyB64 mismatch")
	}

	// Validate key lengths (Curve25519 = 32 bytes)
	pubBytes, _ := base64.StdEncoding.DecodeString(kp.PublicKeyB64)
	secBytes, _ := base64.StdEncoding.DecodeString(kp.SecretKeyB64)
	if len(pubBytes) != 32 {
		t.Errorf("public key length: got %d, want 32", len(pubBytes))
	}
	if len(secBytes) != 32 {
		t.Errorf("secret key length: got %d, want 32", len(secBytes))
	}
}

func TestLoadOrCreateDaemonKeyPair_LoadsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SOLO_HOME", tmpDir)

	// Create first to generate and persist
	kp1, err := LoadOrCreateDaemonKeyPair()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Second call should load the same keys
	kp2, err := LoadOrCreateDaemonKeyPair()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if kp1.PublicKeyB64 != kp2.PublicKeyB64 {
		t.Error("PublicKeyB64 changed between calls")
	}
	if kp1.SecretKeyB64 != kp2.SecretKeyB64 {
		t.Error("SecretKeyB64 changed between calls")
	}
}

func TestLoadOrCreateDaemonKeyPair_RegeneratesOldEd25519Key(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SOLO_HOME", tmpDir)

	// Write an old v=1 keypair (Ed25519 style with 64-byte secret)
	oldSecret := make([]byte, 64)
	oldPublic := make([]byte, 32)
	payload := map[string]interface{}{
		"v":            1,
		"publicKeyB64": base64.StdEncoding.EncodeToString(oldPublic),
		"secretKeyB64": base64.StdEncoding.EncodeToString(oldSecret),
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	filePath := filepath.Join(tmpDir, keypairFilename)
	os.WriteFile(filePath, append(data, '\n'), 0600)

	kp, err := LoadOrCreateDaemonKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have regenerated — new key must differ from old
	secBytes, _ := base64.StdEncoding.DecodeString(kp.SecretKeyB64)
	if len(secBytes) != 32 {
		t.Errorf("expected 32-byte secret after regeneration, got %d", len(secBytes))
	}
}

func TestGeneratePairingOffer(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SOLO_HOME", tmpDir)

	url, err := GeneratePairingOffer("test-server-123", "solo.up2ai.top:443", "https://solo.up2ai.top")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "https://solo.up2ai.top/#offer=") {
		t.Errorf("unexpected URL prefix: %s", url)
	}

	// Decode and verify the offer
	decoded, err := DecodePairingOffer(url)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.ServerID != "test-server-123" {
		t.Errorf("ServerID: got %q, want test-server-123", decoded.ServerID)
	}
	if decoded.Relay.Endpoint != "solo.up2ai.top:443" {
		t.Errorf("Relay.Endpoint: got %q, want solo.up2ai.top:443", decoded.Relay.Endpoint)
	}
	if decoded.DaemonPublicKeyB64 == "" {
		t.Error("expected non-empty DaemonPublicKeyB64")
	}
}

func TestGeneratePairingOffer_TrailingSlash(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SOLO_HOME", tmpDir)

	url, err := GeneratePairingOffer("s1", "relay.test:443", "https://solo.up2ai.top/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(url, "//#offer=") {
		t.Error("trailing slash not stripped properly")
	}
}
