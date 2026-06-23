//go:build !short && !external_api

package pi

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent"
	"github.com/WuErPing/solo/protocol"
)

func TestPiIntegration_EventDebug(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := NewClient("", logger)

	if err := client.IsAvailable(context.Background()); err != nil {
		t.Skipf("pi not available: %v", err)
	}

	config := &protocol.AgentSessionConfig{
		Provider: "pi",
		Cwd:      "/tmp",
	}

	sess, err := client.CreateSession(context.Background(), config)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer sess.Close()

	ch := sess.Subscribe()
	var events []agent.AgentStreamEvent
	done := make(chan struct{})
	go func() {
		for evt := range ch {
			events = append(events, evt)
		}
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sess.Run(ctx, "hi", nil, nil, "")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	fmt.Printf("result: sessionID=%s canceled=%v\n", result.SessionID, result.Canceled)

	sess.Close()
	<-done

	for _, evt := range events {
		payload, ok := evt.Event.(map[string]interface{})
		if !ok {
			continue
		}
		evtType, _ := payload["type"].(string)
		if evtType == "timeline" {
			if item, ok := payload["item"].(protocol.TimelineItem); ok {
				fmt.Printf("timeline: type=%s text=%q callId=%s name=%s status=%s detail=%v error=%v\n", item.Type, item.Text, item.CallID, item.Name, item.Status, item.Detail, item.Error)
			}
		} else {
			fmt.Printf("event: type=%s\n", evtType)
		}
	}
}
