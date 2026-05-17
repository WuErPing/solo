package cmd

import (
	"fmt"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
)

var daemonPairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Print the daemon pairing QR code and link",
	RunE:  runDaemonPair,
}

func init() {
	daemonCmd.AddCommand(daemonPairCmd)
}

func runDaemonPair(cmd *cobra.Command, args []string) error {
	// Read server ID
	serverID, err := client.ReadServerID()
	if err != nil {
		return &output.CommandError{
			Code:    "PAIR_FAILED",
			Message: "Cannot read server ID; is the daemon running?",
			Details: err.Error(),
		}
	}

	// Resolve relay endpoint
	relayEndpoint := "relay.solo.sh:443"
	appBaseURL := "https://app.solo.sh"

	// Read config for relay settings
	home := client.SoloHome()
	if cfg := client.LoadDaemonConfig(home); cfg != nil {
		if cfg.RelayPublicEndpoint != "" {
			relayEndpoint = cfg.RelayPublicEndpoint
		} else if cfg.RelayEndpoint != "" {
			relayEndpoint = cfg.RelayEndpoint
		}
		if cfg.RelayEnabled != nil && !*cfg.RelayEnabled {
			return &output.CommandError{
				Code:    "RELAY_DISABLED",
				Message: "Relay pairing is disabled for this daemon config.",
				Details: "Enable relay and run this command again.",
			}
		}
		if cfg.AppBaseURL != "" {
			appBaseURL = cfg.AppBaseURL
		}
	}

	// Generate pairing offer URL
	url, err := client.GeneratePairingOffer(serverID, relayEndpoint, appBaseURL)
	if err != nil {
		return &output.CommandError{
			Code:    "PAIR_FAILED",
			Message: "Failed to generate pairing offer",
			Details: err.Error(),
		}
	}

	opts := getOutputOpts()
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(output.SingleResult(map[string]string{
			"relayEnabled": "true",
			"url":          url,
		}, nil), opts)
	}

	// Render QR code to terminal
	qr, err := qrcode.New(url, qrcode.Medium)
	if err == nil {
		fmt.Fprintln(output.Stdout, "\nScan to pair:")
		fmt.Fprintln(output.Stdout, qr.ToSmallString(false))
	} else {
		fmt.Fprintln(output.Stdout, "\nScan to pair:")
	}

	fmt.Fprintln(output.Stdout, url)
	return nil
}
