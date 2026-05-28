package main

import (
	"github.com/WuErPing/solo/daemon/internal/config"
	"github.com/WuErPing/solo/daemon/internal/memory/bridge"
	"github.com/WuErPing/solo/daemon/internal/memorysetup"
	"github.com/WuErPing/solo/daemon/internal/server"
)

// init registers the memorysetup builder with the server package so that
// NewDaemon can construct the session-memory feature without the server
// depending on concrete memory implementations. The returned bridge is
// wrapped in bridge.SafeBridge to isolate the main session flow from
// panics, slow recorders, and runaway failure rates.
func init() {
	server.RegisterMemoryFeatureBuilder(func(cfgAny interface{}) (*server.MemoryFeature, error) {
		cfg, ok := cfgAny.(config.MemoryConfig)
		if !ok {
			return nil, nil
		}
		f, err := memorysetup.Build(cfg)
		if err != nil || f == nil {
			return nil, err
		}
		return &server.MemoryFeature{
			Bridge:   bridge.NewSafeBridge(f.Bridge),
			Recorder: f.Recorder,
		}, nil
	})
}
