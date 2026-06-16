package wsconn

import (
	"testing"
	"time"
)

// mockConn implements all three interfaces to verify compile-time compatibility.
type mockConn struct{}

func (m *mockConn) ReadMessage() (int, []byte, error)               { return 0, nil, nil }
func (m *mockConn) WriteMessage(_ int, _ []byte) error              { return nil }
func (m *mockConn) Close() error                                    { return nil }
func (m *mockConn) WriteControl(_ int, _ []byte, _ time.Time) error { return nil }
func (m *mockConn) SetPongHandler(_ func(appData string) error)     {}
func (m *mockConn) SetReadDeadline(_ time.Time) error               { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error              { return nil }

func TestInterfaceCompatibility(_ *testing.T) {
	var c mockConn

	var _ WSConn = &c
	var _ PingableConn = &c
	var _ WriteDeadlineConn = &c
}
