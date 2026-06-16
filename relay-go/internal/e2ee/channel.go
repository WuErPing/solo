// Package e2ee provides end-to-end encrypted channels over a byte transport.
package e2ee

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

const (
	handshakeRetryInterval = time.Second
	maxPendingSends        = 200
)

type channelState int

const (
	stateHandshaking channelState = iota
	stateOpen
	stateClosed
)

// Transport is the interface for the underlying byte-level connection.
type Transport interface {
	Send(msg []byte) error
	Close()
	OnMessage(func([]byte))
	OnClose(func())
}

type e2eeHello struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

type e2eeReady struct {
	Type string `json:"type"`
}

// EncryptedChannel wraps a Transport with E2EE using a pre-computed shared key.
// It handles the e2ee_hello / e2ee_ready handshake and encrypts/decrypts all messages.
type EncryptedChannel struct {
	mu           sync.Mutex
	transport    Transport
	sharedKey    [32]byte
	daemonKP     *KeyPair // non-nil on daemon side; used for re-hello handling
	clientPubKey [32]byte // non-zero on client side (our ephemeral public key)
	state        channelState
	pendingSends [][]byte
	Logger       *slog.Logger // optional, for logging decrypt failures and other events

	onMessageFn func([]byte)
	onOpenFns   []func()
	onCloseFns  []func()
}

// NewClientChannel creates the client-side encrypted channel.
// It immediately sends e2ee_hello and retries every 1s until the channel opens.
func NewClientChannel(transport Transport, daemonPublicKey [32]byte) *EncryptedChannel {
	clientPub, clientSec, err := GenerateKeyPair()
	if err != nil {
		panic("e2ee: generate client keypair: " + err.Error())
	}
	sharedKey := DeriveSharedKey(clientSec, daemonPublicKey)

	ch := &EncryptedChannel{
		transport:    transport,
		sharedKey:    sharedKey,
		clientPubKey: clientPub,
		state:        stateHandshaking,
	}

	transport.OnClose(func() {
		ch.mu.Lock()
		ch.state = stateClosed
		cbs := ch.onCloseFns
		ch.mu.Unlock()
		for _, cb := range cbs {
			cb()
		}
	})

	transport.OnMessage(func(msg []byte) {
		ch.handleMessage(msg)
	})

	helloJSON, _ := json.Marshal(e2eeHello{Type: "e2ee_hello", Key: ExportPublicKey(clientPub)})

	// Send initial hello
	_ = transport.Send(helloJSON)

	// Retry until open
	ticker := time.NewTicker(handshakeRetryInterval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			ch.mu.Lock()
			s := ch.state
			ch.mu.Unlock()
			if s != stateHandshaking {
				return
			}
			_ = transport.Send(helloJSON)
		}
	}()

	return ch
}

// NewDaemonChannel creates the daemon-side encrypted channel.
// It waits for the first e2ee_hello message to complete the handshake.
func NewDaemonChannel(transport Transport, daemonKP KeyPair) *EncryptedChannel {
	ch := &EncryptedChannel{
		transport: transport,
		daemonKP:  &daemonKP,
		state:     stateHandshaking,
	}

	transport.OnClose(func() {
		ch.mu.Lock()
		ch.state = stateClosed
		cbs := ch.onCloseFns
		ch.mu.Unlock()
		for _, cb := range cbs {
			cb()
		}
	})

	transport.OnMessage(func(msg []byte) {
		ch.handleMessage(msg)
	})

	return ch
}

// Send encrypts and sends data. If the channel is still handshaking, the message
// is queued and flushed once the channel opens.
func (c *EncryptedChannel) Send(plaintext []byte) error {
	c.mu.Lock()
	s := c.state
	if s == stateHandshaking {
		if len(c.pendingSends) >= maxPendingSends {
			c.pendingSends = c.pendingSends[1:]
		}
		c.pendingSends = append(c.pendingSends, plaintext)
		c.mu.Unlock()
		return nil
	}
	if s != stateOpen {
		c.mu.Unlock()
		return errChannelNotOpen
	}
	sharedKey := c.sharedKey
	c.mu.Unlock()

	return c.encryptAndSend(sharedKey, plaintext)
}

// OnMessage registers a callback for decrypted messages.
func (c *EncryptedChannel) OnMessage(fn func([]byte)) {
	c.mu.Lock()
	c.onMessageFn = fn
	c.mu.Unlock()
}

// OnOpen registers a callback that fires when the channel enters the open state.
func (c *EncryptedChannel) OnOpen(fn func()) {
	c.mu.Lock()
	c.onOpenFns = append(c.onOpenFns, fn)
	c.mu.Unlock()
}

// OnClose registers a callback that fires when the channel closes.
func (c *EncryptedChannel) OnClose(fn func()) {
	c.mu.Lock()
	c.onCloseFns = append(c.onCloseFns, fn)
	c.mu.Unlock()
}

// IsOpen reports whether the channel is in the open state.
func (c *EncryptedChannel) IsOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == stateOpen
}

// Close closes the underlying transport.
func (c *EncryptedChannel) Close() {
	c.mu.Lock()
	c.state = stateClosed
	c.mu.Unlock()
	c.transport.Close()
}

// SharedKey returns the current shared key (used in tests).
func (c *EncryptedChannel) SharedKey() [32]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sharedKey
}

// PublicKey returns our ephemeral public key (client side only; used in tests).
func (c *EncryptedChannel) PublicKey() [32]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientPubKey
}

func (c *EncryptedChannel) handleMessage(msg []byte) {
	// Try to parse as a handshake control message first.
	var probe struct {
		Type string `json:"type"`
		Key  string `json:"key,omitempty"`
	}
	isJSON := json.Unmarshal(msg, &probe) == nil

	c.mu.Lock()
	state := c.state
	c.mu.Unlock()

	switch state {
	case stateHandshaking:
		if !isJSON {
			return
		}
		switch probe.Type {
		case "e2ee_hello":
			// Daemon side: process hello
			c.processDaemonHello(probe.Key)
		case "e2ee_ready":
			// Client side: handshake complete
			c.transitionToOpen()
		default:
			// Invalid hello message during handshake
			if c.Logger != nil {
				c.Logger.Warn("e2ee invalid hello message", "type", probe.Type, "raw", string(msg))
			}
		}

	case stateOpen:
		if isJSON {
			switch probe.Type {
			case "e2ee_hello":
				// Re-hello (client retry or new client)
				c.processDaemonRehello(probe.Key)
				return
			case "e2ee_ready":
				// Stale ready message; ignore
				return
			default:
				// Plaintext app message on encrypted channel — close with error
				c.transport.Close()
				return
			}
		}
		// Binary or non-JSON: treat as encrypted bundle
		c.decryptAndDeliver(msg)
	}
}

func (c *EncryptedChannel) processDaemonHello(clientKeyB64 string) {
	if c.daemonKP == nil {
		return
	}
	clientPub, err := ImportPublicKey(clientKeyB64)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Warn("e2ee invalid hello: bad public key", "error", err, "key", clientKeyB64)
		}
		return
	}
	sharedKey := DeriveSharedKey(c.daemonKP.SecretKey, clientPub)

	c.mu.Lock()
	c.sharedKey = sharedKey
	c.mu.Unlock()

	readyJSON, _ := json.Marshal(e2eeReady{Type: "e2ee_ready"})
	_ = c.transport.Send(readyJSON)

	c.transitionToOpen()
}

func (c *EncryptedChannel) processDaemonRehello(clientKeyB64 string) {
	if c.daemonKP == nil {
		return
	}
	clientPub, err := ImportPublicKey(clientKeyB64)
	if err != nil {
		return
	}
	newSharedKey := DeriveSharedKey(c.daemonKP.SecretKey, clientPub)

	readyJSON, _ := json.Marshal(e2eeReady{Type: "e2ee_ready"})

	c.mu.Lock()
	same := newSharedKey == c.sharedKey
	if !same {
		c.sharedKey = newSharedKey
		c.pendingSends = nil
	}
	c.mu.Unlock()

	_ = c.transport.Send(readyJSON)
}

func (c *EncryptedChannel) transitionToOpen() {
	c.mu.Lock()
	if c.state != stateHandshaking {
		c.mu.Unlock()
		return
	}
	c.state = stateOpen
	pending := c.pendingSends
	c.pendingSends = nil
	sharedKey := c.sharedKey
	cbs := c.onOpenFns
	c.mu.Unlock()

	for _, cb := range cbs {
		cb()
	}

	for _, p := range pending {
		_ = c.encryptAndSend(sharedKey, p)
	}
}

func (c *EncryptedChannel) encryptAndSend(sharedKey [32]byte, plaintext []byte) error {
	bundle, err := Encrypt(sharedKey, plaintext)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(bundle)
	return c.transport.Send([]byte(encoded))
}

func (c *EncryptedChannel) decryptAndDeliver(msg []byte) {
	// Ciphertext arrives as base64-encoded text
	c.mu.Lock()
	sharedKey := c.sharedKey
	fn := c.onMessageFn
	logger := c.Logger
	c.mu.Unlock()

	bundle, err := base64.StdEncoding.DecodeString(string(msg))
	if err != nil {
		// Fall back: treat as raw bytes
		bundle = msg
	}

	plaintext, err := Decrypt(sharedKey, bundle)
	if err != nil {
		if logger != nil {
			logger.Warn("e2ee decrypt failed", "error", err, "bundleLen", len(bundle))
		}
		return
	}
	if fn != nil {
		fn(plaintext)
	}
}

type channelError string

func (e channelError) Error() string { return string(e) }

const errChannelNotOpen channelError = "e2ee: channel not open"
