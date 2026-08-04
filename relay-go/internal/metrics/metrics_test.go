package relaymetrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsRegistered(t *testing.T) {
	// Ensure each metric can be collected without panic.
	metrics := []prometheus.Collector{
		Sessions,
		Connections,
		FramesForwarded,
		FramesBuffered,
		BufferOverflows,
	}
	for _, m := range metrics {
		count := testutil.CollectAndCount(m)
		if count == 0 {
			t.Error("expected metric to have at least one collected value")
		}
	}
}

func TestMetricNames(t *testing.T) {
	expected := map[prometheus.Collector]string{
		Sessions:        "solo_relay_sessions_total",
		Connections:     "solo_relay_connections_total",
		FramesForwarded: "solo_relay_frames_forwarded_total",
		FramesBuffered:  "solo_relay_frames_buffered_total",
		BufferOverflows: "solo_relay_buffer_overflows_total",
	}
	for metric, name := range expected {
		descs := make(chan *prometheus.Desc, 1)
		metric.Describe(descs)
		close(descs)
		desc := <-descs
		if !strings.Contains(desc.String(), name) {
			t.Errorf("metric descriptor does not contain %q: %s", name, desc.String())
		}
	}
}
