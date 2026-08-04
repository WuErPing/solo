package daemonmetrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	// Imported for side effects: package init registers the daemon metrics
	// with the default registry, which is what the test asserts on.
	_ "github.com/WuErPing/solo/daemon/internal/metrics"
)

func TestMetricsAreRegistered(t *testing.T) {
	// All metrics should be registered with the default registry.
	collectors, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	expected := map[string]bool{
		"solo_daemon_sessions_active":         false,
		"solo_daemon_connections_total":       false,
		"solo_daemon_messages_sent_total":     false,
		"solo_daemon_messages_received_total": false,
	}

	for _, mf := range collectors {
		if _, ok := expected[*mf.Name]; ok {
			expected[*mf.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("metric %q not found in registry", name)
		}
	}
}
