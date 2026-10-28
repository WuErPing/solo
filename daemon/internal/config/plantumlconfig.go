package config

import "time"

// DefaultPlantUMLTimeoutSeconds bounds a single diagram render. JVM cold
// starts are the slow path; a healthy local render finishes well under a
// second after warm-up.
const DefaultPlantUMLTimeoutSeconds = 20

// PlantUMLConfig configures where the daemon renders PlantUML diagrams.
// Rendering is local by design: either a plantuml executable on the daemon
// host or a PlantUML-compatible HTTP server the user runs themselves.
type PlantUMLConfig struct {
	// Command is an explicit path to a plantuml executable. When empty the
	// daemon checks SOLO_PLANTUML_BIN and then PATH.
	Command string `json:"command,omitempty"`
	// ServerURL is the base URL of a PlantUML-compatible server (for example
	// http://127.0.0.1:8080/plantuml); the daemon POSTs source to <url>/svg.
	ServerURL string `json:"serverUrl,omitempty"`
	// TimeoutSeconds bounds one render; zero means the default.
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

// ApplyDefaults fills unset values with sane defaults.
func (c *PlantUMLConfig) ApplyDefaults() {
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = DefaultPlantUMLTimeoutSeconds
	}
}

// Timeout returns the effective render timeout.
func (c PlantUMLConfig) Timeout() time.Duration {
	seconds := c.TimeoutSeconds
	if seconds <= 0 {
		seconds = DefaultPlantUMLTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}
