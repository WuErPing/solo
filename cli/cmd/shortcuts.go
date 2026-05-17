package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	// Top-level shortcuts that delegate to agent subcommands.
	// These register the same flag variables as the agent subcommands,
	// so flag values are shared between `solo run` and `solo agent run`.
	rootCmd.AddCommand(
		makeAgentShortcut("ls", "List agents", agentLsCmd),
		makeAgentShortcut("run", "Create and run an agent", agentRunCmd),
		makeAgentShortcut("attach", "Attach to an agent's output stream", agentAttachCmd),
		makeAgentShortcut("logs", "View agent logs", agentLogsCmd),
		makeAgentShortcut("stop", "Stop a running agent", agentStopCmd),
		makeAgentShortcut("delete", "Delete an agent", agentDeleteCmd),
		makeAgentShortcut("send", "Send a message to an agent", agentSendCmd),
		makeAgentShortcut("wait", "Wait for an agent to finish", agentWaitCmd),
		makeAgentShortcut("inspect", "Show agent details", agentInspectCmd),
		makeAgentShortcut("archive", "Archive an agent", agentArchiveCmd),
	)

	// Top-level daemon shortcuts
	rootCmd.AddCommand(
		makeDaemonShortcut("status", "Show daemon status", daemonStatusCmd),
		makeDaemonShortcut("restart", "Restart daemon", daemonRestartCmd),
	)
}

// makeAgentShortcut creates a top-level command that delegates to an agent subcommand.
// It registers the same flag pointer variables so flags like --provider are shared.
func makeAgentShortcut(name, short string, source *cobra.Command) *cobra.Command {
	shortcut := &cobra.Command{
		Use:   source.Use,
		Short: short,
		Args:  source.Args,
		RunE:  source.RunE,
	}

	// Register the same flag variables that the source command uses.
	// This ensures `solo run --provider X` sets the same variable as `solo agent run --provider X`.
	switch name {
	case "ls":
		shortcut.Flags().BoolVarP(&agentLsAll, "all", "a", false, "Include archived agents")
		shortcut.Flags().StringVar(&agentLsStatus, "status", "", "Filter by status")
		shortcut.Flags().StringVar(&agentLsCwd, "cwd", "", "Filter by working directory")
		shortcut.Flags().StringArrayVar(&agentLsLabel, "label", nil, "Filter by label (key=value)")
		shortcut.Flags().StringVar(&agentLsThinking, "thinking", "", "Filter by thinking option ID")
	case "run":
		shortcut.Flags().BoolVarP(&agentRunDetach, "detach", "d", false, "Run in background (detached)")
		shortcut.Flags().StringVar(&agentRunTitle, "title", "", "Assign a title to the agent")
		shortcut.Flags().StringVar(&agentRunProvider, "provider", "", "Agent provider (e.g. claude, mock)")
		shortcut.Flags().StringVar(&agentRunModel, "model", "", "Model to use")
		shortcut.Flags().StringVar(&agentRunMode, "mode", "", "Provider-specific mode")
		shortcut.Flags().StringVar(&agentRunCwd, "cwd", "", "Working directory (default: current)")
		shortcut.Flags().StringArrayVar(&agentRunLabel, "label", nil, "Add label(s) (key=value)")
		shortcut.Flags().StringVar(&agentRunTimeout, "wait-timeout", "", "Max wait time (e.g. 30s, 5m)")
	case "attach":
		// No additional flags beyond args
	case "logs":
		shortcut.Flags().BoolVarP(&agentLogsFollow, "follow", "f", false, "Follow log output (streaming)")
		shortcut.Flags().IntVar(&agentLogsTail, "tail", 0, "Show last N entries")
		shortcut.Flags().StringVar(&agentLogsFilter, "filter", "", "Filter by event type")
	case "stop":
		shortcut.Flags().BoolVar(&agentStopAll, "all", false, "Stop all running agents")
		shortcut.Flags().StringVar(&agentStopCwd, "cwd", "", "Stop agents in this directory")
	case "delete":
		shortcut.Flags().BoolVar(&agentDeleteAll, "all", false, "Delete all agents")
		shortcut.Flags().StringVar(&agentDeleteCwd, "cwd", "", "Delete agents in this directory")
	case "send":
		shortcut.Flags().BoolVar(&agentSendNoWait, "no-wait", false, "Don't wait for response")
		shortcut.Flags().StringArrayVar(&agentSendImage, "image", nil, "Attach image(s)")
	case "wait":
		shortcut.Flags().StringVar(&agentWaitTimeout, "timeout", "", "Max wait time (e.g. 30s, 5m)")
	case "inspect":
		// No additional flags
	case "archive":
		shortcut.Flags().BoolVar(&agentArchiveForce, "force", false, "Archive even if running")
	}

	return shortcut
}

func makeDaemonShortcut(name, short string, source *cobra.Command) *cobra.Command {
	shortcut := &cobra.Command{
		Use:   source.Use,
		Short: short,
		Args:  source.Args,
		RunE:  source.RunE,
	}

	switch name {
	case "status":
		// No additional flags
	case "restart":
		shortcut.Flags().StringVar(&daemonRestartTimeout, "timeout", "15", "Wait timeout in seconds")
		shortcut.Flags().BoolVar(&daemonRestartForce, "force", false, "Force kill if graceful stop times out")
		shortcut.Flags().StringVar(&daemonRestartPort, "port", "", "Port for restarted daemon")
		shortcut.Flags().BoolVar(&daemonRestartNoRelay, "no-relay", false, "Disable relay on restarted daemon")
		shortcut.Flags().BoolVar(&daemonRestartNoMCP, "no-mcp", false, "Disable MCP on restarted daemon")
	}

	return shortcut
}
