package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

// renderTable writes the command result as an aligned ASCII table.
func renderTable(w io.Writer, result *CommandResult, opts OutputOptions) error {
	if result.Schema == nil || len(result.Schema.Columns) == 0 {
		return nil
	}

	columns := result.Schema.Columns

	// Collect data rows
	var items []interface{}
	if result.IsSingle {
		items = []interface{}{result.Single}
	} else {
		items = result.List
	}

	if len(items) == 0 {
		return nil
	}

	// Calculate column widths
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col.Header)
		if col.Width > widths[i] {
			widths[i] = col.Width
		}
	}

	// Measure data widths
	for _, item := range items {
		for i, col := range columns {
			val := col.FieldFunc(item)
			visible := stripAnsi(val)
			if len(visible) > widths[i] {
				widths[i] = len(visible)
			}
		}
	}

	// Render header
	if !opts.NoHeaders {
		headers := make([]string, len(columns))
		for i, col := range columns {
			headers[i] = padCell(col.Header, widths[i], col.Align)
		}
		headerLine := strings.Join(headers, "  ")
		if !opts.NoColor {
			headerLine = HeaderColor.Sprint(headerLine)
		}
		if _, err := fmt.Fprintln(w, headerLine); err != nil {
			return err
		}
	}

	// Render data rows
	for _, item := range items {
		cells := make([]string, len(columns))
		for i, col := range columns {
			val := col.FieldFunc(item)
			// Apply color if specified
			if col.ColorFunc != nil && !opts.NoColor {
				if colorName := col.ColorFunc(val, item); colorName != "" {
					val = Colorize(colorName, stripAnsi(val))
				}
			}
			cells[i] = padCell(val, widths[i], col.Align)
		}
		if _, err := fmt.Fprintln(w, strings.Join(cells, "  ")); err != nil {
			return err
		}
	}

	return nil
}

// padCell pads a cell value to the given width with the specified alignment.
func padCell(val string, width int, align string) string {
	visible := stripAnsi(val)
	padding := width - len(visible)
	if padding <= 0 {
		return val
	}

	switch align {
	case "right":
		return strings.Repeat(" ", padding) + val
	case "center":
		left := padding / 2
		right := padding - left
		return strings.Repeat(" ", left) + val + strings.Repeat(" ", right)
	default: // "left"
		return val + strings.Repeat(" ", padding)
	}
}

// stripAnsi removes ANSI escape codes from a string.
func stripAnsi(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// Disable color package globally when no-color is requested.
func init() {
	// Respect NO_COLOR environment variable
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		color.NoColor = true
	}
}
