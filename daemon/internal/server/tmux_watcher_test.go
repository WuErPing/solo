package server

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/WuErPing/solo/protocol"
)

func TestTmuxPaneWatcherDetectsActivity(t *testing.T) {
	var mu sync.Mutex
	var broadcasts []protocol.WSOutboundMessage

	w := NewTmuxPaneWatcher(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(msg protocol.WSOutboundMessage) {
			mu.Lock()
			broadcasts = append(broadcasts, msg)
			mu.Unlock()
		},
	)

	origFunc := tmuxListPanesFunc
	defer func() { tmuxListPanesFunc = origFunc }()

	// First poll: baseline activity timestamps.
	tmuxListPanesFunc = func(_ context.Context) (map[string]int64, error) {
		return map[string]int64{"%0": 1000, "%1": 2000}, nil
	}
	w.poll()

	// Second poll: %0 changed, %1 unchanged.
	tmuxListPanesFunc = func(_ context.Context) (map[string]int64, error) {
		return map[string]int64{"%0": 1001, "%1": 2000}, nil
	}
	w.poll()

	mu.Lock()
	defer mu.Unlock()

	if len(broadcasts) != 1 {
		t.Fatalf("expected exactly 1 broadcast, got %d", len(broadcasts))
	}

	msg, ok := broadcasts[0].Message.(*protocol.TmuxPaneChangedNotification)
	if !ok {
		t.Fatalf("expected TmuxPaneChangedNotification, got %T", broadcasts[0].Message)
	}
	if len(msg.Payload.PaneIDs) != 1 || msg.Payload.PaneIDs[0] != "%0" {
		t.Errorf("expected [%%0], got %v", msg.Payload.PaneIDs)
	}
}

func TestTmuxPaneWatcherNoBroadcastWhenUnchanged(t *testing.T) {
	var mu sync.Mutex
	broadcastCount := 0

	w := NewTmuxPaneWatcher(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(protocol.WSOutboundMessage) {
			mu.Lock()
			broadcastCount++
			mu.Unlock()
		},
	)

	origFunc := tmuxListPanesFunc
	defer func() { tmuxListPanesFunc = origFunc }()

	tmuxListPanesFunc = func(_ context.Context) (map[string]int64, error) {
		return map[string]int64{"%0": 1000}, nil
	}
	w.poll()
	w.poll()
	w.poll()

	mu.Lock()
	defer mu.Unlock()
	if broadcastCount != 0 {
		t.Errorf("expected 0 broadcasts for unchanged activity, got %d", broadcastCount)
	}
}

func TestTmuxPaneWatcherPrunesVanishedPanes(t *testing.T) {
	w := NewTmuxPaneWatcher(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(protocol.WSOutboundMessage) {},
	)

	origFunc := tmuxListPanesFunc
	defer func() { tmuxListPanesFunc = origFunc }()

	tmuxListPanesFunc = func(_ context.Context) (map[string]int64, error) {
		return map[string]int64{"%0": 100, "%1": 200}, nil
	}
	w.poll()

	w.mu.Lock()
	if len(w.lastActivity) != 2 {
		t.Fatalf("expected 2 tracked panes, got %d", len(w.lastActivity))
	}
	w.mu.Unlock()

	tmuxListPanesFunc = func(_ context.Context) (map[string]int64, error) {
		return map[string]int64{"%0": 100}, nil
	}
	w.poll()

	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.lastActivity) != 1 {
		t.Fatalf("expected 1 tracked pane after prune, got %d", len(w.lastActivity))
	}
	if _, ok := w.lastActivity["%1"]; ok {
		t.Error("%1 should have been pruned")
	}
}

func TestTmuxPaneWatcherStartOnce(t *testing.T) {
	w := NewTmuxPaneWatcher(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(protocol.WSOutboundMessage) {},
	)
	w.interval = 10_000_000_000 // 10s — won't tick during test

	origFunc := tmuxListPanesFunc
	tmuxListPanesFunc = func(_ context.Context) (map[string]int64, error) {
		return nil, nil
	}
	defer func() { tmuxListPanesFunc = origFunc }()

	w.StartOnce()
	w.StartOnce()
	w.StartOnce()
	w.Stop()
	t.Log("StartOnce is idempotent and Stop does not panic")
}
