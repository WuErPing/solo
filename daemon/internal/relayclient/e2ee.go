package relayclient

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/WuErPing/solo/daemon/internal/wsconn"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/nacl/box"
)

// DaemonKeyPair mirrors the on-disk daemon-keypair.json structure.
type DaemonKeyPair struct {
	PublicKeyB64 string `json:"publicKeyB64"`
	SecretKeyB64 string `json:"secretKeyB64"`
	V            int    `json:"v"`
}

// LoadDaemonKeyPair reads the daemon keypair from the Solo home directory.
// If a legacy Ed25519 keypair (64-byte secret) is found, it is automatically
// regenerated as a Curve25519 keypair so E2EE stays functional.
func LoadDaemonKeyPair(home string) (*DaemonKeyPair, error) {
	path := filepath.Join(home, "daemon-keypair.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read keypair: %w", err)
	}
	var kp DaemonKeyPair
	if err := json.Unmarshal(data, &kp); err != nil {
		return nil, fmt.Errorf("parse keypair: %w", err)
	}

	// Detect legacy Ed25519 key (64 bytes) and auto-regenerate as Curve25519.
	secretBytes, _ := base64.StdEncoding.DecodeString(kp.SecretKeyB64)
	if len(secretBytes) == 64 {
		pub, priv, genErr := box.GenerateKey(rand.Reader)
		if genErr != nil {
			return nil, fmt.Errorf("regenerate keypair: %w", genErr)
		}
		kp = DaemonKeyPair{
			PublicKeyB64: base64.StdEncoding.EncodeToString(pub[:]),
			SecretKeyB64: base64.StdEncoding.EncodeToString(priv[:]),
			V:            2,
		}
		newData, _ := json.MarshalIndent(kp, "", "  ")
		if writeErr := os.WriteFile(path, append(newData, '\n'), 0600); writeErr != nil {
			return nil, fmt.Errorf("write regenerated keypair: %w", writeErr)
		}
		// Cannot use logger here (not available in LoadDaemonKeyPair), but the caller logs the keypair load.
	}
	return &kp, nil
}

// E2EEConn wraps a raw WebSocket connection with NaCl box E2EE encryption.
// It implements the wsconn.WSConn interface so it can be attached directly
// into the WSServer session lifecycle.
type E2EEConn struct {
	conn      *websocket.Conn
	sharedKey [32]byte
	logger    *slog.Logger
}

// PerformE2EEHandshake performs the daemon-side E2EE handshake on a raw
// WebSocket connection.  It expects the peer to send
// {"type":"e2ee_hello","key":"<base64_client_pubkey>"} and replies with
// {"type":"e2ee_ready"}.  On success it returns an *E2EEConn ready for
// encrypted traffic.
func PerformE2EEHandshake(conn *websocket.Conn, secretKeyB64 string, logger *slog.Logger) (*E2EEConn, error) {
	secretKeyBytes, err := base64.StdEncoding.DecodeString(secretKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode secret key: %w", err)
	}
	if len(secretKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid secret key length %d, expected 32", len(secretKeyBytes))
	}
	var secretKey [32]byte
	copy(secretKey[:], secretKeyBytes)

	// Read hello with timeout.
	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	msgType, data, err := conn.ReadMessage()
	_ = conn.SetReadDeadline(time.Time{}) // clear deadline
	if err != nil {
		return nil, fmt.Errorf("read hello: %w", err)
	}
	// Some WebSocket implementations (e.g. React Native) send text
	// messages as binary frames, so accept both.
	_ = msgType

	var hello struct {
		Type string `json:"type"`
		Key  string `json:"key"`
	}
	if err := json.Unmarshal(data, &hello); err != nil {
		return nil, fmt.Errorf("parse hello: %w", err)
	}
	if hello.Type != "e2ee_hello" {
		return nil, fmt.Errorf("expected e2ee_hello, got %s", hello.Type)
	}

	clientPubKeyBytes, err := base64.StdEncoding.DecodeString(hello.Key)
	if err != nil {
		return nil, fmt.Errorf("decode client public key: %w", err)
	}
	if len(clientPubKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid client public key length %d", len(clientPubKeyBytes))
	}
	var clientPubKey [32]byte
	copy(clientPubKey[:], clientPubKeyBytes)

	var sharedKey [32]byte
	box.Precompute(&sharedKey, &clientPubKey, &secretKey)

	// Send e2ee_ready.
	ready := []byte(`{"type":"e2ee_ready"}`)
	if err := conn.WriteMessage(websocket.TextMessage, ready); err != nil {
		return nil, fmt.Errorf("send ready: %w", err)
	}

	logger.Info("e2ee handshake complete")
	return &E2EEConn{conn: conn, sharedKey: sharedKey, logger: logger}, nil
}

// ReadMessage implements wsconn.WSConn.  It reads an encrypted base64 text
// frame, decrypts it, and returns the plaintext.
func (e *E2EEConn) ReadMessage() (int, []byte, error) {
	for {
		msgType, data, err := e.conn.ReadMessage()
		if err != nil {
			return 0, nil, err
		}

		// Some WebSocket implementations (e.g. React Native) send text
		// messages as binary frames, so accept both and convert binary to
		// string for base64 decoding.
		var textData string
		switch msgType {
		case websocket.TextMessage:
			textData = string(data)
		case websocket.BinaryMessage:
			textData = string(data)
		default:
			// Pass through non-text/binary frames (shouldn't happen in E2EE mode).
			return msgType, data, nil
		}

		// Try to decrypt as base64-encoded ciphertext bundle:
		//   [nonce(24)][ciphertext...]
		decoded, b64err := base64.StdEncoding.DecodeString(textData)
		if b64err == nil && len(decoded) >= 24 {
			var nonce [24]byte
			copy(nonce[:], decoded[:24])
			ciphertext := decoded[24:]
			plaintext, ok := box.OpenAfterPrecomputation(nil, ciphertext, &nonce, &e.sharedKey)
			if ok {
				return websocket.TextMessage, plaintext, nil
			}
		}

		// Not encrypted — might be a stray handshake message. Skip it.
		var ctrl struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &ctrl) == nil && (ctrl.Type == "e2ee_hello" || ctrl.Type == "e2ee_ready") {
			e.logger.Debug("skipping stray e2ee handshake frame")
			continue
		}

		// Unrecognized plaintext after handshake — close connection hard.
		previewLen := len(data)
		if previewLen > 200 {
			previewLen = 200
		}
		preview := previewLen
		if preview > len(textData) {
			preview = len(textData)
		}
		e.logger.Warn("plaintext frame on encrypted channel", "framePreview", textData[:preview])
		return 0, nil, fmt.Errorf("plaintext frame on encrypted channel")
	}
}

// WriteMessage implements wsconn.WSConn.  It encrypts the payload and sends
// it as a base64-encoded text frame.
func (e *E2EEConn) WriteMessage(messageType int, data []byte) error {
	if messageType != websocket.TextMessage {
		return e.conn.WriteMessage(messageType, data)
	}

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := box.SealAfterPrecomputation(nil, data, &nonce, &e.sharedKey)
	bundle := make([]byte, 24+len(ciphertext))
	copy(bundle, nonce[:])
	copy(bundle[24:], ciphertext)

	return e.conn.WriteMessage(websocket.TextMessage, []byte(base64.StdEncoding.EncodeToString(bundle)))
}

// Close implements wsconn.WSConn.
func (e *E2EEConn) Close() error {
	return e.conn.Close()
}

// WriteControl implements wsconn.PingableConn. Delegates to the underlying connection.
func (e *E2EEConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return e.conn.WriteControl(messageType, data, deadline)
}

// SetPongHandler implements wsconn.PingableConn. Delegates to the underlying connection.
func (e *E2EEConn) SetPongHandler(h func(appData string) error) {
	e.conn.SetPongHandler(h)
}

// SetReadDeadline implements wsconn.PingableConn. Delegates to the underlying connection.
func (e *E2EEConn) SetReadDeadline(t time.Time) error {
	return e.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements wsconn.WriteDeadlineConn. Delegates to the underlying connection.
func (e *E2EEConn) SetWriteDeadline(t time.Time) error {
	return e.conn.SetWriteDeadline(t)
}

// Ensure E2EEConn implements wsconn.PingableConn.
var _ wsconn.PingableConn = (*E2EEConn)(nil)
var _ wsconn.WriteDeadlineConn = (*E2EEConn)(nil)
