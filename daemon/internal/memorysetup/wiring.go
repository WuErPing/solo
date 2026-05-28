// Package memorysetup wires the session-memory feature (recorder + redactor
// + bridge) from a MemoryConfig. It is intentionally decoupled from the
// server package so that daemon/internal/server depends only on the
// bridge.MemoryBridge interface, not on concrete implementations.
//
// Typical daemon startup:
//
//	feature, err := memorysetup.Build(cfg.Memory)
//	if err != nil { return err }
//	defer feature.Close()
//	ws := server.NewWSServerWithConfig(server.DaemonConfig{
//	    ...
//	    MemoryBridge: feature.Bridge,
//	})
package memorysetup

import (
	"errors"
	"fmt"

	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/memory"
	"github.com/WuErPing/solo/daemon/internal/memory/bridge"
	"github.com/WuErPing/solo/daemon/internal/memory/filebackend"
	"github.com/WuErPing/solo/daemon/internal/memory/redact"
)

// Feature bundles the runtime objects that implement session memory.
// Bridge is the interface injected into the server; Recorder is exposed so
// the daemon can flush/close it on shutdown.
type Feature struct {
	Bridge   *bridge.Bridge
	Recorder memory.TurnRecorder
}

// Close flushes pending turns and releases resources. Idempotent and
// safe on a nil receiver so callers can always `defer feature.Close()`.
func (f *Feature) Close() error {
	if f == nil || f.Recorder == nil {
		return nil
	}
	return f.Recorder.Close()
}

// Build constructs the memory Feature from cfg. When the feature is
// disabled (Enabled explicitly set to false), Build returns (nil, nil)
// — the caller treats a nil Feature as "feature disabled".
func Build(cfg config.MemoryConfig) (*Feature, error) {
	if !cfg.IsEnabled() {
		return nil, nil
	}

	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	recorder, err := buildRecorder(cfg)
	if err != nil {
		return nil, fmt.Errorf("memory: recorder: %w", err)
	}

	r, err := redact.BuildRedactor(cfg.Redact)
	if err != nil {
		_ = recorder.Close()
		return nil, fmt.Errorf("memory: redactor: %w", err)
	}

	b, err := bridge.New(recorder, bridge.WithRedactor(r))
	if err != nil {
		_ = recorder.Close()
		return nil, fmt.Errorf("memory: bridge: %w", err)
	}

	return &Feature{Bridge: b, Recorder: recorder}, nil
}

// buildRecorder selects the recorder implementation per cfg.Backend.
// Phase 1 supports only "file"; sqlite/middleware will be added later.
func buildRecorder(cfg config.MemoryConfig) (memory.TurnRecorder, error) {
	switch cfg.Backend {
	case "file":
		fcfg := filebackend.Config{
			QueueSize:     cfg.QueueSize,
			Overflow:      filebackend.Overflow(cfg.Overflow),
			FlushInterval: filebackend.DefaultFlushInterval,
			Root:          cfg.Root,
			BaseDir:       cfg.SoloHome,
		}
		return filebackend.New(fcfg)
	case "sqlite", "middleware":
		return nil, errors.New("memory: backend " + cfg.Backend + " is not yet implemented")
	default:
		return nil, errors.New("memory: unknown backend " + cfg.Backend)
	}
}
