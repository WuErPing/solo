// Package relaymetrics exposes Prometheus metrics for the relay server.
// It is named relaymetrics to avoid conflicting with the standard library's
// runtime/metrics package basename as flagged by revive.
package relaymetrics

import "github.com/prometheus/client_golang/prometheus"

var (
	Sessions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "solo_relay_sessions_total",
		Help: "Active relay sessions",
	})
	Connections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "solo_relay_connections_total",
		Help: "Active WebSocket connections",
	})
	FramesForwarded = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "solo_relay_frames_forwarded_total",
		Help: "Total frames relayed",
	})
	FramesBuffered = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "solo_relay_frames_buffered_total",
		Help: "Total frames buffered (late server socket)",
	})
	BufferOverflows = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "solo_relay_buffer_overflows_total",
		Help: "Total frames dropped due to buffer full",
	})
)

func init() {
	prometheus.MustRegister(
		Sessions,
		Connections,
		FramesForwarded,
		FramesBuffered,
		BufferOverflows,
	)
}
