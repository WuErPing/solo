package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// capturePaneFunc is overridable in tests to avoid real tmux invocations.
// Returns content, pane column width, error.
var capturePaneFunc = captureTmuxPane

// formatUnixHHMM converts a Unix timestamp (seconds) to "HH:MM" in the daemon's local timezone.
func formatUnixHHMM(unixSec int64) string {
	if unixSec == 0 {
		return ""
	}
	t := time.Unix(unixSec, 0)
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

// formatWindowActivity converts a Unix timestamp (seconds) to a relative time string.
func formatWindowActivity(unixSec int64) string {
	if unixSec == 0 {
		return ""
	}
	diff := time.Now().Unix() - unixSec
	if diff < 60 {
		return "just now"
	}
	mins := diff / 60
	if mins < 60 {
		return fmt.Sprintf("%dm ago", mins)
	}
	hours := mins / 60
	if hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%dd ago", days)
}

// computeContentHash returns a 16-char hex SHA-256 prefix for content dedup.
func computeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8])
}

func captureTmuxPane(paneID string, startLine int, cols int) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	// Use -J to rejoin lines that tmux wrapped because of the host pane width.
	// This lets the server's width-aware rewrapper produce clean breaks at the
	// client's column count, instead of inheriting hard line breaks from a pane
	// that is almost certainly a different width.
	out, err := exec.CommandContext(ctx, "tmux", "capture-pane", "-t", paneID, "-p", "-e", "-J", "-S", strconv.Itoa(startLine)).Output()
	if err != nil {
		return "", 0, err
	}
	content := string(out)
	if cols > 0 {
		content = wrapContentToCols(content, cols)
	}
	paneCols := getTmuxPaneCols(paneID)
	return content, paneCols, nil
}

func getTmuxPaneCols(paneID string) int {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", paneID, "#{pane_width}").Output()
	if err != nil {
		return 0
	}
	w, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return w
}

const tabWidth = 8

// wideRanges lists Unicode ranges approximated as double-width for terminal
// wrapping (CJK, Hangul, fullwidth forms, box drawing, common emoji). Sorted
// ascending by start codepoint and non-overlapping, so runeDisplayWidth can
// stop scanning once a rune falls below a range's start.
var wideRanges = [][2]rune{
	{0x1100, 0x115f},
	{0x2329, 0x232a},
	{0x2e80, 0xa4cf},
	{0xac00, 0xd7a3},
	{0xf900, 0xfaff},
	{0xfe30, 0xfe6f},
	{0xff00, 0xff60},
	{0xffe0, 0xffe6},
	{0x1f300, 0x1f64f},
	{0x1f900, 0x1f9ff},
}

// runeDisplayWidth returns the approximate terminal column count for a rune:
// 2 for wide characters (see wideRanges), 1 otherwise. Tab is context-dependent
// and handled by the caller, not here.
func runeDisplayWidth(r rune) int {
	for _, rng := range wideRanges {
		if r < rng[0] {
			return 1
		}
		if r <= rng[1] {
			return 2
		}
	}
	return 1
}

// scanCSISequence scans a CSI escape sequence (\x1b[...<final byte>) starting at
// line[i] and returns the full sequence plus the index just past it.
func scanCSISequence(line string, i int) (string, int) {
	end := i + 2
	for end < len(line) {
		c := line[end]
		end++
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '~' {
			break
		}
	}
	return line[i:end], end
}

// sgrState tracks the active SGR (color/style) parameters so they can be closed
// at a wrap point and reopened on the continuation line.
type sgrState struct{ params string }

// updateFromSeq folds a CSI sequence into the tracked state. Only SGR sequences
// (\x1b[...m) matter; a reset ("" or "0") clears the state, other params append.
func (s *sgrState) updateFromSeq(seq string) {
	if len(seq) < 3 || seq[len(seq)-1] != 'm' {
		return
	}
	params := seq[2 : len(seq)-1] // strip "\x1b[" and "m"
	if params == "" || params == "0" {
		s.params = ""
		return
	}
	if s.params != "" {
		s.params += ";"
	}
	s.params += params
}

func (s *sgrState) active() bool      { return s.params != "" }
func (s *sgrState) reopenSeq() string { return "\x1b[" + s.params + "m" }

// wrapContentToCols rewraps a terminal capture to a target column count.
// It is ANSI-aware: CSI SGR escape sequences are preserved and carried across
// wrapped lines so that colors/styles stay correct. Wide characters (CJK,
// box drawing) are approximated via runeDisplayWidth, which is sufficient for
// preventing xterm.js from wrapping already-wrapped ASCII box-drawing tables.
func wrapContentToCols(content string, cols int) string {
	if cols <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = wrapLineToCols(line, cols)
	}
	return strings.Join(lines, "\n")
}

// wrapLineToCols rewraps a single line (no embedded newlines) to cols columns,
// closing and reopening the active SGR style at each wrap point.
func wrapLineToCols(line string, cols int) string {
	if len(line) == 0 {
		return ""
	}

	var out, cur strings.Builder
	var sgr sgrState
	visible := 0

	for i := 0; i < len(line); {
		if line[i] == '\x1b' && i+1 < len(line) && line[i+1] == '[' {
			seq, end := scanCSISequence(line, i)
			sgr.updateFromSeq(seq)
			cur.WriteString(seq)
			i = end
			continue
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		width := runeDisplayWidth(r)
		if r == '\t' {
			width = tabWidth - (visible % tabWidth)
		}

		if visible+width > cols && visible > 0 {
			// Wrap before this character. Close active styles on the current
			// line and reopen them on the next line.
			if sgr.active() {
				cur.WriteString("\x1b[0m")
			}
			out.WriteString(cur.String())
			out.WriteByte('\n')
			cur.Reset()
			visible = 0
			if sgr.active() {
				cur.WriteString(sgr.reopenSeq())
			}
		}

		cur.WriteRune(r)
		visible += width
		i += size
	}

	out.WriteString(cur.String())
	return out.String()
}
