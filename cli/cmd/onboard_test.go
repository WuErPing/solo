package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestResolveOnboardHost_Default(t *testing.T) {
	orig := onboardPort
	defer func() { onboardPort = orig }()

	onboardPort = ""
	host := resolveOnboardHost()
	if host == "" {
		t.Error("expected non-empty host")
	}
	// Should not start with ws://
	if strings.HasPrefix(host, "ws://") {
		t.Error("host should not start with ws://")
	}
	// Should not end with /ws
	if strings.HasSuffix(host, "/ws") {
		t.Error("host should not end with /ws")
	}
}

func TestResolveOnboardHost_CustomPort(t *testing.T) {
	orig := onboardPort
	defer func() { onboardPort = orig }()

	onboardPort = "9999"
	host := resolveOnboardHost()
	if host != "127.0.0.1:9999" {
		t.Errorf("expected 127.0.0.1:9999, got %q", host)
	}
}

func TestGeneratePairingURL_NoServerID(t *testing.T) {
	// With a temp SOLO_HOME that has no server-id file, should return error
	t.Setenv("SOLO_HOME", t.TempDir())
	_, err := generatePairingURL(t.TempDir())
	if err == nil {
		t.Error("expected error when server-id file is missing")
	}
}

func TestPrintNextSteps_WithPairing(t *testing.T) {
	var buf bytes.Buffer
	orig := cmdStdout
	cmdStdout = &buf
	defer func() { cmdStdout = orig }()

	printNextSteps("/tmp/solo", "https://solo.up2ai.top/#offer=abc")
	out := buf.String()
	if !strings.Contains(out, "Next steps:") {
		t.Error("expected 'Next steps:' in output")
	}
	if !strings.Contains(out, "scan the QR code") {
		t.Error("expected QR code reference in output")
	}
	if !strings.Contains(out, "Solo is ready!") {
		t.Error("expected 'Solo is ready!' in output")
	}
	if !strings.Contains(out, "/tmp/solo/daemon.log") {
		t.Error("expected daemon.log path in output")
	}
}

func TestPrintNextSteps_WithoutPairing(t *testing.T) {
	var buf bytes.Buffer
	orig := cmdStdout
	cmdStdout = &buf
	defer func() { cmdStdout = orig }()

	printNextSteps("/tmp/solo", "")
	out := buf.String()
	if !strings.Contains(out, "connect to your daemon") {
		t.Error("expected 'connect to your daemon' when no pairing URL")
	}
	if strings.Contains(out, "scan the QR code") {
		t.Error("should not reference QR when no pairing URL")
	}
}
