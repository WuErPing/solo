package relayclient

// E2EE roundtrip correctness tests. The pre-existing handshake tests only
// asserted "ciphertext is non-empty"; these tests derive the shared key on
// the client side (NaCl box Precompute, mirroring relay-go/internal/e2ee's
// DeriveSharedKey) and verify actual decryptability and content equality in
// both directions, plus failure modes for wrong keys and MITM key swapping.

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/nacl/box"
)

// e2eeTestPeer holds everything the simulated client side needs after a
// successful handshake: the raw WebSocket, both keypairs, and the shared key
// derived exactly as a real client would (box.Precompute with the client's
// secret key and the daemon's public key).
type e2eeTestPeer struct {
	serverConn *E2EEConn
	clientConn *websocket.Conn
	sharedKey  [32]byte // client-side derivation; must equal the server's
	daemonPub  *[32]byte
	clientPub  *[32]byte
	clientPriv *[32]byte
}

// clientHelloKey overrides the public key sent in e2ee_hello when non-nil
// (used by the MITM test to swap in an attacker's key).
func setupE2EEPeer(t *testing.T, clientHelloKey *[32]byte) *e2eeTestPeer {
	t.Helper()

	daemonPub, daemonPriv, err := generateBoxKeyPair()
	if err != nil {
		t.Fatalf("generate daemon keypair: %v", err)
	}
	daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])

	clientPub, clientPriv, err := generateBoxKeyPair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	// The real client derives the shared key from the daemon's public key
	// (obtained out-of-band, e.g. via QR code) and its own secret key.
	var sharedKey [32]byte
	box.Precompute(&sharedKey, daemonPub, clientPriv)

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	serverDone := make(chan *E2EEConn, 1)
	handshakeErr := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			handshakeErr <- err
			return
		}
		e2eeConn, err := PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
		if err != nil {
			conn.Close()
			handshakeErr <- err
			return
		}
		serverDone <- e2eeConn
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + srv.URL[4:]
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { clientConn.Close() })

	helloKey := clientPub
	if clientHelloKey != nil {
		helloKey = clientHelloKey
	}
	hello, _ := json.Marshal(map[string]string{
		"type": "e2ee_hello",
		"key":  base64.StdEncoding.EncodeToString(helloKey[:]),
	})
	if err := clientConn.WriteMessage(websocket.TextMessage, hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	select {
	case e2eeConn := <-serverDone:
		// Consume the daemon's plaintext e2ee_ready reply, exactly as a real
		// client does before switching to encrypted traffic.
		if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
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
		if err := clientConn.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}
		return &e2eeTestPeer{
			serverConn: e2eeConn,
			clientConn: clientConn,
			sharedKey:  sharedKey,
			daemonPub:  daemonPub,
			clientPub:  clientPub,
			clientPriv: clientPriv,
		}
	case err := <-handshakeErr:
		t.Fatalf("handshake failed: %v", err)
		return nil
	case <-time.After(2 * time.Second):
		t.Fatal("handshake timeout")
		return nil
	}
}

// clientDecryptFrame reads one raw frame from the client WebSocket and
// decrypts it with the client-side shared key, failing the test on any error.
func clientDecryptFrame(t *testing.T, p *e2eeTestPeer) []byte {
	t.Helper()
	if err := p.clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, raw, err := p.clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("client ReadMessage: %v", err)
	}
	return decryptBundle(t, p.sharedKey, raw)
}

// decryptBundle decodes the base64 [nonce(24)][ciphertext] wire format and
// opens the box with the given key.
func decryptBundle(t *testing.T, key [32]byte, raw []byte) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		t.Fatalf("base64 decode frame: %v", err)
	}
	if len(decoded) < 24 {
		t.Fatalf("frame too short for nonce: %d bytes", len(decoded))
	}
	var nonce [24]byte
	copy(nonce[:], decoded[:24])
	plaintext, ok := box.OpenAfterPrecomputation(nil, decoded[24:], &nonce, &key)
	if !ok {
		t.Fatal("box.Open failed: frame does not decrypt under the negotiated key")
	}
	return plaintext
}

// clientEncryptAndSend encrypts plaintext exactly like a real client
// (SealAfterPrecomputation + base64 bundle) and sends it to the daemon.
func clientEncryptAndSend(t *testing.T, p *e2eeTestPeer, plaintext []byte) {
	t.Helper()
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatalf("generate nonce: %v", err)
	}
	ciphertext := box.SealAfterPrecomputation(nil, plaintext, &nonce, &p.sharedKey)
	bundle := append(nonce[:], ciphertext...)
	if err := p.clientConn.WriteMessage(websocket.TextMessage,
		[]byte(base64.StdEncoding.EncodeToString(bundle))); err != nil {
		t.Fatalf("client WriteMessage: %v", err)
	}
}

// TestE2EEConn_RoundTrip_ServerToClient verifies the full decrypt path for
// daemon→client traffic: the client decrypts with its own derivation of the
// shared key and recovers the exact plaintext.
func TestE2EEConn_RoundTrip_ServerToClient(t *testing.T) {
	p := setupE2EEPeer(t, nil)
	defer p.serverConn.Close()

	msg := []byte(`{"type":"session","message":{"type":"pong"}}`)
	if err := p.serverConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("server WriteMessage: %v", err)
	}

	// The wire frame must not leak plaintext.
	if err := p.clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, raw, err := p.clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("client ReadMessage: %v", err)
	}
	if strings.Contains(string(raw), "pong") {
		t.Fatal("wire frame contains plaintext — encryption is not applied")
	}

	decrypted := decryptBundle(t, p.sharedKey, raw)
	if string(decrypted) != string(msg) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", decrypted, msg)
	}
}

// TestE2EEConn_RoundTrip_ClientToServer verifies client→daemon traffic: the
// client encrypts with SealAfterPrecomputation and E2EEConn.ReadMessage on
// the daemon side returns the exact plaintext.
func TestE2EEConn_RoundTrip_ClientToServer(t *testing.T) {
	p := setupE2EEPeer(t, nil)
	defer p.serverConn.Close()

	msg := []byte(`{"type":"ping","requestId":"r-1"}`)
	clientEncryptAndSend(t, p, msg)

	if err := p.serverConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	msgType, plaintext, err := p.serverConn.ReadMessage()
	if err != nil {
		t.Fatalf("server ReadMessage: %v", err)
	}
	if msgType != websocket.TextMessage {
		t.Errorf("message type: got %d, want TextMessage", msgType)
	}
	if string(plaintext) != string(msg) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", plaintext, msg)
	}
}

// TestE2EEConn_RoundTrip_Bidirectional exchanges several messages in both
// directions and asserts every one decrypts to the exact original content.
func TestE2EEConn_RoundTrip_Bidirectional(t *testing.T) {
	p := setupE2EEPeer(t, nil)
	defer p.serverConn.Close()

	messages := []string{
		`{"type":"ping"}`,
		`{"type":"session","message":{"type":"create_agent","payload":{"prompt":"你好，世界"}}}`,
		strings.Repeat("x", 4096), // larger-than-typical payload
	}

	for _, m := range messages {
		// daemon → client
		if err := p.serverConn.WriteMessage(websocket.TextMessage, []byte(m)); err != nil {
			t.Fatalf("server WriteMessage: %v", err)
		}
		if got := clientDecryptFrame(t, p); string(got) != m {
			t.Fatalf("server→client mismatch: got %q, want %q", got, m)
		}

		// client → daemon
		clientEncryptAndSend(t, p, []byte(m))
		if err := p.serverConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		_, plaintext, err := p.serverConn.ReadMessage()
		if err != nil {
			t.Fatalf("server ReadMessage: %v", err)
		}
		if string(plaintext) != m {
			t.Fatalf("client→server mismatch: got %q, want %q", plaintext, m)
		}
	}
}

// TestE2EEConn_WrongSharedKeyCannotDecrypt mirrors relay-go's
// TestE2EWrongKeyCannotDecrypt at the connection level: a frame encrypted
// under a different shared key must surface as a ReadMessage error on the
// daemon side, never as silently accepted plaintext.
func TestE2EEConn_WrongSharedKeyCannotDecrypt(t *testing.T) {
	p := setupE2EEPeer(t, nil)
	defer p.serverConn.Close()

	// Encrypt with an attacker's unrelated key instead of the negotiated one.
	attackerPub, attackerPriv, err := generateBoxKeyPair()
	if err != nil {
		t.Fatalf("generate attacker keypair: %v", err)
	}
	var wrongKey [32]byte
	box.Precompute(&wrongKey, attackerPub, attackerPriv)

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatalf("generate nonce: %v", err)
	}
	ciphertext := box.SealAfterPrecomputation(nil, []byte(`{"type":"ping"}`), &nonce, &wrongKey)
	bundle := append(nonce[:], ciphertext...)
	if err := p.clientConn.WriteMessage(websocket.TextMessage,
		[]byte(base64.StdEncoding.EncodeToString(bundle))); err != nil {
		t.Fatalf("client WriteMessage: %v", err)
	}

	if err := p.serverConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, _, err = p.serverConn.ReadMessage()
	if err == nil {
		t.Fatal("expected ReadMessage to fail for a frame encrypted under the wrong key")
	}
}

// TestPerformE2EEHandshake_MITMKeySwap pins down the Diffie-Hellman failure
// mode: if an attacker replaces the client's public key in e2ee_hello, the
// daemon negotiates a key with the attacker, and the legitimate client can no
// longer decrypt anything the daemon sends. The handshake itself cannot
// detect this (no key confirmation in the protocol), so the assertion is on
// decryption failure for the honest client.
func TestPerformE2EEHandshake_MITMKeySwap(t *testing.T) {
	attackerPub, _, err := generateBoxKeyPair()
	if err != nil {
		t.Fatalf("generate attacker keypair: %v", err)
	}

	// The honest client's hello carries the ATTACKER's public key.
	p := setupE2EEPeer(t, attackerPub)
	defer p.serverConn.Close()

	// Daemon derives sharedKey(daemonPriv, attackerPub) and encrypts under it.
	secret := []byte(`{"type":"session","message":{"type":"secret"}}`)
	if err := p.serverConn.WriteMessage(websocket.TextMessage, secret); err != nil {
		t.Fatalf("server WriteMessage: %v", err)
	}

	if err := p.clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, raw, err := p.clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("client ReadMessage: %v", err)
	}

	// The honest client derives sharedKey(clientPriv, daemonPub) — a
	// different key — so opening the box must fail.
	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		t.Fatalf("base64 decode frame: %v", err)
	}
	var nonce [24]byte
	copy(nonce[:], decoded[:24])
	if _, ok := box.OpenAfterPrecomputation(nil, decoded[24:], &nonce, &p.sharedKey); ok {
		t.Fatal("honest client decrypted a frame from a MITM-tampered handshake — key separation broken")
	}
}

// TestPerformE2EEHandshake_RejectsMalformedKey verifies the handshake fails
// fast when the advertised client key is not a valid 32-byte Curve25519 key.
func TestPerformE2EEHandshake_RejectsMalformedKey(t *testing.T) {
	cases := map[string]string{
		"invalid base64": "!!!not-base64!!!",
		"wrong length":   base64.StdEncoding.EncodeToString([]byte("too-short")),
	}

	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			_, daemonPriv, err := generateBoxKeyPair()
			if err != nil {
				t.Fatalf("generate daemon keypair: %v", err)
			}
			daemonPrivB64 := base64.StdEncoding.EncodeToString(daemonPriv[:])

			handshakeErr := make(chan error, 1)
			upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					handshakeErr <- err
					return
				}
				defer conn.Close()
				_, err = PerformE2EEHandshake(conn, daemonPrivB64, testLogger())
				handshakeErr <- err
			}))
			defer srv.Close()

			wsURL := "ws" + srv.URL[4:]
			clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			defer clientConn.Close()

			hello, _ := json.Marshal(map[string]string{"type": "e2ee_hello", "key": key})
			if err := clientConn.WriteMessage(websocket.TextMessage, hello); err != nil {
				t.Fatalf("write hello: %v", err)
			}

			select {
			case err := <-handshakeErr:
				if err == nil {
					t.Error("expected handshake error for malformed client key")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("handshake did not return")
			}
		})
	}
}
