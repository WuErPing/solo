package relayclient

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/nacl/box"
)

func TestLoadDaemonKeyPair_Missing(t *testing.T) {
	_, err := LoadDaemonKeyPair(t.TempDir())
	if err == nil {
		t.Error("expected error for missing keypair")
	}
}

func TestLoadDaemonKeyPair_Valid(t *testing.T) {
	home := t.TempDir()
	pub, priv, err := generateBoxKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	kp := DaemonKeyPair{
		PublicKeyB64: base64.StdEncoding.EncodeToString(pub[:]),
		SecretKeyB64: base64.StdEncoding.EncodeToString(priv[:]),
		V:            2,
	}
	data, _ := json.Marshal(kp)
	_ = os.WriteFile(filepath.Join(home, "daemon-keypair.json"), data, 0600)

	loaded, err := LoadDaemonKeyPair(home)
	if err != nil {
		t.Fatalf("LoadDaemonKeyPair: %v", err)
	}
	if loaded.PublicKeyB64 != kp.PublicKeyB64 {
		t.Error("public key mismatch")
	}
	if loaded.V != 2 {
		t.Errorf("version: got %d, want 2", loaded.V)
	}
}

func TestLoadDaemonKeyPair_LegacyRegeneration(t *testing.T) {
	home := t.TempDir()
	// Simulate legacy Ed25519 64-byte secret
	legacyKP := DaemonKeyPair{
		PublicKeyB64: base64.StdEncoding.EncodeToString(make([]byte, 32)),
		SecretKeyB64: base64.StdEncoding.EncodeToString(make([]byte, 64)),
		V:            1,
	}
	data, _ := json.Marshal(legacyKP)
	_ = os.WriteFile(filepath.Join(home, "daemon-keypair.json"), data, 0600)

	loaded, err := LoadDaemonKeyPair(home)
	if err != nil {
		t.Fatalf("LoadDaemonKeyPair: %v", err)
	}
	if loaded.V != 2 {
		t.Errorf("expected regenerated keypair v2, got v%d", loaded.V)
	}
	// Verify it was rewritten
	data2, _ := os.ReadFile(filepath.Join(home, "daemon-keypair.json"))
	var reloaded DaemonKeyPair
	_ = json.Unmarshal(data2, &reloaded)
	if reloaded.V != 2 {
		t.Error("expected persisted v2 after regeneration")
	}
}

func TestPerformE2EEHandshake(t *testing.T) {
	// Generate daemon keys
	_, daemonPriv, err := generateBoxKeyPair()
	if err != nil {
		t.Fatalf("generate daemon keypair: %v", err)
	}
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])

	// Generate client keys
	clientPub, _, err := generateBoxKeyPair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	serverDone := make(chan struct{})
	var e2eeConn *E2EEConn

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}

		e2eeConn, err = PerformE2EEHandshake(conn, daemonPrivB64, testLogger(t))
		if err != nil {
			conn.Close()
			t.Errorf("handshake: %v", err)
			return
		}
		close(serverDone)
		// Keep connection open until test completes; srv.Close() will clean it up
		<-time.After(5 * time.Second)
		conn.Close()
	}))
	defer srv.Close()

	// Client connects
	wsURL := "ws" + srv.URL[4:]
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()

	// Client sends e2ee_hello
	hello := map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(clientPub[:]),
	}
	helloData, _ := json.Marshal(hello)
	if err := clientConn.WriteMessage(websocket.TextMessage, helloData); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	// Wait for server to complete handshake
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handshake timeout")
	}

	if e2eeConn == nil {
		t.Fatal("expected E2EEConn after handshake")
	}

	// Test encrypted roundtrip
	msg := []byte(`{"type":"test"}`)
	if err := e2eeConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	// Client reads encrypted message
	_, encrypted, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("client ReadMessage: %v", err)
	}
	if len(encrypted) == 0 {
		t.Error("expected non-empty encrypted payload")
	}
}

func TestPerformE2EEHandshake_InvalidHelloType(t *testing.T) {
	_, daemonPriv, _ := generateBoxKeyPair()
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		_, err := PerformE2EEHandshake(conn, daemonPrivB64, testLogger(t))
		if err == nil {
			t.Error("expected error for invalid hello type")
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()

	badHello, _ := json.Marshal(map[string]string{"type": "wrong", "key": "abc"})
	_ = clientConn.WriteMessage(websocket.TextMessage, badHello)

	// Give server time to process
	time.Sleep(100 * time.Millisecond)
}

func TestE2EEConn_Interface(t *testing.T) {
	// Ensure E2EEConn implements the interfaces
	var _ interface {
		ReadMessage() (int, []byte, error)
		WriteMessage(int, []byte) error
		Close() error
		WriteControl(int, []byte, time.Time) error
		SetPongHandler(func(appData string) error)
		SetReadDeadline(time.Time) error
		SetWriteDeadline(time.Time) error
	} = (*E2EEConn)(nil)
}

func generateBoxKeyPair() (pub, priv *[32]byte, err error) {
	return box.GenerateKey(rand.Reader)
}

func testLogger(t *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
