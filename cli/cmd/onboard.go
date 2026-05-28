package cmd

import (
	"fmt"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/output"
)

var (
	onboardPort    string
	onboardHome    string
	onboardNoRelay bool
	onboardNoMCP   bool
	onboardTimeout int
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Run first-time setup, start daemon, and print pairing instructions",
	RunE:  runOnboard,
}

func init() {
	onboardCmd.Flags().StringVar(&onboardPort, "port", "", "Listen port")
	onboardCmd.Flags().StringVar(&onboardHome, "home", "", "Solo home directory")
	onboardCmd.Flags().BoolVar(&onboardNoRelay, "no-relay", false, "Disable relay connection")
	onboardCmd.Flags().BoolVar(&onboardNoMCP, "no-mcp", false, "Disable MCP HTTP endpoint")
	onboardCmd.Flags().IntVar(&onboardTimeout, "timeout", 600, "Max seconds to wait for daemon readiness")

	rootCmd.AddCommand(onboardCmd)
}

func runOnboard(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmdStdout, output.Bold("Welcome to Solo!"))
	fmt.Fprintln(cmdStdout)

	home := client.SoloHome()
	if onboardHome != "" {
		home = onboardHome
	}
	fmt.Fprintf(cmdStdout, "Solo home: %s\n", home)

	// Pre-flight: ensure keypair is valid (regenerates legacy Ed25519 keys before daemon starts)
	if _, err := client.LoadOrCreateDaemonKeyPair(); err != nil {
		fmt.Fprintf(cmdStdout, output.Yellow("Warning: could not verify daemon keypair: %v\n"), err)
	}

	// Step 1: Start daemon if not running
	running, pid, _ := client.IsDaemonRunning()
	if running {
		fmt.Fprintf(cmdStdout, "Daemon already running (PID %d)\n", pid)
	} else {
		fmt.Fprintln(cmdStdout, "Starting daemon...")
		if err := startDaemonForOnboard(); err != nil {
			return err
		}
	}

	// Step 2: Wait for daemon ready
	timeout := time.Duration(onboardTimeout) * time.Second
	host := resolveOnboardHost()
	fmt.Fprintln(cmdStdout, "Waiting for daemon to become ready...")
	if err := waitForDaemon(host, timeout); err != nil {
		return &output.CommandError{
			Code:    "DAEMON_START_TIMEOUT",
			Message: fmt.Sprintf("Timed out after %ds waiting for daemon readiness", onboardTimeout),
		}
	}
	fmt.Fprintf(cmdStdout, "Daemon ready on %s\n", host)

	// Step 3: Generate pairing offer
	relayDisabled := onboardNoRelay
	if !relayDisabled {
		if cfg := client.LoadDaemonConfig(home); cfg != nil && cfg.RelayEnabled != nil && !*cfg.RelayEnabled {
			relayDisabled = true
		}
	}

	if relayDisabled {
		fmt.Fprintln(cmdStdout, output.Yellow("Relay is disabled; pairing is unavailable."))
		printNextSteps(home, "")
		return nil
	}

	pairingURL, err := generatePairingURL(home)
	if err != nil {
		fmt.Fprintln(cmdStdout, output.Yellow("Pairing offer unavailable."))
		printNextSteps(home, "")
		return nil
	}

	// Render QR code
	fmt.Fprintln(cmdStdout)
	fmt.Fprintln(cmdStdout, output.Bold("Scan to pair:"))
	qr, err := qrcode.New(pairingURL, qrcode.Medium)
	if err == nil {
		fmt.Fprintln(cmdStdout, qr.ToSmallString(false))
	}

	fmt.Fprintln(cmdStdout, output.Bold("Pairing link:"))
	fmt.Fprintln(cmdStdout, pairingURL)

	printNextSteps(home, pairingURL)
	return nil
}

func startDaemonForOnboard() error {
	daemonBin, err := findDaemonBinary()
	if err != nil {
		return &output.CommandError{Code: "DAEMON_NOT_FOUND", Message: err.Error()}
	}

	execArgs := []string{}
	if onboardPort != "" {
		execArgs = append(execArgs, "--port", onboardPort)
	}
	if onboardHome != "" {
		execArgs = append(execArgs, "--home", onboardHome)
	}
	if onboardNoRelay {
		execArgs = append(execArgs, "--no-relay")
	}
	if onboardNoMCP {
		execArgs = append(execArgs, "--no-mcp")
	}

	pid, err := execDaemon(daemonBin, execArgs)
	if err != nil {
		return &output.CommandError{Code: "DAEMON_START_FAILED", Message: fmt.Sprintf("Failed to start daemon: %v", err)}
	}
	fmt.Fprintf(cmdStdout, "Daemon started (PID %d)\n", pid)
	return nil
}

func resolveOnboardHost() string {
	if onboardPort != "" {
		return "127.0.0.1:" + onboardPort
	}
	wsURL, _ := client.ResolveHost("")
	host := wsURL
	if len(host) > 5 && host[:5] == "ws://" {
		host = host[5:]
	}
	if len(host) > 3 && host[len(host)-3:] == "/ws" {
		host = host[:len(host)-3]
	}
	return host
}

func generatePairingURL(home string) (string, error) {
	serverID, err := client.ReadServerID()
	if err != nil {
		return "", err
	}

	relayEndpoint := "relay.solo.sh:443"
	appBaseURL := "https://solo.up2ai.top"

	if cfg := client.LoadDaemonConfig(home); cfg != nil {
		if cfg.RelayPublicEndpoint != "" {
			relayEndpoint = cfg.RelayPublicEndpoint
		} else if cfg.RelayEndpoint != "" {
			relayEndpoint = cfg.RelayEndpoint
		}
		if cfg.AppBaseURL != "" {
			appBaseURL = cfg.AppBaseURL
		}
	}

	return client.GeneratePairingOffer(serverID, relayEndpoint, appBaseURL)
}

func printNextSteps(home, pairingURL string) {
	fmt.Fprintln(cmdStdout)
	fmt.Fprintln(cmdStdout, output.Bold("Next steps:"))
	if pairingURL != "" {
		fmt.Fprintln(cmdStdout, "  1. Open Solo and scan the QR code above, or paste the pairing link.")
	} else {
		fmt.Fprintln(cmdStdout, "  1. Open Solo and connect to your daemon.")
	}
	fmt.Fprintln(cmdStdout, "  2. Example: solo run \"your prompt\"")
	fmt.Fprintln(cmdStdout, "  3. Docs: https://solo.sh/docs")

	fmt.Fprintln(cmdStdout)
	fmt.Fprintln(cmdStdout, output.Bold("CLI quick reference:"))
	fmt.Fprintln(cmdStdout, "  1. solo --help")
	fmt.Fprintln(cmdStdout, "  2. solo ls")
	fmt.Fprintln(cmdStdout, "  3. solo run \"your prompt\"")
	fmt.Fprintln(cmdStdout, "  4. solo status")
	fmt.Fprintf(cmdStdout, "  5. Daemon logs: %s/daemon.log\n", home)

	fmt.Fprintln(cmdStdout)
	fmt.Fprintln(cmdStdout, output.Green("Solo is ready!"))
}
