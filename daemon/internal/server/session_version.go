package server

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/WuErPing/solo/protocol"
)

// Daemon version switching: the host keeps runnable daemon builds in
// $SoloHome/versions/ (filename = version label) and a pointer file
// $SoloHome/versions/current naming the build the supervisor should spawn.
// Switching writes the pointer and exits with ExitCodeRestartRequested; the
// supervisor resolves the pointer on the next spawn.

// versionsDirName and versionPointerName define the on-disk convention shared
// with the supervisor (which reads the pointer; keep the names in sync with
// supervisor/internal/supervisor).
const (
	versionsDirName     = "versions"
	versionPointerName  = "current"
	versionBinaryPrefix = "solo-"
)

// handleListDaemonVersions lists the runnable builds in the versions dir.
// A missing dir yields an empty list, not an error.
func (s *Session) handleListDaemonVersions(m *protocol.ListDaemonVersionsRequest) {
	versions, current := scanDaemonVersions(s.cfg.SoloHome)
	s.sendMessage(protocol.NewSessionMessage(&protocol.ListDaemonVersionsResponse{
		Type: "list_daemon_versions_response",
		Payload: protocol.ListDaemonVersionsPayload{
			RequestID:      m.RequestID,
			RunningVersion: s.cfg.Version,
			CurrentVersion: current,
			Versions:       versions,
		},
	}))
}

// handleSwitchDaemonVersion points the supervisor at a specific (or the
// latest) build and restarts the daemon onto it. Requires supervision — an
// unsupervised daemon would not be respawned onto the new binary.
func (s *Session) handleSwitchDaemonVersion(m *protocol.SwitchDaemonVersionRequest) {
	if !s.cfg.Supervised {
		s.logger.Info("version switch refused: daemon is not supervised", "requestId", m.RequestID)
		s.sendRPCError(m.RequestID, m.MsgType(),
			"daemon is not supervised; version switching requires solo-supervisor",
			strPtr("NOT_SUPERVISED"))
		return
	}

	versions, current := scanDaemonVersions(s.cfg.SoloHome)
	if len(versions) == 0 {
		s.sendRPCError(m.RequestID, m.MsgType(),
			"no daemon versions found in "+filepath.Join(s.cfg.SoloHome, versionsDirName),
			strPtr("NO_VERSIONS"))
		return
	}

	target := versions[0].Version // newest by mtime
	if m.Version != nil && *m.Version != "" && *m.Version != "latest" {
		target = *m.Version
		found := false
		for _, v := range versions {
			if v.Version == target {
				found = true
				break
			}
		}
		if !found {
			s.sendRPCError(m.RequestID, m.MsgType(),
				"daemon version not found: "+target,
				strPtr("VERSION_NOT_FOUND"))
			return
		}
	}

	// No-op when the pointer already targets this build and the running
	// daemon is that build (filename convention solo-<version>): reply
	// without restarting.
	if current != nil && *current == target && strings.TrimPrefix(target, versionBinaryPrefix) == s.cfg.Version {
		s.logger.Info("already running requested version, no switch needed",
			"requestId", m.RequestID, "version", target)
		s.sendDaemonVersionSwitchRequested(m, target)
		return
	}

	if err := writeVersionPointer(s.cfg.SoloHome, target); err != nil {
		s.sendRPCError(m.RequestID, m.MsgType(), "write version pointer: "+err.Error(), nil)
		return
	}
	s.logger.Info("daemon version switch requested",
		"requestId", m.RequestID, "version", target)
	s.sendDaemonVersionSwitchRequested(m, target)
	s.requestExitAfter(protocol.ExitCodeRestartRequested,
		"switch daemon version to "+target, serverControlFlushDelay)
}

func (s *Session) sendDaemonVersionSwitchRequested(m *protocol.SwitchDaemonVersionRequest, version string) {
	s.sendMessage(protocol.NewSessionMessage(&protocol.StatusMessage{
		Type: "status",
		Payload: protocol.DaemonVersionSwitchRequestedStatusPayload{
			Status:    "daemon_version_switch_requested",
			ClientID:  s.clientID,
			Version:   version,
			RequestID: m.RequestID,
		},
	}))
}

// scanDaemonVersions returns the runnable builds in $soloHome/versions
// (regular executable files named solo-*, newest mtime first) and the pointer
// file content (nil when unset).
func scanDaemonVersions(soloHome string) ([]protocol.DaemonVersionInfo, *string) {
	dir := filepath.Join(soloHome, versionsDirName)
	entries, err := os.ReadDir(dir)
	versions := []protocol.DaemonVersionInfo{}
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), versionBinaryPrefix) {
				continue
			}
			info, err := e.Info()
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
				continue
			}
			versions = append(versions, protocol.DaemonVersionInfo{
				Version: e.Name(),
				MtimeMs: info.ModTime().UnixMilli(),
			})
		}
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].MtimeMs > versions[j].MtimeMs
		})
	}

	var current *string
	if data, err := os.ReadFile(filepath.Join(dir, versionPointerName)); err == nil {
		if name := strings.TrimSpace(string(data)); name != "" {
			current = &name
		}
	}
	return versions, current
}

// writeVersionPointer atomically records the selected build name.
func writeVersionPointer(soloHome, name string) error {
	dir := filepath.Join(soloHome, versionsDirName)
	tmp, err := os.CreateTemp(dir, ".current-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(name + "\n"); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, filepath.Join(dir, versionPointerName))
}
