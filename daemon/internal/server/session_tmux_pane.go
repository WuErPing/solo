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

// wrapContentToCols rewraps a terminal capture to a target column count.
// It is ANSI-aware: CSI SGR escape sequences are preserved and carried across
// wrapped lines so that colors/styles stay correct. Wide characters (CJK,
// box drawing) are approximated as single columns, which is sufficient for
// preventing xterm.js from wrapping already-wrapped ASCII box-drawing tables.
func wrapContentToCols(content string, cols int) string {
	if cols <= 0 {
		return content
	}

	var result strings.Builder
	lines := strings.Split(content, "\n")
	for lineIdx, line := range lines {
		if lineIdx > 0 {
			result.WriteByte('\n')
		}
		if len(line) == 0 {
			continue
		}

		var lineOut strings.Builder
		visible := 0
		var activeSGR strings.Builder

		for i := 0; i < len(line); {
			// CSI escape sequence: \x1b[...<final byte>
			if line[i] == '\x1b' && i+1 < len(line) && line[i+1] == '[' {
				end := i + 2
				for end < len(line) {
					c := line[end]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '~' {
						end++
						break
					}
					end++
				}
				seq := line[i:end]
				if len(seq) >= 3 && seq[len(seq)-1] == 'm' {
					paramStart := 2 // skip "\x1b["
					paramEnd := len(seq) - 1
					params := seq[paramStart:paramEnd]
					if params == "" || params == "0" {
						activeSGR.Reset()
					} else {
						if activeSGR.Len() > 0 {
							activeSGR.WriteByte(';')
						}
						activeSGR.WriteString(params)
					}
				}
				lineOut.WriteString(seq)
				i = end
				continue
			}

			// Plain character.
			r, size := utf8.DecodeRuneInString(line[i:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8: treat as a single replacement column.
				size = 1
			}

			var width int
			switch {
			case r == '\t':
				width = tabWidth - (visible % tabWidth)
			case r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe6f) || (r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) || (r >= 0x1f300 && r <= 0x1f64f) || (r >= 0x1f900 && r <= 0x1f9ff)):
				width = 2
			default:
				width = 1
			}

			if visible+width > cols && visible > 0 {
				// Wrap before this character. Close active styles on the
				// current line and reopen them on the next line.
				if activeSGR.Len() > 0 {
					lineOut.WriteString("\x1b[0m")
				}
				result.WriteString(lineOut.String())
				result.WriteByte('\n')
				lineOut.Reset()
				visible = 0
				if activeSGR.Len() > 0 {
					lineOut.WriteString("\x1b[")
					lineOut.WriteString(activeSGR.String())
					lineOut.WriteByte('m')
				}
			}

			lineOut.WriteRune(r)
			visible += width
			i += size
		}

		result.WriteString(lineOut.String())
	}

	return result.String()
}
