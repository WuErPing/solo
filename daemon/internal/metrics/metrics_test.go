package daemonmetrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	daemonmetrics "github.com/WuErPing/solo/daemon/internal/metrics"
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

func TestSessionsActiveGauge(t *testing.T) {
	// Set gauge to a known value and verify via collector.
	daemonmetrics.SessionsActive.Set(5)

	val, err := getGaugeValue(daemonmetrics.SessionsActive)
	if err != nil {
		t.Fatalf("get gauge value: %v", err)
	}
	if val != 5 {
		t.Errorf("SessionsActive = %v, want 5", val)
	}

	daemonmetrics.SessionsActive.Dec()
	val, err = getGaugeValue(daemonmetrics.SessionsActive)
	if err != nil {
		t.Fatalf("get gauge value after dec: %v", err)
	}
	if val != 4 {
		t.Errorf("SessionsActive = %v, want 4", val)
	}

	daemonmetrics.SessionsActive.Inc()
	val, err = getGaugeValue(daemonmetrics.SessionsActive)
	if err != nil {
		t.Fatalf("get gauge value after inc: %v", err)
	}
	if val != 5 {
		t.Errorf("SessionsActive = %v, want 5", val)
	}
}

func TestConnectionsTotalCounter(t *testing.T) {
	before, err := getCounterValue(daemonmetrics.ConnectionsTotal)
	if err != nil {
		t.Fatalf("get counter value: %v", err)
	}

	daemonmetrics.ConnectionsTotal.Inc()

	after, err := getCounterValue(daemonmetrics.ConnectionsTotal)
	if err != nil {
		t.Fatalf("get counter value after inc: %v", err)
	}

	if after != before+1 {
		t.Errorf("ConnectionsTotal = %v, want %v", after, before+1)
	}
}

func TestMessagesSentTotalCounter(t *testing.T) {
	before, err := getCounterValue(daemonmetrics.MessagesSentTotal)
	if err != nil {
		t.Fatalf("get counter value: %v", err)
	}

	daemonmetrics.MessagesSentTotal.Inc()

	after, err := getCounterValue(daemonmetrics.MessagesSentTotal)
	if err != nil {
		t.Fatalf("get counter value after inc: %v", err)
	}

	if after != before+1 {
		t.Errorf("MessagesSentTotal = %v, want %v", after, before+1)
	}
}

func TestMessagesReceivedTotalCounter(t *testing.T) {
	before, err := getCounterValue(daemonmetrics.MessagesReceivedTotal)
	if err != nil {
		t.Fatalf("get counter value: %v", err)
	}

	daemonmetrics.MessagesReceivedTotal.Inc()

	after, err := getCounterValue(daemonmetrics.MessagesReceivedTotal)
	if err != nil {
		t.Fatalf("get counter value after inc: %v", err)
	}

	if after != before+1 {
		t.Errorf("MessagesReceivedTotal = %v, want %v", after, before+1)
	}
}

func getGaugeValue(g prometheus.Gauge) (float64, error) {
	var m dto.Metric
	if err := g.Write(&m); err != nil {
		return 0, err
	}
	return m.GetGauge().GetValue(), nil
}

func getCounterValue(c prometheus.Counter) (float64, error) {
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		return 0, err
	}
	return m.GetCounter().GetValue(), nil
}
