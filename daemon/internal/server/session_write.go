package server

import (
	"time"
)

var websocketWriteTimeout = 10 * time.Second
var writePumpDrainTimeout = 2 * time.Second

func writeMessageWithDeadline(conn WSConn, messageType int, data []byte) error {
	if conn == nil {
		return nil
	}
	if wc, ok := conn.(WriteDeadlineConn); ok {
		_ = wc.SetWriteDeadline(time.Now().Add(websocketWriteTimeout))
		defer wc.SetWriteDeadline(time.Time{})
	}
	return conn.WriteMessage(messageType, data)
}

func waitForWritePump(done <-chan struct{}, timeout time.Duration) bool {
	if timeout <= 0 {
		<-done
		return true
	}
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func closeWSConn(conn WSConn) {
	if conn != nil {
		_ = conn.Close()
	}
}
