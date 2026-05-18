package server

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WuErPing/solo/protocol"
)

type blockingWriteConn struct {
	readErr      chan error
	readOnce     sync.Once
	writeStarted chan struct{}
	writeOnce    sync.Once
	closed       chan struct{}
	closeOnce    sync.Once
}

func newBlockingWriteConn() *blockingWriteConn {
	return &blockingWriteConn{
		readErr:      make(chan error, 1),
		writeStarted: make(chan struct{}),
		closed:       make(chan struct{}),
	}
}

func (c *blockingWriteConn) ReadMessage() (int, []byte, error) {
	err := <-c.readErr
	return websocket.TextMessage, nil, err
}

func (c *blockingWriteConn) WriteMessage(messageType int, data []byte) error {
	c.writeOnce.Do(func() { close(c.writeStarted) })
	<-c.closed
	return errors.New("connection closed while write was blocked")
}

func (c *blockingWriteConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func (c *blockingWriteConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return nil
}

func (c *blockingWriteConn) SetPongHandler(h func(appData string) error) {}

func (c *blockingWriteConn) SetReadDeadline(t time.Time) error { return nil }

func (c *blockingWriteConn) injectReadError(err error) {
	c.readOnce.Do(func() { c.readErr <- err })
}

func TestSession_RunReadLoop_EntersGraceWhenWritePumpIsBlocked(t *testing.T) {
	conn := newBlockingWriteConn()
	sess := newTestSessionGrace(t, conn, 5*time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.Run()
	}()

	time.Sleep(50 * time.Millisecond)
	sess.sendMessage(protocol.NewPongMessage())

	select {
	case <-conn.writeStarted:
	case <-time.After(time.Second):
		conn.Close()
		t.Fatal("writePump did not start blocked write")
	}

	conn.injectReadError(&websocket.CloseError{Code: websocket.CloseNormalClosure})

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		conn.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("session.Run did not return while writePump was blocked; disconnect cleanup must enter grace without waiting indefinitely")
	}

	if !sess.IsInGrace() {
		t.Fatal("expected session to enter grace after blocked write cleanup")
	}
}
