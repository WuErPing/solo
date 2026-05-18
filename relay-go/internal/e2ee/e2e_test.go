package e2ee_test

// End-to-end tests for E2EE through the relay.
// These tests verify the full encrypted flow: control socket → client socket →
// data socket → E2EE handshake → encrypted exchange → relay opacity.

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
	"github.com/WuErPing/solo/relay/internal/e2ee"
	"github.com/WuErPing/solo/relay/internal/relay"
)

// wsTransport adapts a websocket.Conn to the e2ee.Transport interface.
type wsTransport struct {
	conn     *websocket.Conn
	mu       sync.Mutex
	handler  func([]byte)
	closeFn  func()
	closed   bool
	readDone chan struct{}
}

func newWSTransport(conn *websocket.Conn) *wsTransport {
	t := &wsTransport{
		conn:     conn,
		readDone: make(chan struct{}),
	}
	go t.readLoop()
	return t
}

func (t *wsTransport) readLoop() {
	defer close(t.readDone)
	for {
		_, msg, err := t.conn.ReadMessage()
		if err != nil {
			t.mu.Lock()
			closed := t.closed
			closeFn := t.closeFn
			t.mu.Unlock()
			if !closed && closeFn != nil {
				closeFn()
			}
			return
		}
		t.mu.Lock()
		handler := t.handler
		t.mu.Unlock()
		if handler != nil {
			handler(msg)
		}
	}
}

func (t *wsTransport) Send(msg []byte) error {
	return t.conn.WriteMessage(websocket.TextMessage, msg)
}

func (t *wsTransport) Close() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	t.conn.Close()
}

func (t *wsTransport) OnMessage(fn func([]byte)) {
	t.mu.Lock()
	t.handler = fn
	t.mu.Unlock()
}

func (t *wsTransport) OnClose(fn func()) {
	t.mu.Lock()
	t.closeFn = fn
	t.mu.Unlock()
}

// helper functions

func newE2ETestServer(t *testing.T) (*relay.Server, *httptest.Server) {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := relay.NewSessionStore(200, logger)
	srv := relay.NewServer(store, 200, logger, nil)
	srv.NudgeSyncDelay = 80 * time.Millisecond
	srv.NudgeResetDelay = 40 * time.Millisecond
	ts := httptest.NewServer(srv.Handler())
	return srv, ts
}

func dialE2E(t *testing.T, ts *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial %s failed: %v", path, err)
	}
	return conn
}

func readJSONE2E(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(msg, &v); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return v
}

// Tests

func TestE2EFullFlow(t *testing.T) {
	_, ts := newE2ETestServer(t)
	defer ts.Close()

	serverId := "e2e-full-" + time.Now().Format("150405.000")
	connectionId := "conn_e2e_full"

	// === DAEMON SIDE ===
	// Generate daemon keypair (public key would go in QR code)
	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonPubKeyB64 := e2ee.ExportPublicKey(daemonPub)

	// Daemon connects as control socket
	controlWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=server&v=2")
	defer controlWs.Close()
	readJSONE2E(t, controlWs) // consume sync

	// === CLIENT SIDE ===
	// Client scans QR, gets daemon's public key
	clientPub, clientSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientPubKeyB64 := e2ee.ExportPublicKey(clientPub)

	// Client imports daemon's public key and derives shared key
	daemonPubKeyOnClient, err := e2ee.ImportPublicKey(daemonPubKeyB64)
	if err != nil {
		t.Fatal(err)
	}
	clientSharedKey := e2ee.DeriveSharedKey(clientSec, daemonPubKeyOnClient)

	// Set up listener for "connected" on control
	var connectedWg sync.WaitGroup
	connectedWg.Add(1)
	go func() {
		defer connectedWg.Done()
		for {
			msg := readJSONE2E(t, controlWs)
			if msg["type"] == "connected" && msg["connectionId"] == connectionId {
				return
			}
		}
	}()

	// Client connects to relay
	clientWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=client&connectionId="+connectionId+"&v=2")
	defer clientWs.Close()

	// Wait for daemon to see client connected
	done := make(chan struct{})
	go func() {
		connectedWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for connected notification")
	}

	// Daemon connects data socket
	daemonDataWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=server&connectionId="+connectionId+"&v=2")
	defer daemonDataWs.Close()

	// Client sends e2ee_hello with its public key (not encrypted - it's the handshake)
	helloMsg := map[string]string{"type": "e2ee_hello", "key": clientPubKeyB64}
	helloJSON, _ := json.Marshal(helloMsg)
	if err := clientWs.WriteMessage(websocket.TextMessage, helloJSON); err != nil {
		t.Fatal(err)
	}

	// Daemon receives hello on data socket
	daemonDataWs.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, helloRaw, err := daemonDataWs.ReadMessage()
	if err != nil {
		t.Fatalf("daemon failed to receive hello: %v", err)
	}
	var helloParsed struct {
		Type string `json:"type"`
		Key  string `json:"key"`
	}
	if err := json.Unmarshal(helloRaw, &helloParsed); err != nil {
		t.Fatal(err)
	}
	if helloParsed.Type != "e2ee_hello" {
		t.Fatalf("expected e2ee_hello, got %s", helloParsed.Type)
	}

	// Daemon imports client's public key and derives shared key
	clientPubKeyOnDaemon, err := e2ee.ImportPublicKey(helloParsed.Key)
	if err != nil {
		t.Fatal(err)
	}
	daemonSharedKey := e2ee.DeriveSharedKey(daemonSec, clientPubKeyOnDaemon)

	// Verify both have the same key
	if clientSharedKey != daemonSharedKey {
		t.Fatal("shared keys don't match")
	}

	// Daemon sends e2ee_ready (encrypted)
	readyJSON, _ := json.Marshal(map[string]string{"type": "e2ee_ready"})
	if err := daemonDataWs.WriteMessage(websocket.TextMessage, readyJSON); err != nil {
		t.Fatal(err)
	}

	// Client receives ready
	clientWs.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, readyRaw, err := clientWs.ReadMessage()
	if err != nil {
		t.Fatalf("client failed to receive ready: %v", err)
	}
	var readyParsed struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(readyRaw, &readyParsed); err != nil {
		t.Fatal(err)
	}
	if readyParsed.Type != "e2ee_ready" {
		t.Fatalf("expected e2ee_ready, got %s", readyParsed.Type)
	}

	// Now both sides have the shared key. Exchange encrypted messages.
	// Client sends encrypted message
	clientMsg := "Hello from client!"
	encrypted, err := e2ee.Encrypt(clientSharedKey, []byte(clientMsg))
	if err != nil {
		t.Fatal(err)
	}
	// Send as base64 text (like the real protocol)
	encryptedB64 := base64.StdEncoding.EncodeToString(encrypted)
	if err := clientWs.WriteMessage(websocket.TextMessage, []byte(encryptedB64)); err != nil {
		t.Fatal(err)
	}

	// Daemon receives and decrypts
	daemonDataWs.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, encryptedRaw, err := daemonDataWs.ReadMessage()
	if err != nil {
		t.Fatalf("daemon failed to receive encrypted message: %v", err)
	}
	// Decode base64
	encryptedBytes, err := base64.StdEncoding.DecodeString(string(encryptedRaw))
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := e2ee.Decrypt(daemonSharedKey, encryptedBytes)
	if err != nil {
		t.Fatalf("daemon failed to decrypt: %v", err)
	}
	if string(decrypted) != clientMsg {
		t.Fatalf("expected %q, got %q", clientMsg, string(decrypted))
	}

	// Daemon sends encrypted response
	daemonMsg := "Hello from daemon!"
	daemonEncrypted, err := e2ee.Encrypt(daemonSharedKey, []byte(daemonMsg))
	if err != nil {
		t.Fatal(err)
	}
	daemonEncryptedB64 := base64.StdEncoding.EncodeToString(daemonEncrypted)
	if err := daemonDataWs.WriteMessage(websocket.TextMessage, []byte(daemonEncryptedB64)); err != nil {
		t.Fatal(err)
	}

	// Client receives and decrypts
	clientWs.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, clientReceived, err := clientWs.ReadMessage()
	if err != nil {
		t.Fatalf("client failed to receive encrypted response: %v", err)
	}
	clientEncryptedBytes, err := base64.StdEncoding.DecodeString(string(clientReceived))
	if err != nil {
		t.Fatal(err)
	}
	clientDecrypted, err := e2ee.Decrypt(clientSharedKey, clientEncryptedBytes)
	if err != nil {
		t.Fatalf("client failed to decrypt: %v", err)
	}
	if string(clientDecrypted) != daemonMsg {
		t.Fatalf("expected %q, got %q", daemonMsg, string(clientDecrypted))
	}
}

func TestE2ERelayOpacity(t *testing.T) {
	_, ts := newE2ETestServer(t)
	defer ts.Close()

	serverId := "e2e-opacity-" + time.Now().Format("150405.000")
	connectionId := "conn_opacity"

	// Generate keys
	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientPub, clientSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Derive shared keys
	daemonSharedKey := e2ee.DeriveSharedKey(daemonSec, clientPub)
	clientSharedKey := e2ee.DeriveSharedKey(clientSec, daemonPub)

	// Connect control socket
	controlWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=server&v=2")
	defer controlWs.Close()
	readJSONE2E(t, controlWs) // consume sync

	// Connect client
	clientWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=client&connectionId="+connectionId+"&v=2")
	defer clientWs.Close()

	// Wait a bit for control to see client
	time.Sleep(50 * time.Millisecond)

	// Connect data socket
	daemonDataWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=server&connectionId="+connectionId+"&v=2")
	defer daemonDataWs.Close()

	// Handshake (not encrypted - this is the hello/ready exchange)
	helloJSON, _ := json.Marshal(map[string]string{"type": "e2ee_hello", "key": e2ee.ExportPublicKey(clientPub)})
	clientWs.WriteMessage(websocket.TextMessage, helloJSON)

	daemonDataWs.SetReadDeadline(time.Now().Add(3 * time.Second))
	daemonDataWs.ReadMessage() // consume hello

	readyJSON, _ := json.Marshal(map[string]string{"type": "e2ee_ready"})
	daemonDataWs.WriteMessage(websocket.TextMessage, readyJSON)

	clientWs.SetReadDeadline(time.Now().Add(3 * time.Second))
	clientWs.ReadMessage() // consume ready

	// Send encrypted secret
	secret := "This is a secret that relay cannot read"
	encrypted, err := e2ee.Encrypt(clientSharedKey, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	encryptedB64 := base64.StdEncoding.EncodeToString(encrypted)
	clientWs.WriteMessage(websocket.TextMessage, []byte(encryptedB64))

	// Daemon receives raw bytes
	daemonDataWs.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, rawBytes, err := daemonDataWs.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}

	// The raw bytes should NOT contain the plaintext secret
	rawString := string(rawBytes)
	if strings.Contains(rawString, secret) {
		t.Fatal("relay can see plaintext! Encrypted bytes should not contain the secret")
	}

	// But daemon can decrypt
	encryptedBytes, err := base64.StdEncoding.DecodeString(rawString)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := e2ee.Decrypt(daemonSharedKey, encryptedBytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != secret {
		t.Fatalf("expected %q, got %q", secret, string(decrypted))
	}
}

func TestE2EWrongKeyCannotDecrypt(t *testing.T) {
	// Generate keys for daemon and client
	_, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientPub, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Correct shared key
	correctKey := e2ee.DeriveSharedKey(daemonSec, clientPub)

	// Attacker with different keypair
	attackerPub, attackerSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	_ = attackerPub                                               // not needed for this test
	attackerKey := e2ee.DeriveSharedKey(attackerSec, attackerPub) // wrong key

	// Encrypt with correct key
	secret := "Top secret message"
	encrypted, err := e2ee.Encrypt(correctKey, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	// Attacker cannot decrypt
	_, err = e2ee.Decrypt(attackerKey, encrypted)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key, but it succeeded")
	}
}

// TestE2EWithEncryptedChannel tests the full flow using the EncryptedChannel abstraction
// which handles the e2ee_hello/e2ee_ready handshake protocol automatically.
func TestE2EWithEncryptedChannel(t *testing.T) {
	_, ts := newE2ETestServer(t)
	defer ts.Close()

	serverId := "e2e-channel-" + time.Now().Format("150405.000")
	connectionId := "conn_channel"

	// Generate daemon keypair
	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}

	// Connect control socket
	controlWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=server&v=2")
	defer controlWs.Close()
	readJSONE2E(t, controlWs) // consume sync

	// Set up listener for "connected" on control
	var connectedWg sync.WaitGroup
	connectedWg.Add(1)
	go func() {
		defer connectedWg.Done()
		for {
			msg := readJSONE2E(t, controlWs)
			if msg["type"] == "connected" && msg["connectionId"] == connectionId {
				return
			}
		}
	}()

	// Connect client WebSocket
	clientWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=client&connectionId="+connectionId+"&v=2")
	defer clientWs.Close()

	// Wait for daemon to see client connected
	done := make(chan struct{})
	go func() {
		connectedWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for connected notification")
	}

	// Connect daemon data socket
	daemonDataWs := dialE2E(t, ts, protocol.WSEndpoint+"?serverId="+serverId+"&role=server&connectionId="+connectionId+"&v=2")
	defer daemonDataWs.Close()

	// Create EncryptedChannels
	clientTransport := newWSTransport(clientWs)
	daemonTransport := newWSTransport(daemonDataWs)

	clientChannel := e2ee.NewClientChannel(clientTransport, daemonPub)
	daemonChannel := e2ee.NewDaemonChannel(daemonTransport, daemonKP)

	// Wait for handshake to complete
	var wg sync.WaitGroup
	wg.Add(2)
	clientChannel.OnOpen(func() { wg.Done() })
	daemonChannel.OnOpen(func() { wg.Done() })

	handshakeDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(handshakeDone)
	}()

	select {
	case <-handshakeDone:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake did not complete within 3 seconds")
	}

	// Verify both channels are open
	if !clientChannel.IsOpen() {
		t.Fatal("client channel not open after handshake")
	}
	if !daemonChannel.IsOpen() {
		t.Fatal("daemon channel not open after handshake")
	}

	// Exchange messages using the channels
	var clientReceived sync.WaitGroup
	clientReceived.Add(1)
	clientChannel.OnMessage(func(msg []byte) {
		if string(msg) != "Hello from daemon!" {
			t.Errorf("client received unexpected message: %s", string(msg))
		}
		clientReceived.Done()
	})

	var daemonReceived sync.WaitGroup
	daemonReceived.Add(1)
	daemonChannel.OnMessage(func(msg []byte) {
		if string(msg) != "Hello from client!" {
			t.Errorf("daemon received unexpected message: %s", string(msg))
		}
		daemonReceived.Done()
	})

	// Client sends
	if err := clientChannel.Send([]byte("Hello from client!")); err != nil {
		t.Fatal(err)
	}

	// Daemon sends
	if err := daemonChannel.Send([]byte("Hello from daemon!")); err != nil {
		t.Fatal(err)
	}

	// Wait for both messages
	msgDone := make(chan struct{})
	go func() {
		clientReceived.Wait()
		daemonReceived.Wait()
		close(msgDone)
	}()

	select {
	case <-msgDone:
	case <-time.After(3 * time.Second):
		t.Fatal("message exchange did not complete within 3 seconds")
	}
}
