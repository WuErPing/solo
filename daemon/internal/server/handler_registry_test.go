package server

import (
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestMessageHandlerRegistry_RegisterAndDispatch(t *testing.T) {
	r := newMessageHandlerRegistry()
	called := false
	r.Register("test_msg", func(s *Session, msg protocol.SessionInboundMessage) {
		called = true
	})

	handler, ok := r.handlers["test_msg"]
	if !ok {
		t.Fatal("expected handler to be registered")
	}
	handler(nil, nil)
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestMessageHandlerRegistry_GetMissing(t *testing.T) {
	r := newMessageHandlerRegistry()
	_, ok := r.handlers["missing"]
	if ok {
		t.Error("expected no handler for missing type")
	}
}

func TestMessageHandlerRegistry_Overwrite(t *testing.T) {
	r := newMessageHandlerRegistry()
	first := false
	second := false
	r.Register("test", func(s *Session, msg protocol.SessionInboundMessage) {
		first = true
	})
	r.Register("test", func(s *Session, msg protocol.SessionInboundMessage) {
		second = true
	})

	handler := r.handlers["test"]
	handler(nil, nil)
	if first {
		t.Error("expected first handler to be overwritten")
	}
	if !second {
		t.Error("expected second handler to be called")
	}
}
