// Package daemonmetrics registers Prometheus metrics for the daemon.
package daemonmetrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// SessionsActive tracks the number of currently active WebSocket sessions.
	SessionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "solo_daemon_sessions_active",
		Help: "Number of currently active WebSocket sessions",
	})

	// ConnectionsTotal counts the total number of accepted WebSocket connections.
	ConnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "solo_daemon_connections_total",
		Help: "Total number of accepted WebSocket connections",
	})

	// MessagesSentTotal counts the total number of outbound messages sent.
	MessagesSentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "solo_daemon_messages_sent_total",
		Help: "Total number of outbound messages sent to clients",
	})

	// MessagesReceivedTotal counts the total number of inbound messages received.
	MessagesReceivedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "solo_daemon_messages_received_total",
		Help: "Total number of inbound messages received",
	})

	// TimelineRowsTotal tracks the current number of in-memory timeline rows
	// across all agents.
	TimelineRowsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "solo_daemon_timeline_rows_total",
		Help: "Current number of in-memory timeline rows across all agents",
	})

	// TimelineRowsDroppedTotal counts the number of timeline rows dropped due
	// to per-agent row limits.
	TimelineRowsDroppedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "solo_daemon_timeline_rows_dropped_total",
		Help: "Total number of timeline rows dropped due to per-agent limits",
	})
)

func init() {
	prometheus.MustRegister(
		SessionsActive,
		ConnectionsTotal,
		MessagesSentTotal,
		MessagesReceivedTotal,
		TimelineRowsTotal,
		TimelineRowsDroppedTotal,
	)
}
