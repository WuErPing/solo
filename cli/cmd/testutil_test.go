package cmd

import (
	"bytes"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fatih/color"

	"github.com/WuErPing/solo/cli/internal/clienttest"
)

// resetFlags restores all package-level CLI flags to their default values at
// the start of a test and guarantees they are restored to their pre-test values
// on cleanup. This eliminates order-dependent failures caused by tests mutating
// global flag variables.
func resetFlags(t *testing.T) {
	t.Helper()

	old := map[string]interface{}{
		"flagHost":                flagHost,
		"flagFormat":              flagFormat,
		"flagJSON":                flagJSON,
		"flagQuiet":               flagQuiet,
		"flagNoHeaders":           flagNoHeaders,
		"flagNoColor":             flagNoColor,
		"agentArchiveForce":       agentArchiveForce,
		"agentDeleteAll":          agentDeleteAll,
		"agentDeleteCwd":          agentDeleteCwd,
		"agentStopAll":            agentStopAll,
		"agentStopCwd":            agentStopCwd,
		"agentSendNoWait":         agentSendNoWait,
		"agentSendImage":          agentSendImage,
		"agentLsAll":              agentLsAll,
		"agentLsStatus":           agentLsStatus,
		"agentLsCwd":              agentLsCwd,
		"agentLsLabel":            agentLsLabel,
		"agentLsThinking":         agentLsThinking,
		"agentLogsFollow":         agentLogsFollow,
		"agentLogsTail":           agentLogsTail,
		"agentLogsFilter":         agentLogsFilter,
		"agentModeList":           agentModeList,
		"agentWaitTimeout":        agentWaitTimeout,
		"agentRunDetach":          agentRunDetach,
		"agentRunProvider":        agentRunProvider,
		"agentRunModel":           agentRunModel,
		"agentRunMode":            agentRunMode,
		"agentRunTitle":           agentRunTitle,
		"agentRunCwd":             agentRunCwd,
		"agentRunLabel":           agentRunLabel,
		"agentRunTimeout":         agentRunTimeout,
		"daemonStartPort":         daemonStartPort,
		"daemonStopTimeout":       daemonStopTimeout,
		"daemonStopForce":         daemonStopForce,
		"daemonRestartTimeout":    daemonRestartTimeout,
		"daemonRestartPort":       daemonRestartPort,
		"daemonRestartNoRelay":    daemonRestartNoRelay,
		"daemonRestartNoMCP":      daemonRestartNoMCP,
		"onboardPort":             onboardPort,
		"onboardHome":             onboardHome,
		"onboardNoRelay":          onboardNoRelay,
		"onboardNoMCP":            onboardNoMCP,
		"onboardTimeout":          onboardTimeout,
		"colorNoColor":            color.NoColor,
	}

	flagHost = ""
	flagFormat = "table"
	flagJSON = false
	flagQuiet = false
	flagNoHeaders = false
	flagNoColor = false

	agentArchiveForce = false
	agentDeleteAll = false
	agentDeleteCwd = ""
	agentStopAll = false
	agentStopCwd = ""
	agentSendNoWait = false
	agentSendImage = nil
	agentLsAll = false
	agentLsStatus = ""
	agentLsCwd = ""
	agentLsLabel = nil
	agentLsThinking = ""
	agentLogsFollow = false
	agentLogsTail = 0
	agentLogsFilter = ""
	agentModeList = false
	agentWaitTimeout = ""
	agentRunDetach = false
	agentRunProvider = ""
	agentRunModel = ""
	agentRunMode = ""
	agentRunTitle = ""
	agentRunCwd = ""
	agentRunLabel = nil
	agentRunTimeout = ""

	daemonStartPort = ""
	daemonStopTimeout = "15"
	daemonStopForce = false
	daemonRestartTimeout = "15"
	daemonRestartPort = ""
	daemonRestartNoRelay = false
	daemonRestartNoMCP = false

	onboardPort = ""
	onboardHome = ""
	onboardNoRelay = false
	onboardNoMCP = false
	onboardTimeout = 600

	color.NoColor = false

	t.Cleanup(func() {
		flagHost = old["flagHost"].(string)
		flagFormat = old["flagFormat"].(string)
		flagJSON = old["flagJSON"].(bool)
		flagQuiet = old["flagQuiet"].(bool)
		flagNoHeaders = old["flagNoHeaders"].(bool)
		flagNoColor = old["flagNoColor"].(bool)

		agentArchiveForce = old["agentArchiveForce"].(bool)
		agentDeleteAll = old["agentDeleteAll"].(bool)
		agentDeleteCwd = old["agentDeleteCwd"].(string)
		agentStopAll = old["agentStopAll"].(bool)
		agentStopCwd = old["agentStopCwd"].(string)
		agentSendNoWait = old["agentSendNoWait"].(bool)
		agentSendImage = old["agentSendImage"].([]string)
		agentLsAll = old["agentLsAll"].(bool)
		agentLsStatus = old["agentLsStatus"].(string)
		agentLsCwd = old["agentLsCwd"].(string)
		agentLsLabel = old["agentLsLabel"].([]string)
		agentLsThinking = old["agentLsThinking"].(string)
		agentLogsFollow = old["agentLogsFollow"].(bool)
		agentLogsTail = old["agentLogsTail"].(int)
		agentLogsFilter = old["agentLogsFilter"].(string)
		agentModeList = old["agentModeList"].(bool)
		agentWaitTimeout = old["agentWaitTimeout"].(string)
		agentRunDetach = old["agentRunDetach"].(bool)
		agentRunProvider = old["agentRunProvider"].(string)
		agentRunModel = old["agentRunModel"].(string)
		agentRunMode = old["agentRunMode"].(string)
		agentRunTitle = old["agentRunTitle"].(string)
		agentRunCwd = old["agentRunCwd"].(string)
		agentRunLabel = old["agentRunLabel"].([]string)
		agentRunTimeout = old["agentRunTimeout"].(string)

		daemonStartPort = old["daemonStartPort"].(string)
		daemonStopTimeout = old["daemonStopTimeout"].(string)
		daemonStopForce = old["daemonStopForce"].(bool)
		daemonRestartTimeout = old["daemonRestartTimeout"].(string)
		daemonRestartPort = old["daemonRestartPort"].(string)
		daemonRestartNoRelay = old["daemonRestartNoRelay"].(bool)
		daemonRestartNoMCP = old["daemonRestartNoMCP"].(bool)

		onboardPort = old["onboardPort"].(string)
		onboardHome = old["onboardHome"].(string)
		onboardNoRelay = old["onboardNoRelay"].(bool)
		onboardNoMCP = old["onboardNoMCP"].(bool)
		onboardTimeout = old["onboardTimeout"].(int)

		color.NoColor = old["colorNoColor"].(bool)
	})
}

// setupTestCLI wires up a temporary SOLO_HOME, captures stdout/stderr, starts a
// shared mock daemon, and points the global --host flag at it. Tests that need
// to inspect or tweak the mock may use the returned *clienttest.MockDaemon.
func setupTestCLI(t *testing.T) (*clienttest.MockDaemon, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	resetFlags(t)

	home := t.TempDir()
	os.Setenv("SOLO_HOME", home)
	t.Cleanup(func() { os.Unsetenv("SOLO_HOME") })

	mock := clienttest.NewMockDaemon()
	srv := httptest.NewServer(mock)
	t.Cleanup(srv.Close)

	oldStdout := cmdStdout
	oldStderr := cmdStderr
	var outBuf, errBuf bytes.Buffer
	cmdStdout = &outBuf
	cmdStderr = &errBuf
	t.Cleanup(func() {
		cmdStdout = oldStdout
		cmdStderr = oldStderr
	})

	flagHost = srv.Listener.Addr().String()
	return mock, &outBuf, &errBuf
}

// setupEnhancedCLI is the historical entry point used by cmd_extra_test.go.
// It shares the same mock daemon as setupTestCLI but only returns the capture
// buffers to keep existing call sites unchanged.
func setupEnhancedCLI(t *testing.T) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	_, out, err := setupTestCLI(t)
	return out, err
}
