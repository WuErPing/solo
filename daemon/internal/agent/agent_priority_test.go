package agent

import (
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
)

func TestAgentStreamEventValueImplementsCriticalEventInterface(t *testing.T) {
	evt := AgentStreamEvent{
		Event:     protocol.TurnCompletedStreamEvent{},
		Timestamp: time.Now(),
	}

	if !evt.IsCriticalEvent() {
		t.Fatal("expected turn_completed to be critical")
	}
	if _, ok := interface{}(evt).(base.CriticalEvent); !ok {
		t.Fatal("AgentStreamEvent value must implement base.CriticalEvent for dispatcher priority checks")
	}
}
