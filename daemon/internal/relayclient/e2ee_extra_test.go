package relayclient

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/nacl/box"
)

func TestE2EEConn_Close(t *testing.T) {
	_, daemonPriv, _ := generateBoxKeyPair()
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])
	clientPub, _, _ := generateBoxKeyPair()

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	e2eeDone := make(chan *E2EEConn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		e2eeConn, _ := PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
		e2eeDone <- e2eeConn
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	hello, _ := json.Marshal(map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(clientPub[:]),
	})
	clientConn.WriteMessage(websocket.TextMessage, hello)

	var e2eeConn *E2EEConn
	select {
	case e2eeConn = <-e2eeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	if err := e2eeConn.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	clientConn.Close()
}

func TestE2EEConn_SetPongHandler(t *testing.T) {
	_, daemonPriv, _ := generateBoxKeyPair()
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])
	clientPub, _, _ := generateBoxKeyPair()

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	e2eeDone := make(chan *E2EEConn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		e2eeConn, _ := PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
		e2eeDone <- e2eeConn
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	hello, _ := json.Marshal(map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(clientPub[:]),
	})
	clientConn.WriteMessage(websocket.TextMessage, hello)

	var e2eeConn *E2EEConn
	select {
	case e2eeConn = <-e2eeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	defer func() {
		e2eeConn.Close()
		clientConn.Close()
	}()

	called := false
	e2eeConn.SetPongHandler(func(_ string) error {
		called = true
		return nil
	})
	// SetPongHandler just delegates — verify no panic
	_ = called
}

func TestE2EEConn_SetReadDeadline(t *testing.T) {
	_, daemonPriv, _ := generateBoxKeyPair()
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])
	clientPub, _, _ := generateBoxKeyPair()

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	e2eeDone := make(chan *E2EEConn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		e2eeConn, _ := PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
		e2eeDone <- e2eeConn
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	hello, _ := json.Marshal(map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(clientPub[:]),
	})
	clientConn.WriteMessage(websocket.TextMessage, hello)

	var e2eeConn *E2EEConn
	select {
	case e2eeConn = <-e2eeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	defer func() {
		e2eeConn.Close()
		clientConn.Close()
	}()

	if err := e2eeConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Errorf("SetReadDeadline: %v", err)
	}
}

func TestE2EEConn_SetWriteDeadline(t *testing.T) {
	_, daemonPriv, _ := generateBoxKeyPair()
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])
	clientPub, _, _ := generateBoxKeyPair()

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	e2eeDone := make(chan *E2EEConn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		e2eeConn, _ := PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
		e2eeDone <- e2eeConn
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	hello, _ := json.Marshal(map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(clientPub[:]),
	})
	clientConn.WriteMessage(websocket.TextMessage, hello)

	var e2eeConn *E2EEConn
	select {
	case e2eeConn = <-e2eeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	defer func() {
		e2eeConn.Close()
		clientConn.Close()
	}()

	if err := e2eeConn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Errorf("SetWriteDeadline: %v", err)
	}
}

func TestE2EEConn_WriteControl(t *testing.T) {
	_, daemonPriv, _ := generateBoxKeyPair()
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])
	clientPub, _, _ := generateBoxKeyPair()

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	e2eeDone := make(chan *E2EEConn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		e2eeConn, _ := PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
		e2eeDone <- e2eeConn
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	hello, _ := json.Marshal(map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(clientPub[:]),
	})
	clientConn.WriteMessage(websocket.TextMessage, hello)

	var e2eeConn *E2EEConn
	select {
	case e2eeConn = <-e2eeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	defer func() {
		e2eeConn.Close()
		clientConn.Close()
	}()

	err := e2eeConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
	if err != nil {
		t.Errorf("WriteControl: %v", err)
	}
}

func TestE2EEConn_ReadMessage_Encrypted(t *testing.T) {
	daemonPub, daemonPriv, _ := generateBoxKeyPair()
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])
	clientPub, clientPriv, _ := generateBoxKeyPair()

	// Client-side derivation of the shared key, exactly as a real client
	// would compute it from the daemon's advertised public key.
	var sharedKey [32]byte
	box.Precompute(&sharedKey, daemonPub, clientPriv)

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	e2eeDone := make(chan *E2EEConn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		e2eeConn, err := PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
		if err != nil {
			t.Errorf("handshake: %v", err)
			return
		}
		e2eeDone <- e2eeConn
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	hello, _ := json.Marshal(map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(clientPub[:]),
	})
	clientConn.WriteMessage(websocket.TextMessage, hello)

	var e2eeConn *E2EEConn
	select {
	case e2eeConn = <-e2eeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	defer func() {
		e2eeConn.Close()
		clientConn.Close()
	}()

	// Consume the daemon's plaintext e2ee_ready handshake reply before any
	// encrypted traffic, exactly as a real client does.
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, readyRaw, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("read e2ee_ready: %v", err)
	}
	var ready struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(readyRaw, &ready); err != nil || ready.Type != "e2ee_ready" {
		t.Fatalf("expected e2ee_ready frame, got: %s", readyRaw)
	}

	// Daemon → client: the client must be able to decrypt the frame with its
	// own derivation of the shared key and recover the exact plaintext.
	msg := []byte(`{"type":"test","data":"hello"}`)
	if err := e2eeConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("client ReadMessage: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty encrypted data")
	}
	if string(raw) == string(msg) {
		t.Fatal("frame was sent as plaintext — encryption not applied")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		t.Fatalf("base64 decode frame: %v", err)
	}
	if len(decoded) < 24 {
		t.Fatalf("frame too short for nonce: %d bytes", len(decoded))
	}
	var nonce [24]byte
	copy(nonce[:], decoded[:24])
	plaintext, ok := box.OpenAfterPrecomputation(nil, decoded[24:], &nonce, &sharedKey)
	if !ok {
		t.Fatal("client failed to decrypt daemon frame under the negotiated key")
	}
	if string(plaintext) != string(msg) {
		t.Fatalf("decrypted %q, want %q", plaintext, msg)
	}

	// Client → daemon: a stray handshake frame must be skipped, then
	// ReadMessage must return the decrypted reply (previously the result was
	// discarded without any assertion).
	strayHello, _ := json.Marshal(map[string]string{"type": "e2ee_hello"})
	if err := clientConn.WriteMessage(websocket.TextMessage, strayHello); err != nil {
		t.Fatalf("write stray hello: %v", err)
	}

	reply := []byte(`{"type":"pong"}`)
	var replyNonce [24]byte
	if _, err := rand.Read(replyNonce[:]); err != nil {
		t.Fatalf("generate nonce: %v", err)
	}
	ciphertext := box.SealAfterPrecomputation(nil, reply, &replyNonce, &sharedKey)
	bundle := append(replyNonce[:], ciphertext...)
	if err := clientConn.WriteMessage(websocket.TextMessage,
		[]byte(base64.StdEncoding.EncodeToString(bundle))); err != nil {
		t.Fatalf("write encrypted reply: %v", err)
	}

	e2eeConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, decrypted, err := e2eeConn.ReadMessage()
	if err != nil {
		t.Fatalf("server ReadMessage after stray frame: %v", err)
	}
	if msgType != websocket.TextMessage {
		t.Errorf("message type: got %d, want TextMessage", msgType)
	}
	if string(decrypted) != string(reply) {
		t.Fatalf("server decrypted %q, want %q", decrypted, reply)
	}
}
