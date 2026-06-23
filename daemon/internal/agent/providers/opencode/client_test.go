package opencode

import (
	"log/slog"
	"os"
	"testing"

	"github.com/WuErPing/solo/daemon/internal/agent/providers/contracttest"
)

func TestProviderContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OpenCode contract test in short mode")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient("", logger)
	contracttest.RunProviderContractSuite(t, "opencode", client)
}
