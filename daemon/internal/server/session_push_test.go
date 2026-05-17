package server

import (
	"io"
	"log/slog"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/push"
	"github.com/WuErPing/solo/protocol"
)

func TestSession_HandleRegisterPushToken(t *testing.T) {
	tokenStore := push.NewInMemoryTokenStore()

	// Create a minimal session with just what we need
	session := &Session{
		clientID:       "test-client",
		pushTokenStore: tokenStore,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	msg := &protocol.RegisterPushTokenMessage{
		Token: "expo-token-123",
	}

	session.handleRegisterPushToken(msg)

	// Verify token was stored
	tokens := tokenStore.GetAll()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0] != "expo-token-123" {
		t.Errorf("expected token 'expo-token-123', got %s", tokens[0])
	}
}

func TestSession_HandleRegisterPushToken_Duplicate(t *testing.T) {
	tokenStore := push.NewInMemoryTokenStore()

	session := &Session{
		clientID:       "test-client",
		pushTokenStore: tokenStore,
		logger:         slog.Default(),
	}

	msg1 := &protocol.RegisterPushTokenMessage{Token: "token-1"}
	msg2 := &protocol.RegisterPushTokenMessage{Token: "token-1"}

	session.handleRegisterPushToken(msg1)
	session.handleRegisterPushToken(msg2)

	tokens := tokenStore.GetAll()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after duplicate registration, got %d", len(tokens))
	}
}

func TestSession_HandleRegisterPushToken_Multiple(t *testing.T) {
	tokenStore := push.NewInMemoryTokenStore()

	session := &Session{
		clientID:       "test-client",
		pushTokenStore: tokenStore,
		logger:         slog.Default(),
	}

	session.handleRegisterPushToken(&protocol.RegisterPushTokenMessage{Token: "token-1"})
	session.handleRegisterPushToken(&protocol.RegisterPushTokenMessage{Token: "token-2"})

	tokens := tokenStore.GetAll()
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
}
