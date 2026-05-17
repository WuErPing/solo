package wsconn

import (
	"testing"
	"time"
)

// mockConn implements all three interfaces to verify compile-time compatibility.
type mockConn struct{}

func (m *mockConn) ReadMessage() (int, []byte, error) { return 0, nil, nil }
func (m *mockConn) WriteMessage(messageType int, data []byte) error { return nil }
func (m *mockConn) Close() error { return nil }
func (m *mockConn) WriteControl(messageType int, data []byte, deadline time.Time) error { return nil }
func (m *mockConn) SetPongHandler(h func(appData string) error) {}
func (m *mockConn) SetReadDeadline(t time.Time) error { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestInterfaceCompatibility(t *testing.T) {
	var c mockConn

	var _ WSConn = &c
	var _ PingableConn = &c
	var _ WriteDeadlineConn = &c
}
