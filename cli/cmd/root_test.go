package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

func TestResolveAgentID_ExactMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
		{ID: "def456", Title: "Other"},
	}
	if got := resolveAgentID("abc123", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_PrefixMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
	}
	if got := resolveAgentID("abc", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_TitleMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
	}
	if got := resolveAgentID("My Agent", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_PartialTitleMatch(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "My Agent"},
	}
	if got := resolveAgentID("Agent", agents); got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_AmbiguousPrefix(t *testing.T) {
	agents := []agentEntry{
		{ID: "abc123", Title: "A"},
		{ID: "abc456", Title: "B"},
	}
	// Ambiguous prefix returns first match
	got := resolveAgentID("abc", agents)
	if got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestResolveAgentID_Empty(t *testing.T) {
	agents := []agentEntry{{ID: "abc123", Title: "X"}}
	if got := resolveAgentID("", agents); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if got := resolveAgentID("x", nil); got != "" {
		t.Errorf("expected empty string for nil agents, got %q", got)
	}
}

func TestGetOutputOpts_Default(t *testing.T) {
	opts := getOutputOpts("table", false, false, false, false)
	if opts.Format != output.FormatTable {
		t.Errorf("expected table format, got %v", opts.Format)
	}
}

func TestGetOutputOpts_JSON(t *testing.T) {
	opts := getOutputOpts("table", true, false, false, true)
	if opts.Format != output.FormatJSON {
		t.Errorf("expected json format, got %v", opts.Format)
	}
	if !opts.NoColor {
		t.Error("expected NoColor true")
	}
}

func TestGetOutputOpts_Quiet(t *testing.T) {
	opts := getOutputOpts("table", false, true, false, false)
	if opts.Format != output.FormatQuiet {
		t.Errorf("expected quiet format, got %v", opts.Format)
	}
}

func TestRunDaemonStatus_Table(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	if err := runDaemonStatus(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonStatus failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Daemon is running") {
		t.Errorf("expected 'Daemon is running' in output, got: %s", out)
	}
	if !strings.Contains(out, "test-server-id") {
		t.Errorf("expected server ID in output, got: %s", out)
	}
}

func TestRunDaemonStatus_JSON(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "json"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	if err := runDaemonStatus(cmd, []string{}); err != nil {
		t.Fatalf("runDaemonStatus failed: %v", err)
	}

	out := outBuf.String()
	var result map[string]interface{}
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON output: %v\noutput: %s", err, out)
	}
	if result["status"] != "running" {
		t.Errorf("expected status running, got %v", result["status"])
	}
}

func TestRunDaemonStatus_DaemonNotRunning(t *testing.T) {
	resetFlags(t)

	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	t.Cleanup(func() { os.Unsetenv("SOLO_HOME") })

	oldStdout := cmdStdout
	oldStderr := cmdStderr
	var outBuf, errBuf bytes.Buffer
	cmdStdout = &outBuf
	cmdStderr = &errBuf
	t.Cleanup(func() {
		cmdStdout = oldStdout
		cmdStderr = oldStderr
	})

	// Point to a port that is definitely not listening
	flagHost = "127.0.0.1:1"
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	err := runDaemonStatus(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when daemon is not running")
	}
	ce, ok := err.(*output.CommandError)
	if !ok || ce.Code != "DAEMON_NOT_RUNNING" {
		t.Errorf("expected DAEMON_NOT_RUNNING error, got %v", err)
	}
}

func TestRunProviderLs_Table(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderLs(cmd, []string{}); err != nil {
		t.Fatalf("runProviderLs failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "openai") {
		t.Errorf("expected openai in output, got: %s", out)
	}
	if !strings.Contains(out, "anthropic") {
		t.Errorf("expected anthropic in output, got: %s", out)
	}
}

func TestRunProviderLs_JSON(t *testing.T) {
	_, outBuf, _ := setupTestCLI(t)
	flagFormat = "json"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderLs(cmd, []string{}); err != nil {
		t.Fatalf("runProviderLs failed: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array: %v\noutput: %s", err, outBuf.String())
	}
	if len(result) != 2 {
		t.Errorf("expected 2 providers, got %d", len(result))
	}
}

func TestRunProviderLs_NoProviders(t *testing.T) {
	mock, outBuf, _ := setupTestCLI(t)
	mock.ProvidersSnapshot = &protocol.ProvidersSnapshotPayload{Entries: []protocol.ProviderSnapshotEntry{}}

	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = true

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := runProviderLs(cmd, []string{}); err != nil {
		t.Fatalf("runProviderLs failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "No providers available") {
		t.Errorf("expected 'No providers available', got: %s", out)
	}
}

func TestExecute(t *testing.T) {
	// Execute with no args should not panic or error (it prints help).
	rootCmd.SetArgs([]string{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() with no args: %v", err)
	}
}
