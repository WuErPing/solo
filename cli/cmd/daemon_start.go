package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/client"
	"github.com/WuErPing/solo/cli/internal/output"
)

var (
	daemonStartForeground bool
	daemonStartPort       string
	daemonStartHome       string
	daemonStartNoRelay    bool
	daemonStartNoMCP      bool
)

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Solo daemon",
	RunE:  runDaemonStart,
}

func init() {
	daemonStartCmd.Flags().BoolVarP(&daemonStartForeground, "foreground", "f", false, "Run in foreground")
	daemonStartCmd.Flags().StringVar(&daemonStartPort, "port", "", "Listen port")
	daemonStartCmd.Flags().StringVar(&daemonStartHome, "home", "", "Solo home directory")
	daemonStartCmd.Flags().BoolVar(&daemonStartNoRelay, "no-relay", false, "Disable relay")
	daemonStartCmd.Flags().BoolVar(&daemonStartNoMCP, "no-mcp", false, "Disable MCP")
	daemonCmd.AddCommand(daemonStartCmd)
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	// Check if daemon is already running
	running, pid, _ := client.IsDaemonRunning()
	if running {
		return &output.CommandError{
			Code:    "DAEMON_ALREADY_RUNNING",
			Message: fmt.Sprintf("Daemon is already running (PID %d)", pid),
		}
	}

	// Find daemon binary
	daemonBin, err := findDaemonBinary()
	if err != nil {
		return &output.CommandError{Code: "DAEMON_NOT_FOUND", Message: err.Error()}
	}

	// Build command
	execArgs := []string{}
	if daemonStartPort != "" {
		execArgs = append(execArgs, "--port", daemonStartPort)
	}
	if daemonStartHome != "" {
		execArgs = append(execArgs, "--home", daemonStartHome)
	}
	if daemonStartNoRelay {
		execArgs = append(execArgs, "--no-relay")
	}
	if daemonStartNoMCP {
		execArgs = append(execArgs, "--no-mcp")
	}

	if daemonStartForeground {
		fgCmd := exec.Command(daemonBin, execArgs...)
		fgCmd.Stdout = os.Stdout
		fgCmd.Stderr = os.Stderr
		fmt.Fprintln(output.Stdout, "Starting daemon in foreground...")
		return fgCmd.Run()
	}

	pid, err = execDaemon(daemonBin, execArgs)
	if err != nil {
		return &output.CommandError{Code: "DAEMON_START_FAILED", Message: fmt.Sprintf("Failed to start daemon: %v", err)}
	}

	fmt.Fprintf(output.Stdout, "Daemon starting (PID %d)...\n", pid)

	// Wait for daemon to become healthy
	host := resolveDaemonHost()
	if err := waitForDaemon(host, 10*time.Second); err != nil {
		return &output.CommandError{Code: "DAEMON_START_TIMEOUT", Message: "Daemon did not become healthy in time"}
	}

	fmt.Fprintf(output.Stdout, "Daemon is running at %s\n", host)
	return nil
}

// execDaemon starts the daemon binary as a detached background process.
func execDaemon(bin string, args []string) (int, error) {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func findDaemonBinary() (string, error) {
	// Check same directory as CLI binary
	cliBin, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(cliBin)
		candidate := filepath.Join(dir, "solo-daemon")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		// Also try just "solo" in the same directory
		candidate = filepath.Join(dir, "solo")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Check PATH
	path, err := exec.LookPath("solo-daemon")
	if err == nil {
		return path, nil
	}
	path, err = exec.LookPath("solo")
	if err == nil {
		return path, nil
	}

	return "", fmt.Errorf("cannot find daemon binary; ensure 'solo' or 'solo-daemon' is on your PATH")
}

func resolveDaemonHost() string {
	if daemonStartPort != "" {
		return "127.0.0.1:" + daemonStartPort
	}
	wsURL, _ := client.ResolveHost("")
	// Strip ws:// prefix and /ws suffix for display
	host := wsURL
	if len(host) > 5 && host[:5] == "ws://" {
		host = host[5:]
	}
	if len(host) > 3 && host[len(host)-3:] == "/ws" {
		host = host[:len(host)-3]
	}
	return host
}

func waitForDaemon(host string, timeout time.Duration) error {
	url := fmt.Sprintf("http://%s/api/health", host)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout")
}
