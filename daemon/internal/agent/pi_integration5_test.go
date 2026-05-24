package agent

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WuErPing/solo/daemon/internal/agent/base"
	"github.com/WuErPing/solo/protocol"
)

func TestPiIntegration_PumpDebug(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pm := base.NewProcessManager("pi", logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	stdout, stderr, stdin, cmd, err := pm.Start(ctx, []string{"-p", "hi", "--mode", "json", "--work-dir", "/tmp"}, "/tmp", os.Environ())
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	stdin.Close()
	go pm.DrainStderr(stderr)
	
	go func() {
		cmd.Wait()
		t.Logf("process exited at %v", time.Now().Format("15:04:05.000"))
	}()
	
	start := time.Now()
	scanner := bufio.NewScanner(stdout)
	t.Logf("start scanning at %v", time.Now().Format("15:04:05.000"))
	count := 0
	for scanner.Scan() {
		count++
	}
	t.Logf("scan done at %v, lines=%d, err=%v, elapsed=%v", time.Now().Format("15:04:05.000"), count, scanner.Err(), time.Since(start))
	
	// Now test with EventPump
	stdout2, stderr2, stdin2, cmd2, err := pm.Start(ctx, []string{"-p", "hi", "--mode", "json", "--work-dir", "/tmp"}, "/tmp", os.Environ())
	if err != nil {
		t.Fatalf("start2 failed: %v", err)
	}
	stdin2.Close()
	go pm.DrainStderr(stderr2)
	go func() { cmd2.Wait() }()
	
	dispatcher := base.NewChannelDispatcher(logger)
	pump := base.NewEventPump(logger, dispatcher)
	pump.SetProvider("pi")
	translator := &piTranslator{session: &piSession{base: base.NewBaseSession("pi", &protocol.AgentSessionConfig{}, logger)}}
	detector := &piTerminalDetector{session: &piSession{}}
	
	start = time.Now()
	t.Logf("start pump at %v", time.Now().Format("15:04:05.000"))
	result, err := pump.RunBlocking(ctx, stdout2, translator, detector)
	t.Logf("pump done at %v, result=%+v, err=%v, elapsed=%v", time.Now().Format("15:04:05.000"), result, err, time.Since(start))
}
