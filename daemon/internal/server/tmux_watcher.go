package server

import (
	"context"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

const defaultTmuxWatchInterval = 500 * time.Millisecond

// tmuxListPanesFunc is overridable in tests.
var tmuxListPanesFunc = tmuxListPanesActivity

// TmuxPaneWatcher polls tmux for pane activity and broadcasts a
// tmux/pane_changed notification when any pane's window_activity advances.
// Server-level (one per daemon), lazily started on the first tmux request.
type TmuxPaneWatcher struct {
	logger       *slog.Logger
	broadcast    func(protocol.WSOutboundMessage)
	interval     time.Duration
	lastActivity map[string]int64
	mu           sync.Mutex
	once         sync.Once
	stopCh       chan struct{}
}

func NewTmuxPaneWatcher(logger *slog.Logger, broadcast func(protocol.WSOutboundMessage)) *TmuxPaneWatcher {
	return &TmuxPaneWatcher{
		logger:       logger,
		broadcast:    broadcast,
		interval:     defaultTmuxWatchInterval,
		lastActivity: make(map[string]int64),
		stopCh:       make(chan struct{}),
	}
}

// StartOnce starts the background polling loop at most once.
func (w *TmuxPaneWatcher) StartOnce() {
	w.once.Do(func() {
		go w.loop()
	})
}

func (w *TmuxPaneWatcher) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

func (w *TmuxPaneWatcher) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.poll()
		}
	}
}

func (w *TmuxPaneWatcher) poll() {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()

	panes, err := tmuxListPanesFunc(ctx)
	if err != nil {
		w.logger.Debug("tmux watcher: list-panes failed", "error", err)
		return
	}

	w.mu.Lock()
	var changed []string
	for paneID, activity := range panes {
		if prev, ok := w.lastActivity[paneID]; ok && activity > prev {
			changed = append(changed, paneID)
		}
		w.lastActivity[paneID] = activity
	}
	// Prune panes that no longer exist.
	for paneID := range w.lastActivity {
		if _, ok := panes[paneID]; !ok {
			delete(w.lastActivity, paneID)
		}
	}
	w.mu.Unlock()

	if len(changed) == 0 {
		return
	}

	w.broadcast(protocol.NewSessionMessage(&protocol.TmuxPaneChangedNotification{
		Type: "tmux/pane_changed",
		Payload: protocol.TmuxPaneChangedPayload{
			PaneIDs: changed,
		},
	}))
}

// tmuxListPanesActivity runs tmux list-panes and returns paneID -> window_activity.
func tmuxListPanesActivity(ctx context.Context) (map[string]int64, error) {
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_id} #{window_activity}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		ts, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		result[parts[0]] = ts
	}
	return result, nil
}
