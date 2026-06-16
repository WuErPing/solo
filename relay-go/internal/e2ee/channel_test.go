package e2ee_test

import (
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/WuErPing/solo/relay/internal/e2ee"
)

// mockTransport is a test double that connects two channels.
// Messages sent on one side are delivered to the other side's registered handler
// in order, via a single delivery goroutine per transport.
type mockTransport struct {
	mu      sync.Mutex
	handler func([]byte)
	other   *mockTransport
	sent    [][]byte
	closed  bool
	queue   chan []byte
}

func newMockTransport() *mockTransport {
	m := &mockTransport{queue: make(chan []byte, 256)}
	go func() {
		for msg := range m.queue {
			m.mu.Lock()
			h := m.handler
			m.mu.Unlock()
			if h != nil {
				h(msg)
			}
		}
	}()
	return m
}

func (m *mockTransport) Send(msg []byte) error {
	cp := append([]byte(nil), msg...)
	m.mu.Lock()
	m.sent = append(m.sent, cp)
	m.mu.Unlock()
	m.other.queue <- cp
	return nil
}

func (m *mockTransport) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
}

func (m *mockTransport) OnMessage(h func([]byte)) {
	m.mu.Lock()
	m.handler = h
	m.mu.Unlock()
}

func (m *mockTransport) OnClose(_ func()) {}

func newTransportPair() (*mockTransport, *mockTransport) {
	a := newMockTransport()
	b := newMockTransport()
	a.other = b
	b.other = a
	return a, b
}

func TestChannelHandshakeEstablishes(t *testing.T) {
	daemonTransport, clientTransport := newTransportPair()

	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}

	var wg sync.WaitGroup
	wg.Add(2)

	daemon := e2ee.NewDaemonChannel(daemonTransport, daemonKP)
	daemon.OnOpen(func() { wg.Done() })

	client := e2ee.NewClientChannel(clientTransport, daemonPub)
	client.OnOpen(func() { wg.Done() })

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake did not complete within 3 seconds")
	}

	if !client.IsOpen() {
		t.Error("client channel not open after handshake")
	}
	if !daemon.IsOpen() {
		t.Error("daemon channel not open after handshake")
	}
}

func TestChannelBidirectionalMessages(t *testing.T) {
	daemonTransport, clientTransport := newTransportPair()

	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}

	var daemonReceived [][]byte
	var clientReceived [][]byte
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(2)

	daemon := e2ee.NewDaemonChannel(daemonTransport, daemonKP)
	daemon.OnMessage(func(msg []byte) {
		mu.Lock()
		daemonReceived = append(daemonReceived, append([]byte(nil), msg...))
		mu.Unlock()
	})
	daemon.OnOpen(func() { wg.Done() })

	client := e2ee.NewClientChannel(clientTransport, daemonPub)
	client.OnMessage(func(msg []byte) {
		mu.Lock()
		clientReceived = append(clientReceived, append([]byte(nil), msg...))
		mu.Unlock()
	})
	client.OnOpen(func() { wg.Done() })

	waitOpen := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitOpen)
	}()

	select {
	case <-waitOpen:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timeout")
	}

	if err := client.Send([]byte("Hello from client")); err != nil {
		t.Fatalf("client.Send: %v", err)
	}
	if err := daemon.Send([]byte("Hello from daemon")); err != nil {
		t.Fatalf("daemon.Send: %v", err)
	}
	if err := client.Send([]byte("Second from client")); err != nil {
		t.Fatalf("client.Send 2: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(daemonReceived) != 2 {
		t.Errorf("daemon expected 2 messages, got %d", len(daemonReceived))
	} else {
		if string(daemonReceived[0]) != "Hello from client" {
			t.Errorf("daemon[0] = %q", daemonReceived[0])
		}
		if string(daemonReceived[1]) != "Second from client" {
			t.Errorf("daemon[1] = %q", daemonReceived[1])
		}
	}
	if len(clientReceived) != 1 {
		t.Errorf("client expected 1 message, got %d", len(clientReceived))
	} else if string(clientReceived[0]) != "Hello from daemon" {
		t.Errorf("client[0] = %q", clientReceived[0])
	}
}

func TestMessagesOpaqueToTransport(t *testing.T) {
	daemonTransport, clientTransport := newTransportPair()

	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}

	var wg sync.WaitGroup
	wg.Add(2)

	daemon := e2ee.NewDaemonChannel(daemonTransport, daemonKP)
	daemon.OnOpen(func() { wg.Done() })

	client := e2ee.NewClientChannel(clientTransport, daemonPub)
	client.OnOpen(func() { wg.Done() })

	waitOpen := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitOpen)
	}()

	select {
	case <-waitOpen:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timeout")
	}

	// Clear sent history after handshake
	clientTransport.mu.Lock()
	clientTransport.sent = nil
	clientTransport.mu.Unlock()

	plaintext := []byte("Secret message")
	if err := client.Send(plaintext); err != nil {
		t.Fatalf("client.Send: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	clientTransport.mu.Lock()
	sent := clientTransport.sent
	clientTransport.mu.Unlock()

	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	wire := sent[0]
	if len(wire) == len(plaintext) {
		t.Error("wire message has same length as plaintext — likely not encrypted")
	}
	// Wire should not contain plaintext
	if string(wire) == string(plaintext) {
		t.Error("wire message equals plaintext — not encrypted")
	}
}

func TestDaemonRehelloSameKeyResendReady(t *testing.T) {
	daemonTransport, clientTransport := newTransportPair()

	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}

	var daemonOpenCount int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	daemon := e2ee.NewDaemonChannel(daemonTransport, daemonKP)
	daemon.OnOpen(func() {
		mu.Lock()
		daemonOpenCount++
		mu.Unlock()
		wg.Done()
	})

	client := e2ee.NewClientChannel(clientTransport, daemonPub)
	client.OnOpen(func() { wg.Done() })

	waitOpen := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitOpen)
	}()

	select {
	case <-waitOpen:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timeout")
	}

	// Record shared key before re-hello
	sharedBefore := daemon.SharedKey()

	// Simulate client sending the same hello again (same client pub key)
	clientPub := client.PublicKey()
	helloMsg := `{"type":"e2ee_hello","key":"` + e2ee.ExportPublicKey(clientPub) + `"}`
	daemonTransport.handler([]byte(helloMsg))
	time.Sleep(30 * time.Millisecond)

	sharedAfter := daemon.SharedKey()
	if sharedBefore != sharedAfter {
		t.Error("daemon re-keyed on same-key re-hello — should not have")
	}

	// Verify re-ready was sent (daemonTransport.sent should include e2ee_ready)
	daemonTransport.mu.Lock()
	var foundReady bool
	for _, s := range daemonTransport.sent {
		if string(s) == `{"type":"e2ee_ready"}` {
			foundReady = true
			break
		}
	}
	daemonTransport.mu.Unlock()
	if !foundReady {
		t.Error("daemon did not re-send e2ee_ready on same-key re-hello")
	}
}

func TestDaemonRehelloDifferentKeyRekeyes(t *testing.T) {
	daemonTransport, clientTransport := newTransportPair()

	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}

	var wg sync.WaitGroup
	wg.Add(2)

	daemon := e2ee.NewDaemonChannel(daemonTransport, daemonKP)
	daemon.OnOpen(func() { wg.Done() })

	client := e2ee.NewClientChannel(clientTransport, daemonPub)
	client.OnOpen(func() { wg.Done() })

	waitOpen := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitOpen)
	}()

	select {
	case <-waitOpen:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timeout")
	}

	sharedBefore := daemon.SharedKey()

	// Send a re-hello with a different client key
	newPub, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	helloMsg := `{"type":"e2ee_hello","key":"` + e2ee.ExportPublicKey(newPub) + `"}`
	daemonTransport.handler([]byte(helloMsg))
	time.Sleep(30 * time.Millisecond)

	sharedAfter := daemon.SharedKey()
	if sharedBefore == sharedAfter {
		t.Error("daemon did not re-key on different-key re-hello")
	}
}

func TestClientRetriesHello(t *testing.T) {
	// Transport that never delivers messages (daemon not responding)
	transport := &noopTransport{}
	daemonPub, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	client := e2ee.NewClientChannel(transport, daemonPub)
	_ = client

	// Initial hello should have been sent
	time.Sleep(20 * time.Millisecond)
	count1 := transport.sentCount()

	// After 1s, should have retried
	time.Sleep(1100 * time.Millisecond)
	count2 := transport.sentCount()
	if count2 <= count1 {
		t.Errorf("expected retry hello after 1s, sent count: before=%d after=%d", count1, count2)
	}
}

func TestPendingSendsFlushedAfterHandshake(t *testing.T) {
	// Use a slow transport: daemon side doesn't process anything until we're ready
	daemonTransport, clientTransport := newTransportPair()

	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}

	var received [][]byte
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(2)

	daemon := e2ee.NewDaemonChannel(daemonTransport, daemonKP)
	daemon.OnMessage(func(msg []byte) {
		mu.Lock()
		received = append(received, append([]byte(nil), msg...))
		mu.Unlock()
	})
	daemon.OnOpen(func() { wg.Done() })

	client := e2ee.NewClientChannel(clientTransport, daemonPub)
	client.OnOpen(func() { wg.Done() })

	// Queue sends before open (will be buffered)
	if err := client.Send([]byte("pending-1")); err != nil {
		t.Fatalf("client.Send pending-1: %v", err)
	}
	if err := client.Send([]byte("pending-2")); err != nil {
		t.Fatalf("client.Send pending-2: %v", err)
	}

	waitOpen := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitOpen)
	}()

	select {
	case <-waitOpen:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timeout")
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Errorf("expected 2 pending messages delivered, got %d", len(received))
	}
}

// noopTransport never delivers messages to the other side.
type noopTransport struct {
	mu    sync.Mutex
	count int
}

func (n *noopTransport) Send(_ []byte) error {
	n.mu.Lock()
	n.count++
	n.mu.Unlock()
	return nil
}

func (n *noopTransport) Close()                 {}
func (n *noopTransport) OnMessage(func([]byte)) {}
func (n *noopTransport) OnClose(func())         {}

func (n *noopTransport) sentCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.count
}

// failSendTransport returns an error on the Nth Send call.
type failSendTransport struct {
	mu        sync.Mutex
	handler   func([]byte)
	closeFn   func()
	count     int
	failOnN   int
	failError error
}

func (f *failSendTransport) Send(_ []byte) error {
	f.mu.Lock()
	f.count++
	n := f.count
	f.mu.Unlock()
	if n == f.failOnN {
		return f.failError
	}
	return nil
}

func (f *failSendTransport) Close() {
	f.mu.Lock()
	closeFn := f.closeFn
	f.mu.Unlock()
	if closeFn != nil {
		closeFn()
	}
}

func (f *failSendTransport) OnMessage(fn func([]byte)) {
	f.mu.Lock()
	f.handler = fn
	f.mu.Unlock()
}

func (f *failSendTransport) OnClose(fn func()) {
	f.mu.Lock()
	f.closeFn = fn
	f.mu.Unlock()
}

func TestDaemonRejectsInvalidHello(t *testing.T) {
	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	daemonKP := e2ee.KeyPair{PublicKey: daemonPub, SecretKey: daemonSec}
	_ = daemonPub // not used in this test

	daemonTransport := newMockTransport()
	daemon := e2ee.NewDaemonChannel(daemonTransport, daemonKP)

	// Register a logger to capture warnings
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	daemon.Logger = logger

	// Send invalid hello (wrong type)
	invalidHello := []byte(`{"type":"invalid","key":"dGVzdA=="}`)
	daemonTransport.mu.Lock()
	handler := daemonTransport.handler
	daemonTransport.mu.Unlock()
	if handler != nil {
		handler(invalidHello)
	}

	// Daemon should still be in handshaking state (not open)
	if daemon.IsOpen() {
		t.Error("daemon should not be open after invalid hello")
	}

	// Create client channel to complete handshake
	clientTransport := newMockTransport()
	clientTransport.other = daemonTransport
	daemonTransport.other = clientTransport

	// Create client channel
	client := e2ee.NewClientChannel(clientTransport, daemonPub)

	// Wait for handshake
	var wg sync.WaitGroup
	wg.Add(1)
	client.OnOpen(func() { wg.Done() })

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handshake did not complete after invalid hello")
	}

	if !client.IsOpen() {
		t.Error("client channel not open after handshake")
	}
}

func TestClientHelloRetrySendFailure(t *testing.T) {
	daemonPub, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Create a transport that fails on the second send (first retry)
	transport := &failSendTransport{
		failOnN:   2,
		failError: errors.New("WebSocket not open"),
	}

	// Create client channel - first send should succeed
	client := e2ee.NewClientChannel(transport, daemonPub)

	// The retry should fail but not panic
	time.Sleep(1500 * time.Millisecond) // wait for one retry

	// Client should still be in handshaking state
	if client.IsOpen() {
		t.Error("client should not be open (no daemon to respond)")
	}

	// Close to stop retry timer
	transport.Close()
}
