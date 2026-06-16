package output

import (
	"fmt"
	"io"
)

// Render renders a CommandResult to w based on OutputOptions.
func Render(w io.Writer, result *CommandResult, opts OutputOptions) error {
	if result == nil {
		return nil
	}

	var err error
	switch opts.Format {
	case FormatJSON:
		err = renderJSON(w, result, opts)
	case FormatYAML:
		err = renderYAML(w, result, opts)
	case FormatQuiet:
		err = renderQuiet(w, result, opts)
	default: // table
		err = renderTable(w, result, opts)
	}
	return err
}

// RenderError renders a CommandError to w based on OutputOptions.
func RenderError(w io.Writer, ce *CommandError, opts OutputOptions) {
	switch opts.Format {
	case FormatJSON:
		renderJSONError(w, ce)
	case FormatYAML:
		renderYAMLError(w, ce)
	default:
		msg := ErrorText(ce.Message)
		if ce.Details != "" {
			msg += "\n" + ce.Details
		}
		_, _ = fmt.Fprintln(w, msg)
	}
}

// PrintResult is a convenience function that renders a result and handles errors.
// It returns the appropriate exit code: 0 for success, 1 for error.
func PrintResult(w io.Writer, result *CommandResult, opts OutputOptions) int {
	if result == nil {
		return 0
	}
	if err := Render(w, result, opts); err != nil {
		RenderError(w, &CommandError{
			Code:    "RENDER_FAILED",
			Message: err.Error(),
		}, opts)
		return 1
	}
	return 0
}
