// Package config holds build-time configuration for the Solo CLI.
package config

// Version is injected at build time via
// -ldflags "-X github.com/WuErPing/solo/cli/internal/config.Version=...".
// It stays "dev" for builds without injection (e.g. plain `go build ./cli`).
var Version = "dev"
