package server

import (
	"github.com/WuErPing/solo/protocol"
)

// messageHandler is a function that handles a decoded session inbound message.
type messageHandler func(s *Session, msg protocol.SessionInboundMessage)

// messageHandlerRegistry maps message type strings to their handler functions.
// This replaces the type-switch dispatch in handleSessionMessage with an
// open-closed registry: new message types are added by calling Register,
// without modifying the dispatch logic.
type messageHandlerRegistry struct {
	handlers map[string]messageHandler
}

// newMessageHandlerRegistry creates an empty handler registry.
func newMessageHandlerRegistry() *messageHandlerRegistry {
	return &messageHandlerRegistry{
		handlers: make(map[string]messageHandler),
	}
}

// Register adds a handler for the given message type.
// If a handler already exists for msgType, it is replaced.
func (r *messageHandlerRegistry) Register(msgType string, handler messageHandler) {
	r.handlers[msgType] = handler
}

// HasHandler returns whether a handler is registered for the given message type.
func (r *messageHandlerRegistry) HasHandler(msgType string) bool {
	_, ok := r.handlers[msgType]
	return ok
}

// Handle dispatches msg to the registered handler for msg.MsgType().
// If no handler is registered, it logs a debug message and sends an RPC error.
func (r *messageHandlerRegistry) Handle(s *Session, msg protocol.SessionInboundMessage) {
	handler, ok := r.handlers[msg.MsgType()]
	if !ok {
		s.logger.Debug("unhandled session message", "type", msg.MsgType())
		s.sendRPCError("", msg.MsgType(), "not implemented", nil)
		return
	}
	handler(s, msg)
}
