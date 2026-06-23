//go:build manual_test

package opencode

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func TestManualSSEFlow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client := NewClient("", logger)
	if err := client.IsAvailable(context.Background()); err != nil {
		t.Skip("opencode not available:", err)
	}

	models, err := client.ListModels(context.Background(), "")
	if err != nil {
		t.Fatal("list models:", err)
	}
	t.Logf("Models: %d, first: %s", len(models), models[0].ID)

	cwd := t.TempDir()
	modelID := models[0].ID

	sess, err := client.CreateSession(context.Background(), &protocol.AgentSessionConfig{
		Provider: "opencode",
		Cwd:      cwd,
		Model:    &modelID,
	})
	if err != nil {
		t.Fatal("create session:", err)
	}
	t.Cleanup(func() {
		sess.Close()
		ShutdownOpenCodeServerManager()
	})

	ch := sess.Subscribe()

	go func() {
		for evt := range ch {
			m, ok := evt.Event.(map[string]interface{})
			if !ok {
				continue
			}
			t.Logf("EVENT: %v", m["type"])
			if item, ok := m["item"].(map[string]interface{}); ok {
				text := fmt.Sprint(item["text"])
				if len(text) > 60 {
					text = text[:60] + "..."
				}
				t.Logf("  item type=%v text=%q", item["type"], text)
			}
		}
		t.Log("channel closed")
	}()

	t.Log("Running prompt...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := sess.Run(ctx, "What is 2+2? Answer in one sentence.", nil, nil, "")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	t.Logf("Run result: %+v", result)
	time.Sleep(500 * time.Millisecond)
}
