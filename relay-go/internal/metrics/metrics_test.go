package metrics

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

func TestSessionsMetric(t *testing.T) {
	Sessions.Set(5)
	v := testutil.ToFloat64(Sessions)
	if v != 5 {
		t.Errorf("Sessions = %v, want 5", v)
	}
	Sessions.Set(0) // reset
}

func TestConnectionsMetric(t *testing.T) {
	Connections.Set(3)
	v := testutil.ToFloat64(Connections)
	if v != 3 {
		t.Errorf("Connections = %v, want 3", v)
	}
	Connections.Set(0) // reset
}

func TestFramesForwardedMetric(t *testing.T) {
	before := testutil.ToFloat64(FramesForwarded)
	FramesForwarded.Inc()
	after := testutil.ToFloat64(FramesForwarded)
	if after != before+1 {
		t.Errorf("FramesForwarded incremented by %v, want 1", after-before)
	}
}

func TestFramesBufferedMetric(t *testing.T) {
	before := testutil.ToFloat64(FramesBuffered)
	FramesBuffered.Add(2)
	after := testutil.ToFloat64(FramesBuffered)
	if after != before+2 {
		t.Errorf("FramesBuffered incremented by %v, want 2", after-before)
	}
}

func TestBufferOverflowsMetric(t *testing.T) {
	before := testutil.ToFloat64(BufferOverflows)
	BufferOverflows.Inc()
	after := testutil.ToFloat64(BufferOverflows)
	if after != before+1 {
		t.Errorf("BufferOverflows incremented by %v, want 1", after-before)
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
